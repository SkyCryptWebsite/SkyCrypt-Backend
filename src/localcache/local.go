package localcache

import (
	"os"
	"runtime"
	"strconv"
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
	order      []string
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
		order:      make([]string, 0, max),
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
	_, exists := c.values[key]
	if memoryPressure() {
		c.pruneToLocked(c.maxEntries / 2)
	}
	if !exists && c.maxEntries > 0 && len(c.values) >= c.maxEntries {
		c.pruneLocked(now)
	}
	if !exists {
		c.order = append(c.order, key)
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
	c.pruneToLocked(c.maxEntries - 1)
}

func (c *LocalCache[T]) pruneToLocked(target int) {
	if target < 0 {
		target = 0
	}
	for len(c.values) > target && len(c.order) > 0 {
		key := c.order[0]
		c.order = c.order[1:]
		delete(c.values, key)
	}
	if len(c.values) <= target {
		return
	}
	for key := range c.values {
		delete(c.values, key)
		if len(c.values) <= target {
			break
		}
	}
}

func memoryPressure() bool {
	limitMB := localCacheHeapLimitMB()
	if limitMB <= 0 {
		return false
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc > uint64(limitMB)*1024*1024
}

var (
	heapLimitOnce sync.Once
	heapLimitMB   int
)

func localCacheHeapLimitMB() int {
	heapLimitOnce.Do(func() {
		heapLimitMB = 512
		if raw := os.Getenv("LOCAL_CACHE_HEAP_LIMIT_MB"); raw != "" {
			if value, err := strconv.Atoi(raw); err == nil {
				heapLimitMB = value
			}
		}
	})
	return heapLimitMB
}
