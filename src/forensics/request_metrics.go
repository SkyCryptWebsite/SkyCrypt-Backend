package forensics

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type requestMetricsKey struct{}

type DependencySummary struct {
	Kind      string  `json:"kind"`
	Operation string  `json:"operation"`
	Name      string  `json:"name"`
	Count     int     `json:"count"`
	TotalMs   float64 `json:"total_ms"`
	AvgMs     float64 `json:"avg_ms"`
}

type RepeatedDependency struct {
	Kind       string  `json:"kind"`
	Operation  string  `json:"operation"`
	Name       string  `json:"name"`
	Count      int     `json:"count"`
	TotalMs    float64 `json:"total_ms"`
	AvgMs      float64 `json:"avg_ms"`
	Suggestion string  `json:"suggestion"`
}

type RequestSummary struct {
	RequestID             string               `json:"request_id"`
	Method                string               `json:"method"`
	Path                  string               `json:"path"`
	Route                 string               `json:"route"`
	CacheStatus           string               `json:"cache_status"`
	ResponseCacheStatus   string               `json:"response_cache_status,omitempty"`
	ResponseCacheEndpoint string               `json:"response_cache_endpoint,omitempty"`
	StatusCode            int                  `json:"status_code"`
	DurationMs            int64                `json:"duration_ms"`
	ResponseSize          int                  `json:"response_size"`
	AllocDeltaBytes       int64                `json:"alloc_delta_bytes"`
	CacheHits             int                  `json:"cache_hits"`
	CacheMisses           int                  `json:"cache_misses"`
	CacheSets             int                  `json:"cache_sets"`
	RedisCalls            int                  `json:"redis_calls"`
	RedisMs               float64              `json:"redis_ms"`
	HTTPCalls             int                  `json:"http_calls"`
	HTTPMs                float64              `json:"http_ms"`
	HTTPErrors            int                  `json:"http_errors"`
	HTTPRateLimits        int                  `json:"http_rate_limits"`
	MongoCalls            int                  `json:"mongo_calls"`
	MongoMs               float64              `json:"mongo_ms"`
	Dependencies          []DependencySummary  `json:"dependencies"`
	RepeatedDependencies  []RepeatedDependency `json:"repeated_dependencies"`
}

type RequestMetrics struct {
	mu sync.Mutex

	requestID string
	method    string
	path      string
	route     string

	cacheHits   int
	cacheMisses int
	cacheSets   int

	responseCacheStatus   string
	responseCacheEndpoint string

	redisCalls int
	redisMs    float64

	httpCalls      int
	httpMs         float64
	httpErrors     int
	httpRateLimits int

	mongoCalls int
	mongoMs    float64

	dependencies map[string]*DependencySummary
}

func RecordResponseCache(ctx context.Context, endpoint, status string) {
	metrics := MetricsFromContext(ctx)
	if metrics == nil {
		return
	}

	metrics.mu.Lock()
	metrics.responseCacheEndpoint = endpoint
	metrics.responseCacheStatus = status
	metrics.mu.Unlock()
}

func NewRequestMetrics(requestID, method, path string) *RequestMetrics {
	return &RequestMetrics{
		requestID:    requestID,
		method:       method,
		path:         path,
		route:        path,
		dependencies: make(map[string]*DependencySummary),
	}
}

func WithRequestMetrics(ctx context.Context, metrics *RequestMetrics) context.Context {
	return context.WithValue(ctx, requestMetricsKey{}, metrics)
}

func MetricsFromContext(ctx context.Context) *RequestMetrics {
	if ctx == nil {
		return nil
	}
	metrics, _ := ctx.Value(requestMetricsKey{}).(*RequestMetrics)
	return metrics
}

func SetRequestRoute(ctx context.Context, route string) {
	if metrics := MetricsFromContext(ctx); metrics != nil && route != "" {
		metrics.mu.Lock()
		metrics.route = route
		metrics.mu.Unlock()
	}
}

