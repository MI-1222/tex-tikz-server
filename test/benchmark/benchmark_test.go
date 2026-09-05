// Package benchmark は TikZ レンダリングサーバーの負荷テストおよびベンチマークを提供する。
package benchmark

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"tex-tikz-server/api/gen"
	"tex-tikz-server/internal/auth"
	"tex-tikz-server/internal/cache"
	"tex-tikz-server/internal/handler"
	"tex-tikz-server/internal/tex"
)

const benchmarkAPIKey = "bench_secret_key_12345"

// BenchmarkComputeKey は SHA-256 キャッシュキー計算関数のスループットを測定する。
func BenchmarkComputeKey(b *testing.B) {
	code := "\\begin{tikzpicture}\n\\draw (0,0) circle (1cm);\n\\node at (0,0) {日本語};\n\\end{tikzpicture}"
	preamble := "\\usetikzlibrary{arrows.meta}"
	format := "svg"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cache.ComputeKey(code, preamble, format)
		}
	})
}

// BenchmarkLRUCache_Get は LRU キャッシュの Get 操作のスループットを測定する。
func BenchmarkLRUCache_Get(b *testing.B) {
	c := cache.New(2000)
	key := cache.ComputeKey("test_code", "", "svg")
	c.Set(key, "<svg>benchmark_cached_svg_data</svg>")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = c.Get(key)
		}
	})
}

// BenchmarkLRUCache_Set は LRU キャッシュの Set 操作 (並行書き込み) のスループットを測定する。
func BenchmarkLRUCache_Set(b *testing.B) {
	c := cache.New(2000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench_key_%d", i%500)
			c.Set(key, "<svg>data</svg>")
			i++
		}
	})
}

// BenchmarkHandler_CacheHit は HTTP ハンドラー経由でのキャッシュヒット時スループットを測定する。
func BenchmarkHandler_CacheHit(b *testing.B) {
	// モックエンジンを使用
	engine, err := tex.NewEngine(tex.EngineConfig{FontDir: "fonts"})
	if err != nil {
		b.Fatalf("Engine init error: %v", err)
	}

	lruCache := cache.New(2000)
	h := handler.NewHandler(engine, lruCache, 10*time.Second)
	strictHandler := gen.NewStrictHandler(h, nil)

	r := chi.NewRouter()
	r.Use(auth.NewMiddleware(benchmarkAPIKey))
	gen.HandlerFromMux(strictHandler, r)

	// 事前にキャッシュを登録
	code := "\\begin{tikzpicture}\\draw (0,0) circle (1cm);\\end{tikzpicture}"
	key := cache.ComputeKey(code, "", "svg")
	lruCache.Set(key, "<svg>mock-svg</svg>")

	reqBody := `{"code":"\\begin{tikzpicture}\\draw (0,0) circle (1cm);\\end{tikzpicture}"}`

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/render/tikz", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(auth.HeaderAPIKey, benchmarkAPIKey)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				b.Fatalf("非 200 ステータス: %d", rec.Code)
			}
		}
	})
}

// TestLoad_ConcurrentCompiles は複数 Goroutine からの同時コンパイル時のセマフォ制御と安定性を検証する。
func TestLoad_ConcurrentCompiles(t *testing.T) {
	// fonts ディレクトリパスの解決
	fontDir := "fonts"
	if _, err := os.Stat(fontDir); os.IsNotExist(err) {
		if _, err := os.Stat("../../fonts"); err == nil {
			fontDir = "../../fonts"
		}
	}

	// 同時実行数を 4 に制限したエンジン
	const maxConcurrency = 4
	engine, err := tex.NewEngine(tex.EngineConfig{
		FontDir:        fontDir,
		MaxConcurrency: maxConcurrency,
	})
	if err != nil {
		t.Fatalf("Engine init error: %v", err)
	}

	lruCache := cache.New(100)
	h := handler.NewHandler(engine, lruCache, 15*time.Second)
	strictHandler := gen.NewStrictHandler(h, nil)

	r := chi.NewRouter()
	r.Use(auth.NewMiddleware(benchmarkAPIKey))
	gen.HandlerFromMux(strictHandler, r)

	ts := httptest.NewServer(r)
	defer ts.Close()

	// 10 並行で異なる TikZ コードを同時にリクエスト
	const totalRequests = 10
	var wg sync.WaitGroup
	errCh := make(chan error, totalRequests)

	startTime := time.Now()

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// 各 Goroutine で一意な図形を描画 (キャッシュミスを発生させる)
			code := fmt.Sprintf("\\begin{tikzpicture}\\draw[thick] (0,0) -- (%d,%d);\\node at (%d,0) {並行テスト%d};\\end{tikzpicture}", idx+1, idx+1, idx+1, idx)
			reqPayload := fmt.Sprintf(`{"code":%q}`, code)

			req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/render/tikz", strings.NewReader(reqPayload))
			if err != nil {
				errCh <- fmt.Errorf("リクエスト生成エラー: %w", err)
				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(auth.HeaderAPIKey, benchmarkAPIKey)

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				errCh <- fmt.Errorf("HTTP リクエスト送信エラー (idx=%d): %w", idx, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errCh <- fmt.Errorf("ステータスコードエラー (idx=%d): %d", idx, resp.StatusCode)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	elapsed := time.Since(startTime)
	t.Logf("%d 並行コンパイル完了 (総所要時間: %v, 平均: %v/req)", totalRequests, elapsed, elapsed/time.Duration(totalRequests))

	for err := range errCh {
		t.Errorf("並行負荷テストエラー: %v", err)
	}
}
