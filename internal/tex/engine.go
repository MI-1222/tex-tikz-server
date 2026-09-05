// Package tex は TeX / TikZ コンパイル処理およびテンプレート管理を提供するパッケージ。
package tex

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"time"
)

// CompileError は TeX コンパイル失敗時のエラー情報を保持するカスタムエラー型。
type CompileError struct {
	// Message はエラーの要約メッセージ。
	Message string

	// Log は LaTeX コンパイルログの末尾抜粋。
	Log string

	// Cause は元となったエラー。
	Cause error
}

// Error はエラー文字列を返却する。
func (e *CompileError) Error() string {
	return fmt.Sprintf("TeX コンパイルエラー: %s", e.Message)
}

// Unwrap は原因となったエラーを返却する。
func (e *CompileError) Unwrap() error {
	return e.Cause
}

// EngineConfig は TeX エンジンの設定オプション。
type EngineConfig struct {
	// FontDir は日本語フォントおよび fontmap.map が配置されたディレクトリパス。
	FontDir string

	// MaxConcurrency は最大同時実行コンパイル数(0 の場合は CPU 数に基づく)。
	MaxConcurrency int

	// Compiler は使用するコンパイララッパー(nil の場合はデフォルトが使用される)。
	Compiler *Compiler
}

// Engine は TikZ レンダリング処理全体を統括し、並行実行制御を行うエンジン構造体。
type Engine struct {
	tmpl     *TemplateEngine
	compiler *Compiler
	fontDir  string
	sem      chan struct{}
}

// NewEngine は新しい Engine インスタンスを生成する。
func NewEngine(cfg EngineConfig) (*Engine, error) {
	tmpl, err := NewTemplateEngine()
	if err != nil {
		return nil, fmt.Errorf("テンプレートエンジンの初期化に失敗しました: %w", err)
	}

	compiler := cfg.Compiler
	if compiler == nil {
		compiler = NewCompiler()
	}

	concurrency := cfg.MaxConcurrency
	if concurrency <= 0 {
		concurrency = runtime.NumCPU() * 2
		if concurrency < 2 {
			concurrency = 2
		}
	}

	return &Engine{
		tmpl:     tmpl,
		compiler: compiler,
		fontDir:  cfg.FontDir,
		sem:      make(chan struct{}, concurrency),
	}, nil
}

// RenderOptions は TikZ レンダリング実行時の指定オプション。
type RenderOptions struct {
	// Code は描画対象の TikZ コードブロック。
	Code string

	// Preamble は追加のプリアンブル定義。
	Preamble string

	// Timeout はリクエスト単位の実行タイムアウト。
	Timeout time.Duration
}

// RenderOutput は TikZ レンダリングの実行結果。
type RenderOutput struct {
	// SVG は生成された SVG XML 文字列。
	SVG string

	// CompileTimeMs はコンパイルおよび SVG 変換に要した時間(ミリ秒)。
	CompileTimeMs int
}

// Render は TikZ コードを uplatex + dvisvgm により SVG に変換する。
// セマフォによる同時実行制御、一時ディレクトリ管理、実行時間計測を行う。
//
// 1. 並列実行数の制御
// 2. 一時ワークスペースの作成
// 3. フォントマップおよびフォントの配置
// 4. LaTeX ソースコードの生成
// 5. main.tex の書き込み
// 6. uplatex の実行
// 7. dvisvgm の実行
// 8. ログ解析
// 9. 一時ワークスペースの削除
// 10. SVG の返却
func (e *Engine) Render(ctx context.Context, opts RenderOptions) (*RenderOutput, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// 1. 並行実行数の制御。
	select {
	case e.sem <- struct{}{}:
		defer func() {
			<-e.sem
		}()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	startTime := time.Now()

	// 2. 一時ワークスペースの作成。
	ws, err := NewWorkspace()
	if err != nil {
		return nil, fmt.Errorf("ワークスペースの作成に失敗しました: %w", err)
	}
	defer func() {
		_ = ws.Cleanup()
	}()

	// 3. フォントマップおよびフォントの配置。
	if err := ws.PrepareFontmap(e.fontDir); err != nil {
		return nil, fmt.Errorf("フォント設定の初期化に失敗しました: %w", err)
	}

	// 4. LaTeX ソースコードの生成。
	texSource, err := e.tmpl.Render(TemplateParams{
		CustomPreamble: opts.Preamble,
		TikzCode:       opts.Code,
	})
	if err != nil {
		return nil, fmt.Errorf("LaTeX ソースの生成に失敗しました: %w", err)
	}

	// 5. main.tex の書き込み。
	if err := ws.WriteFile("main.tex", []byte(texSource)); err != nil {
		return nil, fmt.Errorf("main.tex の書き込みに失敗しました: %w", err)
	}

	// 6. uplatex の実行。
	dviOutput, err := e.compiler.CompileDVI(ctx, ws)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		logRes := ExtractLogFromWorkspace(ws, dviOutput)
		return nil, &CompileError{
			Message: logRes.Summary,
			Log:     logRes.Tail,
			Cause:   err,
		}
	}

	// 7. dvisvgm の実行。
	svgData, err := e.compiler.ConvertSVG(ctx, ws)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("SVG 変換に失敗しました: %w", err)
	}

	// 8. ログ解析。
	logRes := ExtractLogFromWorkspace(ws, dviOutput)
	if len(logRes.ErrorLines) > 0 {
		return nil, &CompileError{
			Message: logRes.Summary,
			Log:     logRes.Tail,
			Cause:   errors.New("コンパイルログにエラーが検出されました。"),
		}
	}

	// 9. 一時ワークスペースの削除。
	if err := ws.Cleanup(); err != nil {
		// クリーンアップ失敗は致命的ではないため、ログ出力のみ行い続行する。
		log.Printf("ワークスペースクリーンアップ失敗: %v", err)
	}
	compileDuration := time.Since(startTime)

	// 10. SVG の返却。
	return &RenderOutput{
		SVG:           svgData,
		CompileTimeMs: int(compileDuration.Milliseconds()),
	}, nil
}
