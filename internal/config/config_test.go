// Package config_test は internal/config パッケージの単体テストを提供する。
package config_test

import (
	"testing"
	"time"

	"tex-tikz-server/internal/config"
)

// TestLoad_Defaults は必須環境変数 API_KEY のみ与えた場合にデフォルト値が設定されることを検証する。
func TestLoad_Defaults(t *testing.T) {
	t.Setenv("API_KEY", "secret-test-key")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() に失敗しました: %v", err)
	}

	if cfg.APIKey != "secret-test-key" {
		t.Errorf("APIKey が不正です: %s", cfg.APIKey)
	}
	if cfg.Port != config.DefaultPort {
		t.Errorf("Port デフォルト値が不正です: 期待値=%d, 実際=%d", config.DefaultPort, cfg.Port)
	}
	if cfg.CacheSize != config.DefaultCacheSize {
		t.Errorf("CacheSize デフォルト値が不正です: 期待値=%d, 実際=%d", config.DefaultCacheSize, cfg.CacheSize)
	}
	if cfg.ExecTimeoutSec != config.DefaultExecTimeoutSec {
		t.Errorf("ExecTimeoutSec デフォルト値が不正です: 期待値=%d, 実際=%d", config.DefaultExecTimeoutSec, cfg.ExecTimeoutSec)
	}
	if cfg.FontDir != config.DefaultFontDir {
		t.Errorf("FontDir デフォルト値が不正です: 期待値=%s, 実際=%s", config.DefaultFontDir, cfg.FontDir)
	}
}

// TestLoad_CustomValues は全環境変数をカスタム値で指定した場合のパース結果を検証する。
func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("API_KEY", "custom-api-key")
	t.Setenv("PORT", "3000")
	t.Setenv("CACHE_SIZE", "500")
	t.Setenv("EXEC_TIMEOUT_SEC", "15")
	t.Setenv("FONT_DIR", "/usr/share/fonts/opentype")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() に失敗しました: %v", err)
	}

	if cfg.APIKey != "custom-api-key" {
		t.Errorf("APIKey が不正です: %s", cfg.APIKey)
	}
	if cfg.Port != 3000 {
		t.Errorf("Port が不正です: 期待値=3000, 実際=%d", cfg.Port)
	}
	if cfg.CacheSize != 500 {
		t.Errorf("CacheSize が不正です: 期待値=500, 実際=%d", cfg.CacheSize)
	}
	if cfg.ExecTimeoutSec != 15 {
		t.Errorf("ExecTimeoutSec が不正です: 期待値=15, 実際=%d", cfg.ExecTimeoutSec)
	}
	if cfg.FontDir != "/usr/share/fonts/opentype" {
		t.Errorf("FontDir が不正です: %s", cfg.FontDir)
	}
}

// TestLoad_MissingAPIKey は API_KEY が未設定または空白の場合のエラーを検証する。
func TestLoad_MissingAPIKey(t *testing.T) {
	testCases := []struct {
		name   string
		apiKey string
		set    bool
	}{
		{name: "未設定", apiKey: "", set: false},
		{name: "空文字列", apiKey: "", set: true},
		{name: "空白のみ", apiKey: "   ", set: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv("API_KEY", tc.apiKey)
			}

			cfg, err := config.Load()
			if err == nil {
				t.Fatalf("API_KEY 未設定時にエラーが返却されませんでした: cfg=%+v", cfg)
			}
		})
	}
}

// TestLoad_InvalidPort は不正な PORT 値指定時のバリデーションエラーを検証する。
func TestLoad_InvalidPort(t *testing.T) {
	testCases := []struct {
		name    string
		portVal string
	}{
		{name: "文字列", portVal: "invalid_port"},
		{name: "ゼロ", portVal: "0"},
		{name: "負数", portVal: "-80"},
		{name: "上限超過", portVal: "65536"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("API_KEY", "valid-key")
			t.Setenv("PORT", tc.portVal)

			cfg, err := config.Load()
			if err == nil {
				t.Fatalf("不正な PORT (%s) でエラーが返却されませんでした: cfg=%+v", tc.portVal, cfg)
			}
		})
	}
}

// TestLoad_InvalidCacheSize は不正な CACHE_SIZE 値指定時のバリデーションエラーを検証する。
func TestLoad_InvalidCacheSize(t *testing.T) {
	testCases := []struct {
		name     string
		cacheVal string
	}{
		{name: "文字列", cacheVal: "abc"},
		{name: "ゼロ", cacheVal: "0"},
		{name: "負数", cacheVal: "-10"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("API_KEY", "valid-key")
			t.Setenv("CACHE_SIZE", tc.cacheVal)

			cfg, err := config.Load()
			if err == nil {
				t.Fatalf("不正な CACHE_SIZE (%s) でエラーが返却されませんでした: cfg=%+v", tc.cacheVal, cfg)
			}
		})
	}
}

// TestLoad_InvalidTimeout は不正な EXEC_TIMEOUT_SEC 値指定時のバリデーションエラーを検証する。
func TestLoad_InvalidTimeout(t *testing.T) {
	testCases := []struct {
		name       string
		timeoutVal string
	}{
		{name: "文字列", timeoutVal: "fast"},
		{name: "ゼロ", timeoutVal: "0"},
		{name: "負数", timeoutVal: "-5"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("API_KEY", "valid-key")
			t.Setenv("EXEC_TIMEOUT_SEC", tc.timeoutVal)

			cfg, err := config.Load()
			if err == nil {
				t.Fatalf("不正な EXEC_TIMEOUT_SEC (%s) でエラーが返却されませんでした: cfg=%+v", tc.timeoutVal, cfg)
			}
		})
	}
}

// TestConfig_Helpers は Addr() および ExecTimeout() ヘルパーメソッドの動作を検証する。
func TestConfig_Helpers(t *testing.T) {
	cfg := &config.Config{
		Port:           9000,
		ExecTimeoutSec: 20,
	}

	if cfg.Addr() != ":9000" {
		t.Errorf("Addr() が不正です: 期待値=:9000, 実際=%s", cfg.Addr())
	}
	if cfg.ExecTimeout() != 20*time.Second {
		t.Errorf("ExecTimeout() が不正です: 期待値=20s, 実際=%v", cfg.ExecTimeout())
	}
}
