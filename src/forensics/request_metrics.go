package forensics

import (
	"context"
	"errors"
	"net"
	"net/url"
	"skycrypt/src/security"
	"slices"
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
	// HTTP outcomes, so errors and rate limits can be told apart per host.
	Errors     int `json:"errors,omitempty"`
	RateLimits int `json:"rate_limits,omitempty"`
	Timeouts   int `json:"timeouts,omitempty"`
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
	RequestID             string  `json:"request_id"`
	Method                string  `json:"method"`
	Path                  string  `json:"path"`
	Route                 string  `json:"route"`
	CacheStatus           string  `json:"cache_status"`
	ResponseCacheStatus   string  `json:"response_cache_status,omitempty"`
	ResponseCacheEndpoint string  `json:"response_cache_endpoint,omitempty"`
	StatusCode            int     `json:"status_code"`
	DurationMs            int64   `json:"duration_ms"`
	ResponseSize          int     `json:"response_size"`
	AllocDeltaBytes       int64   `json:"alloc_delta_bytes"`
	CacheHits             int     `json:"cache_hits"`
	CacheMisses           int     `json:"cache_misses"`
	CacheSets             int     `json:"cache_sets"`
	RedisCalls            int     `json:"redis_calls"`
	RedisMs               float64 `json:"redis_ms"`
	HTTPCalls             int     `json:"http_calls"`
	HTTPMs                float64 `json:"http_ms"`
	HTTPErrors            int     `json:"http_errors"`
	HTTPRateLimits        int     `json:"http_rate_limits"`
	PostgresCalls         int     `json:"postgres_calls"`
	PostgresMs            float64 `json:"postgres_ms"`
	// UpstreamWallMs is how long the request had at least one upstream call open. HTTPMs adds calls that ran side
	// by side, so it can exceed the whole request; this can't. UpstreamWallByHost splits it per host.
	UpstreamWallMs     float64            `json:"upstream_wall_ms"`
	UpstreamWallByHost map[string]float64 `json:"upstream_wall_by_host,omitempty"`
	// BuildWaitMs is time spent waiting for a free build slot; SharedWaitMs is time spent waiting on a build or
	// lookup another request had already started.
	BuildWaitMs          float64              `json:"build_wait_ms,omitempty"`
	SharedWaitMs         float64              `json:"shared_wait_ms,omitempty"`
	Dependencies         []DependencySummary  `json:"dependencies"`
	RepeatedDependencies []RepeatedDependency `json:"repeated_dependencies"`
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

	postgresCalls int
	postgresMs    float64

	httpIntervals []httpInterval
	buildWaitMs   float64
	sharedWaitMs  float64

	dependencies map[string]*DependencySummary
}

// httpInterval is when one upstream call was open.
type httpInterval struct {
	start, end time.Time
	host       string
}

// RecordBuildWait records time a request waited for a free build slot.
func RecordBuildWait(ctx context.Context, duration time.Duration) {
	if metrics := MetricsFromContext(ctx); metrics != nil {
		metrics.mu.Lock()
		metrics.buildWaitMs += durationToMs(duration)
		metrics.mu.Unlock()
	}
}

// RecordSharedWait records time a request waited on work another request had started.
func RecordSharedWait(ctx context.Context, duration time.Duration) {
	if metrics := MetricsFromContext(ctx); metrics != nil {
		metrics.mu.Lock()
		metrics.sharedWaitMs += durationToMs(duration)
		metrics.mu.Unlock()
	}
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
	end := time.Now()
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.httpCalls++
	metrics.httpMs += durationToMs(duration)
	metrics.httpIntervals = append(metrics.httpIntervals, httpInterval{start: end.Add(-duration), end: end, host: dependencyHost(name)})
	dep := metrics.addDependencyLocked("http", method, name, duration)
	if err != nil || statusCode >= 500 {
		metrics.httpErrors++
		dep.Errors++
	}
	if statusCode == 429 {
		metrics.httpRateLimits++
		dep.RateLimits++
	}
	if isTimeout(err) {
		dep.Timeouts++
	}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout())
}

