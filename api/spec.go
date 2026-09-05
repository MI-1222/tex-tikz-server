// Package api は OpenAPI 3.1 スキーマおよびコード自動生成を管理するパッケージ。
package api

import (
	_ "embed"
)

// OpenAPISpec は埋め込まれた OpenAPI 3.1 仕様書の YAML バイトデータ。
//
//go:embed openapi.yaml
var OpenAPISpec []byte
