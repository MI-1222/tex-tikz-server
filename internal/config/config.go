// Package config は環境変数からの設定読み込みおよびバリデーションを提供するパッケージ。
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// デフォルト設定値の定義。
const (
	DefaultPort           = 8080
	DefaultCacheSize      = 2000
	DefaultExecTimeoutSec = 10
	DefaultFontDir        = "fonts"
)

// Config はサーバー全体の動作設定を保持する構造体。
type Config struct {
	// Port は HTTP サーバーの待受ポート番号。
	Port int

	// APIKey はクライアント認証に必要なシークレットキー。
	APIKey string

	// CacheSize はインメモリ LRU キャッシュの最大保持件数。
	CacheSize int

	// ExecTimeoutSec は 1 回のコンパイルリクエストあたりの最大実行秒数。
	ExecTimeoutSec int

	// FontDir は日本語フォントおよび fontmap.map が配置されたディレクトリパス。
	FontDir string
}

// Addr は HTTP サーバーのリッスン用アドレス文字列 (例: ":8080") を返却する。
func (c *Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}

// ExecTimeout は最大コンパイルタイムアウト時間を time.Duration 型で返却する。
func (c *Config) ExecTimeout() time.Duration {
	return time.Duration(c.ExecTimeoutSec) * time.Second
}

// Load は OS の環境変数から設定を読み込み、バリデーション済みの Config インスタンスを返却する。
func Load() (*Config, error) {
	return loadWithLookup(os.LookupEnv)
}

// loadWithLookup は環境変数の取得関数を受け取り、Config を生成する内部関数。
func loadWithLookup(lookup func(string) (string, bool)) (*Config, error) {
	cfg := &Config{
		Port:           DefaultPort,
		CacheSize:      DefaultCacheSize,
		ExecTimeoutSec: DefaultExecTimeoutSec,
		FontDir:        DefaultFontDir,
	}

	// API_KEY の検証 (必須項目)
	if apiKey, ok := lookup("API_KEY"); ok && strings.TrimSpace(apiKey) != "" {
		cfg.APIKey = strings.TrimSpace(apiKey)
	} else {
		return nil, errors.New("環境変数 API_KEY は必須です。")
	}

	// PORT のパースと検証 (任意項目)
	if portStr, ok := lookup("PORT"); ok && strings.TrimSpace(portStr) != "" {
		port, err := strconv.Atoi(strings.TrimSpace(portStr))
		if err != nil {
			return nil, fmt.Errorf("環境変数 PORT のパースに失敗しました: %w", err)
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("環境変数 PORT は 1 から 65535 の範囲で指定してください: %d", port)
		}
		cfg.Port = port
	}

	// CACHE_SIZE のパースと検証 (任意項目)
	if cacheSizeStr, ok := lookup("CACHE_SIZE"); ok && strings.TrimSpace(cacheSizeStr) != "" {
		cacheSize, err := strconv.Atoi(strings.TrimSpace(cacheSizeStr))
		if err != nil {
			return nil, fmt.Errorf("環境変数 CACHE_SIZE のパースに失敗しました: %w", err)
		}
		if cacheSize <= 0 {
			return nil, fmt.Errorf("環境変数 CACHE_SIZE は 1 以上の整数を指定してください: %d", cacheSize)
		}
		cfg.CacheSize = cacheSize
	}

	// EXEC_TIMEOUT_SEC のパースと検証 (任意項目)
	if timeoutStr, ok := lookup("EXEC_TIMEOUT_SEC"); ok && strings.TrimSpace(timeoutStr) != "" {
		timeout, err := strconv.Atoi(strings.TrimSpace(timeoutStr))
		if err != nil {
			return nil, fmt.Errorf("環境変数 EXEC_TIMEOUT_SEC のパースに失敗しました: %w", err)
		}
		if timeout <= 0 {
			return nil, fmt.Errorf("環境変数 EXEC_TIMEOUT_SEC は 1 以上の整数を指定してください: %d", timeout)
		}
		cfg.ExecTimeoutSec = timeout
	}

	// FONT_DIR のパース (任意項目)
	if fontDir, ok := lookup("FONT_DIR"); ok && strings.TrimSpace(fontDir) != "" {
		cfg.FontDir = strings.TrimSpace(fontDir)
	}

	return cfg, nil
}
