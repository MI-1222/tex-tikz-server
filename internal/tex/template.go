// Package tex は TeX / TikZ コンパイル処理およびテンプレート管理を提供するパッケージ。
package tex

import (
	"bytes"
	"fmt"
	"text/template"
)

// latexTemplateSource は LaTeX ドキュメントの基本テンプレート文字列。
//
// uplatex + standalone + jsarticle + dvisvgm ドライバを基盤とし、
// 代表的な TikZ ライブラリおよび pgfplots を組み込む。
const latexTemplateSource = `\def\pgfsysdriver{pgfsys-dvisvgm.def}
\documentclass[uplatex, tikz, border=0pt, class=jsarticle]{standalone}
\usetikzlibrary{arrows.meta, positioning, calc, backgrounds, quotes, angles, fpu}
\usepackage{amsmath, amssymb}
\usepackage{pgfplots}
\pgfplotsset{
  compat=1.18,
  every axis/.append style={
    restrict y to domain=-1000:1000, 
    restrict x to domain=-1000:1000
  }
}
{{.CustomPreamble}}
\errorcontextlines=10
\begin{document}
{{.TikzCode}}
\end{document}
`

// TemplateParams は LaTeX テンプレート展開用のパラメータ構造体。
type TemplateParams struct {
	// CustomPreamble はユーザー指定の追加プリアンブル。
	CustomPreamble string

	// TikzCode はレンダリング対象の TikZ コード。
	TikzCode string
}

// TemplateEngine は LaTeX ソースコードを生成するテンプレートエンジン。
type TemplateEngine struct {
	tmpl *template.Template
}

// NewTemplateEngine は新しい TemplateEngine インスタンスを生成する。
func NewTemplateEngine() (*TemplateEngine, error) {
	tmpl, err := template.New("latex_document").Parse(latexTemplateSource)
	if err != nil {
		return nil, fmt.Errorf("LaTeX テンプレートのパースに失敗: %w", err)
	}
	return &TemplateEngine{tmpl: tmpl}, nil
}

// Render はパラメータをもとに完全な LaTeX ソースコードを組み立てて返却する。
func (e *TemplateEngine) Render(params TemplateParams) (string, error) {
	var buf bytes.Buffer
	if err := e.tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("LaTeX テンプレートの展開に失敗: %w", err)
	}
	return buf.String(), nil
}
