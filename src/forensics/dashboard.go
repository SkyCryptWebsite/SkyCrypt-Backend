package forensics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

var startTime = time.Now()

const (
	defaultDashboardWindow = 6 * time.Hour
	defaultDashboardLimit  = 50000
)

func DashboardHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		report := generateDashboardReport(dashboardOptionsFromCtx(c))
		return c.JSON(report)
	}
}

func DashboardHTMLHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		opts := dashboardOptionsFromCtx(c)
		report := generateDashboardReport(opts)
		html := renderDashboardHTML(report, opts)
		c.Set("Content-Type", "text/html")
		return c.SendString(html)
	}
}

type dashboardOptions struct {
	Window time.Duration
	Limit  int
}

type dashboardReport struct {
	GeneratedAt             string                    `json:"generated_at"`
	Uptime                  string                    `json:"uptime"`
	Window                  string                    `json:"window"`
	Limit                   int                       `json:"limit"`
	Runtime                 runtimeReport             `json:"runtime"`
	CacheStats              cacheReport               `json:"cache_stats"`
	TopSpans                []spanReportEntry         `json:"top_operations"`
	SlowestRequests         []slowRequestEntry        `json:"slowest_requests"`
	CacheLatencyByEndpoint  []cacheLatencyEntry       `json:"cache_latency_by_endpoint"`
	ResponseCacheByEndpoint []responseCacheEntry      `json:"response_cache_by_endpoint"`
	RecentRequests          []recentRequestEntry      `json:"recent_requests"`
	DependencySummary       []dependencyReportEntry   `json:"dependency_summary"`
	RepeatedDependencyCalls []repeatedDependencyEntry `json:"repeated_dependency_calls"`
	CacheKeySummary         []dependencyReportEntry   `json:"cache_key_summary"`
	UpstreamSummary         []dependencyReportEntry   `json:"upstream_summary"`
	RedisSummary            []dependencyReportEntry   `json:"redis_summary"`
	MongoSummary            []dependencyReportEntry   `json:"mongo_summary"`
	LogStats                logStatsReport            `json:"log_stats"`
}

type slowRequestEntry struct {
	Rank         int     `json:"rank"`
	RequestID    string  `json:"request_id"`
	Method       string  `json:"method"`
	Path         string  `json:"path"`
	StatusCode   int     `json:"status_code"`
	DurationMs   int64   `json:"duration_ms"`
	ResponseSize int     `json:"response_size"`
	Timestamp    string  `json:"timestamp"`
	DurationSec  float64 `json:"duration_sec"`
}

type runtimeReport struct {
	Goroutines  int    `json:"goroutines"`
	HeapAllocMB uint64 `json:"heap_alloc_mb"`
	HeapSysMB   uint64 `json:"heap_sys_mb"`
	HeapObjects uint64 `json:"heap_objects"`
	SysMemMB    uint64 `json:"sys_mem_mb"`
	NumGC       uint32 `json:"num_gc"`
	LastPauseMs uint64 `json:"last_gc_pause_ms"`
	GoVersion   string `json:"go_version"`
	NumCPU      int    `json:"num_cpu"`
	PID         int    `json:"pid"`
}

type cacheReport struct {
	Hits               uint64  `json:"hits"`
	Misses             uint64  `json:"misses"`
	HitRate            float64 `json:"hit_rate_percent"`
	ResponseRAMHits    uint64  `json:"response_ram_hits"`
	ResponseRedisHits  uint64  `json:"response_redis_hits"`
	ResponseCold       uint64  `json:"response_cold"`
	ResponseRAMHitRate float64 `json:"response_ram_hit_rate_percent"`
	ResponseHitRate    float64 `json:"response_hit_rate_percent"`
	AllCachedAvgMs     float64 `json:"all_cached_avg_ms"`
	AllCachedP95Ms     float64 `json:"all_cached_p95_ms"`
	MixedAvgMs         float64 `json:"mixed_avg_ms"`
	MixedP95Ms         float64 `json:"mixed_p95_ms"`
	ColdAvgMs          float64 `json:"cold_avg_ms"`
	ColdP95Ms          float64 `json:"cold_p95_ms"`
	UpstreamErrors     int     `json:"upstream_errors"`
	UpstreamRateLimits int     `json:"upstream_rate_limits"`
}

type spanReportEntry struct {
	Rank       int     `json:"rank"`
	Operation  string  `json:"operation"`
	CallCount  uint64  `json:"call_count"`
	TotalMs    float64 `json:"total_ms"`
	AvgMs      float64 `json:"avg_ms"`
	MinMs      float64 `json:"min_ms"`
	MaxMs      float64 `json:"max_ms"`
	PctOfTotal float64 `json:"pct_of_total"`
	Critical   bool    `json:"critical"`
}

type cacheLatencyEntry struct {
	Route          string  `json:"route"`
	Method         string  `json:"method"`
	CacheStatus    string  `json:"cache_status"`
	Count          int     `json:"count"`
	AvgMs          float64 `json:"avg_ms"`
	P50Ms          float64 `json:"p50_ms"`
	P95Ms          float64 `json:"p95_ms"`
	MaxMs          float64 `json:"max_ms"`
	AvgRedisMs     float64 `json:"avg_redis_ms"`
	AvgHTTPMs      float64 `json:"avg_http_ms"`
	AvgMongoMs     float64 `json:"avg_mongo_ms"`
	AvgCacheHits   float64 `json:"avg_cache_hits"`
	AvgCacheMisses float64 `json:"avg_cache_misses"`
}

