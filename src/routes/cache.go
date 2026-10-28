package routes

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"skycrypt/src/api"
	"skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/lib"
	"skycrypt/src/utility"
	"strconv"
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

// cachedJSON returns the cached response under cacheKey, or nil when there is none.
func cachedJSON(ctx context.Context, cacheKey responseCacheHandle) *jsonOutput {
	if !processedResponseCacheEnabled() {
		return nil
	}

	entry, err := db.GetBytesContext(ctx, cacheKey.key)
	if err != nil || len(entry) == 0 {
		return nil
	}
	cached, err := decodeCachedResponse(entry)
	if err != nil {
		return nil
	}

	recordResponseCache(ctx, cacheKey.endpoint, "redis")
	out := &jsonOutput{
		Status:       fiber.StatusOK,
		ContentType:  fiber.MIMEApplicationJSONCharsetUTF8,
		CacheControl: responseCacheControl(cacheKey.endpoint),
		Cache:        "redis",
		ETag:         cached.etag,
	}
	encoding := requestEncodingFrom(ctx)
	switch {
	case etagMatches(encoding.ifNoneMatch, cached.etag):
		out.Status = fiber.StatusNotModified
	case cached.gzipped != nil && encoding.acceptsGzip:
		out.ContentEncoding = "gzip"
		out.Body = cached.gzipped
	default:
		body, err := cached.body()
		if err != nil {
			return nil
		}
		out.Body = body
	}
	return out
}

func cacheJSON(ctx context.Context, cacheKey responseCacheHandle, value interface{}, ttlSeconds int) (*jsonOutput, error) {
	body, err := marshalBody(value)
	if err != nil {
		return nil, err
	}

	cacheBody(ctx, cacheKey, body, ttlSeconds)
	return jsonBody(ctx, cacheKey, body), nil
}

// cacheBody stores a response unless caching is off or it was built from stale fallback data.
func cacheBody(ctx context.Context, cacheKey responseCacheHandle, body []byte, ttlSeconds int) {
	if !processedResponseCacheEnabled() || api.ServedStale(ctx) {
		return
	}
	recordResponseCache(ctx, cacheKey.endpoint, "cold")
	cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = db.SetRawContext(cacheCtx, cacheKey.key, encodeCachedResponse(body), ttlSeconds)
}

// jsonBody is the output for a freshly built response. ctx decides the cache header, so pass the request's own
// context.
func jsonBody(ctx context.Context, cacheKey responseCacheHandle, body []byte) *jsonOutput {
	out := &jsonOutput{Status: fiber.StatusOK, ContentType: fiber.MIMEApplicationJSONCharsetUTF8, Body: body}
	if fetchedAt := api.StaleFetchedAt(ctx); !fetchedAt.IsZero() {
		out.Stale = fetchedAt.UTC().Format(time.RFC3339)
	}
	if !processedResponseCacheEnabled() || api.ServedStale(ctx) {
		out.Cache = "bypass"
		return out
	}
	out.CacheControl = responseCacheControl(cacheKey.endpoint)
	out.Cache = "miss"
	return out
}

var responseGroup singleflight.Group

// A build parses a multi-MB profile and decodes its inventories, allocating tens of MB. Prefork children run on one
// core, so a burst of builds can't finish sooner, it only multiplies memory; extra builds wait for a slot.
var buildSlots = make(chan struct{}, maxConcurrentBuilds())

// errServerBusy is a build that waited for a slot until its deadline.
var errServerBusy = errors.New("the server is busy; try again shortly")

// maxConcurrentBuilds is SKYCRYPT_MAX_BUILDS (default 8): profile builds one process runs at once.
func maxConcurrentBuilds() int {
	if n, err := strconv.Atoi(os.Getenv("SKYCRYPT_MAX_BUILDS")); err == nil && n > 0 {
		return n
	}
	return 8
}

// sharedResponseTimeout bounds a shared build, which no single caller can cancel.
const sharedResponseTimeout = 60 * time.Second

// sharedResponse builds a response body once for concurrent requests with the same cache key, so a profile link
// shared on Discord doesn't make every viewer repeat the same work. The body is cached once, under cacheKey and
// rawCacheKey (the request as given, before resolving the player), and returned to every caller, who then sends it
// with jsonBody. Callers that joined under another spelling get no copy of their own; they still hit cacheKey.
func sharedResponse(ctx context.Context, cacheKey responseCacheHandle, rawCacheKey responseCacheHandle, ttlSeconds int, build func(ctx context.Context) ([]byte, error)) ([]byte, error) {
	return api.Share(ctx, &responseGroup, cacheKey.key, sharedResponseTimeout, func(flightCtx context.Context) ([]byte, error) {
		select {
		case buildSlots <- struct{}{}:
			defer func() { <-buildSlots }()
		case <-flightCtx.Done():
			return nil, errServerBusy
		}
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

// marshalBody encodes a response for the response cache.
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

func responseCacheControl(endpoint string) string {
	if endpoint == "embed" {
		return "public, max-age=3600, s-maxage=3600, stale-while-revalidate=30, stale-if-error=60"
	}
	return ""
}

func processedResponseCacheEnabled() bool {
	return os.Getenv("DEV") != "true"
}

func recordResponseCache(ctx context.Context, endpoint string, status string) {
	if utility.IsForensicsEnabled() {
		forensics.RecordResponseCache(ctx, endpoint, status)
	}
}
