// Package tex は TeX / TikZ コンパイル処理およびテンプレート管理を提供するパッケージ。
package tex

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// MaxLogTailChars はクライアントに返却するログ抜粋の最大文字数。
const MaxLogTailChars = 2000

// MaxLogTailLines はログ末尾から取得する最大行数。
const MaxLogTailLines = 30

// ParseResult は TeX ログの解析結果。
type ParseResult struct {
	// Summary は主要なエラーメッセージの要約(! で始まる行など)。
	Summary string

	// Tail はクライアント返却用のログ末尾抜粋。
	Tail string

	// ErrorLines は検出された全エラー行のリスト。
	ErrorLines []string
}

// ParseLog は TeX コンパイルログ文字列を解析し、エラー要約と末尾抜粋を抽出する。
func ParseLog(logContent string) ParseResult {
	if strings.TrimSpace(logContent) == "" {
		return ParseResult{
			Summary:    "uplatex compilation failed with no log output.",
			Tail:       "",
			ErrorLines: nil,
		}
	}

	var errorLines []string
	var allLines []string

	scanner := bufio.NewScanner(strings.NewReader(logContent))
	for scanner.Scan() {
		line := scanner.Text()
		allLines = append(allLines, line)

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "!") {
			errorLines = append(errorLines, trimmed)
		}
	}

	// エラー要約の選定
	var summary string
	if len(errorLines) > 0 {
		summary = errorLines[0]
	} else {
		summary = "uplatex compilation failed."
	}

	// ログ末尾の抽出(最大行数制限)
	startLine := 0
	if len(allLines) > MaxLogTailLines {
		startLine = len(allLines) - MaxLogTailLines
	}
	tail := strings.Join(allLines[startLine:], "\n")

	// 文字数制限の適用
	if len(tail) > MaxLogTailChars {
		tail = tail[len(tail)-MaxLogTailChars:]
	}

	return ParseResult{
		Summary:    summary,
		Tail:       tail,
		ErrorLines: errorLines,
	}
}

// ExtractLogFromWorkspace はワークスペースの一時ディレクトリから
// main.log を読み取って解析する。
// main.log が存在しない場合は fallbackOutput を用いて解析を行う。
func ExtractLogFromWorkspace(ws *Workspace, fallbackOutput []byte) ParseResult {
	if ws != nil && ws.Dir != "" {
		logPath := filepath.Join(ws.Dir, "main.log")
		if data, err := os.ReadFile(logPath); err == nil && len(data) > 0 {
			return ParseLog(string(data))
		}
	}

	return ParseLog(string(fallbackOutput))
}