type responseCacheEntry struct {
	Endpoint   string  `json:"endpoint"`
	Route      string  `json:"route"`
	Method     string  `json:"method"`
	Count      int     `json:"count"`
	RAMHits    int     `json:"ram_hits"`
	RedisHits  int     `json:"redis_hits"`
	Cold       int     `json:"cold"`
	RAMHitRate float64 `json:"ram_hit_rate_percent"`
	HitRate    float64 `json:"hit_rate_percent"`
	AvgMs      float64 `json:"avg_ms"`
	P50Ms      float64 `json:"p50_ms"`
	P95Ms      float64 `json:"p95_ms"`
	MaxMs      float64 `json:"max_ms"`
}

type recentRequestEntry struct {
	RequestID           string  `json:"request_id"`
	Method              string  `json:"method"`
	Path                string  `json:"path"`
	Route               string  `json:"route"`
	CacheStatus         string  `json:"cache_status"`
	ResponseCacheStatus string  `json:"response_cache_status,omitempty"`
	StatusCode          int     `json:"status_code"`
	DurationMs          int64   `json:"duration_ms"`
	CacheHits           int     `json:"cache_hits"`
	CacheMisses         int     `json:"cache_misses"`
	RedisCalls          int     `json:"redis_calls"`
	RedisMs             float64 `json:"redis_ms"`
	HTTPCalls           int     `json:"http_calls"`
	HTTPMs              float64 `json:"http_ms"`
	MongoCalls          int     `json:"mongo_calls"`
	MongoMs             float64 `json:"mongo_ms"`
	Timestamp           string  `json:"timestamp"`
}

type dependencyReportEntry struct {
	Kind      string  `json:"kind"`
	Operation string  `json:"operation"`
	Name      string  `json:"name"`
	Count     int     `json:"count"`
	TotalMs   float64 `json:"total_ms"`
	AvgMs     float64 `json:"avg_ms"`
}

type repeatedDependencyEntry struct {
	Route      string  `json:"route"`
	RequestID  string  `json:"request_id"`
	Kind       string  `json:"kind"`
	Operation  string  `json:"operation"`
	Name       string  `json:"name"`
	Count      int     `json:"count"`
	TotalMs    float64 `json:"total_ms"`
	AvgMs      float64 `json:"avg_ms"`
	Suggestion string  `json:"suggestion"`
}

type logStatsReport struct {
	LinesProcessed int    `json:"lines_processed"`
	LogFileSize    string `json:"log_file_size"`
	ParseTimeMs    int64  `json:"parse_time_ms"`
}

type logLine struct {
	Msg          string  `json:"msg"`
	Level        string  `json:"level"`
	Timestamp    string  `json:"timestamp"`
	Span         string  `json:"span,omitempty"`
	DurationUs   int64   `json:"duration_us,omitempty"`
	Duration     float64 `json:"duration,omitempty"`
	DurationMs   int64   `json:"duration_ms,omitempty"`
	Method       string  `json:"method,omitempty"`
	Path         string  `json:"path,omitempty"`
	Route        string  `json:"route,omitempty"`
	StatusCode   int     `json:"status_code,omitempty"`
	ResponseSize int     `json:"response_size,omitempty"`
	Key          string  `json:"key,omitempty"`
	RequestID    string  `json:"request_id,omitempty"`

	CacheStatus           string                 `json:"cache_status,omitempty"`
	ResponseCacheStatus   string                 `json:"response_cache_status,omitempty"`
	ResponseCacheEndpoint string                 `json:"response_cache_endpoint,omitempty"`
	CacheHits             int                    `json:"cache_hits,omitempty"`
	CacheMisses           int                    `json:"cache_misses,omitempty"`
	CacheSets             int                    `json:"cache_sets,omitempty"`
	RedisCalls            int                    `json:"redis_calls,omitempty"`
	RedisMs               float64                `json:"redis_ms,omitempty"`
	HTTPCalls             int                    `json:"http_calls,omitempty"`
	HTTPMs                float64                `json:"http_ms,omitempty"`
	HTTPErrors            int                    `json:"http_errors,omitempty"`
	HTTPRateLimits        int                    `json:"http_rate_limits,omitempty"`
	MongoCalls            int                    `json:"mongo_calls,omitempty"`
	MongoMs               float64                `json:"mongo_ms,omitempty"`
	AllocDeltaBytes       int64                  `json:"alloc_delta_bytes,omitempty"`
	Dependencies          []DependencySummary    `json:"dependencies,omitempty"`
	RepeatedDeps          []RepeatedDependency   `json:"repeated_dependencies,omitempty"`
	Extra                 map[string]interface{} `json:"-"`
}

type spanAgg struct {
	Count   uint64
	TotalUs int64
	MinUs   int64
	MaxUs   int64
}

type latencyAgg struct {
	Route       string
	Method      string
	CacheStatus string
	Durations   []int64
	RedisMs     float64
	HTTPMs      float64
	MongoMs     float64
	CacheHits   int
	CacheMisses int
}

type responseCacheAgg struct {
	Endpoint  string
	Route     string
	Method    string
	Durations []int64
	RAMHits   int
	RedisHits int
	Cold      int
}

func dashboardOptionsFromCtx(c *fiber.Ctx) dashboardOptions {
	opts := dashboardOptions{Window: defaultDashboardWindow, Limit: defaultDashboardLimit}
	if windowRaw := c.Query("window"); windowRaw != "" {
		if window, err := time.ParseDuration(windowRaw); err == nil && window > 0 {
			opts.Window = window
		}
	}
	if limitRaw := c.Query("limit"); limitRaw != "" {
		if limit, err := strconv.Atoi(limitRaw); err == nil && limit > 0 {
			opts.Limit = limit
		}
	}
	return opts
}

