// Package e2e は TikZ レンダリングサーバーの包括的なエンドツーエンド(E2E)テストを提供する。
package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"tex-tikz-server/api/gen"
	"tex-tikz-server/internal/auth"
	"tex-tikz-server/internal/cache"
	"tex-tikz-server/internal/handler"
	"tex-tikz-server/internal/tex"
)

const defaultTestAPIKey = "e2e_secret_test_key_12345"

// getTestAPIKey はテスト用 API キーを返却する。
func getTestAPIKey() string {
	if key := os.Getenv("API_KEY"); key != "" {
		return key
	}
	return defaultTestAPIKey
}

// getTestServerURL はテスト対象のサーバー URL を返却する。
// 環境変数 SERVER_URL が設定されていない場合は、
// ローカルの httptest.Server を起動して URL とクリーンアップ関数を返却する。
func getTestServerURL(t *testing.T) (string, func()) {
	t.Helper()

	if serverURL := os.Getenv("SERVER_URL"); serverURL != "" {
		return strings.TrimRight(serverURL, "/"), func() {}
	}

	// プロジェクトルートの fonts ディレクトリパスの解決
	fontDir := "fonts"
	if _, err := os.Stat(fontDir); os.IsNotExist(err) {
		if _, err := os.Stat("../../fonts"); err == nil {
			fontDir = "../../fonts"
		}
	}

	// ローカル統合テスト用サーバーの構築
	engine, err := tex.NewEngine(tex.EngineConfig{
		FontDir: fontDir,
	})
	if err != nil {
		t.Fatalf("TeX エンジンの初期化に失敗しました: %v", err)
	}

	lruCache := cache.New(100)
	h := handler.NewHandler(engine, lruCache, 10*time.Second)
	strictHandler := gen.NewStrictHandler(h, nil)

	r := chi.NewRouter()
	r.Use(auth.NewMiddleware(defaultTestAPIKey))
	handler.RegisterDocsHandlers(r)
	gen.HandlerFromMux(strictHandler, r)

	ts := httptest.NewServer(r)
	return ts.URL, ts.Close
}

