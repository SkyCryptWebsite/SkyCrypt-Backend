package routes

import (
	"context"
	"os"
	"testing"
)

func TestProcessedResponseCacheDisabledInDevelopment(t *testing.T) {
	t.Setenv("DEV", "true")

	if processedResponseCacheEnabled() {
		t.Fatal("processed response caching should be disabled when DEV=true")
	}
}

func TestProcessedResponseCacheEnabledOutsideDevelopment(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		previous, existed := os.LookupEnv("DEV")
		if err := os.Unsetenv("DEV"); err != nil {
			t.Fatalf("failed to unset DEV: %v", err)
		}
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv("DEV", previous)
			} else {
				_ = os.Unsetenv("DEV")
			}
		})
		if !processedResponseCacheEnabled() {
			t.Fatal("processed response caching should be enabled when DEV is unset")
		}
	})

	t.Run("false", func(t *testing.T) {
		t.Setenv("DEV", "false")
		if !processedResponseCacheEnabled() {
			t.Fatal("processed response caching should be enabled when DEV=false")
		}
	})
}

func TestCachedJSONIgnoresRAMCacheInDevelopment(t *testing.T) {
	t.Setenv("DEV", "true")
	if cached := cachedJSON(context.Background(), responseCacheKey("stats", "development-ram-read")); cached != nil {
		t.Fatalf("cachedJSON() = %+v, want nil", cached)
	}
}

func TestCacheJSONBypassesRAMCacheInDevelopment(t *testing.T) {
	t.Setenv("DEV", "true")
	out, err := cacheJSON(context.Background(), responseCacheKey("stats", "development-ram-write"), map[string]bool{"computed": true}, 60)
	if err != nil {
		t.Fatal(err)
	}
	if string(out.Body) != `{"computed":true}` {
		t.Fatalf("body = %q, want computed JSON response", out.Body)
	}
	if out.Cache != "bypass" {
		t.Fatalf("cache header = %q, want %q", out.Cache, "bypass")
	}
}

func TestEmbedResponseCacheHeaders(t *testing.T) {
	want := "public, max-age=3600, s-maxage=3600, stale-while-revalidate=30, stale-if-error=60"
	if got := responseCacheControl("embed"); got != want {
		t.Fatalf("Cache-Control = %q, want %q", got, want)
	}
}

func TestEnabledPacksCachePartPreservesNormalizedOrder(t *testing.T) {
	got := enabledPacksCachePart([]string{"fsr", "HPLUS", "", "unknown", "hplus"})
	want := "enabled-v7:FSR,HYPIXEL_PLUS"
	if got != want {
		t.Fatalf("enabledPacksCachePart() = %q, want %q", got, want)
	}
}

func TestEnabledPacksCachePartIsOrderSensitive(t *testing.T) {
	first := enabledPacksCachePart([]string{"FSR", "HYPIXEL_PLUS"})
	second := enabledPacksCachePart([]string{"HYPIXEL_PLUS", "FSR"})
	if first == second {
		t.Fatalf("enabled pack cache parts should differ by order: %q", first)
	}
}

func TestEnabledPacksCachePartPreservesVanillaOnlyPreference(t *testing.T) {
	if got := enabledPacksCachePart([]string{}); got != "enabled-v7:" {
		t.Fatalf("enabledPacksCachePart([]) = %q, want %q", got, "enabled-v7:")
	}
}
