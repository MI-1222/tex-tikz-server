// Package cache_test は internal/cache パッケージの単体テストを提供する。
package cache_test

import (
	"fmt"
	"sync"
	"testing"

	"tex-tikz-server/internal/cache"
)

// TestComputeKey は SHA-256 キー生成関数の一意性と再現性を検証する。
func TestComputeKey(t *testing.T) {
	// 同一入力に対する決定論的なハッシュ生成検証
	key1 := cache.ComputeKey("\\draw (0,0) circle (1cm);", "\\usepackage{tikz}", "svg")
	key2 := cache.ComputeKey("\\draw (0,0) circle (1cm);", "\\usepackage{tikz}", "svg")
	if key1 != key2 {
		t.Errorf("同一の入力に対して異なるハッシュが生成されました: key1=%s, key2=%s", key1, key2)
	}

	// 64文字の16進数文字列であることの検証
	if len(key1) != 64 {
		t.Errorf("ハッシュキーの長さが不正です: 期待値=64, 実際=%d", len(key1))
	}

	// 異なるパラメータ境界でのハッシュ衝突防止検証
	keyA := cache.ComputeKey("ab", "c", "svg")
	keyB := cache.ComputeKey("a", "bc", "svg")
	if keyA == keyB {
		t.Errorf("パラメータ境界が異なる入力でハッシュ衝突が発生しました: keyA=%s, keyB=%s", keyA, keyB)
	}

	// 空文字列パラメータの検証
	keyEmpty := cache.ComputeKey("", "", "")
	if len(keyEmpty) != 64 {
		t.Errorf("空文字列に対するハッシュキー生成に失敗しました: %s", keyEmpty)
	}
}

// TestLRUCache_Basic は基本的な Get, Set, Delete, Clear 操作を検証する。
func TestLRUCache_Basic(t *testing.T) {
	c := cache.New(3)

	if c.Capacity() != 3 {
		t.Errorf("Capacity() が不正です: 期待値=3, 実際=%d", c.Capacity())
	}
	if c.Len() != 0 {
		t.Errorf("初期状態の Len() が不正です: 期待値=0, 実際=%d", c.Len())
	}

	// 未存在キーの取得 (Miss)
	val, ok := c.Get("nonexistent")
	if ok || val != "" {
		t.Errorf("存在しないキーで値が取得されました: val=%s, ok=%v", val, ok)
	}

	// Set & Get (Hit)
	c.Set("k1", "v1")
	c.Set("k2", "v2")
	if c.Len() != 2 {
		t.Errorf("要素追加後の Len() が不正です: 期待値=2, 実際=%d", c.Len())
	}

	val, ok = c.Get("k1")
	if !ok || val != "v1" {
		t.Errorf("k1 の取得結果が不正です: val=%s, ok=%v", val, ok)
	}

	// 既存キーの更新
	c.Set("k1", "v1_updated")
	val, ok = c.Get("k1")
	if !ok || val != "v1_updated" {
		t.Errorf("k1 更新後の値が不正です: val=%s, ok=%v", val, ok)
	}
	if c.Len() != 2 {
		t.Errorf("既存キー更新後の Len() が増加しています: 実際=%d", c.Len())
	}

	// Delete
	if !c.Delete("k1") {
		t.Errorf("存在するキー k1 の削除に失敗しました")
	}
	if c.Delete("k1") {
		t.Errorf("存在しないキー k1 の削除が成功しました")
	}
	if c.Len() != 1 {
		t.Errorf("削除後の Len() が不正です: 期待値=1, 実際=%d", c.Len())
	}

	// Clear
	c.Set("k3", "v3")
	c.Clear()
	if c.Len() != 0 {
		t.Errorf("Clear後の Len() が不正です: 期待値=0, 実際=%d", c.Len())
	}
	if _, ok := c.Get("k2"); ok {
		t.Errorf("Clear後にもかかわらずエントリが取得されました")
	}
}

