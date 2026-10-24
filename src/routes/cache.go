package routes

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"skycrypt/src/api"
	"skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/lib"
	"skycrypt/src/utility"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	jsoniter "github.com/json-iterator/go"
	"golang.org/x/sync/singleflight"
)

const responseCachePrefix = "response:"

type responseCacheHandle struct {
	endpoint string
	key      string
}

// responseCacheKey lowercases the parts: UUIDs and usernames ignore case, and a profile ID or cute name that differs
// only in case names the same profile, so every spelling shares one entry.
func responseCacheKey(endpoint string, parts ...string) responseCacheHandle {
	h := sha1.New()
	_, _ = h.Write([]byte(endpoint))
	_, _ = h.Write([]byte{0})
	for _, part := range parts {
		_, _ = h.Write([]byte(strings.ToLower(part)))
		_, _ = h.Write([]byte{0})
	}
	return responseCacheHandle{
		endpoint: endpoint,
		key:      responseCachePrefix + hex.EncodeToString(h.Sum(nil)),
	}
}

func inventoryCacheKey(uuid string, profileID string, enabledPacks []string) responseCacheHandle {
	return responseCacheHandle{
		endpoint: "inventory",
		key:      fmt.Sprintf("items:%s:%s:%s", strings.ToLower(profileID), strings.ToLower(uuid), enabledPacksCachePart(enabledPacks)),
	}
}

func enabledPacksCachePart(enabledPacks []string) string {
	normalized := lib.NormalizeEnabledPacks(enabledPacks)
	return "enabled-v7:" + strings.Join(normalized, ",")
}

func sendCachedJSON(c fiber.Ctx, cacheKey responseCacheHandle) (bool, error) {
	if !processedResponseCacheEnabled() {
		return false, nil
	}

	cached, err := db.GetContext(c.Context(), cacheKey.key)
	if err != nil || cached == "" {
		return false, nil
	}

	recordResponseCache(c.Context(), cacheKey.endpoint, "redis")
	setResponseCacheHeaders(c, cacheKey.endpoint)
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	c.Set("X-SkyCrypt-Backend-Cache", "redis")
	return true, c.SendString(cached)
}

func sendAndCacheJSON(c fiber.Ctx, ctx context.Context, cacheKey responseCacheHandle, value interface{}, ttlSeconds int) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cached response: %v", err)
	}

	return sendAndCacheBody(c, ctx, cacheKey, string(payload), ttlSeconds)
}

// sendAndCacheBody is sendAndCacheJSON for a body that is already JSON.
func sendAndCacheBody(c fiber.Ctx, ctx context.Context, cacheKey responseCacheHandle, body string, ttlSeconds int) error {
	cacheBody(ctx, cacheKey, []byte(body), ttlSeconds)
	return sendBody(c, ctx, cacheKey, []byte(body))
}

// cacheBody stores a response unless caching is off or it was built from stale fallback data.
func cacheBody(ctx context.Context, cacheKey responseCacheHandle, body []byte, ttlSeconds int) {
	if !processedResponseCacheEnabled() || api.ServedStale(ctx) {
		return
	}
	recordResponseCache(ctx, cacheKey.endpoint, "cold")
	cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = db.SetContext(cacheCtx, cacheKey.key, body, ttlSeconds)
}

// sendBody sends a freshly built response. ctx decides the cache header, so pass the request's own context.
func sendBody(c fiber.Ctx, ctx context.Context, cacheKey responseCacheHandle, body []byte) error {
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	if !processedResponseCacheEnabled() || api.ServedStale(ctx) {
		c.Set("X-SkyCrypt-Backend-Cache", "bypass")
		return c.Send(body)
	}
	setResponseCacheHeaders(c, cacheKey.endpoint)
	c.Set("X-SkyCrypt-Backend-Cache", "miss")
	return c.Send(body)
}

var responseGroup singleflight.Group

// sharedResponseTimeout bounds a shared build, which no single caller can cancel.
const sharedResponseTimeout = 60 * time.Second

// sharedResponse builds a response body once for concurrent requests with the same cache key, so a profile link
// shared on Discord doesn't make every viewer repeat the same work. The body is cached once, under cacheKey and
// rawCacheKey (the request as given, before resolving the player), and returned to every caller, who then sends it
// with sendBody. Callers that joined under another spelling get no copy of their own; they still hit cacheKey.
func sharedResponse(ctx context.Context, cacheKey responseCacheHandle, rawCacheKey responseCacheHandle, ttlSeconds int, build func(ctx context.Context) ([]byte, error)) ([]byte, error) {
	return api.Share(ctx, &responseGroup, cacheKey.key, sharedResponseTimeout, func(flightCtx context.Context) ([]byte, error) {
		body, err := build(flightCtx)
		if err != nil {
			return nil, err
		}
		cacheBody(flightCtx, cacheKey, body, ttlSeconds)
		if rawCacheKey.key != cacheKey.key {
			cacheBody(flightCtx, rawCacheKey, body, ttlSeconds)
		}
		return body, nil
	})
}

// marshalBody encodes a response the same way sendAndCacheJSON does.
func marshalBody(value interface{}) ([]byte, error) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %v", err)
	}
	return body, nil
}

// renderedImageCacheControl is for head, potion and leather images. Their URL fully decides the image, so browsers
// and the CDN may keep them; a week rather than forever so a renderer fix still reaches everyone.
const renderedImageCacheControl = "public, max-age=604800"

func setResponseCacheHeaders(c fiber.Ctx, endpoint string) {
	if endpoint == "embed" {
		c.Set(fiber.HeaderCacheControl, "public, max-age=3600, s-maxage=3600, stale-while-revalidate=30, stale-if-error=60")
	}
}

func processedResponseCacheEnabled() bool {
	return os.Getenv("DEV") != "true"
}

func recordResponseCache(ctx context.Context, endpoint string, status string) {
	if utility.IsForensicsEnabled() {
		forensics.RecordResponseCache(ctx, endpoint, status)
	}
}
