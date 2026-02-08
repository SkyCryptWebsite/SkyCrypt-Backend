package forensics

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// InstrumentedHTTPClient wraps an http.Client with forensic logging.
type InstrumentedHTTPClient struct {
	client *http.Client
	logger *zap.Logger
}

// NewInstrumentedHTTPClient creates an instrumented HTTP client wrapper.
func NewInstrumentedHTTPClient(client *http.Client) *InstrumentedHTTPClient {
	return &InstrumentedHTTPClient{
		client: client,
		logger: Logger,
	}
}

func (ihc *InstrumentedHTTPClient) Get(url string) (*http.Response, error) {
	start := time.Now()

	ihc.logger.Debug("http_request_start",
		zap.String("method", "GET"),
		zap.String("url", url),
	)

	resp, err := ihc.client.Get(url)
	duration := time.Since(start)

	if err != nil {
		ihc.logger.Error("http_request_error",
			zap.String("method", "GET"),
			zap.String("url", url),
			zap.Error(err),
			zap.Duration("duration", duration),
			zap.Int64("duration_ms", duration.Milliseconds()),
		)
		return nil, err
	}

	fields := []zap.Field{
		zap.String("method", "GET"),
		zap.String("url", url),
		zap.Int("status_code", resp.StatusCode),
		zap.Int64("content_length", resp.ContentLength),
		zap.Duration("duration", duration),
		zap.Int64("duration_ms", duration.Milliseconds()),
	}

	ihc.logger.Info("http_request_completed", fields...)

	if resp.StatusCode == 429 {
		ihc.logger.Error("rate_limit_exceeded",
			zap.String("url", url),
			zap.String("action_required", "Implement rate limiting or request queuing"),
		)
	} else if resp.StatusCode >= 500 {
		ihc.logger.Error("http_upstream_error",
			zap.String("url", url),
			zap.Int("status_code", resp.StatusCode),
		)
	} else if resp.StatusCode >= 400 {
		ihc.logger.Warn("http_client_error",
			zap.String("url", url),
			zap.Int("status_code", resp.StatusCode),
		)
	}

	ihc.flagSlowHTTP("GET", url, duration)
	return resp, nil
}

func (ihc *InstrumentedHTTPClient) Do(req *http.Request) (*http.Response, error) {
	start := time.Now()

	ihc.logger.Debug("http_request_start",
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
	)

	resp, err := ihc.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		ihc.logger.Error("http_request_error",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Error(err),
			zap.Duration("duration", duration),
			zap.Int64("duration_ms", duration.Milliseconds()),
		)
		return nil, err
	}

	ihc.logger.Info("http_request_completed",
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
		zap.Int("status_code", resp.StatusCode),
		zap.Int64("content_length", resp.ContentLength),
		zap.Duration("duration", duration),
		zap.Int64("duration_ms", duration.Milliseconds()),
	)

	if resp.StatusCode == 429 {
		ihc.logger.Error("rate_limit_exceeded",
			zap.String("url", req.URL.String()),
			zap.String("method", req.Method),
		)
	}

	ihc.flagSlowHTTP(req.Method, req.URL.String(), duration)
	return resp, nil
}

// RoundTripper returns an http.RoundTripper that instruments all requests going
// through the underlying transport. This can be injected into an http.Client
// to instrument all traffic without changing call sites.
func InstrumentedRoundTripper(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &loggingRoundTripper{base: base}
}

type loggingRoundTripper struct {
	base http.RoundTripper
}

func (lrt *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	Logger.Debug("http_roundtrip_start",
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
		zap.String("host", req.URL.Host),
	)

	resp, err := lrt.base.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		Logger.Error("http_roundtrip_error",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
		return nil, err
	}

	logLevel := zap.InfoLevel
	if resp.StatusCode >= 500 {
		logLevel = zap.ErrorLevel
	} else if resp.StatusCode == 429 {
		logLevel = zap.ErrorLevel
	} else if resp.StatusCode >= 400 {
		logLevel = zap.WarnLevel
	}

	Logger.Log(logLevel, "http_roundtrip_completed",
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
		zap.String("host", req.URL.Host),
		zap.Int("status_code", resp.StatusCode),
		zap.Int64("content_length", resp.ContentLength),
		zap.Duration("duration", duration),
		zap.Int64("duration_ms", duration.Milliseconds()),
	)

	if duration > 100*time.Millisecond {
		Logger.Warn("slow_http_request",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Duration("duration", duration),
			zap.String("suggestion", "External API is slow. Consider caching or async processing"),
		)
	}

	if duration > 1*time.Second {
		Logger.Error("critical_slow_http_request",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Duration("duration", duration),
			zap.String("action_required", "Implement timeout, circuit breaker, or async queue"),
		)
	}

	return resp, nil
}

func (ihc *InstrumentedHTTPClient) flagSlowHTTP(method, url string, duration time.Duration) {
	if duration > 100*time.Millisecond {
		ihc.logger.Warn("slow_http_request",
			zap.String("method", method),
			zap.String("url", url),
			zap.Duration("duration", duration),
			zap.String("suggestion", "External API is slow. Consider caching or async processing"),
		)
	}

	if duration > 1*time.Second {
		ihc.logger.Error("critical_slow_http_request",
			zap.String("method", method),
			zap.String("url", url),
			zap.Duration("duration", duration),
			zap.String("action_required", "Implement timeout, circuit breaker, or async queue"),
		)
	}
}
