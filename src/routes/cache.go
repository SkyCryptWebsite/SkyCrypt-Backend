package routes

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/localcache"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
)

const responseCachePrefix = "response:"

type responseCacheHandle struct {
	endpoint string
	key      string
}

var (
	responseCachesMu sync.RWMutex
	responseCaches   = make(map[string]*localcache.LocalCache[string])
)

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

func disabledPacksCachePart(disabledPacks []string) string {
	if len(disabledPacks) == 0 {
		return ""
	}
	return strings.Join(disabledPacks, ",")
}

func sendCachedJSON(c *fiber.Ctx, cacheKey responseCacheHandle) (bool, error) {
	if cached, ok, _ := responseCacheForEndpoint(cacheKey.endpoint).Get(cacheKey.key); ok {
		forensics.RecordResponseCache(c.UserContext(), cacheKey.endpoint, "ram")
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		c.Set("X-SkyCrypt-Backend-Cache", "ram")
		return true, c.SendString(cached)
	}

	cached, err := db.GetContext(c.UserContext(), cacheKey.key)
	if err != nil || cached == "" {
		return false, nil
	}

	responseCacheForEndpoint(cacheKey.endpoint).Set(cacheKey.key, cached, 30*time.Second, 30*time.Second)
	forensics.RecordResponseCache(c.UserContext(), cacheKey.endpoint, "redis")
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
	responseCacheForEndpoint(cacheKey.endpoint).Set(cacheKey.key, body, 30*time.Second, 30*time.Second)
	forensics.RecordResponseCache(ctx, cacheKey.endpoint, "cold")
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = db.SetContext(cacheCtx, cacheKey.key, body, ttlSeconds)
	}()

	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	c.Set("X-SkyCrypt-Backend-Cache", "miss")
	return c.SendString(body)
}

func responseCacheForEndpoint(endpoint string) *localcache.LocalCache[string] {
	responseCachesMu.RLock()
	cache := responseCaches[endpoint]
	responseCachesMu.RUnlock()
	if cache != nil {
		return cache
	}

	responseCachesMu.Lock()
	defer responseCachesMu.Unlock()
	if cache = responseCaches[endpoint]; cache != nil {
		return cache
	}
	cache = localcache.NewLocalCache[string](256)
	responseCaches[endpoint] = cache
	return cache
}
