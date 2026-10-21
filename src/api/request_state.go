package api

import (
	"context"
	"errors"
	"sync/atomic"
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

// sharedFetch runs fetch once per key across concurrent callers. The shared call gets its own RequestState, and its
// stale and notCached flags are copied onto every caller's state, not just the caller that happened to start it.
func sharedFetch[T any](ctx context.Context, key string, fetch func(fetchCtx context.Context) (T, error)) (T, error) {
	result, err, _ := hypixelFetchGroup.Do(flightKey(ctx, key), func() (interface{}, error) {
		flightState := &RequestState{CacheOnly: isCacheOnly(ctx)}
		fetchCtx, cancel := detachedFetchContext(WithRequestState(ctx, flightState))
		defer cancel()

		value, err := fetch(fetchCtx)
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

// flightKey keeps cache-only and fetching callers out of each other's singleflight calls.
func flightKey(ctx context.Context, key string) string {
	if isCacheOnly(ctx) {
		return key + ":cache-only"
	}
	return key
}
