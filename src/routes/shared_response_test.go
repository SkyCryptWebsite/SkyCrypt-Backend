package routes

import (
	"context"
	"errors"
	"fmt"
	"skycrypt/src/api"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSharedResponseBuildsOnceForConcurrentCallers(t *testing.T) {
	t.Setenv("DEV", "true") // keep Redis out of it
	key := responseCacheKey("test", "shared")
	release := make(chan struct{})
	var builds atomic.Int32

	callers := make([]context.Context, 5)
	bodies := make([][]byte, 5)
	var wg sync.WaitGroup
	for i := range callers {
		callers[i] = api.WithRequestState(context.Background(), &api.RequestState{})
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body, err := sharedResponse(callers[i], key, key, 60, func(ctx context.Context) ([]byte, error) {
				builds.Add(1)
				<-release
				return []byte(`{"ok":true}`), nil
			})
			if err != nil {
				t.Error(err)
			}
			bodies[i] = body
		}(i)
	}
	time.Sleep(50 * time.Millisecond) // let every caller join the build
	close(release)
	wg.Wait()

	if builds.Load() != 1 {
		t.Fatalf("expected one shared build, got %d", builds.Load())
	}
	for i, body := range bodies {
		if string(body) != `{"ok":true}` {
			t.Fatalf("caller %d got %q", i, body)
		}
	}
}

func TestSharedResponseKeepsBuildErrorMessage(t *testing.T) {
	t.Setenv("DEV", "true")
	_, err := sharedResponse(context.Background(), responseCacheKey("test", "error"), responseCacheKey("test", "error"), 60, func(ctx context.Context) ([]byte, error) {
		return nil, failedTo("Failed to fetch data", api.ErrProfileNotFound)
	})
	var built buildError
	if !errors.As(err, &built) || built.message != "Failed to fetch data" || !errors.Is(err, api.ErrProfileNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestFailedToKeepsInnerBuildErrorMessage(t *testing.T) {
	inner := failedTo("Failed to resolve player", api.ErrPlayerNotFound)
	err := failedTo("failed to load data", fmt.Errorf("failed to load profile: %w", inner))
	var built buildError
	if !errors.As(err, &built) || built.message != "Failed to resolve player" || !errors.Is(err, api.ErrPlayerNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestResponseCacheKeysIgnoreCase(t *testing.T) {
	if responseCacheKey("stats", "DarthGigi", "Apple") != responseCacheKey("stats", "darthgigi", "apple") {
		t.Fatal("response cache key should ignore case")
	}
	if inventoryCacheKey("ABCDEF", "Apple", nil) != inventoryCacheKey("abcdef", "apple", nil) {
		t.Fatal("inventory cache key should ignore case")
	}
}
