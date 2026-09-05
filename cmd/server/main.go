// Package main は TikZ / TeX レンダリングサーバーのエントリポイント。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/chi/v5"

	"tex-tikz-server/api/gen"
	"tex-tikz-server/internal/auth"
	"tex-tikz-server/internal/cache"
	"tex-tikz-server/internal/config"
	"tex-tikz-server/internal/handler"
	"tex-tikz-server/internal/tex"
)

func main() {
	log.Println("TikZ / TeX レンダリングサーバーを初期化しています...")

	// 1. 設定の読み込み
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("設定の読み込みに失敗しました: %v", err)
	}

	// 2. キャッシュの初期化
	lruCache := cache.New(cfg.CacheSize)
	log.Printf("LRU キャッシュを初期化しました (容量: %d 件)", cfg.CacheSize)

	// 3. TeX エンジンの初期化
	engine, err := tex.NewEngine(tex.EngineConfig{
		FontDir: cfg.FontDir,
	})
	if err != nil {
		log.Fatalf("TeX エンジンの初期化に失敗しました: %v", err)
	}
	log.Printf("TeX エンジンを初期化しました (フォントディレクトリ: %s)", cfg.FontDir)

	// 4. ハンドラーおよび StrictHandler の生成
	h := handler.NewHandler(engine, lruCache, cfg.ExecTimeout())
	strictHandler := gen.NewStrictHandler(h, nil)

	// 5. HTTP ルーターの構築
	r := chi.NewRouter()

	// 標準ミドルウェアの適用
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// API キー認証ミドルウェアの適用 (/health, /docs, /openapi.yaml はホワイトリスト除外)
	r.Use(auth.NewMiddleware(cfg.APIKey))

	// ドキュメント配信エンドポイントの登録 (/docs, /openapi.yaml)
	handler.RegisterDocsHandlers(r)

	// OpenAPI 自動生成ルーティングの登録 (/health, /api/v1/render/tikz)
	gen.HandlerFromMux(strictHandler, r)

	// 6. HTTP サーバーの作成
	srv := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 7. サーバー起動 (Goroutine)
	go func() {
		log.Printf("サーバーを起動しました (アドレス: %s)", cfg.Addr())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP サーバーの実行エラー: %v", err)
		}
	}()

	// 8. シグナル受信によるグレースフルシャットダウン待機
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit
	log.Printf("シャットダウンシグナルを受信しました: %v", sig)

	// シャットダウン用のコンテキスト作成 (最大 10 秒待機)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("サーバーの強制停止エラー: %v", err)
	}

	log.Println("サーバーが正常に終了しました。")
}
