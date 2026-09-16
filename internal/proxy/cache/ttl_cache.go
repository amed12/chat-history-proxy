// Package cache provides a tiny in-memory TTL cache with no external
// dependency — Option A (docs/CHAT_HISTORY_PROXY.md) is
// database-free by design, and a cache this small doesn't need a library.
package cache

import (
	"sync"
	"time"
)

// TTLCache is a generic in-memory cache where every entry expires after ttl.
// Safe for concurrent use.
type TTLCache[V any] struct {
	mu      sync.RWMutex
	entries map[string]entry[V]
	ttl     time.Duration
	now     func() time.Time
}

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// New creates a TTLCache where every Set entry expires after ttl.
func New[V any](ttl time.Duration) *TTLCache[V] {
	return &TTLCache[V]{
		entries: make(map[string]entry[V]),
		ttl:     ttl,
		now:     time.Now,
	}
}

// Get returns the cached value for key and true, or the zero value and
// false if it's missing or expired.
func (c *TTLCache[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.entries[key]
	if !ok || c.now().After(e.expiresAt) {
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set stores value under key, expiring after the cache's configured TTL.
func (c *TTLCache[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = entry[V]{value: value, expiresAt: c.now().Add(c.ttl)}
}
