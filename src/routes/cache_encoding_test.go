package routes

import (
	"bytes"
	"testing"

	"github.com/gofiber/fiber/v3/middleware/etag"
)

func TestCachedResponseRoundTrip(t *testing.T) {
	body := bytes.Repeat([]byte(`{"skill":"farming","level":60},`), 2000)
	cached, err := decodeCachedResponse(encodeCachedResponse(body))
	if err != nil {
		t.Fatal(err)
	}
	if cached.etag != string(etag.GenerateWeak(body)) {
		t.Fatalf("etag %q, want the middleware's %q", cached.etag, etag.GenerateWeak(body))
	}
	if len(cached.gzipped) == 0 || len(cached.gzipped) >= len(body) {
		t.Fatalf("gzipped %d bytes from %d", len(cached.gzipped), len(body))
	}
	plain, err := cached.body()
	if err != nil || !bytes.Equal(plain, body) {
		t.Fatalf("body round trip failed: %v", err)
	}
}

func TestCachedResponseOldFormat(t *testing.T) {
	cached, err := decodeCachedResponse([]byte(`{"old":true}`))
	if err != nil || cached.gzipped != nil || cached.etag != "" {
		t.Fatalf("old entry decoded as %+v, %v", cached, err)
	}
	if plain, _ := cached.body(); string(plain) != `{"old":true}` {
		t.Fatalf("old entry body %q", plain)
	}
	if _, err := decodeCachedResponse([]byte(cachedResponseMagic + "\xff")); err == nil {
		t.Fatal("a truncated entry should fail")
	}
}

func TestAcceptsGzip(t *testing.T) {
	for header, want := range map[string]bool{
		"":                  false,
		"gzip":              true,
		"br, gzip, deflate": true,
		"gzip;q=0, br":      false,
		"GZIP ; q=0.5":      true,
		"*":                 true,
		"br, *;q=0":         false,
		"deflate, identity": false,
		"gzip; q=0 , *":     false,
	} {
		if got := acceptsGzip(header); got != want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", header, got, want)
		}
	}
}

func TestETagMatches(t *testing.T) {
	tag := `W/"12-34"`
	for header, want := range map[string]bool{
		``:                   false,
		`W/"12-34"`:          true,
		`"12-34"`:            true,
		`"other", W/"12-34"`: true,
		`*`:                  true,
		`W/"12-35"`:          false,
	} {
		if got := etagMatches(header, tag); got != want {
			t.Errorf("etagMatches(%q) = %v, want %v", header, got, want)
		}
	}
}

// Huma sends Status as is, so a JSON output left at 0 went out as a 500.
func TestJSONBodyIsOK(t *testing.T) {
	out := jsonBody(t.Context(), responseCacheHandle{endpoint: "stats", key: "response:test"}, []byte(`{}`))
	if out.Status != 200 {
		t.Fatalf("status %d, want 200", out.Status)
	}
}
