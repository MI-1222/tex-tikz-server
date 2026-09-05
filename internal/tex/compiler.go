// Package tex は TeX / TikZ コンパイル処理およびテンプレート管理を提供するパッケージ。
package tex

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Compiler は uplatex および dvisvgm 外部コマンドを実行して
// TikZ コードを SVG に変換するラッパー構造体。
type Compiler struct {
	// UplatexCmd は uplatex コマンドのパス(デフォルト: "uplatex")。
	UplatexCmd string

	// DvisvgmCmd は dvisvgm コマンドのパス(デフォルト: "dvisvgm")。
	DvisvgmCmd string
}

// NewCompiler は新しい Compiler インスタンスを生成する。
func NewCompiler() *Compiler {
	return &Compiler{
		UplatexCmd: "uplatex",
		DvisvgmCmd: "dvisvgm",
	}
}

// CompileResult はコンパイルコマンドの実行結果。
type CompileResult struct {
	// Output はコマンドの標準出力・標準エラー出力。
	Output []byte

	// Err は実行時に発生したエラー。
	Err error
}

// CompileDVI はワークスペース内の main.tex に対し uplatex を実行して main.dvi を生成する。
// 安全のため -no-shell-escape を強制し、非対話モード (-interaction=nonstopmode) で実行する。
func (c *Compiler) CompileDVI(ctx context.Context, ws *Workspace) ([]byte, error) {
	if ws == nil || ws.Dir == "" {
		return nil, fmt.Errorf("無効なワークスペースが指定されました。")
	}

	cmdName := c.UplatexCmd
	if cmdName == "" {
		cmdName = "uplatex"
	}

	args := []string{
		"-interaction=nonstopmode",
		"-no-shell-escape",
		"-file-line-error",
		"main.tex",
	}

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = ws.Dir

	var outputBuf bytes.Buffer
	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	err := cmd.Run()
	output := outputBuf.Bytes()

	if err != nil {
		return output, fmt.Errorf("uplatex 実行エラー: %w", err)
	}

	return output, nil
}

// ConvertSVG はワークスペース内の main.dvi に対し、
// dvisvgm を実行し、SVG 文字列を生成して返却する。
// --fontmap=fontmap.map を適用し、--stdout で標準出力から SVG XML を取得する。
func (c *Compiler) ConvertSVG(ctx context.Context, ws *Workspace) (string, error) {
	if ws == nil || ws.Dir == "" {
		return "", fmt.Errorf("無効なワークスペースが指定されました。")
	}

	cmdName := c.DvisvgmCmd
	if cmdName == "" {
		cmdName = "dvisvgm"
	}

	args := []string{
		"--no-fonts",
		"--fontmap=fontmap.map",
		"--clipjoin",
		"--precision=2",
		"--stdout",
		"main.dvi",
	}

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = ws.Dir

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("dvisvgm 実行エラー: %w (stderr: %s)", err, stderrBuf.String())
	}

	svgData := stdoutBuf.String()
	if svgData == "" {
		return "", fmt.Errorf("dvisvgm からの出力が空です (stderr: %s)", stderrBuf.String())
	}

	return svgData, nil
}

// BuildDVIArgs は uplatex 実行用引数スライスを返却するヘルパー関数。
func BuildDVIArgs() []string {
	return []string{
		"-interaction=nonstopmode",
		"-no-shell-escape",
		"-file-line-error",
		"main.tex",
	}
}

// BuildSVGArgs は dvisvgm 実行用引数スライスを返却するヘルパー関数。
func BuildSVGArgs(mapFileName string) []string {
	if mapFileName == "" {
		mapFileName = "fontmap.map"
	}
	return []string{
		"--no-fonts",
		filepath.Join("--fontmap=" + mapFileName),
		"--clipjoin",
		"--precision=2",
		"--stdout",
		"main.dvi",
	}
}
