package forensics

import (
	"context"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func RequestTracingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := uuid.New().String()
		c.Locals("request_id", requestID)
		requestMetrics := NewRequestMetrics(requestID, c.Method(), c.Path())
		baseCtx := c.UserContext()
		if baseCtx == nil {
			baseCtx = context.Background()
		}
		metricsCtx := WithRequestMetrics(baseCtx, requestMetrics)
		c.SetUserContext(metricsCtx)

		start := time.Now()
		startAlloc := GetAllocBytes()

		Logger.Info("request_started",
			zap.String("request_id", requestID),
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.String("ip", c.IP()),
			zap.String("user_agent", c.Get("User-Agent")),
			zap.Int("content_length", len(c.Body())),
			zap.String("query", string(c.Context().QueryArgs().QueryString())),
		)

		err := c.Next()

		duration := time.Since(start)
		statusCode := c.Response().StatusCode()
		endAlloc := GetAllocBytes()
		route := c.Path()
		if c.Route() != nil && c.Route().Path != "" {
			route = c.Route().Path
		}
		SetRequestRoute(c.UserContext(), route)

		logLevel := zap.InfoLevel
		if statusCode >= 500 {
			logLevel = zap.ErrorLevel
		} else if statusCode >= 400 {
			logLevel = zap.WarnLevel
		}
		if duration > 1*time.Second {
			logLevel = zap.ErrorLevel
		} else if duration > 100*time.Millisecond && logLevel < zap.WarnLevel {
			logLevel = zap.WarnLevel
		}

		var allocDelta int64
		if endAlloc >= startAlloc {
			allocDelta = int64(endAlloc - startAlloc)
		}

		fields := []zap.Field{
			zap.String("request_id", requestID),
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status_code", statusCode),
			zap.Duration("duration", duration),
			zap.Int64("duration_ms", duration.Milliseconds()),
			zap.Int("response_size", len(c.Response().Body())),
			zap.Uint64("alloc_bytes", endAlloc),
			zap.Int64("alloc_delta_bytes", allocDelta),
			zap.Int("num_goroutines", runtime.NumGoroutine()),
		}

		if err != nil {
			fields = append(fields, zap.Error(err))
			logLevel = zap.ErrorLevel
		}

		Logger.Log(logLevel, "request_completed", fields...)
		FinalizeRequestMetrics(c.UserContext(), statusCode, duration, len(c.Response().Body()), allocDelta)

		return err
	}
}
