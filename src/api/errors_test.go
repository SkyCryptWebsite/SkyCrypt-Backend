package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUpstreamErrorClassifiesTimeouts(t *testing.T) {
	if err := upstreamError("Hypixel", context.DeadlineExceeded); !errors.Is(err, ErrUpstreamTimeout) {
		t.Fatalf("a deadline must be a timeout, got %v", err)
	}
	if err := upstreamError("Hypixel", errors.New("connection reset")); !errors.Is(err, ErrUpstream) || errors.Is(err, ErrUpstreamTimeout) {
		t.Fatalf("a connection error must be a plain upstream failure, got %v", err)
	}
}

func TestGetHypixelBodyClassifiesFailures(t *testing.T) {
	t.Setenv("HYPIXEL_API_KEY", "test-key")

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer failing.Close()
	if _, err := getHypixelBody(context.Background(), failing.URL, nil); !errors.Is(err, ErrUpstream) {
		t.Fatalf("a Hypixel 503 must be ErrUpstream, got %v", err)
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer slow.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := getHypixelBody(ctx, slow.URL, nil); !errors.Is(err, ErrUpstreamTimeout) {
		t.Fatalf("a request past its deadline must be ErrUpstreamTimeout, got %v", err)
	}
}

func TestResolvePlayerKeepsNotFoundThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("error resolving UUID for username '%s': %w", "nobody", fmt.Errorf("%w: invalid username or UUID provided", ErrPlayerNotFound))
	if !errors.Is(wrapped, ErrPlayerNotFound) {
		t.Fatal("ErrPlayerNotFound must survive the resolver's wrapping")
	}
}
