// Package handler は OpenAPI 定義に基づく HTTP リクエストハンドラーを提供するパッケージ。
package handler

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tex-tikz-server/api"
)

// swaggerUIHTML は Swagger UI を CDN 経由で読み込む HTML テンプレート。
const swaggerUIHTML = `<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="UTF-8">
  <title>TikZ Rendering Service - API ドキュメント</title>
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5/favicon-32x32.png" sizes="32x32" />
  <style>
    html {
      box-sizing: border-box;
      overflow: -moz-scrollbars-vertical;
      overflow-y: scroll;
    }
    *, *:before, *:after {
      box-sizing: inherit;
    }
    body {
      margin: 0;
      background: #fafafa;
    }
    .topbar {
      display: none;
    }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" charset="UTF-8"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js" charset="UTF-8"></script>
  <script>
    window.onload = function() {
      const ui = SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "BaseLayout"
      });
      window.ui = ui;
    };
  </script>
</body>
</html>`

// RegisterDocsHandlers は Swagger UI および OpenAPI 定義ファイルの配信エンドポイントを登録する。
func RegisterDocsHandlers(r chi.Router) {
	// OpenAPI 仕様ファイルの配信
	r.Get("/openapi.yaml", ServeOpenAPISpec)

	// Swagger UI ドキュメント画面の配信
	r.Get("/docs", ServeSwaggerUI)
	r.Get("/docs/", ServeSwaggerUI)
	r.Get("/docs/*", ServeSwaggerUI)
}

// ServeOpenAPISpec は埋め込まれた openapi.yaml を Content-Type: application/yaml で返却する。
func ServeOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(api.OpenAPISpec)
}

// ServeSwaggerUI は Swagger UI HTML を返却する。
func ServeSwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, swaggerUIHTML)
}