func generateDashboardReport(opts dashboardOptions) dashboardReport {
	report := dashboardReport{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Uptime:      time.Since(startTime).Round(time.Second).String(),
		Window:      opts.Window.String(),
		Limit:       opts.Limit,
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	lastPauseMs := uint64(0)
	if m.NumGC > 0 {
		lastPauseMs = m.PauseNs[(m.NumGC+255)%256] / 1_000_000
	}
	report.Runtime = runtimeReport{
		Goroutines:  runtime.NumGoroutine(),
		HeapAllocMB: m.HeapAlloc / 1024 / 1024,
		HeapSysMB:   m.HeapSys / 1024 / 1024,
		HeapObjects: m.HeapObjects,
		SysMemMB:    m.Sys / 1024 / 1024,
		NumGC:       m.NumGC,
		LastPauseMs: lastPauseMs,
		GoVersion:   runtime.Version(),
		NumCPU:      runtime.NumCPU(),
		PID:         os.Getpid(),
	}

	parseLogFile(&report, opts)
	return report
}

func parseLogFile(report *dashboardReport, opts dashboardOptions) {
	parseStart := time.Now()
	f, err := os.Open("logs/app.log")
	if err != nil {
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("Error closing log file: %v\n", err)
		}
	}()

	info, _ := f.Stat()
	if info != nil {
		sizeMB := float64(info.Size()) / 1024 / 1024
		if sizeMB >= 1 {
			report.LogStats.LogFileSize = fmt.Sprintf("%.1f MB", sizeMB)
		} else {
			report.LogStats.LogFileSize = fmt.Sprintf("%.0f KB", float64(info.Size())/1024)
		}
	}

	cutoff := time.Now().Add(-opts.Window)
	spans := make(map[string]*spanAgg)
	latency := make(map[string]*latencyAgg)
	responseCache := make(map[string]*responseCacheAgg)
	deps := make(map[string]*dependencyReportEntry)
	cacheDeps := make(map[string]*dependencyReportEntry)
	upstreamDeps := make(map[string]*dependencyReportEntry)
	redisDeps := make(map[string]*dependencyReportEntry)
	mongoDeps := make(map[string]*dependencyReportEntry)
	var allCachedDurations, mixedDurations, coldDurations []int64
	var cacheHits, cacheMisses uint64
	var legacyCacheHits, legacyCacheMisses uint64
	var responseRAMHits, responseRedisHits, responseCold uint64
	var responseRAMEligibleHits, responseRAMEligibleTotal uint64
	var slowest []slowRequestEntry
	var slowestMinMs int64
	var recent []recentRequestEntry
	summaryCount := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 512*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}

		var entry logLine
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		report.LogStats.LinesProcessed++

		if entry.Timestamp != "" {
			if ts, err := parseLogTimestamp(entry.Timestamp); err == nil && ts.Before(cutoff) {
				continue
			}
		}

		switch entry.Msg {
		case "forensic_request_summary":
			summaryCount++
			if opts.Limit > 0 && summaryCount > opts.Limit {
				continue
			}
			route := entry.Route
			if route == "" {
				route = entry.Path
			}
			if route == "/api/forensics/dashboard" || route == "/api/forensics" {
				continue
			}

			cacheHits += uint64(entry.CacheHits)
			cacheMisses += uint64(entry.CacheMisses)
			switch entry.CacheStatus {
			case "all_cached":
				allCachedDurations = append(allCachedDurations, entry.DurationMs)
			case "mixed":
				mixedDurations = append(mixedDurations, entry.DurationMs)
			case "cold":
				coldDurations = append(coldDurations, entry.DurationMs)
			}
			report.CacheStats.UpstreamErrors += entry.HTTPErrors
			report.CacheStats.UpstreamRateLimits += entry.HTTPRateLimits

			key := entry.Method + "|" + route + "|" + entry.CacheStatus
			agg := latency[key]
			if agg == nil {
				agg = &latencyAgg{Route: route, Method: entry.Method, CacheStatus: entry.CacheStatus}
				latency[key] = agg
			}
			agg.Durations = append(agg.Durations, entry.DurationMs)
			agg.RedisMs += entry.RedisMs
			agg.HTTPMs += entry.HTTPMs
			agg.MongoMs += entry.MongoMs
			agg.CacheHits += entry.CacheHits
			agg.CacheMisses += entry.CacheMisses

			if entry.ResponseCacheStatus != "" {
				endpoint := entry.ResponseCacheEndpoint
				if endpoint == "" {
					endpoint = route
				}
				responseKey := entry.Method + "|" + endpoint + "|" + route
				responseAgg := responseCache[responseKey]
				if responseAgg == nil {
					responseAgg = &responseCacheAgg{Endpoint: endpoint, Route: route, Method: entry.Method}
					responseCache[responseKey] = responseAgg
				}
				responseAgg.Durations = append(responseAgg.Durations, entry.DurationMs)
				switch entry.ResponseCacheStatus {
				case "ram":
					responseAgg.RAMHits++
					responseRAMHits++
				case "redis":
					responseAgg.RedisHits++
					responseRedisHits++
				case "cold":
					responseAgg.Cold++
					responseCold++
				}
				if responseCacheRAMEligibleEndpoint(endpoint) {
					responseRAMEligibleTotal++
					if entry.ResponseCacheStatus == "ram" {
						responseRAMEligibleHits++
					}
				}
			}

			recent = append(recent, recentRequestEntry{
				RequestID:           entry.RequestID,
				Method:              entry.Method,
				Path:                entry.Path,
				Route:               route,
				CacheStatus:         entry.CacheStatus,
				ResponseCacheStatus: entry.ResponseCacheStatus,
				StatusCode:          entry.StatusCode,
				DurationMs:          entry.DurationMs,
				CacheHits:           entry.CacheHits,
				CacheMisses:         entry.CacheMisses,
				RedisCalls:          entry.RedisCalls,
				RedisMs:             entry.RedisMs,
				HTTPCalls:           entry.HTTPCalls,
				HTTPMs:              entry.HTTPMs,
				MongoCalls:          entry.MongoCalls,
				MongoMs:             entry.MongoMs,
				Timestamp:           entry.Timestamp,
			})
			if len(recent) > 50 {
				recent = recent[1:]
			}

			addSlowest(&slowest, &slowestMinMs, slowRequestEntry{
				RequestID:    entry.RequestID,
				Method:       entry.Method,
				Path:         entry.Path,
				StatusCode:   entry.StatusCode,
				DurationMs:   entry.DurationMs,
				DurationSec:  float64(entry.DurationMs) / 1000.0,
				ResponseSize: entry.ResponseSize,
				Timestamp:    entry.Timestamp,
			})

			for _, dep := range entry.Dependencies {
				addDependency(deps, dep)
				switch dep.Kind {
				case "redis":
					addDependency(cacheDeps, dep)
					addDependency(redisDeps, dep)
				case "http":
					addDependency(upstreamDeps, dep)
				case "mongo":
					addDependency(mongoDeps, dep)
				}
			}
			for _, repeated := range entry.RepeatedDeps {
				report.RepeatedDependencyCalls = append(report.RepeatedDependencyCalls, repeatedDependencyEntry{
					Route:      route,
					RequestID:  entry.RequestID,
					Kind:       repeated.Kind,
					Operation:  repeated.Operation,
					Name:       repeated.Name,
					Count:      repeated.Count,
					TotalMs:    repeated.TotalMs,
					AvgMs:      repeated.AvgMs,
					Suggestion: repeated.Suggestion,
				})
			}

		case "span_completed":
			if entry.Span == "" {
				continue
			}
			agg := spans[entry.Span]
			if agg == nil {
				agg = &spanAgg{MinUs: entry.DurationUs, MaxUs: entry.DurationUs}
				spans[entry.Span] = agg
			}
			agg.Count++
			agg.TotalUs += entry.DurationUs
			if entry.DurationUs < agg.MinUs {
				agg.MinUs = entry.DurationUs
			}
			if entry.DurationUs > agg.MaxUs {
				agg.MaxUs = entry.DurationUs
			}

		case "redis_cache_hit":
			legacyCacheHits++
		case "redis_cache_miss":
			legacyCacheMisses++
		case "request_completed":
			durMs := entry.DurationMs
			if durMs == 0 && entry.Duration > 0 {
				durMs = int64(entry.Duration * 1000)
			}
			if entry.Path == "/api/forensics/dashboard" || entry.Path == "/api/forensics" {
				continue
			}
			addSlowest(&slowest, &slowestMinMs, slowRequestEntry{
				RequestID:    entry.RequestID,
				Method:       entry.Method,
				Path:         entry.Path,
				StatusCode:   entry.StatusCode,
				DurationMs:   durMs,
				DurationSec:  entry.Duration,
				ResponseSize: entry.ResponseSize,
				Timestamp:    entry.Timestamp,
			})
		}
	}

	if summaryCount == 0 {
		cacheHits = legacyCacheHits
		cacheMisses = legacyCacheMisses
	}
	finalizeCacheStats(report, cacheHits, cacheMisses, responseRAMHits, responseRedisHits, responseCold, responseRAMEligibleHits, responseRAMEligibleTotal, allCachedDurations, mixedDurations, coldDurations)
	report.TopSpans = buildSpanEntries(spans)
	report.CacheLatencyByEndpoint = buildLatencyEntries(latency)
	report.ResponseCacheByEndpoint = buildResponseCacheEntries(responseCache)
	report.RecentRequests = recent
	report.DependencySummary = dependencyEntries(deps, 40)
	report.CacheKeySummary = dependencyEntries(cacheDeps, 25)
	report.UpstreamSummary = dependencyEntries(upstreamDeps, 25)
	report.RedisSummary = dependencyEntries(redisDeps, 25)
	report.MongoSummary = dependencyEntries(mongoDeps, 25)
	sort.Slice(report.RepeatedDependencyCalls, func(i, j int) bool {
		return report.RepeatedDependencyCalls[i].TotalMs > report.RepeatedDependencyCalls[j].TotalMs
	})
	if len(report.RepeatedDependencyCalls) > 50 {
		report.RepeatedDependencyCalls = report.RepeatedDependencyCalls[:50]
	}
	for i := range slowest {
		slowest[i].Rank = i + 1
	}
	report.SlowestRequests = slowest
	report.LogStats.ParseTimeMs = time.Since(parseStart).Milliseconds()
}

