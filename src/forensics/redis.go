package forensics

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type InstrumentedRedis struct {
	client *redis.Client
	logger *zap.Logger
	stats  *CacheStats
}

type CacheStats struct {
	hits   uint64
	misses uint64
}

var globalCacheStats = &CacheStats{}

func NewInstrumentedRedis(client *redis.Client) *InstrumentedRedis {
	return &InstrumentedRedis{
		client: client,
		logger: Logger,
		stats:  globalCacheStats,
	}
}

func (ir *InstrumentedRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	start := time.Now()

	ir.logger.Debug("redis_operation_start",
		zap.String("operation", "GET"),
		zap.String("key", key),
	)

	result := ir.client.Get(ctx, key)
	duration := time.Since(start)

	if err := result.Err(); err != nil {
		if err == redis.Nil {
			ir.logger.Debug("redis_cache_miss",
				zap.String("key", key),
				zap.Duration("duration", duration),
			)
			atomic.AddUint64(&ir.stats.misses, 1)
		} else {
			ir.logger.Error("redis_error",
				zap.String("operation", "GET"),
				zap.String("key", key),
				zap.Error(err),
				zap.Duration("duration", duration),
			)
		}
	} else {
		ir.logger.Debug("redis_cache_hit",
			zap.String("key", key),
			zap.Duration("duration", duration),
			zap.Int("value_size", len(result.Val())),
		)
		atomic.AddUint64(&ir.stats.hits, 1)
	}

	ir.flagSlowRedis("GET", key, duration)
	return result
}

func (ir *InstrumentedRedis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	start := time.Now()

	ir.logger.Debug("redis_operation_start",
		zap.String("operation", "SET"),
		zap.String("key", key),
		zap.Duration("ttl", expiration),
	)

	result := ir.client.Set(ctx, key, value, expiration)
	duration := time.Since(start)

	if err := result.Err(); err != nil {
		ir.logger.Error("redis_error",
			zap.String("operation", "SET"),
			zap.String("key", key),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
	} else {
		ir.logger.Debug("redis_set_completed",
			zap.String("key", key),
			zap.Duration("duration", duration),
			zap.Duration("ttl", expiration),
		)
	}

	ir.flagSlowRedis("SET", key, duration)
	return result
}

func (ir *InstrumentedRedis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	start := time.Now()

	result := ir.client.Del(ctx, keys...)
	duration := time.Since(start)

	if err := result.Err(); err != nil {
		ir.logger.Error("redis_error",
			zap.String("operation", "DEL"),
			zap.Strings("keys", keys),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
	} else {
		ir.logger.Debug("redis_del_completed",
			zap.Strings("keys", keys),
			zap.Duration("duration", duration),
			zap.Int64("deleted_count", result.Val()),
		)
	}

	ir.flagSlowRedis("DEL", strings.Join(keys, ","), duration)
	return result
}

func (ir *InstrumentedRedis) Ping(ctx context.Context) *redis.StatusCmd {
	start := time.Now()

	result := ir.client.Ping(ctx)
	duration := time.Since(start)

	ir.logger.Debug("redis_ping",
		zap.Duration("duration", duration),
		zap.Error(result.Err()),
	)

	return result
}

func (ir *InstrumentedRedis) flagSlowRedis(operation, key string, duration time.Duration) {
	if duration > 5*time.Millisecond {
		ir.logger.Warn("slow_redis_operation",
			zap.String("operation", operation),
			zap.String("key", key),
			zap.Duration("duration", duration),
			zap.Int64("duration_us", duration.Microseconds()),
			zap.String("suggestion", "Redis should be sub-millisecond. Check network latency or Redis load"),
		)
	}
}

func GetCacheStats() (hits, misses uint64, hitRate float64) {
	hits = atomic.LoadUint64(&globalCacheStats.hits)
	misses = atomic.LoadUint64(&globalCacheStats.misses)
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}
	return
}

func LogCacheStatsPeriodically(wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		hits, misses, hitRate := GetCacheStats()

		Logger.Info("cache_statistics",
			zap.Uint64("hits", hits),
			zap.Uint64("misses", misses),
			zap.Float64("hit_rate_percent", hitRate),
		)

		if hitRate < 50 && (hits+misses) > 100 {
			Logger.Warn("low_cache_hit_rate",
				zap.Float64("hit_rate_percent", hitRate),
				zap.String("suggestion", "Cache hit rate below 50%. Review caching strategy and TTLs"),
			)
		}
	}
}

func WrapRedisGet(key string, originalGetFunc func(string) (string, error)) (string, error) {
	start := time.Now()

	val, err := originalGetFunc(key)
	duration := time.Since(start)

	if err != nil {
		Logger.Error("redis_get_error",
			zap.String("key", key),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
		return val, err
	}

	if val == "" {
		Logger.Debug("redis_cache_miss",
			zap.String("key", key),
			zap.Duration("duration", duration),
		)
		atomic.AddUint64(&globalCacheStats.misses, 1)
	} else {
		Logger.Debug("redis_cache_hit",
			zap.String("key", key),
			zap.Duration("duration", duration),
			zap.Int("value_size", len(val)),
		)
		atomic.AddUint64(&globalCacheStats.hits, 1)
	}

	if duration > 5*time.Millisecond {
		Logger.Warn("slow_redis_get",
			zap.String("key", key),
			zap.Duration("duration", duration),
			zap.String("suggestion", "Redis GET should be sub-millisecond"),
		)
	}

	return val, err
}

func WrapRedisSet(key string, value interface{}, expirationSeconds int, originalSetFunc func(string, interface{}, int) error) error {
	start := time.Now()

	err := originalSetFunc(key, value, expirationSeconds)
	duration := time.Since(start)

	if err != nil {
		Logger.Error("redis_set_error",
			zap.String("key", key),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
	} else {
		Logger.Debug("redis_set_completed",
			zap.String("key", key),
			zap.Duration("duration", duration),
			zap.Int("ttl_seconds", expirationSeconds),
			zap.String("value_size", fmt.Sprintf("%d bytes", len(fmt.Sprintf("%v", value)))),
		)
	}

	if duration > 5*time.Millisecond {
		Logger.Warn("slow_redis_set",
			zap.String("key", key),
			zap.Duration("duration", duration),
		)
	}

	return err
}
