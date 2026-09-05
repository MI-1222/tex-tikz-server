// Package auth_test は internal/auth パッケージの単体テストを提供する。
package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tex-tikz-server/api/gen"
	"tex-tikz-server/internal/auth"
)

// nextHandler はミドルウェア通過を検証するためのダミーハンドラー。
var nextHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
})

// TestMiddleware_Authorized は正しい API キーでリクエストが成功することを検証する。
func TestMiddleware_Authorized(t *testing.T) {
	const validKey = "secret-test-api-key"
	middleware := auth.NewMiddleware(validKey)
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/render/tikz", nil)
	req.Header.Set(auth.HeaderAPIKey, validKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコードが不正です: 期待値=%d, 実際=%d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("レスポンスボディが不正です: %s", rec.Body.String())
	}
}

// TestMiddleware_MissingKey は API キー未指定時に 401 エラーとなることを検証する。
func TestMiddleware_MissingKey(t *testing.T) {
	middleware := auth.NewMiddleware("secret-api-key")
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/render/tikz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("ステータスコードが不正です: 期待値=%d, 実際=%d", http.StatusUnauthorized, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type が不正です: 期待値=application/json, 実際=%s", contentType)
	}

	var errResp gen.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("JSON パースに失敗しました: %v", err)
	}

	if errResp.Status != "error" || errResp.ErrorCode != "UNAUTHORIZED" {
		t.Errorf("ErrorResponse の値が不正です: %+v", errResp)
	}
}

// TestMiddleware_InvalidKey は不正な API キー指定時に 401 エラーとなることを検証する。
func TestMiddleware_InvalidKey(t *testing.T) {
	middleware := auth.NewMiddleware("secret-api-key")
	handler := middleware(nextHandler)

	testKeys := []string{
		"wrong-key",
		"secret-api-key-extended",
		"secret-api-ke",
		" ",
	}

	for _, key := range testKeys {
		t.Run("Key_"+key, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/render/tikz", nil)
			req.Header.Set(auth.HeaderAPIKey, key)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("不正なキー (%s) で 401 以外のステータスが返却されました: %d", key, rec.Code)
			}

			var errResp gen.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("JSON パースに失敗しました: %v", err)
			}

			if errResp.ErrorCode != "UNAUTHORIZED" {
				t.Errorf("ErrorCode が不正です: %s", errResp.ErrorCode)
			}
		})
	}
}

// TestMiddleware_Whitelist はホワイトリスト対象パスで API キーが不要であることを検証する。
func TestMiddleware_Whitelist(t *testing.T) {
	middleware := auth.NewMiddleware("secret-api-key")
	handler := middleware(nextHandler)

	whitelistPaths := []string{
		"/health",
		"/health/",
		"/docs",
		"/docs/",
		"/docs/index.html",
		"/docs/swagger.json",
		"/openapi.yaml",
		"/openapi.yaml/",
	}

	for _, path := range whitelistPaths {
		t.Run("Path_"+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			// API キーは指定しない
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("ホワイトリスト対象パス (%s) で認証エラーが発生しました: status=%d", path, rec.Code)
			}
		})
	}
}

// TestMiddleware_ProtectedPaths は保護されたパスで API キーが必要であることを検証する。
func TestMiddleware_ProtectedPaths(t *testing.T) {
	middleware := auth.NewMiddleware("secret-api-key")
	handler := middleware(nextHandler)

	protectedPaths := []string{
		"/api/v1/render/tikz",
		"/api/v1/health",
		"/doc",
		"/docs-secret",
		"/admin",
	}

	for _, path := range protectedPaths {
		t.Run("Path_"+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("保護されたパス (%s) で認証が要求されませんでした: status=%d", path, rec.Code)
			}
		})
	}
}
