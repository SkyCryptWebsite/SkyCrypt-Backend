package localcache

import (
	"testing"
	"time"
)

func TestSweepExpiredEveryDropsUnreadEntries(t *testing.T) {
	cache := NewLocalCache[[]byte](16)
	cache.Set("profile", make([]byte, 1<<20), 20*time.Millisecond, 0)
	cache.SweepExpiredEvery(10 * time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		cache.mu.RLock()
		remaining := len(cache.values)
		cache.mu.RUnlock()
		if remaining == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("an expired entry nobody read again was never dropped")
}

func TestExpiredReadsDontGrowTheOrder(t *testing.T) {
	cache := NewLocalCache[int](100)
	keys := []string{"a", "b", "c"}
	for range 200 {
		for _, key := range keys {
			cache.Set(key, 1, time.Nanosecond, 0)
		}
		time.Sleep(time.Microsecond)
		for _, key := range keys {
			if _, ok, _ := cache.Get(key); ok {
				t.Fatalf("%s should have expired", key)
			}
		}
	}
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	if cache.order.Len() != len(cache.values) || cache.order.Len() > len(keys) {
		t.Fatalf("order holds %d keys for %d values", cache.order.Len(), len(cache.values))
	}
}

func TestFullCacheEvictsTheOldest(t *testing.T) {
	cache := NewLocalCache[int](3)
	for i, key := range []string{"a", "b", "c", "d", "e"} {
		cache.Set(key, i, time.Hour, 0)
	}
	for _, key := range []string{"a", "b"} {
		if _, ok, _ := cache.Get(key); ok {
			t.Errorf("%s should have been evicted", key)
		}
	}
	for _, key := range []string{"c", "d", "e"} {
		if _, ok, _ := cache.Get(key); !ok {
			t.Errorf("%s should still be cached", key)
		}
	}
}
