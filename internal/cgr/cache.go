package cgr

import (
	"container/list"
	"sync"
)

// cacheMax bounds each cache size: distinct entries in a single process
// lifetime. On overflow the LEAST recently used entry is evicted.
const cacheMax = 100

// cacheMaxLight is used for light families (contable, instructivos, consolidados, cuentas, dictamenes).
const cacheMaxLight = 100

// cacheMaxHeavy is used for heavy families (auditoria, legislacion) to avoid
// filling the LRU with large payloads.
const cacheMaxHeavy = 20

// lruCache is an in-memory LRU cache keyed by a caller-defined string.
// Safe for concurrent use. Duplicated from bcn etagCache but without ETag
// — CGR does not send ETag/304.
type lruCache[T any] struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	order   *list.List // front = most recently used
	max     int
}

type cacheItem[T any] struct {
	key   string
	value T
}

func newLRUCache[T any]() *lruCache[T] {
	return newLRUCacheWithMax[T](cacheMax)
}

func newLRUCacheWithMax[T any](max int) *lruCache[T] {
	if max <= 0 {
		max = cacheMax
	}
	return &lruCache[T]{
		entries: make(map[string]*list.Element),
		order:   list.New(),
		max:     max,
	}
}

func (c *lruCache[T]) get(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[key]
	if !ok {
		var zero T
		return zero, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*cacheItem[T]).value, true
}

func (c *lruCache[T]) put(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, exists := c.entries[key]; exists {
		el.Value.(*cacheItem[T]).value = value
		c.order.MoveToFront(el)
		return
	}
	if len(c.entries) >= c.max {
		back := c.order.Back()
		if back != nil {
			delete(c.entries, back.Value.(*cacheItem[T]).key)
			c.order.Remove(back)
		}
	}
	c.entries[key] = c.order.PushFront(&cacheItem[T]{key: key, value: value})
}