func finalizeCacheStats(report *dashboardReport, hits, misses, responseRAMHits, responseRedisHits, responseCold, responseRAMEligibleHits, responseRAMEligibleTotal uint64, allCached, mixed, cold []int64) {
	total := hits + misses
	if total > 0 {
		report.CacheStats.HitRate = float64(hits) / float64(total) * 100
	}
	responseTotal := responseRAMHits + responseRedisHits + responseCold
	if responseTotal > 0 {
		report.CacheStats.ResponseHitRate = float64(responseRAMHits+responseRedisHits) / float64(responseTotal) * 100
	}
	if responseRAMEligibleTotal > 0 {
		report.CacheStats.ResponseRAMHitRate = float64(responseRAMEligibleHits) / float64(responseRAMEligibleTotal) * 100
	}
	report.CacheStats.Hits = hits
	report.CacheStats.Misses = misses
	report.CacheStats.ResponseRAMHits = responseRAMHits
	report.CacheStats.ResponseRedisHits = responseRedisHits
	report.CacheStats.ResponseCold = responseCold
	report.CacheStats.AllCachedAvgMs = avgDuration(allCached)
	report.CacheStats.AllCachedP95Ms = percentileDuration(allCached, 0.95)
	report.CacheStats.MixedAvgMs = avgDuration(mixed)
	report.CacheStats.MixedP95Ms = percentileDuration(mixed, 0.95)
	report.CacheStats.ColdAvgMs = avgDuration(cold)
	report.CacheStats.ColdP95Ms = percentileDuration(cold, 0.95)
}