func RecordRedisDependency(ctx context.Context, operation, key, cacheResult string, duration time.Duration, err error) {
	metrics := MetricsFromContext(ctx)
	if metrics == nil {
		return
	}

	group := RedisKeyGroup(key)
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.redisCalls++
	metrics.redisMs += durationToMs(duration)

	switch cacheResult {
	case "hit":
		metrics.cacheHits++
	case "miss":
		metrics.cacheMisses++
	case "set":
		metrics.cacheSets++
	case "error":
		// Redis errors are dependency failures, not cache misses.
	}

	name := group
	if group == "" {
		name = key
	}
	metrics.addDependencyLocked("redis", operation, name, duration)
}

func RecordRedisBatchDependency(ctx context.Context, operation, keyGroup string, hits, misses int, duration time.Duration, err error) {
	metrics := MetricsFromContext(ctx)
	if metrics == nil {
		return
	}

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.redisCalls++
	metrics.redisMs += durationToMs(duration)
	if err == nil {
		metrics.cacheHits += hits
		metrics.cacheMisses += misses
	}

	metrics.addDependencyLocked("redis", operation, keyGroup, duration)
}

func RecordRedisBatchSetDependency(ctx context.Context, operation, keyGroup string, keyCount int, duration time.Duration, err error) {
	metrics := MetricsFromContext(ctx)
	if metrics == nil {
		return
	}

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.redisCalls++
	metrics.redisMs += durationToMs(duration)
	if err == nil {
		metrics.cacheSets += keyCount
	}

	metrics.addDependencyLocked("redis", operation, keyGroup, duration)
}

func RecordHTTPDependency(ctx context.Context, method, rawURL string, statusCode int, duration time.Duration, err error) {
	metrics := MetricsFromContext(ctx)
	if metrics == nil {
		return
	}

	name := HTTPDependencyName(rawURL)
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.httpCalls++
	metrics.httpMs += durationToMs(duration)
	if err != nil || statusCode >= 500 {
		metrics.httpErrors++
	}
	if statusCode == 429 {
		metrics.httpRateLimits++
	}

	metrics.addDependencyLocked("http", method, name, duration)
}

func RecordMongoDependency(ctx context.Context, collection, operation string, duration time.Duration, err error) {
	metrics := MetricsFromContext(ctx)
	if metrics == nil {
		return
	}

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.mongoCalls++
	metrics.mongoMs += durationToMs(duration)
	metrics.addDependencyLocked("mongo", operation, collection, duration)
}

