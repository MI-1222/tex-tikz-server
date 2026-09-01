//go:build tools
// +build tools

// Package tools はビルドおよびコード生成に必要な開発ツールの依存関係を管理するパッケージ。
package tools

import (
	_ "github.com/go-chi/chi/v5"
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
	_ "github.com/oapi-codegen/runtime"
)
