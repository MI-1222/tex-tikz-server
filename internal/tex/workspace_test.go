package tex

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewWorkspace は一時作業ディレクトリの作成と削除をテストする。
func TestNewWorkspace(t *testing.T) {
	ws, err := NewWorkspace()
	if err != nil {
		t.Fatalf("NewWorkspace() の作成エラー: %v", err)
	}

	if ws.Dir == "" {
		t.Fatal("Workspace.Dir が空文字列である。")
	}

	// ディレクトリが存在することを確認
	info, err := os.Stat(ws.Dir)
	if err != nil {
		t.Fatalf("作成されたディレクトリにアクセスできない: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s はディレクトリではない。", ws.Dir)
	}

	// Cleanup を実行
	if err := ws.Cleanup(); err != nil {
		t.Fatalf("Cleanup() エラー: %v", err)
	}

	// ディレクトリが削除されたことを確認
	if _, err := os.Stat(ws.Dir); !os.IsNotExist(err) {
		t.Fatalf("Cleanup() 後もディレクトリが存在している: %s", ws.Dir)
	}
}

// TestWorkspace_WriteFile は一時ディレクトリへのファイル書き込みをテストする。
func TestWorkspace_WriteFile(t *testing.T) {
	ws, err := NewWorkspace()
	if err != nil {
		t.Fatalf("NewWorkspace() の作成エラー: %v", err)
	}
	defer func() {
		_ = ws.Cleanup()
	}()

	content := []byte("\\documentclass{article}\n\\begin{document}\nHello\n\\end{document}")
	if err := ws.WriteFile("main.tex", content); err != nil {
		t.Fatalf("WriteFile() エラー: %v", err)
	}

	readData, err := os.ReadFile(filepath.Join(ws.Dir, "main.tex"))
	if err != nil {
		t.Fatalf("書き込んだファイルの読み取りエラー: %v", err)
	}
	if string(readData) != string(content) {
		t.Fatalf("期待される内容と異なる: got %s, want %s", string(readData), string(content))
	}
}

// TestWorkspace_PrepareFontmap はフォントマップおよびフォントファイルの配置をテストする。
func TestWorkspace_PrepareFontmap(t *testing.T) {
	// テスト用のダミーフォントディレクトリを作成
	dummyFontDir, err := os.MkdirTemp("", "test-fonts-*")
	if err != nil {
		t.Fatalf("ダミーフォントディレクトリの作成エラー: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(dummyFontDir)
	}()

	// ダミーファイル作成
	_ = os.WriteFile(filepath.Join(dummyFontDir, "fontmap.map"), []byte("dummy map"), 0600)
	_ = os.WriteFile(filepath.Join(dummyFontDir, "font.otf"), []byte("dummy otf"), 0600)
	_ = os.WriteFile(filepath.Join(dummyFontDir, "readme.txt"), []byte("ignore me"), 0600)

	ws, err := NewWorkspace()
	if err != nil {
		t.Fatalf("NewWorkspace() の作成エラー: %v", err)
	}
	defer func() {
		_ = ws.Cleanup()
	}()

	if err := ws.PrepareFontmap(dummyFontDir); err != nil {
		t.Fatalf("PrepareFontmap() エラー: %v", err)
	}

	// fontmap.map と font.otf が配置されていることを確認
	if _, err := os.Stat(filepath.Join(ws.Dir, "fontmap.map")); err != nil {
		t.Errorf("fontmap.map が配置されていない: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.Dir, "font.otf")); err != nil {
		t.Errorf("font.otf が配置されていない: %v", err)
	}
	// txt ファイルはコピー/リンク対象外であることを確認
	if _, err := os.Stat(filepath.Join(ws.Dir, "readme.txt")); !os.IsNotExist(err) {
		t.Errorf("対象外の readme.txt が配置されている。")
	}
}
