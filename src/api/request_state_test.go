package api

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithFallbackCacheOnlyNeverFetches(t *testing.T) {
	ctx := WithRequestState(context.Background(), &RequestState{CacheOnly: true})

	_, fresh, err := withFallback(ctx, "profiles:uncached", func() ([]byte, error) {
		t.Fatal("cache-only requests must not reach Hypixel")
		return nil, nil
	}, func(context.Context) ([]byte, bool) { return nil, false })
	if !errors.Is(err, ErrNotCached) || fresh {
		t.Fatalf("got fresh=%v err=%v, want ErrNotCached", fresh, err)
	}
	if !MissedCache(ctx) {
		t.Fatal("the miss must be recorded so the middleware can answer NOT_CACHED")
	}
}

func TestWithFallbackFetchesForFullAccess(t *testing.T) {
	ctx := WithRequestState(context.Background(), &RequestState{})

	body, fresh, err := withFallback(ctx, "profiles:x", func() ([]byte, error) { return []byte(`{}`), nil },
		func(context.Context) ([]byte, bool) {
			t.Fatal("a successful fetch must not read the store")
			return nil, false
		})
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

func TestSharedFetchCopiesFlagsToEveryCaller(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var fetches atomic.Int32
	fetch := func(fetchCtx context.Context) (string, error) {
		fetches.Add(1)
		close(started)
		<-release
		requestState(fetchCtx).stale.Store(true)
		return "body", nil
	}

	leader := WithRequestState(context.Background(), &RequestState{})
	follower := WithRequestState(context.Background(), &RequestState{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := sharedFetch(leader, "test:shared", fetch); err != nil {
			t.Error(err)
		}
	}()
	<-started

	followerDone := make(chan string)
	go func() {
		body, _ := sharedFetch(follower, "test:shared", fetch)
		followerDone <- body
	}()
	time.Sleep(50 * time.Millisecond) // let the follower join the in-flight call
	close(release)
	<-done

	if body := <-followerDone; body != "body" {
		t.Fatalf("follower got %q", body)
	}
	if fetches.Load() != 1 {
		t.Fatalf("expected one shared fetch, got %d", fetches.Load())
	}
	if !ServedStale(leader) || !ServedStale(follower) {
		t.Fatal("stale data must be flagged for the follower too, or its response gets cached as fresh")
	}
}
