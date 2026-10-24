package db

import (
	"context"
	"fmt"
	"os"
	"skycrypt/src/forensics"
	"skycrypt/src/utility"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/gofiber/fiber/v3"

	"github.com/klauspost/compress/zstd"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var ctx = context.Background()
var redisClient *redis.Client
var clientMutex sync.RWMutex

// Values at least this big are stored zstd-compressed; small identity keys aren't worth the frame overhead.
const compressMinBytes = 1024

var zstdEncoder, _ = zstd.NewWriter(nil)
var zstdDecoder, _ = zstd.NewReader(nil)

const zstdMagic = "\x28\xb5\x2f\xfd"

func compressValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		if len(v) >= compressMinBytes {
			return zstdEncoder.EncodeAll([]byte(v), nil)
		}
	case []byte:
		if len(v) >= compressMinBytes {
			return zstdEncoder.EncodeAll(v, nil)
		}
	}
	return value
}

// decompressValue passes plain values through, so keys written before compression still read fine.
func decompressValue(value string) (string, error) {
	if !strings.HasPrefix(value, zstdMagic) {
		return value, nil
	}
	out, err := zstdDecoder.DecodeAll([]byte(value), nil)
	return string(out), err
}

func InitRedis(addr string, password string, db int) error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	// Don't use sync.Once with prefork mode - each process needs its own connection
	poolSize := redisOptionInt("REDIS_POOL_SIZE", 25)
	minIdleConns := redisOptionInt("REDIS_MIN_IDLE_CONNS", 10)
	poolTimeoutSeconds := redisOptionInt("REDIS_POOL_TIMEOUT_SECONDS", 2)

	redisClient = redis.NewClient(&redis.Options{
		Addr:            addr,
		Password:        password,
		DB:              db,
		PoolSize:        poolSize,
		MinIdleConns:    minIdleConns,
		MaxRetries:      3,
		PoolTimeout:     time.Duration(poolTimeoutSeconds) * time.Second,
		ConnMaxIdleTime: time.Minute * 5,
		DialTimeout:     time.Second * 5,
		ReadTimeout:     time.Second * 3,
		WriteTimeout:    time.Second * 3,
		PoolFIFO:        false,
		// v9 ignores context deadlines unless this is set; v8 always honored them.
		ContextTimeoutEnabled: true,
	})

	// Retry ping in case Redis is still loading its dataset
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*5)
		_, err := redisClient.Ping(ctxTimeout).Result()
		cancel()
		if err == nil {
			break
		}
		if i == maxRetries-1 {
			_ = redisClient.Close()
			redisClient = nil
			return fmt.Errorf("could not connect to Redis: %v", err)
		}
		if !fiber.IsChild() {
			fmt.Printf("[REDIS] Waiting for Redis to be ready (attempt %d/%d): %v\n", i+1, maxRetries, err)
		}
		time.Sleep(time.Second * 2)
	}

	if !fiber.IsChild() {
		fmt.Print("[REDIS] Redis connected successfully\n")
	}
	if utility.IsForensicsEnabled() {
		go forensics.NewPoolMonitor().MonitorRedisPool(redisClient)
	}

	return nil
}

func Get(key string) (string, error) {
	return GetContext(context.Background(), key)
}

func GetContext(ctx context.Context, key string) (string, error) {
	val, err := getRaw(ctx, key)
	if err != nil || val == "" {
		return "", err
	}
	return decompressValue(val)
}

// GetBytesContext is GetContext for callers that parse the value. It skips the string copies GetContext makes,
// which matter for multi-megabyte profile bodies. A missing key returns nil.
func GetBytesContext(ctx context.Context, key string) ([]byte, error) {
	val, err := getRaw(ctx, key)
	if err != nil || val == "" {
		return nil, err
	}
	if strings.HasPrefix(val, zstdMagic) {
		// DecodeAll only reads its input and keeps no reference, so it can read the string's bytes in place.
		return zstdDecoder.DecodeAll(unsafe.Slice(unsafe.StringData(val), len(val)), nil)
	}
	return []byte(val), nil
}

