package forensics

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClassifyCacheStatus(t *testing.T) {
	tests := []struct {
		name      string
		hits      int
		misses    int
		httpCalls int
		want      string
	}{
		{name: "all hits", hits: 3, want: "all_cached"},
		{name: "hit and miss", hits: 2, misses: 1, want: "mixed"},
		{name: "hit and upstream", hits: 2, httpCalls: 1, want: "mixed"},
		{name: "miss only", misses: 1, want: "cold"},
		{name: "no cache", want: "not_cacheable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyCacheStatus(tt.hits, tt.misses, tt.httpCalls); got != tt.want {
				t.Fatalf("classifyCacheStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRequestMetricsConcurrentRecordingAndRepeatedDependencies(t *testing.T) {
	metrics := NewRequestMetrics("req-1", "GET", "/api/test")
	ctx := WithRequestMetrics(context.Background(), metrics)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			RecordRedisDependency(ctx, "GET", "username:abc", "hit", time.Millisecond, nil)
		}()
	}
	wg.Wait()

	summary := FinalizeRequestMetrics(ctx, 200, 25*time.Millisecond, 128, 64)
	if summary.CacheStatus != "all_cached" {
		t.Fatalf("CacheStatus = %q, want all_cached", summary.CacheStatus)
	}
	if summary.RedisCalls != 100 || summary.CacheHits != 100 {
		t.Fatalf("RedisCalls/CacheHits = %d/%d, want 100/100", summary.RedisCalls, summary.CacheHits)
	}
	if len(summary.RepeatedDependencies) != 1 {
		t.Fatalf("RepeatedDependencies len = %d, want 1", len(summary.RepeatedDependencies))
	}
	repeated := summary.RepeatedDependencies[0]
	if repeated.Kind != "redis" || repeated.Operation != "GET" || repeated.Name != "username" || repeated.Count != 100 {
		t.Fatalf("unexpected repeated dependency: %+v", repeated)
	}
}

func TestDashboardParsesForensicSummariesAndLegacyLogs(t *testing.T) {
	tmp := t.TempDir()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	if err := os.MkdirAll("logs", 0755); err != nil {
		t.Fatal(err)
	}

	now := time.Now().Format(time.RFC3339Nano)
	lines := []map[string]interface{}{
		{
			"level": "info", "timestamp": now, "msg": "forensic_request_summary",
			"request_id": "req-1", "method": "GET", "path": "/api/stats/ducky/Kiwi", "route": "/api/stats/:uuid/:profileId",
			"cache_status": "all_cached", "status_code": 200, "duration_ms": 20, "response_size": 100,
			"cache_hits": 3, "cache_misses": 0, "redis_calls": 3, "redis_ms": 5.0,
			"dependencies": []DependencySummary{
				{Kind: "redis", Operation: "GET", Name: "profiles", Count: 3, TotalMs: 5, AvgMs: 1.6},
			},
			"repeated_dependencies": []RepeatedDependency{
				{Kind: "redis", Operation: "GET", Name: "profiles", Count: 3, TotalMs: 5, AvgMs: 1.6, Suggestion: "memoize"},
			},
		},
		{
			"level": "info", "timestamp": now, "msg": "forensic_request_summary",
			"request_id": "req-2", "method": "GET", "path": "/api/stats/ducky/Kiwi", "route": "/api/stats/:uuid/:profileId",
			"cache_status": "cold", "status_code": 200, "duration_ms": 120, "response_size": 100,
			"cache_hits": 0, "cache_misses": 1, "redis_calls": 1, "redis_ms": 1.0, "http_calls": 1, "http_ms": 90.0,
			"dependencies": []DependencySummary{
				{Kind: "http", Operation: "GET", Name: "api.hypixel.net/v2/player", Count: 1, TotalMs: 90, AvgMs: 90},
			},
		},
		{"level": "info", "timestamp": now, "msg": "request_completed", "request_id": "req-2", "method": "GET", "path": "/api/stats/ducky/Kiwi", "status_code": 200, "duration_ms": 120, "response_size": 100},
		{"level": "info", "timestamp": now, "msg": "span_completed", "span": "api.GetProfiles", "duration_us": 1500},
		{"level": "info", "timestamp": now, "msg": "request_completed", "request_id": "legacy", "method": "GET", "path": "/api/legacy", "status_code": 200, "duration_ms": 50, "response_size": 10},
		{"level": "error", "timestamp": now, "msg": "error_recorded", "error_type": "ignored", "error": "ignored"},
	}

	var b strings.Builder
	for _, line := range lines {
		payload, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(payload)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join("logs", "app.log"), []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}

	report := generateDashboardReport(dashboardOptions{Window: time.Hour, Limit: 50000})
	if report.CacheStats.Hits != 3 || report.CacheStats.Misses != 1 {
		t.Fatalf("cache hits/misses = %d/%d, want 3/1", report.CacheStats.Hits, report.CacheStats.Misses)
	}
	if len(report.CacheLatencyByEndpoint) != 2 {
		t.Fatalf("CacheLatencyByEndpoint len = %d, want 2", len(report.CacheLatencyByEndpoint))
	}
	if len(report.RepeatedDependencyCalls) != 1 {
		t.Fatalf("RepeatedDependencyCalls len = %d, want 1", len(report.RepeatedDependencyCalls))
	}
	if len(report.TopSpans) != 1 || report.TopSpans[0].Operation != "api.GetProfiles" {
		t.Fatalf("unexpected TopSpans: %+v", report.TopSpans)
	}
	seenReq2 := 0
	for _, req := range report.SlowestRequests {
		if req.RequestID == "req-2" {
			seenReq2++
		}
	}
	if seenReq2 != 1 {
		t.Fatalf("req-2 slowest count = %d, want 1: %+v", seenReq2, report.SlowestRequests)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "error_summary") || strings.Contains(string(encoded), "nplus1_patterns") {
		t.Fatalf("removed dashboard fields leaked into report JSON: %s", encoded)
	}
}

func TestRedactURL(t *testing.T) {
	got := RedactURL("https://api.hypixel.net/v2/player?key=secret&uuid=abc")
	if strings.Contains(got, "secret") {
		t.Fatalf("RedactURL leaked secret: %s", got)
	}
	if !strings.Contains(got, "key=REDACTED") {
		t.Fatalf("RedactURL did not redact key: %s", got)
	}
}