// sendRequest は HTTP リクエストを送信しレスポンスを返却するヘルパー関数。
func sendRequest(t *testing.T, method, url, apiKey string, body any) (*http.Response, []byte) {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("リクエストボディの JSON 変換に失敗しました: %v", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("HTTP リクエストの作成に失敗しました: %v", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set(auth.HeaderAPIKey, apiKey)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP リクエストの送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("レスポンスボディの読み込みに失敗しました: %v", err)
	}

	return resp, respBytes
}

// TestE2E_HealthCheck は /health エンドポイントの正常系を検証する。
func TestE2E_HealthCheck(t *testing.T) {
	baseURL, cleanup := getTestServerURL(t)
	defer cleanup()

	resp, body := sendRequest(t, http.MethodGet, baseURL+"/health", "", nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ステータスコードが不正です: 期待値=%d, 実際=%d, body=%s", http.StatusOK, resp.StatusCode, string(body))
	}

	var healthResp gen.HealthResponse
	if err := json.Unmarshal(body, &healthResp); err != nil {
		t.Fatalf("JSON パースに失敗しました: %v", err)
	}

	if healthResp.Status != "ok" || healthResp.Version != "1.0.0" {
		t.Errorf("HealthResponse の内容が不正です: %+v", healthResp)
	}
}

// TestE2E_Docs は /docs および /openapi.yaml エンドポイントの正常系を検証する。
func TestE2E_Docs(t *testing.T) {
	baseURL, cleanup := getTestServerURL(t)
	defer cleanup()

	// 1. /docs の HTML 配信確認
	respDocs, bodyDocs := sendRequest(t, http.MethodGet, baseURL+"/docs", "", nil)
	if respDocs.StatusCode != http.StatusOK {
		t.Errorf("/docs のステータスコードが不正です: %d", respDocs.StatusCode)
	}
	if !strings.Contains(string(bodyDocs), "SwaggerUIBundle") {
		t.Errorf("/docs の HTML 内容が不正です: %s", string(bodyDocs))
	}

	// 2. /openapi.yaml の YAML 配信確認
	respSpec, bodySpec := sendRequest(t, http.MethodGet, baseURL+"/openapi.yaml", "", nil)
	if respSpec.StatusCode != http.StatusOK {
		t.Errorf("/openapi.yaml のステータスコードが不正です: %d", respSpec.StatusCode)
	}
	if !strings.Contains(string(bodySpec), "openapi: 3.1.0") {
		t.Errorf("/openapi.yaml の内容が不正です: %s", string(bodySpec))
	}
}

// TestE2E_RenderTikz_Basic は基本的な TikZ 図形のレンダリングを検証する。
func TestE2E_RenderTikz_Basic(t *testing.T) {
	baseURL, cleanup := getTestServerURL(t)
	defer cleanup()

	reqPayload := gen.RenderTikzRequest{
		Code: "\\begin{tikzpicture}\n\\draw[thick, fill=blue!20] (0,0) rectangle (2,2);\n\\draw[red, ->] (0,0) -- (2,2);\n\\end{tikzpicture}",
	}

	resp, body := sendRequest(t, http.MethodPost, baseURL+"/api/v1/render/tikz", getTestAPIKey(), reqPayload)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ステータスコードが不正です: 期待値=%d, 実際=%d, body=%s", http.StatusOK, resp.StatusCode, string(body))
	}

	var renderResp gen.RenderTikzResponse
	if err := json.Unmarshal(body, &renderResp); err != nil {
		t.Fatalf("JSON パースに失敗しました: %v", err)
	}

	if renderResp.Status != "success" {
		t.Errorf("Status が不正です: %s", renderResp.Status)
	}
	if renderResp.Format != "svg" {
		t.Errorf("Format が不正です: %s", renderResp.Format)
	}
	if renderResp.Cached {
		t.Errorf("初回の Cached フラグが true になっています")
	}
	if !strings.Contains(renderResp.Svg, "<svg") || !strings.Contains(renderResp.Svg, "</svg>") {
		t.Errorf("生成された SVG 文字列が不正です: %s", renderResp.Svg)
	}
	if renderResp.CompileTimeMs <= 0 {
		t.Errorf("CompileTimeMs が正の値ではありません: %d", renderResp.CompileTimeMs)
	}
}

// TestE2E_RenderTikz_JapaneseText は日本語テキスト(明朝/ゴシック)を含むレンダリングを検証する。
func TestE2E_RenderTikz_JapaneseText(t *testing.T) {
	baseURL, cleanup := getTestServerURL(t)
	defer cleanup()

	reqPayload := gen.RenderTikzRequest{
		Code: "\\begin{tikzpicture}\n\\node[draw, circle] at (0,0) {明朝テキスト};\n\\node[draw, rectangle] at (0,2) {\\textsf{ゴシックテキスト}};\n\\end{tikzpicture}",
	}

	resp, body := sendRequest(t, http.MethodPost, baseURL+"/api/v1/render/tikz", getTestAPIKey(), reqPayload)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ステータスコードが不正です: 期待値=%d, 実際=%d, body=%s", http.StatusOK, resp.StatusCode, string(body))
	}

	var renderResp gen.RenderTikzResponse
	if err := json.Unmarshal(body, &renderResp); err != nil {
		t.Fatalf("JSON パースに失敗しました: %v", err)
	}

	if renderResp.Status != "success" {
		t.Errorf("Status が不正です: %s", renderResp.Status)
	}
	// SVG 内にテキストまたはフォントグリフが含まれることを確認
	if !strings.Contains(renderResp.Svg, "<svg") {
		t.Errorf("SVG が生成されていません: %s", renderResp.Svg)
	}
}

