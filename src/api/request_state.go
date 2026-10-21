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

// flightKey keeps cache-only and fetching callers out of each other's singleflight calls.
func flightKey(ctx context.Context, key string) string {
	if isCacheOnly(ctx) {
		return key + ":cache-only"
	}
	return key
}
