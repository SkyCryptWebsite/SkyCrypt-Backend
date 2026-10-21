package api

import (
	"context"
	"errors"
	"testing"
)

func TestWithFallbackCacheOnlyNeverFetches(t *testing.T) {
	ctx := WithRequestState(context.Background(), &RequestState{CacheOnly: true})

	_, fresh, err := withFallback(ctx, "profiles:uncached", func() ([]byte, error) {
		t.Fatal("cache-only requests must not reach Hypixel")
		return nil, nil
	})
	if !errors.Is(err, ErrNotCached) || fresh {
		t.Fatalf("got fresh=%v err=%v, want ErrNotCached", fresh, err)
	}
	if !MissedCache(ctx) {
		t.Fatal("the miss must be recorded so the middleware can answer NOT_CACHED")
	}
}

func TestWithFallbackFetchesForFullAccess(t *testing.T) {
	ctx := WithRequestState(context.Background(), &RequestState{})

	body, fresh, err := withFallback(ctx, "profiles:x", func() ([]byte, error) { return []byte(`{}`), nil })
	if err != nil || !fresh || string(body) != `{}` {
		t.Fatalf("got body=%q fresh=%v err=%v", body, fresh, err)
	}
	if ServedStale(ctx) || MissedCache(ctx) {
		t.Fatal("a fresh fetch is neither stale nor a cache miss")
	}
	if flightKey(ctx, "profiles:x") == flightKey(WithRequestState(ctx, &RequestState{CacheOnly: true}), "profiles:x") {
		t.Fatal("cache-only and fetching callers must not share a singleflight call")
	}
}
