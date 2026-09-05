// Package cache は TikZ レンダリング結果のインメモリキャッシュを提供するパッケージ。
package cache

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// DefaultCapacity はデフォルトのキャッシュ最大保持件数。
const DefaultCapacity = 2000

// ComputeKey は TikZ コード、プリアンブル、フォーマットから一意な SHA-256 ハッシュ文字列を生成する。
// 各パラメータの境界を明確にし衝突を防止するため、Null バイト区切りでハッシュ計算を行う。
func ComputeKey(code, preamble, format string) string {
	hasher := sha256.New()
	hasher.Write([]byte(code))
	hasher.Write([]byte{0x00})
	hasher.Write([]byte(preamble))
	hasher.Write([]byte{0x00})
	hasher.Write([]byte(format))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Stats はキャッシュの利用統計情報。
type Stats struct {
	// Hits はキャッシュヒット回数。
	Hits uint64 `json:"hits"`

	// Misses はキャッシュミス回数。
	Misses uint64 `json:"misses"`

	// Evictions は容量超過による破棄回数。
	Evictions uint64 `json:"evictions"`

	// Capacity はキャッシュの最大容量。
	Capacity int `json:"capacity"`

	// Items は現在保持されているエントリ数。
	Items int `json:"items"`
}

// entry は LRU リストおよびマップに格納されるキーと値のペア。
type entry struct {
	key   string
	value string
}

// LRUCache はスレッドセーフなインメモリ LRU (Least Recently Used) キャッシュ構造体。
type LRUCache struct {
	mu        sync.Mutex
	capacity  int
	items     map[string]*list.Element
	evictList *list.List
	stats     Stats
}

// New は指定された容量を持つ新しい LRUCache インスタンスを生成する。
// capacity が 0 以下の場合は DefaultCapacity が設定される。
func New(capacity int) *LRUCache {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}

	return &LRUCache{
		capacity:  capacity,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
		stats: Stats{
			Capacity: capacity,
		},
	}
}

// Get は指定されたキーに対応するキャッシュ値を取得する。
// キーが存在する場合はアクセス順を最新(リスト先頭)に更新し、value と true を返却する。
// 存在しない場合は空文字と false を返却する。
func (c *LRUCache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.evictList.MoveToFront(elem)
		c.stats.Hits++
		return elem.Value.(*entry).value, true
	}

	c.stats.Misses++
	return "", false
}

// Set は指定されたキーと値のペアをキャッシュに保存または更新する。
// キーが既に存在する場合は値を更新して最新順に移動する。
// 新規キーの追加時に容量制限を超過した場合は、最も長期間参照されていない最古のエントリを破棄する。
func (c *LRUCache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 既存エントリの更新
	if elem, ok := c.items[key]; ok {
		c.evictList.MoveToFront(elem)
		elem.Value.(*entry).value = value
		return
	}

	// 新規エントリの追加
	ent := &entry{key: key, value: value}
	elem := c.evictList.PushFront(ent)
	c.items[key] = elem
	c.stats.Items = len(c.items)

	// 容量超過時の最古アイテム破棄
	if c.evictList.Len() > c.capacity {
		c.removeOldest()
	}
}

// removeOldest は最も古いエントリをリストおよびマップから破棄する内部メソッド。
// 呼び出し元でロックが取得されている必要がある。
func (c *LRUCache) removeOldest() {
	elem := c.evictList.Back()
	if elem == nil {
		return
	}

	c.evictList.Remove(elem)
	ent := elem.Value.(*entry)
	delete(c.items, ent.key)
	c.stats.Evictions++
	c.stats.Items = len(c.items)
}

// Delete は指定されたキーのエントリをキャッシュから削除する。
// エントリが存在して削除された場合は true、存在しなかった場合は false を返却する。
func (c *LRUCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.evictList.Remove(elem)
		delete(c.items, key)
		c.stats.Items = len(c.items)
		return true
	}

	return false
}

// Clear はキャッシュ内のすべてのエントリを破棄する。
// 統計情報の Hits, Misses, Evictions は維持され、Items のみが 0 に更新される。
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictList.Init()
	c.stats.Items = 0
}

// Len は現在キャッシュに保持されているエントリ数を返却する。
func (c *LRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// Capacity はキャッシュの最大保持件数を返却する。
func (c *LRUCache) Capacity() int {
	return c.capacity
}

// Stats は現在のキャッシュ統計情報のスナップショットを返却する。
func (c *LRUCache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}
