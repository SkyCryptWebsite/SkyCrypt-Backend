package db

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
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
		redisClient.Close()
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
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("could not get value from Redis: %v", err)
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
	if err != nil {
		return fmt.Errorf("could not set value in Redis: %v", err)
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
