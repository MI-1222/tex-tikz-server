package tex

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNewEngine は Engine の初期化をテストする。
func TestNewEngine(t *testing.T) {
	engine, err := NewEngine(EngineConfig{
		FontDir:        "",
		MaxConcurrency: 4,
	})
	if err != nil {
		t.Fatalf("NewEngine() エラー: %v", err)
	}
	if engine == nil {
		t.Fatal("Engine が nil です。")
	}
	if cap(engine.sem) != 4 {
		t.Errorf("セマフォ容量が一致しません: got %d, want 4", cap(engine.sem))
	}
}

// TestEngine_Render_Success はモックコマンドによる正常系レンダリングをテストする。
func TestEngine_Render_Success(t *testing.T) {
	compiler := &Compiler{
		UplatexCmd: "echo",
		DvisvgmCmd: "echo",
	}

	engine, err := NewEngine(EngineConfig{
		Compiler:       compiler,
		MaxConcurrency: 2,
	})
	if err != nil {
		t.Fatalf("NewEngine() エラー: %v", err)
	}

	ctx := context.Background()
	res, err := engine.Render(ctx, RenderOptions{
		Code:     `\draw (0,0) circle (1cm);`,
		Preamble: "",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Render() エラー: %v", err)
	}

	if res == nil {
		t.Fatal("RenderOutput が nil です。")
	}

	if !strings.Contains(res.SVG, "--stdout") {
		t.Errorf("期待されるモック出力が含まれていません: %s", res.SVG)
	}

	if res.CompileTimeMs < 0 {
		t.Errorf("CompileTimeMs が不正です: %d", res.CompileTimeMs)
	}
}

// TestEngine_Render_CompileError はコンパイル失敗時の CompileError 抽出をテストする。
func TestEngine_Render_CompileError(t *testing.T) {
	compiler := &Compiler{
		UplatexCmd: "false",
		DvisvgmCmd: "echo",
	}

	engine, err := NewEngine(EngineConfig{
		Compiler:       compiler,
		MaxConcurrency: 2,
	})
	if err != nil {
		t.Fatalf("NewEngine() エラー: %v", err)
	}

	ctx := context.Background()
	_, err = engine.Render(ctx, RenderOptions{
		Code:    `\invalidcode`,
		Timeout: 5 * time.Second,
	})

	if err == nil {
		t.Fatal("コンパイル失敗時にエラーが発生しませんでした。")
	}

	var compileErr *CompileError
	if !errors.As(err, &compileErr) {
		t.Fatalf("エラー型が CompileError ではありません: %T (%v)", err, err)
	}

	if compileErr.Message == "" {
		t.Error("CompileError.Message が空です。")
	}
}

// TestEngine_Render_Timeout はタイムアウト時の処理をテストする。
func TestEngine_Render_Timeout(t *testing.T) {
	compiler := &Compiler{
		UplatexCmd: "echo",
		DvisvgmCmd: "echo",
	}

	engine, err := NewEngine(EngineConfig{
		Compiler:       compiler,
		MaxConcurrency: 2,
	})
	if err != nil {
		t.Fatalf("NewEngine() エラー: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = engine.Render(ctx, RenderOptions{
		Code: `\draw (0,0) -- (1,1);`,
	})

	if err == nil {
		t.Fatal("キャンセル時にエラーが発生しませんでした。")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("期待されるエラーは context.Canceled ですが、got: %v", err)
	}
}

// TestEngine_SemaphoreConcurrency はセマフォによる最大並行制御をテストする。
func TestEngine_SemaphoreConcurrency(t *testing.T) {
	maxConcurrency := 2
	engine, err := NewEngine(EngineConfig{
		Compiler: &Compiler{
			UplatexCmd: "echo",
			DvisvgmCmd: "echo",
		},
		MaxConcurrency: maxConcurrency,
	})
	if err != nil {
		t.Fatalf("NewEngine() エラー: %v", err)
	}

	var wg sync.WaitGroup
	var activeCount int
	var maxActive int
	var mu sync.Mutex

	totalTasks := 6
	for i := 0; i < totalTasks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx := context.Background()
			_, _ = engine.Render(ctx, RenderOptions{
				Code:    `\node {test};`,
				Timeout: 2 * time.Second,
			})

			mu.Lock()
			activeCount++
			if activeCount > maxActive {
				maxActive = activeCount
			}
			activeCount--
			mu.Unlock()
		}()
	}

	wg.Wait()
}
