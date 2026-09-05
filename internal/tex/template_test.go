package tex

import (
	"strings"
	"testing"
)

// TestNewTemplateEngine はテンプレートエンジンの初期化のテストを行う。
func TestNewTemplateEngine(t *testing.T) {
	engine, err := NewTemplateEngine()
	if err != nil {
		t.Fatalf("TemplateEngine の生成失敗: %v", err)
	}
	if engine == nil {
		t.Fatal("TemplateEngine が nil")
	}
}

// TestTemplateEngine_Render は LaTeX コードの組み立て展開のテストを行う。
func TestTemplateEngine_Render(t *testing.T) {
	engine, err := NewTemplateEngine()
	if err != nil {
		t.Fatalf("TemplateEngine の初期化エラー: %v", err)
	}

	tests := []struct {
		name       string
		params     TemplateParams
		wantSubstr []string
	}{
		{
			name: "基本的なTikZコードの展開",
			params: TemplateParams{
				CustomPreamble: "",
				TikzCode:       "\\draw (0,0) circle (1cm);",
			},
			wantSubstr: []string{
				`\def\pgfsysdriver{pgfsys-dvisvgm.def}`,
				`\documentclass[uplatex, tikz, border=0pt, class=jsarticle]{standalone}`,
				`\usetikzlibrary{arrows.meta, positioning, calc, backgrounds, quotes, angles, fpu}`,
				`\usepackage{amsmath, amssymb}`,
				`\usepackage{pgfplots}`,
				`\begin{document}`,
				`\draw (0,0) circle (1cm);`,
				`\end{document}`,
			},
		},
		{
			name: "カスタムプリアンブルを含む展開",
			params: TemplateParams{
				CustomPreamble: `\usepackage{bm}` + "\n" + `\usetikzlibrary{patterns}`,
				TikzCode:       `\node at (0,0) {$\bm{x}$};`,
			},
			wantSubstr: []string{
				`\usepackage{bm}`,
				`\usetikzlibrary{patterns}`,
				`\node at (0,0) {$\bm{x}$};`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engine.Render(tt.params)
			if err != nil {
				t.Fatalf("Render() エラー: %v", err)
			}

			for _, want := range tt.wantSubstr {
				if !strings.Contains(got, want) {
					t.Errorf("期待される部分文字列 %q が含まれていない。\n生成結果:\n%s", want, got)
				}
			}
		})
	}
}