func buildSpanEntries(spans map[string]*spanAgg) []spanReportEntry {
	var totalUs int64
	for _, agg := range spans {
		totalUs += agg.TotalUs
	}

	entries := make([]spanReportEntry, 0, len(spans))
	for name, agg := range spans {
		pct := 0.0
		if totalUs > 0 {
			pct = float64(agg.TotalUs) / float64(totalUs) * 100
		}
		avgUs := agg.TotalUs
		if agg.Count > 0 {
			avgUs = agg.TotalUs / int64(agg.Count)
		}
		entries = append(entries, spanReportEntry{
			Operation:  name,
			CallCount:  agg.Count,
			TotalMs:    float64(agg.TotalUs) / 1000.0,
			AvgMs:      float64(avgUs) / 1000.0,
			MinMs:      float64(agg.MinUs) / 1000.0,
			MaxMs:      float64(agg.MaxUs) / 1000.0,
			PctOfTotal: pct,
			Critical:   pct > 10,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].TotalMs > entries[j].TotalMs })
	if len(entries) > 25 {
		entries = entries[:25]
	}
	for i := range entries {
		entries[i].Rank = i + 1
	}
	return entries
}

func buildLatencyEntries(latency map[string]*latencyAgg) []cacheLatencyEntry {
	entries := make([]cacheLatencyEntry, 0, len(latency))
	for _, agg := range latency {
		count := len(agg.Durations)
		if count == 0 {
			continue
		}
		entries = append(entries, cacheLatencyEntry{
			Route:          agg.Route,
			Method:         agg.Method,
			CacheStatus:    agg.CacheStatus,
			Count:          count,
			AvgMs:          avgDuration(agg.Durations),
			P50Ms:          percentileDuration(agg.Durations, 0.50),
			P95Ms:          percentileDuration(agg.Durations, 0.95),
			MaxMs:          float64(maxDuration(agg.Durations)),
			AvgRedisMs:     agg.RedisMs / float64(count),
			AvgHTTPMs:      agg.HTTPMs / float64(count),
			AvgMongoMs:     agg.MongoMs / float64(count),
			AvgCacheHits:   float64(agg.CacheHits) / float64(count),
			AvgCacheMisses: float64(agg.CacheMisses) / float64(count),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].P95Ms > entries[j].P95Ms })
	if len(entries) > 50 {
		entries = entries[:50]
	}
	return entries
}

