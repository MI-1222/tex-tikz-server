package tex

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestParseLog_WithErrors はエラー行を含むログのパース処理をテストする。
func TestParseLog_WithErrors(t *testing.T) {
	sampleLog := `This is e-pTeX, Version 3.141592653-p3.8.3-u1.25
entering extended mode
(./main.tex
LaTeX2e <2020-10-01> patch level 4
! Package pgfkeys Error: I do not know the key '/tikz/invalid_key' and I am going to ignore it.
See the pgfkeys package documentation for explanation.
Type  H <return>  for immediate help.
 ...                                              
l.12 \draw[invalid_key]
                        (0,0) -- (1,1);
! Undefined control sequence.
l.13 \invalidcmd
? 
! Emergency stop.
`
	res := ParseLog(sampleLog)

	if !strings.Contains(res.Summary, "! Package pgfkeys Error") {
		t.Errorf("期待される Summary が抽出されていない: got %s", res.Summary)
	}

	if len(res.ErrorLines) != 3 {
		t.Errorf("期待されるエラー行数: 3, got %d", len(res.ErrorLines))
	}

	if !strings.Contains(res.Tail, "! Emergency stop.") {
		t.Errorf("Tail に末尾の内容が含まれていない: got %s", res.Tail)
	}
}

// TestParseLog_NoErrors は ! を含まないログのパース処理をテストする。
func TestParseLog_NoErrors(t *testing.T) {
	sampleLog := `This is e-pTeX...
Output written on main.dvi (1 page, 400 bytes).
Transcript written on main.log.`

	res := ParseLog(sampleLog)
	if res.Summary != "uplatex compilation failed." {
		t.Errorf("エラー行がない場合の Summary が異なる: got %s", res.Summary)
	}
	if !strings.Contains(res.Tail, "Transcript written on main.log.") {
		t.Errorf("Tail が正しく取得できていない: got %s", res.Tail)
	}
}

// TestParseLog_Empty は空ログの処理をテストする。
func TestParseLog_Empty(t *testing.T) {
	res := ParseLog("")
	if res.Summary != "uplatex compilation failed with no log output." {
		t.Errorf("空ログ時の Summary が異なる: got %s", res.Summary)
	}
	if res.Tail != "" {
		t.Errorf("空ログ時の Tail は空である必要があります: got %s", res.Tail)
	}
}

// TestParseLog_LongLog は長大なログの末尾切り詰めをテストする。
func TestParseLog_LongLog(t *testing.T) {
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "This is line of log output with some dummy data to increase length.")
	}
	lines = append(lines, "! Final critical error in line 101.")
	longLog := strings.Join(lines, "\n")

	res := ParseLog(longLog)
	if res.Summary != "! Final critical error in line 101." {
		t.Errorf("Summary が異なる: got %s", res.Summary)
	}
	if len(res.Tail) > MaxLogTailChars {
		t.Errorf("Tail の長さが上限を超えています: len=%d, max=%d", len(res.Tail), MaxLogTailChars)
	}
}

// TestExtractLogFromWorkspace はワークスペースからのログ抽出をテストする。
func TestExtractLogFromWorkspace(t *testing.T) {
	ws, err := NewWorkspace()
	if err != nil {
		t.Fatalf("NewWorkspace() エラー: %v", err)
	}
	defer func() {
		_ = ws.Cleanup()
	}()

	logContent := "! Error from workspace main.log file."
	if err := ws.WriteFile("main.log", []byte(logContent)); err != nil {
		t.Fatalf("WriteFile エラー: %v", err)
	}

	res := ExtractLogFromWorkspace(ws, []byte("fallback output"))
	if res.Summary != "! Error from workspace main.log file." {
		t.Errorf("main.log から正しく抽出されていません: got %s", res.Summary)
	}

	// main.log が存在しない場合のフォールバック。
	_ = ws.Cleanup()
	wsEmpty := &Workspace{Dir: filepath.Join(ws.Dir, "non-existent")}
	fallbackRes := ExtractLogFromWorkspace(wsEmpty, []byte("! Error from fallback stdout."))
	if fallbackRes.Summary != "! Error from fallback stdout." {
		t.Errorf("フォールバック出力から正しく抽出されていません: got %s", fallbackRes.Summary)
	}
}
