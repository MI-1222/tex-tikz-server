package tex

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestNewCompiler は Compiler の初期化をテストする。
func TestNewCompiler(t *testing.T) {
	c := NewCompiler()
	if c == nil {
		t.Fatal("NewCompiler() が nil")
	}
	if c.UplatexCmd != "uplatex" {
		t.Errorf("期待される UplatexCmd: uplatex, got: %s", c.UplatexCmd)
	}
	if c.DvisvgmCmd != "dvisvgm" {
		t.Errorf("期待される DvisvgmCmd: dvisvgm, got: %s", c.DvisvgmCmd)
	}
}

// TestBuildArgs はコマンド引数の構築をテストする。
func TestBuildArgs(t *testing.T) {
	dviArgs := BuildDVIArgs()
	expectedDVI := []string{"-interaction=nonstopmode", "-no-shell-escape", "-file-line-error", "main.tex"}
	if len(dviArgs) != len(expectedDVI) {
		t.Fatalf("BuildDVIArgs の引数の数が一致しない: got %d, want %d", len(dviArgs), len(expectedDVI))
	}
	for i, arg := range expectedDVI {
		if dviArgs[i] != arg {
			t.Errorf("BuildDVIArgs[%d] が異なる: got %s, want %s", i, dviArgs[i], arg)
		}
	}

	svgArgs := BuildSVGArgs("custom.map")
	expectedSVG := []string{"--no-fonts", "--fontmap=custom.map", "--clipjoin", "--precision=2", "--stdout", "main.dvi"}
	if len(svgArgs) != len(expectedSVG) {
		t.Fatalf("BuildSVGArgs の引数の数が一致しない: got %d, want %d", len(svgArgs), len(expectedSVG))
	}
	for i, arg := range expectedSVG {
		if svgArgs[i] != arg {
			t.Errorf("BuildSVGArgs[%d] が異なる: got %s, want %s", i, svgArgs[i], arg)
		}
	}
}

// TestCompiler_InvalidWorkspace は nil または無効なワークスペースでのエラーハンドリングをテストする。
func TestCompiler_InvalidWorkspace(t *testing.T) {
	c := NewCompiler()
	ctx := context.Background()

	if _, err := c.CompileDVI(ctx, nil); err == nil {
		t.Error("CompileDVI(nil) でエラーが発生しませんでした。")
	}

	if _, err := c.ConvertSVG(ctx, nil); err == nil {
		t.Error("ConvertSVG(nil) でエラーが発生しませんでした。")
	}

	emptyWS := &Workspace{Dir: ""}
	if _, err := c.CompileDVI(ctx, emptyWS); err == nil {
		t.Error("CompileDVI(emptyWS) でエラーが発生しませんでした。")
	}

	if _, err := c.ConvertSVG(ctx, emptyWS); err == nil {
		t.Error("ConvertSVG(emptyWS) でエラーが発生しませんでした。")
	}
}

// TestCompiler_Timeout はコンテキストのタイムアウトによるプロセス中断をテストする。
func TestCompiler_Timeout(t *testing.T) {
	ws, err := NewWorkspace()
	if err != nil {
		t.Fatalf("NewWorkspace() エラー: %v", err)
	}
	defer func() {
		_ = ws.Cleanup()
	}()

	// 存在しない、または即時終了しないコマンドとして sleep コマンドをモック指定
	c := &Compiler{
		UplatexCmd: "sleep",
		DvisvgmCmd: "sleep",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = c.CompileDVI(ctx, ws)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("タイムアウト時にエラーが発生しませんでした。")
	}

	// タイムアウトにより概ね短時間で打ち切られていることを確認
	if elapsed > 2*time.Second {
		t.Errorf("タイムアウト処理に時間がかかりすぎています: %v", elapsed)
	}
}

// TestCompiler_MockExecution はモックコマンド(echo等)を用いた正常系レスポンス抽出をテストする。
func TestCompiler_MockExecution(t *testing.T) {
	ws, err := NewWorkspace()
	if err != nil {
		t.Fatalf("NewWorkspace() エラー: %v", err)
	}
	defer func() {
		_ = ws.Cleanup()
	}()

	// echo コマンドで SVG 出力を模擬
	c := &Compiler{
		UplatexCmd: "echo",
		DvisvgmCmd: "echo",
	}

	ctx := context.Background()
	svg, err := c.ConvertSVG(ctx, ws)
	if err != nil {
		t.Fatalf("ConvertSVG() エラー: %v", err)
	}

	if !strings.Contains(svg, "--stdout") {
		t.Errorf("モック echo の出力に期待される引数が含まれていません: %s", svg)
	}
}
