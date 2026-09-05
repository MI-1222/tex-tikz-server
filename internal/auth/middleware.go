// Package auth は API キー認証および認証ミドルウェアを提供するパッケージ。
package auth

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"tex-tikz-server/api/gen"
)

// HeaderAPIKey はクライアントから送信される API キーの HTTP ヘッダー名。
const HeaderAPIKey = "X-API-Key"

// isWhitelisted は指定されたリクエストパスが認証除外対象(公開エンドポイント)かどうかを判定する。
func isWhitelisted(path string) bool {
	// パスの正規化(末尾のスラッシュを考慮)
	cleanPath := strings.TrimSuffix(path, "/")
	if cleanPath == "" {
		cleanPath = "/"
	}

	// ヘルスチェックおよびドキュメントエンドポイントの許可
	if cleanPath == "/health" || cleanPath == "/docs" || strings.HasPrefix(path, "/docs/") || cleanPath == "/openapi.yaml" {
		return true
	}

	return false
}

// NewMiddleware は API キー認証を行う HTTP ミドルウェア関数を生成する。
// リクエストヘッダー X-API-Key の値を subtle.ConstantTimeCompare で検証し、
// 一致しない場合またはキーが欠落している場合は 401 Unauthorized を返却する。
// /health および /docs 等のホワイトリスト対象パスは認証をスキップする。
func NewMiddleware(expectedAPIKey string) func(http.Handler) http.Handler {
	expectedBytes := []byte(expectedAPIKey)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// ホワイトリストに一致するパスは認証をスキップ
			if isWhitelisted(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			apiKey := r.Header.Get(HeaderAPIKey)
			if apiKey == "" {
				respondUnauthorized(w, "API キーが指定されていません。X-API-Key ヘッダーを設定してください。")
				return
			}

			// タイミング攻撃を防ぐため定数時間比較を行う
			keyBytes := []byte(apiKey)
			if subtle.ConstantTimeCompare(keyBytes, expectedBytes) != 1 {
				respondUnauthorized(w, "無効な API キーです。")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// respondUnauthorized は 401 Unauthorized エラーレスポンスを JSON 形式でクライアントに返却する。
func respondUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	errResp := gen.ErrorResponse{
		Status:    "error",
		ErrorCode: "UNAUTHORIZED",
		Message:   message,
	}

	_ = json.NewEncoder(w).Encode(errResp)
}