// dependencyHost is the host part of an HTTP dependency name (host + path).
func dependencyHost(name string) string {
	if i := strings.IndexByte(name, '/'); i >= 0 {
		return name[:i]
	}
	return name
}

// upstreamWall returns how long at least one call was open, overall and per host: overlapping calls count once.
func upstreamWall(intervals []httpInterval) (float64, map[string]float64) {
	if len(intervals) == 0 {
		return 0, nil
	}
	byHost := map[string][]httpInterval{}
	for _, interval := range intervals {
		byHost[interval.host] = append(byHost[interval.host], interval)
	}
	perHost := make(map[string]float64, len(byHost))
	for host, hostIntervals := range byHost {
		perHost[host] = unionMs(hostIntervals)
	}
	return unionMs(intervals), perHost
}

func unionMs(intervals []httpInterval) float64 {
	sorted := slices.Clone(intervals)
	slices.SortFunc(sorted, func(a, b httpInterval) int { return a.start.Compare(b.start) })
	var total time.Duration
	start, end := sorted[0].start, sorted[0].end
	for _, interval := range sorted[1:] {
		if interval.start.After(end) {
			total += end.Sub(start)
			start, end = interval.start, interval.end
			continue
		}
		if interval.end.After(end) {
			end = interval.end
		}
	}
	total += end.Sub(start)
	return durationToMs(total)
}

func RecordPostgresDependency(ctx context.Context, table, operation string, duration time.Duration, err error) {
	metrics := MetricsFromContext(ctx)
	if metrics == nil {
		return
	}

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.postgresCalls++
	metrics.postgresMs += durationToMs(duration)
	metrics.addDependencyLocked("postgres", operation, table, duration)
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
	wallMs, wallByHost := upstreamWall(metrics.httpIntervals)
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
		PostgresCalls:         metrics.postgresCalls,
		PostgresMs:            metrics.postgresMs,
		UpstreamWallMs:        wallMs,
		UpstreamWallByHost:    wallByHost,
		BuildWaitMs:           metrics.buildWaitMs,
		SharedWaitMs:          metrics.sharedWaitMs,
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
			zap.Int("postgres_calls", summary.PostgresCalls),
			zap.Float64("postgres_ms", summary.PostgresMs),
			zap.Float64("upstream_wall_ms", summary.UpstreamWallMs),
			zap.Any("upstream_wall_by_host", summary.UpstreamWallByHost),
			zap.Float64("build_wait_ms", summary.BuildWaitMs),
			zap.Float64("shared_wait_ms", summary.SharedWaitMs),
			zap.Any("dependencies", summary.Dependencies),
			zap.Any("repeated_dependencies", summary.RepeatedDependencies),
		)
	}

	return summary
}

func (m *RequestMetrics) addDependencyLocked(kind, operation, name string, duration time.Duration) *DependencySummary {
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
	return dep
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
	redactedURL := RedactURL(rawURL)
	if redactedURL == "[REDACTED INVALID URL]" {
		return "[REDACTED INVALID URL]"
	}
	parsed, err := url.Parse(redactedURL)
	if err != nil {
		return "[REDACTED INVALID URL]"
	}
	if parsed.Host == "" {
		return redactedURL
	}
	return parsed.Host + parsed.Path
}

func RedactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "[REDACTED INVALID URL]"
	}
	query := parsed.Query()
	for key := range query {
		switch strings.ToLower(key) {
		case "key", "api_key", "apikey", "token":
			query.Set(key, "REDACTED")
		}
	}
	parsed.RawQuery = query.Encode()
	return security.RedactString(parsed.String())
}

func durationToMs(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000.0
}

func isRepeatedDependency(dep *DependencySummary) bool {
	switch dep.Kind {
	case "redis":
		return (dep.Operation == "GET" || dep.Operation == "MGET") && dep.Count >= 3
	case "http", "postgres":
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
	case "postgres":
		return "Repeated Postgres operation. Consider batching, aggregation, or index review."
	default:
		return "Repeated dependency calls detected."
	}
}
