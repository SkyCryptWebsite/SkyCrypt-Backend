package localcache

import (
	"sync"
	"time"
)

type entry[T any] struct {
	value        T
	expiresAt    time.Time
	refreshAfter time.Time
}

type LocalCache[T any] struct {
	mu         sync.RWMutex
	values     map[string]entry[T]
	refreshing map[string]struct{}
	maxEntries int
}

func NewLocalCache[T any](maxEntries ...int) *LocalCache[T] {
	max := 1024
	if len(maxEntries) > 0 && maxEntries[0] > 0 {
		max = maxEntries[0]
	}
	return &LocalCache[T]{
		values:     make(map[string]entry[T]),
		refreshing: make(map[string]struct{}),
		maxEntries: max,
	}
}

func (c *LocalCache[T]) Get(key string) (T, bool, bool) {
	now := time.Now()

	c.mu.RLock()
	item, ok := c.values[key]
	c.mu.RUnlock()

	var zero T
	if !ok {
		return zero, false, false
	}
	if now.After(item.expiresAt) {
		c.mu.Lock()
		if latest, exists := c.values[key]; exists && now.After(latest.expiresAt) {
			delete(c.values, key)
		}
		c.mu.Unlock()
		return zero, false, false
	}

	return item.value, true, now.After(item.refreshAfter)
}

func (c *LocalCache[T]) Set(key string, value T, ttl time.Duration, refreshAfter time.Duration) {
	now := time.Now()
	if refreshAfter <= 0 || refreshAfter > ttl {
		refreshAfter = ttl
	}

	c.mu.Lock()
	if _, exists := c.values[key]; !exists && c.maxEntries > 0 && len(c.values) >= c.maxEntries {
		c.pruneLocked(now)
	}
	c.values[key] = entry[T]{
		value:        value,
		expiresAt:    now.Add(ttl),
		refreshAfter: now.Add(refreshAfter),
	}
	c.mu.Unlock()
}

func (c *LocalCache[T]) StartRefresh(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.refreshing[key]; ok {
		return false
	}
	c.refreshing[key] = struct{}{}
	return true
}

func (c *LocalCache[T]) FinishRefresh(key string) {
	c.mu.Lock()
	delete(c.refreshing, key)
	c.mu.Unlock()
}

func (c *LocalCache[T]) pruneLocked(now time.Time) {
	for key, item := range c.values {
		if now.After(item.expiresAt) {
			delete(c.values, key)
		}
	}
	for len(c.values) >= c.maxEntries {
		for key := range c.values {
			delete(c.values, key)
			break
		}
	}
}
