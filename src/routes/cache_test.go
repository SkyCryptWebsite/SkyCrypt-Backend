package routes

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
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

func TestSendCachedJSONIgnoresRAMCacheInDevelopment(t *testing.T) {
	t.Setenv("DEV", "true")
	cacheKey := responseCacheKey("stats", "development-ram-read")
	responseCacheForEndpoint(cacheKey.endpoint).Set(cacheKey.key, `{"cached":true}`, time.Minute, time.Minute)

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		hit, err := sendCachedJSON(c, cacheKey)
		if err != nil {
			return err
		}
		if hit {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	response, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusNoContent)
	}
}

func TestSendAndCacheJSONBypassesRAMCacheInDevelopment(t *testing.T) {
	t.Setenv("DEV", "true")
	cacheKey := responseCacheKey("stats", "development-ram-write")

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		return sendAndCacheJSON(c, context.Background(), cacheKey, map[string]bool{"computed": true}, 60)
	})

	response, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	if string(body) != `{"computed":true}` {
		t.Fatalf("body = %q, want computed JSON response", body)
	}
	if got := response.Header.Get("X-SkyCrypt-Backend-Cache"); got != "bypass" {
		t.Fatalf("cache header = %q, want %q", got, "bypass")
	}
	if _, ok, _ := responseCacheForEndpoint(cacheKey.endpoint).Get(cacheKey.key); ok {
		t.Fatal("development response should not populate the RAM cache")
	}
}

func TestEmbedResponseCacheHeaders(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		setResponseCacheHeaders(c, "embed")
		return c.SendStatus(fiber.StatusNoContent)
	})

	response, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	want := "public, max-age=300, s-maxage=300, stale-while-revalidate=30, stale-if-error=60"
	if got := response.Header.Get(fiber.HeaderCacheControl); got != want {
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
