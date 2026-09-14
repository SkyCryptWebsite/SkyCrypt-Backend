package routes

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/lib"
	"skycrypt/src/utility"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
)

const responseCachePrefix = "response:"

type responseCacheHandle struct {
	endpoint string
	key      string
}

func responseCacheKey(endpoint string, parts ...string) responseCacheHandle {
	h := sha1.New()
	_, _ = h.Write([]byte(endpoint))
	_, _ = h.Write([]byte{0})
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return responseCacheHandle{
		endpoint: endpoint,
		key:      responseCachePrefix + hex.EncodeToString(h.Sum(nil)),
	}
}

func enabledPacksCachePart(enabledPacks []string) string {
	normalized := lib.NormalizeEnabledPacks(enabledPacks)
	return "enabled-v7:" + strings.Join(normalized, ",")
}

func sendCachedJSON(c *fiber.Ctx, cacheKey responseCacheHandle) (bool, error) {
	if !processedResponseCacheEnabled() {
		return false, nil
	}

	cached, err := db.GetContext(c.UserContext(), cacheKey.key)
	if err != nil || cached == "" {
		return false, nil
	}

	recordResponseCache(c.UserContext(), cacheKey.endpoint, "redis")
	setResponseCacheHeaders(c, cacheKey.endpoint)
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	c.Set("X-SkyCrypt-Backend-Cache", "redis")
	return true, c.SendString(cached)
}

func sendAndCacheJSON(c *fiber.Ctx, ctx context.Context, cacheKey responseCacheHandle, value interface{}, ttlSeconds int) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cached response: %v", err)
	}

	body := string(payload)
	if !processedResponseCacheEnabled() {
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		c.Set("X-SkyCrypt-Backend-Cache", "bypass")
		return c.SendString(body)
	}

	recordResponseCache(ctx, cacheKey.endpoint, "cold")
	cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = db.SetContext(cacheCtx, cacheKey.key, body, ttlSeconds)

	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	setResponseCacheHeaders(c, cacheKey.endpoint)
	c.Set("X-SkyCrypt-Backend-Cache", "miss")
	return c.SendString(body)
}

func setResponseCacheHeaders(c *fiber.Ctx, endpoint string) {
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