func FinalizeRequestMetrics(ctx context.Context, statusCode int, duration time.Duration, responseSize int, allocDelta int64) RequestSummary {
	metrics := MetricsFromContext(ctx)
	if metrics == nil {
		return RequestSummary{
			CacheStatus:     "not_cacheable",
			StatusCode:      statusCode,
			DurationMs:      duration.Milliseconds(),
			ResponseSize:    responseSize,
			AllocDeltaBytes: allocDelta,
		}
	}

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	deps := make([]DependencySummary, 0, len(metrics.dependencies))
	repeated := make([]RepeatedDependency, 0)
	for _, dep := range metrics.dependencies {
		if dep.Count > 0 {
			dep.AvgMs = dep.TotalMs / float64(dep.Count)
		}
		deps = append(deps, *dep)

		if isRepeatedDependency(dep) {
			repeated = append(repeated, RepeatedDependency{
				Kind:       dep.Kind,
				Operation:  dep.Operation,
				Name:       dep.Name,
				Count:      dep.Count,
				TotalMs:    dep.TotalMs,
				AvgMs:      dep.AvgMs,
				Suggestion: dependencySuggestion(dep.Kind, dep.Name),
			})
		}
	}

	cacheStatus := classifyCacheStatus(metrics.cacheHits, metrics.cacheMisses, metrics.httpCalls)
	summary := RequestSummary{
		RequestID:             metrics.requestID,
		Method:                metrics.method,
		Path:                  metrics.path,
		Route:                 metrics.route,
		CacheStatus:           cacheStatus,
		ResponseCacheStatus:   metrics.responseCacheStatus,
		ResponseCacheEndpoint: metrics.responseCacheEndpoint,
		StatusCode:            statusCode,
		DurationMs:            duration.Milliseconds(),
		ResponseSize:          responseSize,
		AllocDeltaBytes:       allocDelta,
		CacheHits:             metrics.cacheHits,
		CacheMisses:           metrics.cacheMisses,
		CacheSets:             metrics.cacheSets,
		RedisCalls:            metrics.redisCalls,
		RedisMs:               metrics.redisMs,
		HTTPCalls:             metrics.httpCalls,
		HTTPMs:                metrics.httpMs,
		HTTPErrors:            metrics.httpErrors,
		HTTPRateLimits:        metrics.httpRateLimits,
		MongoCalls:            metrics.mongoCalls,
		MongoMs:               metrics.mongoMs,
		Dependencies:          deps,
		RepeatedDependencies:  repeated,
	}

	if Logger != nil {
		Logger.Info("forensic_request_summary",
			zap.String("request_id", summary.RequestID),
			zap.String("method", summary.Method),
			zap.String("path", summary.Path),
			zap.String("route", summary.Route),
			zap.String("cache_status", summary.CacheStatus),
			zap.String("response_cache_status", summary.ResponseCacheStatus),
			zap.String("response_cache_endpoint", summary.ResponseCacheEndpoint),
			zap.Int("status_code", summary.StatusCode),
			zap.Int64("duration_ms", summary.DurationMs),
			zap.Int("response_size", summary.ResponseSize),
			zap.Int64("alloc_delta_bytes", summary.AllocDeltaBytes),
			zap.Int("cache_hits", summary.CacheHits),
			zap.Int("cache_misses", summary.CacheMisses),
			zap.Int("cache_sets", summary.CacheSets),
			zap.Int("redis_calls", summary.RedisCalls),
			zap.Float64("redis_ms", summary.RedisMs),
			zap.Int("http_calls", summary.HTTPCalls),
			zap.Float64("http_ms", summary.HTTPMs),
			zap.Int("http_errors", summary.HTTPErrors),
			zap.Int("http_rate_limits", summary.HTTPRateLimits),
			zap.Int("mongo_calls", summary.MongoCalls),
			zap.Float64("mongo_ms", summary.MongoMs),
			zap.Any("dependencies", summary.Dependencies),
			zap.Any("repeated_dependencies", summary.RepeatedDependencies),
		)
	}

	return summary
}

func (m *RequestMetrics) addDependencyLocked(kind, operation, name string, duration time.Duration) {
	key := kind + "|" + operation + "|" + name
	dep := m.dependencies[key]
	if dep == nil {
		dep = &DependencySummary{
			Kind:      kind,
			Operation: operation,
			Name:      name,
		}
		m.dependencies[key] = dep
	}
	dep.Count++
	dep.TotalMs += durationToMs(duration)
}

func classifyCacheStatus(hits, misses, httpCalls int) string {
	if hits+misses == 0 {
		return "not_cacheable"
	}
	if misses == 0 && httpCalls == 0 {
		return "all_cached"
	}
	if hits > 0 {
		return "mixed"
	}
	return "cold"
}

func RedisKeyGroup(key string) string {
	if key == "" {
		return ""
	}
	if idx := strings.Index(key, ":"); idx > 0 {
		return key[:idx]
	}
	return key
}

func HTTPDependencyName(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}
	return parsed.Host + parsed.Path
}

func RedactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	for _, key := range []string{"key", "api_key", "apikey", "token"} {
		if query.Has(key) {
			query.Set(key, "REDACTED")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func durationToMs(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000.0
}

func isRepeatedDependency(dep *DependencySummary) bool {
	switch dep.Kind {
	case "redis":
		return (dep.Operation == "GET" || dep.Operation == "MGET") && dep.Count >= 3
	case "http", "mongo":
		return dep.Count >= 2
	default:
		return false
	}
}

func dependencySuggestion(kind, name string) string {
	switch kind {
	case "redis":
		if name == "username" || name == "uuid" || strings.HasPrefix(name, "mowojang") {
			return "Repeated identity cache lookups. Consider request-local memoization or batching co-op member names."
		}
		return "Repeated Redis access. Consider request-local memoization or coalescing related reads."
	case "http":
		return "Repeated upstream calls. Consider singleflight, prefetching, or a broader cache key."
	case "mongo":
		return "Repeated Mongo operation. Consider batching, aggregation, or index review."
	default:
		return "Repeated dependency calls detected."
	}
}
