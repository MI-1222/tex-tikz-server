// Package handler は OpenAPI 定義に基づく HTTP リクエストハンドラーを提供するパッケージ。
package handler

import (
	"context"
	"errors"
	"strings"
	"time"

	"tex-tikz-server/api/gen"
	"tex-tikz-server/internal/cache"
	"tex-tikz-server/internal/tex"
)

// ServerVersion は API サーバーのバージョン情報。
const ServerVersion = "1.0.0"

// TeXRenderer は TeX コンパイル実行を行うインターフェース。
// テスト時のモック化を容易にするために定義。
type TeXRenderer interface {
	Render(ctx context.Context, opts tex.RenderOptions) (*tex.RenderOutput, error)
}

// ResultCacher はキャッシュ操作を行うインターフェース。
type ResultCacher interface {
	Get(key string) (string, bool)
	Set(key, value string)
}

// Handler は StrictServerInterface を実装するハンドラー構造体。
type Handler struct {
	engine         TeXRenderer
	cache          ResultCacher
	defaultTimeout time.Duration
}

// NewHandler は新しい Handler インスタンスを生成する。
func NewHandler(engine TeXRenderer, cache ResultCacher, defaultTimeout time.Duration) *Handler {
	if defaultTimeout <= 0 {
		defaultTimeout = 10 * time.Second
	}

	return &Handler{
		engine:         engine,
		cache:          cache,
		defaultTimeout: defaultTimeout,
	}
}

// CheckHealth はヘルスチェックエンドポイント (GET /health) のハンドラー。
func (h *Handler) CheckHealth(ctx context.Context, request gen.CheckHealthRequestObject) (gen.CheckHealthResponseObject, error) {
	return gen.CheckHealth200JSONResponse{
		Status:  "ok",
		Version: ServerVersion,
	}, nil
}

// RenderTikz は TikZ レンダリングエンドポイント (POST /api/v1/render/tikz) のハンドラー。
func (h *Handler) RenderTikz(ctx context.Context, request gen.RenderTikzRequestObject) (gen.RenderTikzResponseObject, error) {
	startTime := time.Now()

	// 1. リクエストボディの存在確認
	if request.Body == nil {
		return gen.RenderTikz400JSONResponse{
			Status:    "error",
			ErrorCode: "INVALID_REQUEST",
			Message:   "リクエストボディが空です。",
		}, nil
	}

	code := strings.TrimSpace(request.Body.Code)
	if code == "" {
		return gen.RenderTikz400JSONResponse{
			Status:    "error",
			ErrorCode: "INVALID_REQUEST",
			Message:   "code フィールドは必須であり、空文字は許可されません。",
		}, nil
	}

	// 2. パラメータの抽出とデフォルト値の適用
	format := "svg"
	if request.Body.Format != nil && string(*request.Body.Format) != "" {
		format = string(*request.Body.Format)
	}

	preamble := ""
	if request.Body.Preamble != nil {
		preamble = *request.Body.Preamble
	}

	timeout := h.defaultTimeout
	if request.Body.TimeoutMs != nil && *request.Body.TimeoutMs > 0 {
		timeout = time.Duration(*request.Body.TimeoutMs) * time.Millisecond
	}

	// 3. キャッシュキーの計算
	hashKey := cache.ComputeKey(code, preamble, format)

	// 4. キャッシュの検索
	if cachedSVG, ok := h.cache.Get(hashKey); ok {
		elapsedMs := int(time.Since(startTime).Milliseconds())
		return gen.RenderTikz200JSONResponse{
			Status:        "success",
			Format:        format,
			Svg:           cachedSVG,
			Cached:        true,
			Hash:          hashKey,
			CompileTimeMs: elapsedMs,
		}, nil
	}

	// 5. TeX エンジンの実行 (コンパイル)
	renderOpts := tex.RenderOptions{
		Code:     code,
		Preamble: preamble,
		Timeout:  timeout,
	}

	output, err := h.engine.Render(ctx, renderOpts)
	if err != nil {
		// コンパイルエラーのハンドリング
		var compileErr *tex.CompileError
		if errors.As(err, &compileErr) {
			return gen.RenderTikz422JSONResponse{
				Status:    "error",
				ErrorCode: "COMPILATION_FAILED",
				Message:   compileErr.Message,
				Log:       compileErr.Log,
			}, nil
		}

		// タイムアウトエラーのハンドリング
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "タイムアウト") {
			return gen.RenderTikz504JSONResponse{
				Status:    "error",
				ErrorCode: "TIMEOUT",
				Message:   "TikZ のレンダリング処理がタイムアウトしました。",
			}, nil
		}

		// その他の予期しないエラー
		return nil, err
	}

	// 6. 成功結果をキャッシュに保存
	h.cache.Set(hashKey, output.SVG)

	return gen.RenderTikz200JSONResponse{
		Status:        "success",
		Format:        format,
		Svg:           output.SVG,
		Cached:        false,
		Hash:          hashKey,
		CompileTimeMs: output.CompileTimeMs,
	}, nil
}
