// Package handler_test は internal/handler パッケージの単体テストを提供する。
package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"tex-tikz-server/api/gen"
	"tex-tikz-server/internal/cache"
	"tex-tikz-server/internal/handler"
	"tex-tikz-server/internal/tex"
)

// mockTeXEngine は TeXRenderer インターフェースのモック構造体。
type mockTeXEngine struct {
	renderFunc func(ctx context.Context, opts tex.RenderOptions) (*tex.RenderOutput, error)
}

func (m *mockTeXEngine) Render(ctx context.Context, opts tex.RenderOptions) (*tex.RenderOutput, error) {
	if m.renderFunc != nil {
		return m.renderFunc(ctx, opts)
	}
	return &tex.RenderOutput{
		SVG:           "<svg>mock-svg</svg>",
		CompileTimeMs: 150,
	}, nil
}

// setupTestServer はテスト用の chi.Mux ルーターを構築する。
func setupTestServer(engine handler.TeXRenderer, c handler.ResultCacher) *chi.Mux {
	h := handler.NewHandler(engine, c, 5*time.Second)
	strictHandler := gen.NewStrictHandler(h, nil)

	r := chi.NewRouter()
	handler.RegisterDocsHandlers(r)
	gen.HandlerFromMux(strictHandler, r)

	return r
}

// TestCheckHealth は CheckHealth エンドポイントのレスポンスを検証する。
func TestCheckHealth(t *testing.T) {
	c := cache.New(10)
	engine := &mockTeXEngine{}
	r := setupTestServer(engine, c)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコードが不正です: 期待値=%d, 実際=%d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"status":"ok"`) || !strings.Contains(body, `"version":"1.0.0"`) {
		t.Errorf("レスポンスボディが不正です: %s", body)
	}
}

// TestRenderTikz_Success_CacheMiss は初回レンダリング (キャッシュミス) の正常系を検証する。
func TestRenderTikz_Success_CacheMiss(t *testing.T) {
	c := cache.New(10)
	called := false
	engine := &mockTeXEngine{
		renderFunc: func(ctx context.Context, opts tex.RenderOptions) (*tex.RenderOutput, error) {
			called = true
			if opts.Code != "\\draw (0,0) circle (1cm);" {
				t.Errorf("渡された Code が不正です: %s", opts.Code)
			}
			return &tex.RenderOutput{
				SVG:           "<svg>circle</svg>",
				CompileTimeMs: 250,
			}, nil
		},
	}
	r := setupTestServer(engine, c)

	reqBody := `{"code":"\\draw (0,0) circle (1cm);","format":"svg"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/render/tikz", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコードが不正です: 期待値=%d, 実際=%d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if !called {
		t.Errorf("TeX エンジンの Render が呼び出されませんでした")
	}

	var resp gen.RenderTikzResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON パースに失敗しました: %v", err)
	}

	if resp.Status != "success" || resp.Cached || resp.Svg != "<svg>circle</svg>" || resp.CompileTimeMs != 250 {
		t.Errorf("レスポンス内容が不正です: %+v", resp)
	}
}

// TestRenderTikz_Success_CacheHit はキャッシュヒット時の高速応答を検証する。
func TestRenderTikz_Success_CacheHit(t *testing.T) {
	c := cache.New(10)
	callCount := 0
	engine := &mockTeXEngine{
		renderFunc: func(ctx context.Context, opts tex.RenderOptions) (*tex.RenderOutput, error) {
			callCount++
			return &tex.RenderOutput{
				SVG:           "<svg>cached-circle</svg>",
				CompileTimeMs: 300,
			}, nil
		},
	}
	r := setupTestServer(engine, c)

	reqBody := `{"code":"\\draw (0,0) circle (2cm);"}`

	// 1回目のリクエスト (キャッシュミス)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/render/tikz", strings.NewReader(reqBody))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("1回目のリクエストが失敗しました: %s", rec1.Body.String())
	}
	if callCount != 1 {
		t.Fatalf("1回目の呼び出し回数が不正です: %d", callCount)
	}

	// 2回目のリクエスト (キャッシュヒット)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/render/tikz", strings.NewReader(reqBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("2回目のステータスコードが不正です: %d", rec2.Code)
	}
	if callCount != 1 {
		t.Errorf("キャッシュヒットにもかかわらず TeX エンジンが再実行されました: callCount=%d", callCount)
	}

	var resp2 gen.RenderTikzResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("JSON パースに失敗しました: %v", err)
	}

	if resp2.Status != "success" || !resp2.Cached || resp2.Svg != "<svg>cached-circle</svg>" {
		t.Errorf("キャッシュヒットレスポンスが不正です: %+v", resp2)
	}
}

