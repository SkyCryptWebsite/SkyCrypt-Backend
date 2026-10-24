// Package apikeys verifies better-auth API keys through the frontend and enforces their rate limits in Redis.
// Verification results are cached for a minute; request counts are synced back to the frontend so its
// apikey.request_count shows real usage.
package apikeys

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"skycrypt/src/db"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

const (
	verifyCacheTTL = time.Minute
	// Matches the frontend's better-auth default, used when a key has no window of its own.
	defaultWindow = 5 * time.Minute
	activeKeysSet = "apikey:active"
	syncInterval  = 15 * time.Second
	syncBatchSize = 500
)

var httpClient = &http.Client{Timeout: 5 * time.Second}
var verifyGroup singleflight.Group

// Key is the part of a better-auth API key the backend needs.
type Key struct {
	ID                  string              `json:"id"`
	Permissions         map[string][]string `json:"permissions"`
	RateLimitEnabled    bool                `json:"rateLimitEnabled"`
	RateLimitMax        int64               `json:"rateLimitMax"`
	RateLimitTimeWindow int64               `json:"rateLimitTimeWindow"`
}

func (k *Key) HasScope(scope string) bool {
	return slices.Contains(k.Permissions["api"], scope)
}

// Verification mirrors better-auth's verifyApiKey result.
type Verification struct {
	Valid bool `json:"valid"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Key *Key `json:"key"`
}

func Verify(ctx context.Context, rawKey string) (*Verification, error) {
	hash := sha256.Sum256([]byte(rawKey))
	cacheKey := "apikey:verify:" + hex.EncodeToString(hash[:])
	if cached, err := db.GetContext(ctx, cacheKey); err == nil && cached != "" {
		var verification Verification
		if json.Unmarshal([]byte(cached), &verification) == nil {
			return &verification, nil
		}
	}

	result, err, _ := verifyGroup.Do(cacheKey, func() (interface{}, error) {
		// Detached: the shared call must not fail for everyone when the first caller disconnects.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), httpClient.Timeout)
		defer cancel()
		var verification Verification
		if err := callFrontend(ctx, "/api/internal/api-keys/verify", map[string]string{"key": rawKey}, &verification); err != nil {
			return nil, err
		}
		if encoded, err := json.Marshal(verification); err == nil {
			_ = db.SetContext(ctx, cacheKey, string(encoded), int(verifyCacheTTL.Seconds()))
		}
		return &verification, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*Verification), nil
}

type Usage struct {
	Allowed bool
	// Limit is 0 for keys without a rate limit.
	Limit     int64
	Remaining int64
	Reset     time.Time
}

// Counts the request in the key's current fixed window and remembers the key for the next usage sync.
var consumeScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'bucket') ~= ARGV[1] then
	redis.call('HSET', KEYS[1], 'bucket', ARGV[1], 'count', 0)
end
local count = redis.call('HINCRBY', KEYS[1], 'count', 1)
redis.call('HSET', KEYS[1], 'last', ARGV[2])
redis.call('PEXPIRE', KEYS[1], ARGV[3])
redis.call('SADD', KEYS[2], ARGV[4])
return count
`)

// Consume counts one request for key, including requests it then rejects, so usage shows abuse attempts too.
func Consume(ctx context.Context, key *Key) (Usage, error) {
	client, err := db.Redis()
	if err != nil {
		return Usage{}, err
	}

	window := time.Duration(key.RateLimitTimeWindow) * time.Millisecond
	if window <= 0 {
		window = defaultWindow
	}
	now := time.Now()
	bucket := now.UnixMilli() / window.Milliseconds()
	count, err := consumeScript.Run(ctx, client, []string{usageKey(key.ID), activeKeysSet},
		bucket, now.UnixMilli(), (2 * window).Milliseconds(), key.ID).Int64()
	if err != nil {
		return Usage{}, fmt.Errorf("could not count API key request: %v", err)
	}

	usage := Usage{Allowed: true, Reset: time.UnixMilli((bucket + 1) * window.Milliseconds())}
	if !key.RateLimitEnabled || key.RateLimitMax <= 0 {
		return usage, nil
	}
	usage.Limit = key.RateLimitMax
	usage.Remaining = max(0, key.RateLimitMax-count)
	usage.Allowed = count <= key.RateLimitMax
	return usage, nil
}

func usageKey(id string) string {
	return "apikey:usage:" + id
}

// StartUsageSync periodically writes request counts to the frontend. Run it in one process only.
func StartUsageSync() {
	go func() {
		for range time.Tick(syncInterval) {
			if err := syncUsage(context.Background()); err != nil {
				fmt.Printf("[APIKEYS] could not sync API key usage: %v\n", err)
			}
		}
	}()
}

type keyUsage struct {
	ID           string `json:"id"`
	RequestCount int64  `json:"requestCount"`
	LastRequest  int64  `json:"lastRequest"`
}

func syncUsage(ctx context.Context) error {
	client, err := db.Redis()
	if err != nil {
		return err
	}

	for {
		ids, err := client.SPopN(ctx, activeKeysSet, syncBatchSize).Result()
		if err != nil || len(ids) == 0 {
			return err
		}

		pipe := client.Pipeline()
		reads := make([]*redis.SliceCmd, len(ids))
		for i, id := range ids {
			reads[i] = pipe.HMGet(ctx, usageKey(id), "count", "last")
		}
		if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			client.SAdd(ctx, activeKeysSet, toInterfaces(ids)...)
			return err
		}

		usages := make([]keyUsage, 0, len(ids))
		for i, read := range reads {
			values := read.Val()
			if len(values) != 2 {
				continue
			}
			count, _ := values[0].(string)
			last, _ := values[1].(string)
			requestCount, _ := strconv.ParseInt(count, 10, 64)
			lastRequest, _ := strconv.ParseInt(last, 10, 64)
			if lastRequest > 0 {
				usages = append(usages, keyUsage{ID: ids[i], RequestCount: requestCount, LastRequest: lastRequest})
			}
		}

		if err := callFrontend(ctx, "/api/internal/api-keys/usage", map[string][]keyUsage{"keys": usages}, nil); err != nil {
			// Put the keys back so the next sync retries them.
			client.SAdd(ctx, activeKeysSet, toInterfaces(ids)...)
			return err
		}
		if len(ids) < syncBatchSize {
			return nil
		}
	}
}

func toInterfaces(values []string) []interface{} {
	out := make([]interface{}, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func callFrontend(ctx context.Context, path string, body interface{}, out interface{}) error {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("FRONTEND_INTERNAL_URL")), "/")
	token := strings.TrimSpace(os.Getenv("SERVER_API_TOKEN"))
	if base == "" || token == "" {
		return errors.New("FRONTEND_INTERNAL_URL and SERVER_API_TOKEN must be set")
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("frontend request failed: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("frontend %s returned status %d", path, resp.StatusCode)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
