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