// TestLRUCache_Eviction は容量超過時の最古アイテム破棄 (Eviction) を検証する。
func TestLRUCache_Eviction(t *testing.T) {
	c := cache.New(2)

	c.Set("k1", "v1")
	c.Set("k2", "v2")

	// 容量上限(2)に達した状態で k3 を追加 -> 最古の k1 が破棄されるべき
	c.Set("k3", "v3")

	if _, ok := c.Get("k1"); ok {
		t.Errorf("最古の k1 が Eviction されていません")
	}
	if val, ok := c.Get("k2"); !ok || val != "v2" {
		t.Errorf("k2 が保持されていません: val=%s, ok=%v", val, ok)
	}
	if val, ok := c.Get("k3"); !ok || val != "v3" {
		t.Errorf("k3 が保持されていません: val=%s, ok=%v", val, ok)
	}

	// k2 にアクセス (Get) して最新順にする
	c.Get("k2")

	// k4 を追加 -> 次に破棄されるのは k3 であるべき
	c.Set("k4", "v4")

	if _, ok := c.Get("k3"); ok {
		t.Errorf("k3 が Eviction されていません")
	}
	if _, ok := c.Get("k2"); !ok {
		t.Errorf("アクセスされた k2 が誤って Eviction されました")
	}
	if _, ok := c.Get("k4"); !ok {
		t.Errorf("新規追加の k4 が取得できません")
	}
}

// TestLRUCache_Stats はキャッシュ統計情報（Hits, Misses, Evictions）の正確性を検証する。
func TestLRUCache_Stats(t *testing.T) {
	c := cache.New(2)

	// Miss 2回
	c.Get("k1")
	c.Get("k2")

	// Hit 1回, Set 2回
	c.Set("k1", "v1")
	c.Get("k1") // Hit 1
	c.Set("k2", "v2")

	// Eviction 1回
	c.Set("k3", "v3") // k1 が Eviction

	stats := c.Stats()
	if stats.Hits != 1 {
		t.Errorf("Hits が不正です: 期待値=1, 実際=%d", stats.Hits)
	}
	if stats.Misses != 2 {
		t.Errorf("Misses が不正です: 期待値=2, 実際=%d", stats.Misses)
	}
	if stats.Evictions != 1 {
		t.Errorf("Evictions が不正です: 期待値=1, 実際=%d", stats.Evictions)
	}
	if stats.Capacity != 2 {
		t.Errorf("Capacity が不正です: 期待値=2, 実際=%d", stats.Capacity)
	}
	if stats.Items != 2 {
		t.Errorf("Items が不正です: 期待値=2, 実際=%d", stats.Items)
	}
}

// TestLRUCache_DefaultCapacity は capacity が 0 以下のときの挙動を検証する。
func TestLRUCache_DefaultCapacity(t *testing.T) {
	c := cache.New(0)
	if c.Capacity() != cache.DefaultCapacity {
		t.Errorf("DefaultCapacity が適用されていません: 実際=%d", c.Capacity())
	}

	cNegative := cache.New(-10)
	if cNegative.Capacity() != cache.DefaultCapacity {
		t.Errorf("DefaultCapacity が適用されていません: 実際=%d", cNegative.Capacity())
	}
}

// TestLRUCache_Concurrency は並行アクセス時のデータ競合とスレッド安全性を検証する。
func TestLRUCache_Concurrency(t *testing.T) {
	const (
		concurrency = 30
		iterations  = 200
		cacheCap    = 50
	)

	c := cache.New(cacheCap)
	var wg sync.WaitGroup

	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("key_%d", (workerID*10+i)%80)
				val := fmt.Sprintf("val_%d_%d", workerID, i)

				// Set 操作
				c.Set(key, val)

				// Get 操作
				c.Get(key)

				// Stats 取得
				_ = c.Stats()

				// 一部キーの削除
				if i%20 == 0 {
					c.Delete(key)
				}
			}
		}(worker)
	}

	wg.Wait()

	if c.Len() > cacheCap {
		t.Errorf("キャッシュ保持件数が最大容量を超過しています: 容量=%d, 実際=%d", cacheCap, c.Len())
	}

	stats := c.Stats()
	if stats.Hits+stats.Misses == 0 {
		t.Errorf("Stats の合計アクセス数が 0 です")
	}
}
