package db

import (
	"context"
	"fmt"
	"os"
	"skycrypt/src/forensics"
	"skycrypt/src/utility"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

var ctx = context.Background()
var redisClient *redis.Client
var redisAddr string
var redisPassword string
var redisDB int
var clientMutex sync.RWMutex

type RedisClient struct {
	client *redis.Client
}

func InitRedis(addr string, password string, db int) error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	redisAddr = addr
	redisPassword = password
	redisDB = db

	// Don't use sync.Once with prefork mode - each process needs its own connection
	redisClient = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		PoolTimeout:  time.Second * 4,
		IdleTimeout:  time.Minute * 5,
		DialTimeout:  time.Second * 5,
		ReadTimeout:  time.Second * 3,
		WriteTimeout: time.Second * 3,
		PoolFIFO:     false,
	})

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	_, err := redisClient.Ping(ctxTimeout).Result()
	if err != nil {
		_ = redisClient.Close()
		redisClient = nil
		return fmt.Errorf("could not connect to Redis: %v", err)
	}

	if os.Getenv("FIBER_PREFORK_CHILD") == "" {
		fmt.Print("Redis connected successfully\n")
	}

	return nil
}

func (r *RedisClient) Set(key string, value interface{}, expirationSeconds int) error {
	expiration := time.Duration(expirationSeconds) * time.Second
	err := r.client.Set(ctx, key, value, expiration).Err()
	if err != nil {
		return fmt.Errorf("could not set value in Redis: %v", err)
	}
	return nil
}

func Get(key string) (string, error) {
	start := time.Now()

	clientMutex.RLock()
	client := redisClient
	clientMutex.RUnlock()

	if client == nil {
		clientMutex.Lock()
		if redisClient == nil {
			if redisAddr != "" {
				err := InitRedis(redisAddr, redisPassword, redisDB)
				if err != nil {
					clientMutex.Unlock()
					return "", fmt.Errorf("redis client not initialized and re-initialization failed: %v", err)
				}
			} else {
				clientMutex.Unlock()
				return "", fmt.Errorf("redis client not initialized. Call InitRedis() first")
			}
		}
		client = redisClient
		clientMutex.Unlock()
	}

	val, err := client.Get(ctx, key).Result()
	duration := time.Since(start)

	if err != nil {
		if err == redis.Nil {
			if utility.IsForensicsEnabled() {
				forensics.Logger.Debug("redis_cache_miss",
					zap.String("key", key),
					zap.Duration("duration", duration),
				)
			}
			return "", nil
		}
		if utility.IsForensicsEnabled() {
			forensics.Logger.Error("redis_get_error",
				zap.String("key", key),
				zap.Error(err),
				zap.Duration("duration", duration),
			)
		}
		return "", fmt.Errorf("could not get value from Redis: %v", err)
	}

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

func NewRedisClient(addr string, password string, db int) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		PoolTimeout:  time.Second * 4,
		IdleTimeout:  time.Minute * 5,
		DialTimeout:  time.Second * 5,
		ReadTimeout:  time.Second * 3,
		WriteTimeout: time.Second * 3,
		PoolFIFO:     false,
	})

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	_, err := rdb.Ping(ctxTimeout).Result()
	if err != nil {
		panic(fmt.Errorf("could not connect to Redis: %v", err))
	}

	return &RedisClient{client: rdb}
}

func Set(key string, value interface{}, expirationSeconds int) error {
	start := time.Now()

	clientMutex.RLock()
	client := redisClient
	clientMutex.RUnlock()

	if client == nil {
		clientMutex.Lock()
		if redisClient == nil {
			if redisAddr != "" {
				err := InitRedis(redisAddr, redisPassword, redisDB)
				if err != nil {
					clientMutex.Unlock()
					return fmt.Errorf("redis client not initialized and re-initialization failed: %v", err)
				}
			} else {
				clientMutex.Unlock()
				return fmt.Errorf("redis client not initialized. Call InitRedis() first")
			}
		}
		client = redisClient
		clientMutex.Unlock()
	}

	expiration := time.Duration(expirationSeconds) * time.Second
	err := client.Set(ctx, key, value, expiration).Err()
	duration := time.Since(start)

	if err != nil {
		if utility.IsForensicsEnabled() {
			forensics.Logger.Error("redis_set_error",
				zap.String("key", key),
				zap.Error(err),
				zap.Duration("duration", duration),
			)
		}
		return fmt.Errorf("could not set value in Redis: %v", err)
	}

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

func (r *RedisClient) Get(key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("could not get value from Redis: %v", err)
	}
	return val, nil
}