func buildResponseCacheEntries(responseCache map[string]*responseCacheAgg) []responseCacheEntry {
	entries := make([]responseCacheEntry, 0, len(responseCache))
	for _, agg := range responseCache {
		count := len(agg.Durations)
		if count == 0 {
			continue
		}
		hits := agg.RAMHits + agg.RedisHits
		entries = append(entries, responseCacheEntry{
			Endpoint:   agg.Endpoint,
			Route:      agg.Route,
			Method:     agg.Method,
			Count:      count,
			RAMHits:    agg.RAMHits,
			RedisHits:  agg.RedisHits,
			Cold:       agg.Cold,
			RAMHitRate: float64(agg.RAMHits) / float64(count) * 100,
			HitRate:    float64(hits) / float64(count) * 100,
			AvgMs:      avgDuration(agg.Durations),
			P50Ms:      percentileDuration(agg.Durations, 0.50),
			P95Ms:      percentileDuration(agg.Durations, 0.95),
			MaxMs:      float64(maxDuration(agg.Durations)),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count == entries[j].Count {
			return entries[i].Endpoint < entries[j].Endpoint
		}
		return entries[i].Count > entries[j].Count
	})
	return entries
}

func responseCacheRAMEligibleEndpoint(endpoint string) bool {
	switch endpoint {
	case "embed", "stats", "combined", "uuid", "username":
		return true
	default:
		return false
	}
}

func addSlowest(slowest *[]slowRequestEntry, slowestMinMs *int64, req slowRequestEntry) {
	if req.RequestID != "" {
		for i := range *slowest {
			if (*slowest)[i].RequestID == req.RequestID {
				if req.DurationMs > (*slowest)[i].DurationMs {
					(*slowest)[i] = req
					sort.Slice(*slowest, func(i, j int) bool {
						return (*slowest)[i].DurationMs > (*slowest)[j].DurationMs
					})
					if len(*slowest) == 25 {
						*slowestMinMs = (*slowest)[24].DurationMs
					}
				}
				return
			}
		}
	}
	if len(*slowest) < 25 || req.DurationMs > *slowestMinMs {
		*slowest = append(*slowest, req)
		sort.Slice(*slowest, func(i, j int) bool {
			return (*slowest)[i].DurationMs > (*slowest)[j].DurationMs
		})
		if len(*slowest) > 25 {
			*slowest = (*slowest)[:25]
		}
		if len(*slowest) == 25 {
			*slowestMinMs = (*slowest)[24].DurationMs
		}
	}
}

func addDependency(target map[string]*dependencyReportEntry, dep DependencySummary) {
	key := dep.Kind + "|" + dep.Operation + "|" + dep.Name
	entry := target[key]
	if entry == nil {
		entry = &dependencyReportEntry{
			Kind:      dep.Kind,
			Operation: dep.Operation,
			Name:      dep.Name,
		}
		target[key] = entry
	}
	entry.Count += dep.Count
	entry.TotalMs += dep.TotalMs
	if entry.Count > 0 {
		entry.AvgMs = entry.TotalMs / float64(entry.Count)
	}
}

func dependencyEntries(source map[string]*dependencyReportEntry, limit int) []dependencyReportEntry {
	entries := make([]dependencyReportEntry, 0, len(source))
	for _, entry := range source {
		entries = append(entries, *entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].TotalMs > entries[j].TotalMs })
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

func avgDuration(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total int64
	for _, value := range values {
		total += value
	}
	return float64(total) / float64(len(values))
}

func percentileDuration(values []int64, pct float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copied := append([]int64(nil), values...)
	sort.Slice(copied, func(i, j int) bool { return copied[i] < copied[j] })
	idx := int(float64(len(copied)-1) * pct)
	return float64(copied[idx])
}

func maxDuration(values []int64) int64 {
	var max int64
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

func parseLogTimestamp(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05-0700",
	}
	var lastErr error
	for _, layout := range layouts {
		ts, err := time.Parse(layout, value)
		if err == nil {
			return ts, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

func renderDashboardHTML(r dashboardReport, opts dashboardOptions) string {
	window := html.EscapeString(opts.Window.String())
	limit := opts.Limit
	out := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<title>SkyCrypt Forensic Dashboard</title>
<meta charset="utf-8">
<meta http-equiv="refresh" content="10">
<style>
  body { font-family: 'Segoe UI', monospace; background: #0d1117; color: #c9d1d9; margin: 0; padding: 20px; }
  h1 { color: #58a6ff; border-bottom: 1px solid #30363d; padding-bottom: 10px; }
  h2 { color: #79c0ff; margin-top: 30px; }
  .meta { color: #8b949e; font-size: 14px; margin-bottom: 20px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 15px; margin: 15px 0; }
  .card { background: #161b22; border: 1px solid #30363d; border-radius: 6px; padding: 15px; }
  .card .label { color: #8b949e; font-size: 12px; text-transform: uppercase; }
  .card .value { color: #f0f6fc; font-size: 24px; font-weight: bold; margin-top: 5px; }
  table { width: 100%%; border-collapse: collapse; margin: 10px 0; }
  th { text-align: left; color: #8b949e; font-size: 12px; text-transform: uppercase; padding: 8px; border-bottom: 1px solid #30363d; }
  td { padding: 8px; border-bottom: 1px solid #21262d; font-size: 13px; vertical-align: top; }
  tr:hover { background: #161b22; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: bold; }
  .badge-red { background: #f8514933; color: #f85149; }
  .badge-green { background: #2ea04333; color: #3fb950; }
  .badge-yellow { background: #d2992233; color: #d29922; }
  .badge-blue { background: #388bfd33; color: #58a6ff; }
  .empty { color: #484f58; font-style: italic; padding: 20px; text-align: center; }
  .section-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .section-header h2 { margin: 0; }
  .btn { background: #21262d; color: #c9d1d9; border: 1px solid #30363d; border-radius: 6px; padding: 6px 14px; cursor: pointer; font-size: 12px; font-family: inherit; }
  .btn:hover { background: #30363d; }
  .btn-group { display: flex; gap: 8px; flex-wrap: wrap; }
  .status-ok { color: #3fb950; }
  .status-err { color: #f85149; }
  .status-warn { color: #d29922; }
  .slow-req { background: #f8514911; }
</style>
</head>
<body>
<h1>SkyCrypt Forensic Dashboard</h1>
<div class="meta">Generated: %s &nbsp;|&nbsp; Uptime: %s &nbsp;|&nbsp; PID: %d &nbsp;|&nbsp; Window: %s &nbsp;|&nbsp; Limit: %d
<br>Log: %s (%d lines parsed in %dms)</div>`,
		esc(r.GeneratedAt), esc(r.Uptime), r.Runtime.PID, window, limit,
		esc(r.LogStats.LogFileSize), r.LogStats.LinesProcessed, r.LogStats.ParseTimeMs,
	)

	out += fmt.Sprintf(`
<h2>Runtime</h2>
<div class="grid">
  <div class="card"><div class="label">Goroutines</div><div class="value">%d</div></div>
  <div class="card"><div class="label">Heap Alloc</div><div class="value">%d MB</div></div>
  <div class="card"><div class="label">Heap Sys</div><div class="value">%d MB</div></div>
  <div class="card"><div class="label">Heap Objects</div><div class="value">%d</div></div>
  <div class="card"><div class="label">GC Runs</div><div class="value">%d</div></div>
  <div class="card"><div class="label">Last GC Pause</div><div class="value">%d ms</div></div>
</div>

<h2>Cache Response Timing</h2>
<div class="grid">
  <div class="card"><div class="label">All Cached Avg / P95</div><div class="value">%.1f / %.1f ms</div></div>
  <div class="card"><div class="label">Mixed Avg / P95</div><div class="value">%.1f / %.1f ms</div></div>
  <div class="card"><div class="label">Cold Avg / P95</div><div class="value">%.1f / %.1f ms</div></div>
  <div class="card"><div class="label">Redis Hit Rate</div><div class="value">%.1f%%</div></div>
  <div class="card"><div class="label">Response RAM Hit Rate (RAM Routes)</div><div class="value">%.1f%%</div></div>
  <div class="card"><div class="label">Response Total Hit Rate</div><div class="value">%.1f%%</div></div>
  <div class="card"><div class="label">Response RAM / Redis / Cold</div><div class="value">%d / %d / %d</div></div>
  <div class="card"><div class="label">Upstream Errors</div><div class="value">%d</div></div>
  <div class="card"><div class="label">Rate Limits</div><div class="value">%d</div></div>
</div>`,
		r.Runtime.Goroutines, r.Runtime.HeapAllocMB, r.Runtime.HeapSysMB, r.Runtime.HeapObjects, r.Runtime.NumGC, r.Runtime.LastPauseMs,
		r.CacheStats.AllCachedAvgMs, r.CacheStats.AllCachedP95Ms,
		r.CacheStats.MixedAvgMs, r.CacheStats.MixedP95Ms,
		r.CacheStats.ColdAvgMs, r.CacheStats.ColdP95Ms,
		r.CacheStats.HitRate,
		r.CacheStats.ResponseRAMHitRate, r.CacheStats.ResponseHitRate,
		r.CacheStats.ResponseRAMHits, r.CacheStats.ResponseRedisHits, r.CacheStats.ResponseCold,
		r.CacheStats.UpstreamErrors, r.CacheStats.UpstreamRateLimits,
	)

	out += renderCacheLatencyTable(r.CacheLatencyByEndpoint)
	out += renderResponseCacheTable(r.ResponseCacheByEndpoint)
	out += renderRecentRequestsTable(r.RecentRequests)
	out += renderRepeatedTable(r.RepeatedDependencyCalls)
	out += renderDependencyTable("Slow External APIs", "upstream-table", r.UpstreamSummary)
	out += renderDependencyTable("Slow Redis Keys / Key Groups", "redis-table", r.RedisSummary)
	out += renderDependencyTable("Mongo Operations", "mongo-table", r.MongoSummary)
	out += renderOperationsTable(r.TopSpans)
	out += renderSlowestRequestsTable(r.SlowestRequests)

	out += `
<div style="margin-top:30px;padding-top:15px;border-top:1px solid #30363d;">
  <div class="btn-group">
    <button class="btn" onclick="exportJSON()">Export Full Report (JSON)</button>
    <button class="btn" onclick="window.location.href='/api/forensics?window=` + window + `&limit=` + strconv.Itoa(limit) + `'">Raw JSON API</button>
  </div>
</div>
<script>
function exportTable(tableId, filename) {
  const table = document.getElementById(tableId);
  if (!table) return;
  let csv = [];
  for (const row of table.rows) {
    let cols = [];
    for (const cell of row.cells) cols.push('"' + cell.innerText.replace(/"/g, '""') + '"');
    csv.push(cols.join(','));
  }
  downloadFile(csv.join('\n'), filename, 'text/csv');
}
function exportJSON() {
  fetch('/api/forensics?window=` + window + `&limit=` + strconv.Itoa(limit) + `').then(r => r.text()).then(data => {
    downloadFile(data, 'forensics_' + new Date().toISOString().slice(0,19).replace(/:/g,'-') + '.json', 'application/json');
  });
}
function downloadFile(content, filename, mime) {
  const blob = new Blob([content], {type: mime});
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = filename;
  a.click();
  URL.revokeObjectURL(a.href);
}
</script></body></html>`
	return out
}

func renderCacheLatencyTable(entries []cacheLatencyEntry) string {
	out := `<div class="section-header"><h2>Cache Timing by Endpoint</h2><div class="btn-group"><button class="btn" onclick="exportTable('cache-latency-table','cache_latency.csv')">Export CSV</button></div></div>`
	if len(entries) == 0 {
		return out + `<div class="empty">No request summaries recorded yet.</div>`
	}
	out += `<table id="cache-latency-table"><tr><th>Route</th><th>Method</th><th>Cache</th><th>Count</th><th>Avg</th><th>P50</th><th>P95</th><th>Max</th><th>Redis</th><th>HTTP</th><th>Mongo</th><th>Hits</th><th>Misses</th></tr>`
	for _, e := range entries {
		out += fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f</td></tr>`,
			esc(e.Route), esc(e.Method), cacheBadge(e.CacheStatus), e.Count, e.AvgMs, e.P50Ms, e.P95Ms, e.MaxMs, e.AvgRedisMs, e.AvgHTTPMs, e.AvgMongoMs, e.AvgCacheHits, e.AvgCacheMisses)
	}
	return out + `</table>`
}

func renderResponseCacheTable(entries []responseCacheEntry) string {
	out := `<div class="section-header"><h2>Response Cache by Endpoint</h2><div class="btn-group"><button class="btn" onclick="exportTable('response-cache-table','response_cache.csv')">Export CSV</button></div></div>`
	if len(entries) == 0 {
		return out + `<div class="empty">No response-cache summaries recorded yet.</div>`
	}
	out += `<table id="response-cache-table"><tr><th>Endpoint</th><th>Route</th><th>Method</th><th>Count</th><th>RAM</th><th>Redis</th><th>Cold</th><th>RAM Hit</th><th>Total Hit</th><th>Avg</th><th>P50</th><th>P95</th><th>Max</th></tr>`
	for _, e := range entries {
		out += fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%.1f%%</td><td>%.1f%%</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f</td></tr>`,
			esc(e.Endpoint), esc(e.Route), esc(e.Method), e.Count, e.RAMHits, e.RedisHits, e.Cold, e.RAMHitRate, e.HitRate, e.AvgMs, e.P50Ms, e.P95Ms, e.MaxMs)
	}
	return out + `</table>`
}

func renderRecentRequestsTable(entries []recentRequestEntry) string {
	out := `<div class="section-header"><h2>Recent Requests</h2><div class="btn-group"><button class="btn" onclick="exportTable('recent-table','recent_requests.csv')">Export CSV</button></div></div>`
	if len(entries) == 0 {
		return out + `<div class="empty">No recent request summaries recorded.</div>`
	}
	out += `<table id="recent-table"><tr><th>Time</th><th>Method</th><th>Route</th><th>Status</th><th>Cache</th><th>Response</th><th>Duration</th><th>Hits</th><th>Misses</th><th>Redis</th><th>HTTP</th><th>Mongo</th><th>Request ID</th></tr>`
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		out += fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td><td>%s</td><td>%d ms</td><td>%d</td><td>%d</td><td>%d / %.1f ms</td><td>%d / %.1f ms</td><td>%d / %.1f ms</td><td style="font-size:11px;color:#8b949e">%s</td></tr>`,
			esc(e.Timestamp), esc(e.Method), esc(e.Route), e.StatusCode, cacheBadge(e.CacheStatus), responseCacheBadge(e.ResponseCacheStatus), e.DurationMs, e.CacheHits, e.CacheMisses, e.RedisCalls, e.RedisMs, e.HTTPCalls, e.HTTPMs, e.MongoCalls, e.MongoMs, esc(e.RequestID))
	}
	return out + `</table>`
}

func renderRepeatedTable(entries []repeatedDependencyEntry) string {
	out := `<div class="section-header"><h2>Repeated Dependency Calls</h2><div class="btn-group"><button class="btn" onclick="exportTable('repeated-table','repeated_dependencies.csv')">Export CSV</button></div></div>`
	if len(entries) == 0 {
		return out + `<div class="empty">No repeated dependency patterns detected.</div>`
	}
	out += `<table id="repeated-table"><tr><th>Route</th><th>Kind</th><th>Operation</th><th>Name</th><th>Count</th><th>Total</th><th>Avg</th><th>Suggestion</th><th>Request ID</th></tr>`
	for _, e := range entries {
		out += fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%.1f ms</td><td>%.1f ms</td><td>%s</td><td style="font-size:11px;color:#8b949e">%s</td></tr>`,
			esc(e.Route), esc(e.Kind), esc(e.Operation), esc(e.Name), e.Count, e.TotalMs, e.AvgMs, esc(e.Suggestion), esc(e.RequestID))
	}
	return out + `</table>`
}

func renderDependencyTable(title, tableID string, entries []dependencyReportEntry) string {
	out := fmt.Sprintf(`<div class="section-header"><h2>%s</h2><div class="btn-group"><button class="btn" onclick="exportTable('%s','%s.csv')">Export CSV</button></div></div>`, esc(title), esc(tableID), esc(strings.ReplaceAll(tableID, "-", "_")))
	if len(entries) == 0 {
		return out + `<div class="empty">No dependency data recorded.</div>`
	}
	out += fmt.Sprintf(`<table id="%s"><tr><th>Kind</th><th>Operation</th><th>Name</th><th>Calls</th><th>Total</th><th>Avg</th></tr>`, esc(tableID))
	for _, e := range entries {
		out += fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%.1f ms</td><td>%.1f ms</td></tr>`,
			esc(e.Kind), esc(e.Operation), esc(e.Name), e.Count, e.TotalMs, e.AvgMs)
	}
	return out + `</table>`
}

func renderOperationsTable(entries []spanReportEntry) string {
	out := `<div class="section-header"><h2>Top Operations</h2><div class="btn-group"><button class="btn" onclick="exportTable('ops-table','operations.csv')">Export CSV</button></div></div>`
	if len(entries) == 0 {
		return out + `<div class="empty">No spans recorded yet.</div>`
	}
	out += `<table id="ops-table"><tr><th>#</th><th>Operation</th><th>Calls</th><th>Total</th><th>Avg</th><th>Min</th><th>Max</th><th>% Total</th><th>Status</th></tr>`
	for _, s := range entries {
		badge := `<span class="badge badge-green">OK</span>`
		if s.Critical {
			badge = `<span class="badge badge-red">BOTTLENECK</span>`
		} else if s.PctOfTotal > 5 {
			badge = `<span class="badge badge-yellow">WATCH</span>`
		}
		out += fmt.Sprintf(`<tr><td>%d</td><td>%s</td><td>%d</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f%%</td><td>%s</td></tr>`,
			s.Rank, esc(s.Operation), s.CallCount, s.TotalMs, s.AvgMs, s.MinMs, s.MaxMs, s.PctOfTotal, badge)
	}
	return out + `</table>`
}

func renderSlowestRequestsTable(entries []slowRequestEntry) string {
	out := `<div class="section-header"><h2>Slowest Requests</h2><div class="btn-group"><button class="btn" onclick="exportTable('slow-table','slowest_requests.csv')">Export CSV</button></div></div>`
	if len(entries) == 0 {
		return out + `<div class="empty">No requests recorded yet.</div>`
	}
	out += `<table id="slow-table"><tr><th>#</th><th>Method</th><th>Path</th><th>Status</th><th>Duration</th><th>Size</th><th>Time</th><th>Request ID</th></tr>`
	for _, req := range entries {
		statusClass := "status-ok"
		if req.StatusCode >= 500 {
			statusClass = "status-err"
		} else if req.StatusCode >= 400 {
			statusClass = "status-warn"
		}
		rowClass := ""
		if req.DurationMs > 1000 {
			rowClass = ` class="slow-req"`
		}
		out += fmt.Sprintf(`<tr%s><td>%d</td><td>%s</td><td>%s</td><td class="%s">%d</td><td>%d ms</td><td>%d</td><td>%s</td><td style="font-size:11px;color:#8b949e">%s</td></tr>`,
			rowClass, req.Rank, esc(req.Method), esc(req.Path), statusClass, req.StatusCode, req.DurationMs, req.ResponseSize, esc(req.Timestamp), esc(req.RequestID))
	}
	return out + `</table>`
}

func cacheBadge(status string) string {
	switch status {
	case "all_cached":
		return `<span class="badge badge-green">all_cached</span>`
	case "mixed":
		return `<span class="badge badge-yellow">mixed</span>`
	case "cold":
		return `<span class="badge badge-red">cold</span>`
	default:
		return `<span class="badge badge-blue">not_cacheable</span>`
	}
}

func responseCacheBadge(status string) string {
	switch status {
	case "ram":
		return `<span class="badge badge-green">ram</span>`
	case "redis":
		return `<span class="badge badge-blue">redis</span>`
	case "cold":
		return `<span class="badge badge-red">cold</span>`
	default:
		return `<span class="badge">none</span>`
	}
}

func esc(value string) string {
	return html.EscapeString(value)
}
