package api

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

// ErrNotCached is returned to callers that may only read cached data when nothing is cached for the request.
var ErrNotCached = errors.New("data isn't cached and the caller may not fetch it from Hypixel")

// RequestState carries per-request caching rules from the auth middleware down to the upstream fetchers.
type RequestState struct {
	// CacheOnly stops requests to Hypixel: Hypixel's policy forbids proxying it for third parties.
	CacheOnly bool
	stale     atomic.Bool
	notCached atomic.Bool
}

type requestStateKey struct{}

func WithRequestState(ctx context.Context, state *RequestState) context.Context {
	return context.WithValue(ctx, requestStateKey{}, state)
}

func requestState(ctx context.Context) *RequestState {
	state, _ := ctx.Value(requestStateKey{}).(*RequestState)
	return state
}

// ServedStale reports whether the request used fallback data, which must not be cached as fresh.
func ServedStale(ctx context.Context) bool {
	state := requestState(ctx)
	return state != nil && state.stale.Load()
}

// SkipCaching keeps a response that was built with a part missing out of the caches, like stale fallback data.
func SkipCaching(ctx context.Context) {
	if state := requestState(ctx); state != nil {
		state.stale.Store(true)
	}
}

// MissedCache reports whether a cache-only request needed data that wasn't cached.
func MissedCache(ctx context.Context) bool {
	state := requestState(ctx)
	return state != nil && state.notCached.Load()
}

func isCacheOnly(ctx context.Context) bool {
	state := requestState(ctx)
	return state != nil && state.CacheOnly
}

type flightResult[T any] struct {
	value     T
	stale     bool
	notCached bool
}

// Share runs fn once per key across concurrent callers in group. The shared call gets its own RequestState and a
// context no single caller can cancel, bounded by timeout; its stale and notCached flags are copied onto every
// caller's state, not just the caller that happened to start it.
func Share[T any](ctx context.Context, group *singleflight.Group, key string, timeout time.Duration, fn func(ctx context.Context) (T, error)) (T, error) {
	result, err, _ := group.Do(flightKey(ctx, key), func() (interface{}, error) {
		flightState := &RequestState{CacheOnly: isCacheOnly(ctx)}
		flightCtx, cancel := context.WithTimeout(context.WithoutCancel(WithRequestState(ctx, flightState)), timeout)
		defer cancel()

		value, err := fn(flightCtx)
		return flightResult[T]{value: value, stale: flightState.stale.Load(), notCached: flightState.notCached.Load()}, err
	})

	flight, _ := result.(flightResult[T])
	if state := requestState(ctx); state != nil {
		if flight.stale {
			state.stale.Store(true)
		}
		if flight.notCached {
			state.notCached.Store(true)
		}
	}
	if err != nil {
		var zero T
		return zero, err
	}
	return flight.value, nil
}

// sharedFetch runs an upstream fetch once per key across concurrent callers.
func sharedFetch[T any](ctx context.Context, key string, fetch func(fetchCtx context.Context) (T, error)) (T, error) {
	return Share(ctx, &hypixelFetchGroup, key, hypixelRequestTimeout, fetch)
}

// flightKey keeps cache-only and fetching callers out of each other's singleflight calls.
func flightKey(ctx context.Context, key string) string {
	if isCacheOnly(ctx) {
		return key + ":cache-only"
	}
	return key
}