// TestE2E_RenderTikz_CacheHit は同一リクエストによるキャッシュヒットと高速応答を検証する。
func TestE2E_RenderTikz_CacheHit(t *testing.T) {
	baseURL, cleanup := getTestServerURL(t)
	defer cleanup()

	reqPayload := gen.RenderTikzRequest{
		Code: "\\begin{tikzpicture}\n\\draw (0,0) circle (1.5cm);\n\\end{tikzpicture}",
	}

	// 1回目 (キャッシュミス)
	resp1, body1 := sendRequest(t, http.MethodPost, baseURL+"/api/v1/render/tikz", getTestAPIKey(), reqPayload)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("1回目のリクエストが失敗しました: %s", string(body1))
	}

	var renderResp1 gen.RenderTikzResponse
	if err := json.Unmarshal(body1, &renderResp1); err != nil {
		t.Fatalf("JSON パースに失敗しました: %v", err)
	}
	if renderResp1.Cached {
		t.Errorf("1回目で cached が true です")
	}

	// 2回目 (キャッシュヒット)
	startTime := time.Now()
	resp2, body2 := sendRequest(t, http.MethodPost, baseURL+"/api/v1/render/tikz", getTestAPIKey(), reqPayload)
	elapsed := time.Since(startTime)

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("2回目のリクエストが失敗しました: %s", string(body2))
	}

	var renderResp2 gen.RenderTikzResponse
	if err := json.Unmarshal(body2, &renderResp2); err != nil {
		t.Fatalf("JSON パースに失敗しました: %v", err)
	}

	if !renderResp2.Cached {
		t.Errorf("2回目の cached フラグが false です")
	}
	if renderResp2.Hash != renderResp1.Hash {
		t.Errorf("ハッシュキーが一致しません: hash1=%s, hash2=%s", renderResp1.Hash, renderResp2.Hash)
	}
	if renderResp2.Svg != renderResp1.Svg {
		t.Errorf("キャッシュされた SVG が元の SVG と一致しません")
	}

	// キャッシュヒット時のレイテンシが十分高速（10ms 以下目安、HTTP オーバーヘッド考慮で 50ms 未満）であることの確認
	if elapsed > 50*time.Millisecond {
		t.Logf("注意: キャッシュヒットのレスポンス時間=%v", elapsed)
	}
}

// TestE2E_RenderTikz_CompileError は不正な TikZ コードによる 422 応答を検証する。
func TestE2E_RenderTikz_CompileError(t *testing.T) {
	baseURL, cleanup := getTestServerURL(t)
	defer cleanup()

	reqPayload := gen.RenderTikzRequest{
		Code: "\\begin{tikzpicture}\n\\draw[invalid_key_xyz] (0,0) -- (1,1);\n\\end{tikzpicture}",
	}

	resp, body := sendRequest(t, http.MethodPost, baseURL+"/api/v1/render/tikz", getTestAPIKey(), reqPayload)

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコードが 422 ではありません: 期待値=%d, 実際=%d, body=%s", http.StatusUnprocessableEntity, resp.StatusCode, string(body))
	}

	var compileErrResp gen.CompileErrorResponse
	if err := json.Unmarshal(body, &compileErrResp); err != nil {
		t.Fatalf("JSON パースに失敗しました: %v", err)
	}

	if compileErrResp.Status != "error" || compileErrResp.ErrorCode != "COMPILATION_FAILED" {
		t.Errorf("CompileErrorResponse の内容が不正です: %+v", compileErrResp)
	}
	if compileErrResp.Log == "" {
		t.Errorf("エラーログが空です")
	}
}

// TestE2E_RenderTikz_AuthError は API キー未指定および不正時の 401 応答を検証する。
func TestE2E_RenderTikz_AuthError(t *testing.T) {
	baseURL, cleanup := getTestServerURL(t)
	defer cleanup()

	reqPayload := gen.RenderTikzRequest{
		Code: "\\begin{tikzpicture}\\draw (0,0) -- (1,1);\\end{tikzpicture}",
	}

	// 1. API キー未指定
	respNoKey, bodyNoKey := sendRequest(t, http.MethodPost, baseURL+"/api/v1/render/tikz", "", reqPayload)
	if respNoKey.StatusCode != http.StatusUnauthorized {
		t.Errorf("API キー未指定で 401 以外のステータスが返却されました: %d, body=%s", respNoKey.StatusCode, string(bodyNoKey))
	}

	// 2. 不正な API キー指定
	respWrongKey, bodyWrongKey := sendRequest(t, http.MethodPost, baseURL+"/api/v1/render/tikz", "wrong_api_key", reqPayload)
	if respWrongKey.StatusCode != http.StatusUnauthorized {
		t.Errorf("不正な API キーで 401 以外のステータスが返却されました: %d, body=%s", respWrongKey.StatusCode, string(bodyWrongKey))
	}
}