// TestRenderTikz_ValidationErrors は無効なリクエストボディや空コードでの 400 応答を検証する。
func TestRenderTikz_ValidationErrors(t *testing.T) {
	c := cache.New(10)
	engine := &mockTeXEngine{}
	r := setupTestServer(engine, c)

	testCases := []struct {
		name string
		body string
	}{
		{name: "空ボディ", body: `{}`},
		{name: "空文字コード", body: `{"code":""}`},
		{name: "空白文字のみ", body: `{"code":"   \n\t  "}`},
		{name: "不正なJSON", body: `{"code":`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/render/tikz", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("ステータスコードが 400 ではありません: code=%d, body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestRenderTikz_CompileError は TeX コンパイルエラー時の 422 応答を検証する。
func TestRenderTikz_CompileError(t *testing.T) {
	c := cache.New(10)
	engine := &mockTeXEngine{
		renderFunc: func(ctx context.Context, opts tex.RenderOptions) (*tex.RenderOutput, error) {
			return nil, &tex.CompileError{
				Message: "uplatex compilation failed",
				Log:     "! Undefined control sequence: \\invalidcmd",
				Cause:   errors.New("exit status 1"),
			}
		},
	}
	r := setupTestServer(engine, c)

	reqBody := `{"code":"\\invalidcmd"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/render/tikz", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコードが 422 ではありません: %d, body=%s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "COMPILATION_FAILED") || !strings.Contains(body, "Undefined control sequence") {
		t.Errorf("422 レスポンスの内容が不正です: %s", body)
	}
}

// TestRenderTikz_Timeout はコンパイルタイムアウト時の 504 応答を検証する。
func TestRenderTikz_Timeout(t *testing.T) {
	c := cache.New(10)
	engine := &mockTeXEngine{
		renderFunc: func(ctx context.Context, opts tex.RenderOptions) (*tex.RenderOutput, error) {
			return nil, context.DeadlineExceeded
		},
	}
	r := setupTestServer(engine, c)

	reqBody := `{"code":"\\draw (0,0) -- (1,1);","timeout_ms":2000}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/render/tikz", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("ステータスコードが 504 ではありません: %d, body=%s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "TIMEOUT") {
		t.Errorf("504 レスポンスの内容が不正です: %s", body)
	}
}

// TestDocsEndpoints は Swagger UI および OpenAPI 定義ファイルの配信を検証する。
func TestDocsEndpoints(t *testing.T) {
	c := cache.New(10)
	engine := &mockTeXEngine{}
	r := setupTestServer(engine, c)

	// GET /openapi.yaml
	reqSpec := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	recSpec := httptest.NewRecorder()
	r.ServeHTTP(recSpec, reqSpec)

	if recSpec.Code != http.StatusOK {
		t.Errorf("/openapi.yaml のステータスコードが不正です: %d", recSpec.Code)
	}
	if !strings.Contains(recSpec.Body.String(), "openapi: 3.1.0") {
		t.Errorf("/openapi.yaml の内容が不正です: %s", recSpec.Body.String())
	}

	// GET /docs
	reqDocs := httptest.NewRequest(http.MethodGet, "/docs", nil)
	recDocs := httptest.NewRecorder()
	r.ServeHTTP(recDocs, reqDocs)

	if recDocs.Code != http.StatusOK {
		t.Errorf("/docs のステータスコードが不正です: %d", recDocs.Code)
	}
	if !strings.Contains(recDocs.Body.String(), "SwaggerUIBundle") {
		t.Errorf("/docs の HTML 内容が不正です: %s", recDocs.Body.String())
	}
}
