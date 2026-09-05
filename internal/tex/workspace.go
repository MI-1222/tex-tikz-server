// Package tex は TeX / TikZ コンパイル処理およびテンプレート管理を提供するパッケージ。
package tex

import (
	"fmt"
	"os"
	"path/filepath"
)

// Workspace は TeX コンパイルを実行するための隔離された一時作業領域。
type Workspace struct {
	// Dir は作成された一時作業ディレクトリの絶対パス。
	Dir string
}

// NewWorkspace は新しいジョブ用の一時作業ディレクトリを作成して返却する。
func NewWorkspace() (*Workspace, error) {
	tempDir, err := os.MkdirTemp("", "tikz-job-*")
	if err != nil {
		return nil, fmt.Errorf("一時作業ディレクトリの作成に失敗した: %w", err)
	}
	return &Workspace{Dir: tempDir}, nil
}

// WriteFile は一時作業ディレクトリ内に指定されたファイル名とデータでファイルを書き込む。
func (w *Workspace) WriteFile(filename string, data []byte) error {
	filePath := filepath.Join(w.Dir, filename)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("ファイル %s の書き込みに失敗した: %w", filename, err)
	}
	return nil
}

// PrepareFontmap は指定されたフォントディレクトリからフォントマップおよびフォントファイルを
// 一時作業ディレクトリ内にシンボリックリンク（失敗時はコピー）で配置する。
func (w *Workspace) PrepareFontmap(fontDir string) error {
	if fontDir == "" {
		return nil
	}

	mapSrc := filepath.Join(fontDir, "fontmap.map")
	if _, err := os.Stat(mapSrc); err == nil {
		mapDst := filepath.Join(w.Dir, "fontmap.map")
		if err := linkOrCopy(mapSrc, mapDst); err != nil {
			return fmt.Errorf("フォントマップの配置に失敗した: %w", err)
		}
	}

	// フォントファイル（.otf, .ttf, .ttc）のシンボリックリンク作成
	entries, err := os.ReadDir(fontDir)
	if err != nil {
		return fmt.Errorf("フォントディレクトリの読み取りに失敗した: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext == ".otf" || ext == ".ttf" || ext == ".ttc" {
			src := filepath.Join(fontDir, entry.Name())
			dst := filepath.Join(w.Dir, entry.Name())
			if err := linkOrCopy(src, dst); err != nil {
				return fmt.Errorf("フォントファイル %s の配置に失敗した: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// Cleanup は一時作業ディレクトリとその配下の全ファイルを削除する。
func (w *Workspace) Cleanup() error {
	if w.Dir == "" {
		return nil
	}
	if err := os.RemoveAll(w.Dir); err != nil {
		return fmt.Errorf("一時作業ディレクトリの削除に失敗した (%s): %w", w.Dir, err)
	}
	return nil
}

// linkOrCopy はシンボリックリンクを作成し、失敗した場合はファイルのコピーを試行する。
func linkOrCopy(src, dst string) error {
	// 既に存在する場合は何もしない
	if _, err := os.Lstat(dst); err == nil {
		return nil
	}

	if err := os.Symlink(src, dst); err == nil {
		return nil
	}

	// シンボリックリンクが失敗した場合はファイルコピー
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}
