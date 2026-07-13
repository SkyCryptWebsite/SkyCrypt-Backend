package forensics

import (
	"context"
	"fmt"
	"skycrypt/src/security"
	"time"

	"go.uber.org/zap"
)

type InstrumentedRedisDB struct {
	logger    *zap.Logger
	getFunc   func(string) (string, error)
	setFunc   func(string, interface{}, int) error
	closeFunc func() error
}

func NewInstrumentedRedisDB(getFunc func(string) (string, error), setFunc func(string, interface{}, int) error) *InstrumentedRedisDB {
	return &InstrumentedRedisDB{
		logger:    Logger,
		getFunc:   getFunc,
		setFunc:   setFunc,
		closeFunc: nil,
	}
}

func (ir *InstrumentedRedisDB) Get(key string) (string, error) {
	start := time.Now()

	val, err := ir.getFunc(key)
	duration := time.Since(start)

	if err != nil {
		ir.logger.Error("redis_db_get_error",
			zap.String("key", key),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
		return val, err
	}

	if val == "" {
		ir.logger.Debug("redis_db_cache_miss",
			zap.String("key", key),
			zap.Duration("duration", duration),
		)
		RecordCacheMiss()
	} else {
		ir.logger.Debug("redis_db_cache_hit",
			zap.String("key", key),
			zap.Duration("duration", duration),
			zap.Int("value_size", len(val)),
		)
		RecordCacheHit()
	}

	if duration > 5*time.Millisecond {
		ir.logger.Warn("slow_redis_db_get",
			zap.String("key", key),
			zap.Duration("duration", duration),
		)
	}

	return val, err
}

func (ir *InstrumentedRedisDB) Set(key string, value interface{}, expirationSeconds int) error {
	start := time.Now()

	err := ir.setFunc(key, value, expirationSeconds)
	duration := time.Since(start)

	if err != nil {
		ir.logger.Error("redis_db_set_error",
			zap.String("key", key),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
	} else {
		ir.logger.Debug("redis_db_set_completed",
			zap.String("key", key),
			zap.Duration("duration", duration),
			zap.Int("ttl_seconds", expirationSeconds),
		)
	}

	if duration > 5*time.Millisecond {
		ir.logger.Warn("slow_redis_db_set",
			zap.String("key", key),
			zap.Duration("duration", duration),
		)
	}

	return err
}

func RecordCacheHit() {
	if globalCacheStats != nil {
		globalCacheStats.hits++
	}
}

func RecordCacheMiss() {
	if globalCacheStats != nil {
		globalCacheStats.misses++
	}
}

func TrackAPICall(apiName string, url string, statusCode int, duration time.Duration, err error) {
	redactedAPIName := security.RedactString(apiName)
	redactedURL := RedactURL(url)
	fields := []zap.Field{
		zap.String("api", redactedAPIName),
		zap.String("url", redactedURL),
		zap.Duration("duration", duration),
		zap.Int64("duration_ms", duration.Milliseconds()),
	}

	if err != nil {
		redactedErr := security.RedactError(err)
		fields = append(fields, zap.Error(redactedErr))
		Logger.Error("api_call_failed", fields...)
		RecordError("api_call_error", redactedErr, map[string]interface{}{
			"api": redactedAPIName,
			"url": redactedURL,
		})
		return
	}

	fields = append(fields, zap.Int("status_code", statusCode))

	if statusCode == 429 {
		Logger.Error("api_rate_limited", fields...)
	} else if statusCode >= 500 {
		Logger.Error("api_upstream_error", fields...)
	} else if statusCode >= 400 {
		Logger.Warn("api_client_error", fields...)
	} else {
		Logger.Info("api_call_completed", fields...)
	}

	if duration > 100*time.Millisecond {
		Logger.Warn("slow_api_call",
			zap.String("api", redactedAPIName),
			zap.String("url", redactedURL),
			zap.Duration("duration", duration),
		)
	}
}

func TrackHandler(ctx context.Context, handlerName string) func() {
	start := time.Now()

	Logger.Info("handler_started",
		zap.String("handler", handlerName),
	)

	return func() {
		duration := time.Since(start)

		Logger.Info("handler_completed",
			zap.String("handler", handlerName),
			zap.Duration("duration", duration),
			zap.Int64("duration_ms", duration.Milliseconds()),
		)

		RecordSpan(fmt.Sprintf("handler.%s", handlerName), duration)
	}
}
