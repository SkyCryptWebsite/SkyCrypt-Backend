package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"skycrypt/src/models"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestUniqueUnresolvedUUIDs(t *testing.T) {
	resolved := map[string]*models.MowojangResponse{
		"resolved": {UUID: "resolved", Name: "Resolved"},
	}

	got := uniqueUnresolvedUUIDs([]string{"", "a", "resolved", "a", "b", "b"}, resolved)
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q: %v", i, got[i], want[i], got)
		}
	}
}

func TestUniqueUnresolvedUsernames(t *testing.T) {
	usernames := map[string]string{
		"resolved": "Resolved",
	}

	got := uniqueUnresolvedUsernames([]string{"", "a", "resolved", "a", "b", "b"}, usernames)
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q: %v", i, got[i], want[i], got)
		}
	}
}

// A build resolves the player and looks up member names at the same time; both must share one Mojang fetch.
func TestResolvePlayerAndGetUsernameShareOneFetch(t *testing.T) {
	const uuid = "0123456789abcdef0123456789abcdef"

	originalClient := HTTPClient
	t.Cleanup(func() { HTTPClient = originalClient })

	var sessionCalls atomic.Int32
	release := make(chan struct{})
	HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Host, "sessionserver.mojang.com") {
				sessionCalls.Add(1)
				<-release
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"id":"` + uuid + `","name":"Shared"}`))),
				}, nil
			}
			// The mirror is down, as it was in production.
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader(nil))}, nil
		}),
	}

	var wg sync.WaitGroup
	var resolvedName, username string
	wg.Go(func() {
		resolved, err := ResolvePlayerContext(context.Background(), uuid)
		if err == nil {
			resolvedName = resolved.Name
		}
	})
	wg.Go(func() {
		username, _ = GetUsernameContext(context.Background(), uuid)
	})
	// Give both callers time to reach the fetch before Mojang answers.
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()

	if resolvedName != "Shared" || username != "Shared" {
		t.Fatalf("got names %q and %q, want Shared", resolvedName, username)
	}
	if got := sessionCalls.Load(); got != 1 {
		t.Fatalf("sessionserver called %d times, want 1", got)
	}

	// The name is now known to this process: neither call asks Mojang again.
	if _, err := ResolvePlayerContext(context.Background(), uuid); err != nil {
		t.Fatal(err)
	}
	if got := sessionCalls.Load(); got != 1 {
		t.Fatalf("sessionserver called %d times after the name was cached, want 1", got)
	}
}

// A stalled mirror must not hold the lookup: after mowojangTimeout() it falls back to Mojang.
func TestResolvePlayerFallsBackWhenMirrorStalls(t *testing.T) {
	const uuid = "fedcba9876543210fedcba9876543210"

	originalClient, originalTimeout := HTTPClient, mowojangTimeout
	t.Cleanup(func() { HTTPClient, mowojangTimeout = originalClient, originalTimeout })
	mowojangTimeout = func() time.Duration { return 50 * time.Millisecond }

	HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Host, "sessionserver.mojang.com") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"id":"` + uuid + `","name":"Fallback"}`))),
				}, nil
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		}),
	}

	start := time.Now()
	resolved, err := ResolvePlayerContext(context.Background(), uuid)
	if err != nil || resolved.Name != "Fallback" {
		t.Fatalf("got %#v, %v; want the fallback name", resolved, err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("lookup took %s with a stalled mirror", elapsed)
	}
}

// A name no player has is remembered: asking again must not cost another Mojang call.
func TestMissingPlayerIsCached(t *testing.T) {
	const username = "NoSuchPlayer_Cached"

	originalClient := HTTPClient
	t.Cleanup(func() { HTTPClient = originalClient })

	var mojangCalls atomic.Int32
	HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Host, "api.mojang.com") {
				mojangCalls.Add(1)
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewReader(nil))}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("player not found")))}, nil
		}),
	}

	for range 3 {
		if _, err := GetUUIDContext(context.Background(), username); !errors.Is(err, ErrPlayerNotFound) {
			t.Fatalf("err = %v, want ErrPlayerNotFound", err)
		}
	}
	if uuid, err := GetUUIDContext(context.Background(), strings.ToUpper(username), false); uuid != "" || err != nil {
		t.Fatalf("got %q, %v; want an empty answer without an error", uuid, err)
	}
	if got := mojangCalls.Load(); got != 1 {
		t.Fatalf("Mojang called %d times, want 1", got)
	}
}