// getRaw returns the stored value as is, or "" when the key is missing.
func getRaw(ctx context.Context, key string) (string, error) {
	start := time.Now()

	client, err := getRedisClient()
	if err != nil {
		return "", err
	}

	val, err := client.Get(ctx, key).Result()
	duration := time.Since(start)

	if err != nil {
		if err == redis.Nil {
			recordRedisDependency(ctx, "GET", key, "miss", duration, nil)
			if utility.IsForensicsEnabled() {
				forensics.Logger.Debug("redis_cache_miss",
					zap.String("key", key),
					zap.Duration("duration", duration),
				)
			}
			return "", nil
		}
		recordRedisDependency(ctx, "GET", key, "error", duration, err)
		if utility.IsForensicsEnabled() {
			forensics.Logger.Error("redis_get_error",
				zap.String("key", key),
				zap.Error(err),
				zap.Duration("duration", duration),
			)
		}
		return "", fmt.Errorf("could not get value from Redis: %v", err)
	}

	recordRedisDependency(ctx, "GET", key, "hit", duration, nil)
	if utility.IsForensicsEnabled() {
		forensics.Logger.Debug("redis_cache_hit",
			zap.String("key", key),
			zap.Duration("duration", duration),
			zap.Int("value_size", len(val)),
		)
	}

	if duration > 5*time.Millisecond {
		if utility.IsForensicsEnabled() {
			forensics.Logger.Warn("slow_redis_get",
				zap.String("key", key),
				zap.Duration("duration", duration),
			)
		}
	}

	return val, nil
}

func GetManyContext(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	if len(keys) == 0 {
		return result, nil
	}

	start := time.Now()
	client, err := getRedisClient()
	if err != nil {
		return result, err
	}

	values, err := client.MGet(ctx, keys...).Result()
	duration := time.Since(start)
	keyGroup := ""
	if utility.IsForensicsEnabled() {
		keyGroup = forensics.RedisKeyGroup(keys[0])
	}
	if err != nil {
		recordRedisBatchDependency(ctx, "MGET", keyGroup, 0, 0, duration, err)
		if utility.IsForensicsEnabled() {
			forensics.Logger.Error("redis_mget_error",
				zap.Strings("keys", keys),
				zap.Error(err),
				zap.Duration("duration", duration),
			)
		}
		return result, fmt.Errorf("could not get values from Redis: %v", err)
	}

	hits, misses := 0, 0
	for i, value := range values {
		if value == nil {
			misses++
			continue
		}
		str, ok := value.(string)
		if !ok || str == "" {
			misses++
			continue
		}
		str, err := decompressValue(str)
		if err != nil {
			misses++
			continue
		}
		hits++
		result[keys[i]] = str
	}

	recordRedisBatchDependency(ctx, "MGET", keyGroup, hits, misses, duration, nil)
	if utility.IsForensicsEnabled() {
		forensics.Logger.Debug("redis_mget_completed",
			zap.String("key_group", keyGroup),
			zap.Int("key_count", len(keys)),
			zap.Int("hits", hits),
			zap.Int("misses", misses),
			zap.Duration("duration", duration),
		)
	}

	if duration > 5*time.Millisecond && utility.IsForensicsEnabled() {
		forensics.Logger.Warn("slow_redis_mget",
			zap.String("key_group", keyGroup),
			zap.Int("key_count", len(keys)),
			zap.Duration("duration", duration),
		)
	}

	return result, nil
}

// Redis returns the shared client for callers that need commands beyond Get and Set.
func Redis() (*redis.Client, error) {
	return getRedisClient()
}

func getRedisClient() (*redis.Client, error) {
	clientMutex.RLock()
	client := redisClient
	clientMutex.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("redis client not initialized. Call InitRedis() first")
	}
	return client, nil
}

// CloseRedis closes the shared client during shutdown.
func CloseRedis() {
	clientMutex.Lock()
	defer clientMutex.Unlock()
	if redisClient != nil {
		_ = redisClient.Close()
		redisClient = nil
	}
}

func redisOptionInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func recordRedisDependency(ctx context.Context, operation, key, cacheResult string, duration time.Duration, err error) {
	if utility.IsForensicsEnabled() {
		forensics.RecordRedisDependency(ctx, operation, key, cacheResult, duration, err)
	}
}

func recordRedisBatchDependency(ctx context.Context, operation, keyGroup string, hits, misses int, duration time.Duration, err error) {
	if utility.IsForensicsEnabled() {
		forensics.RecordRedisBatchDependency(ctx, operation, keyGroup, hits, misses, duration, err)
	}
}

func recordRedisBatchSetDependency(ctx context.Context, operation, keyGroup string, keyCount int, duration time.Duration, err error) {
	if utility.IsForensicsEnabled() {
		forensics.RecordRedisBatchSetDependency(ctx, operation, keyGroup, keyCount, duration, err)
	}
}

func Set(key string, value interface{}, expirationSeconds int) error {
	return SetContext(context.Background(), key, value, expirationSeconds)
}

func SetManyContext(ctx context.Context, values map[string]interface{}, expirationSeconds int) error {
	if len(values) == 0 {
		return nil
	}

	start := time.Now()
	client, err := getRedisClient()
	if err != nil {
		return err
	}

	expiration := time.Duration(expirationSeconds) * time.Second
	pipe := client.Pipeline()
	for key, value := range values {
		pipe.Set(ctx, key, compressValue(value), expiration)
	}
	_, err = pipe.Exec(ctx)
	duration := time.Since(start)

	if err != nil {
		recordRedisBatchSetDependency(ctx, "PIPELINE_SET", "identity_cache", len(values), duration, err)
		if utility.IsForensicsEnabled() {
			forensics.Logger.Error("redis_pipeline_set_error",
				zap.Int("key_count", len(values)),
				zap.Error(err),
				zap.Duration("duration", duration),
			)
		}
		return fmt.Errorf("could not set values in Redis: %v", err)
	}

	recordRedisBatchSetDependency(ctx, "PIPELINE_SET", "identity_cache", len(values), duration, nil)
	if utility.IsForensicsEnabled() {
		forensics.Logger.Debug("redis_pipeline_set_completed",
			zap.Int("key_count", len(values)),
			zap.Int("ttl_seconds", expirationSeconds),
			zap.Duration("duration", duration),
		)
	}

	if duration > 5*time.Millisecond && utility.IsForensicsEnabled() {
		forensics.Logger.Warn("slow_redis_pipeline_set",
			zap.Int("key_count", len(values)),
			zap.Duration("duration", duration),
		)
	}

	return nil
}

func SetContext(ctx context.Context, key string, value interface{}, expirationSeconds int) error {
	start := time.Now()

	client, err := getRedisClient()
	if err != nil {
		return err
	}

	expiration := time.Duration(expirationSeconds) * time.Second
	err = client.Set(ctx, key, compressValue(value), expiration).Err()
	duration := time.Since(start)

	if err != nil {
		recordRedisDependency(ctx, "SET", key, "error", duration, err)
		if utility.IsForensicsEnabled() {
			forensics.Logger.Error("redis_set_error",
				zap.String("key", key),
				zap.Error(err),
				zap.Duration("duration", duration),
			)
		}
		return fmt.Errorf("could not set value in Redis: %v", err)
	}

	recordRedisDependency(ctx, "SET", key, "set", duration, nil)
	if utility.IsForensicsEnabled() {
		forensics.Logger.Debug("redis_set_completed",
			zap.String("key", key),
			zap.Duration("duration", duration),
			zap.Int("ttl_seconds", expirationSeconds),
		)
	}

	if duration > 5*time.Millisecond {
		if utility.IsForensicsEnabled() {
			forensics.Logger.Warn("slow_redis_set",
				zap.String("key", key),
				zap.Duration("duration", duration),
			)
		}
	}

	return nil
}
