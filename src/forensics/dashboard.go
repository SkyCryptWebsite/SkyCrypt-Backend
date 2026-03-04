package forensics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/gofiber/fiber/v2"
)

var startTime = time.Now()

func DashboardHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		report := generateDashboardReport()
		return c.JSON(report)
	}
}

func DashboardHTMLHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		report := generateDashboardReport()
		html := renderDashboardHTML(report)
		c.Set("Content-Type", "text/html")
		return c.SendString(html)
	}
}

type dashboardReport struct {
	GeneratedAt     string             `json:"generated_at"`
	Uptime          string             `json:"uptime"`
	Runtime         runtimeReport      `json:"runtime"`
	CacheStats      cacheReport        `json:"cache_stats"`
	TopSpans        []spanReportEntry  `json:"top_operations"`
	ErrorSummary    []errorReportEntry `json:"error_summary"`
	NPlus1          []nplus1Entry      `json:"nplus1_patterns"`
	SlowestRequests []slowRequestEntry `json:"slowest_requests"`
	LogStats        logStatsReport     `json:"log_stats"`
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
	Hits    uint64  `json:"hits"`
	Misses  uint64  `json:"misses"`
	HitRate float64 `json:"hit_rate_percent"`
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

type errorReportEntry struct {
	ErrorType    string   `json:"error_type"`
	Count        uint64   `json:"count"`
	LastOccurred string   `json:"last_occurred"`
	Examples     []string `json:"examples"`
}

type nplus1Entry struct {
	RequestID string `json:"request_id"`
	Query     string `json:"query"`
	Count     uint64 `json:"count"`
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
	StatusCode   int     `json:"status_code,omitempty"`
	ResponseSize int     `json:"response_size,omitempty"`
	ErrorType    string  `json:"error_type,omitempty"`
	ErrorMsg     string  `json:"error,omitempty"`
	Key          string  `json:"key,omitempty"`
	RequestID    string  `json:"request_id,omitempty"`
	Query        string  `json:"query,omitempty"`
	ExecCount    uint64  `json:"execution_count,omitempty"`
}

type spanAgg struct {
	Count   uint64
	TotalUs int64
	MinUs   int64
	MaxUs   int64
}

type errorAgg struct {
	Count        uint64
	LastOccurred string
	Examples     []string
}

func generateDashboardReport() dashboardReport {
	report := dashboardReport{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Uptime:      time.Since(startTime).Round(time.Second).String(),
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

	parseLogFile(&report)

	return report
}

func parseLogFile(report *dashboardReport) {
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
	fileSize := ""
	if info != nil {
		sizeMB := float64(info.Size()) / 1024 / 1024
		if sizeMB >= 1 {
			fileSize = fmt.Sprintf("%.1f MB", sizeMB)
		} else {
			fileSize = fmt.Sprintf("%.0f KB", float64(info.Size())/1024)
		}
	}

	spans := make(map[string]*spanAgg)
	errors := make(map[string]*errorAgg)
	nplus1 := make(map[string]*nplus1Entry)
	var cacheHits, cacheMisses uint64
	linesProcessed := 0

	// Track slowest requests (keep top 25)
	slowest := make([]slowRequestEntry, 0, 26)
	var slowestMinMs int64

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 512*1024), 512*1024) // 512KB line buffer

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}

		var entry logLine
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		linesProcessed++

		switch entry.Msg {
		case "span_completed":
			if entry.Span == "" {
				continue
			}
			agg, exists := spans[entry.Span]
			if !exists {
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

		case "error_recorded":
			if entry.ErrorType == "" {
				continue
			}
			agg, exists := errors[entry.ErrorType]
			if !exists {
				agg = &errorAgg{Examples: make([]string, 0, 5)}
				errors[entry.ErrorType] = agg
			}
			agg.Count++
			agg.LastOccurred = entry.Timestamp
			if len(agg.Examples) < 5 && entry.ErrorMsg != "" {
				agg.Examples = append(agg.Examples, entry.ErrorMsg)
			}

		case "redis_cache_hit":
			cacheHits++

		case "redis_cache_miss":
			cacheMisses++

		case "nplus1_query_detected":
			key := entry.RequestID + ":" + entry.Query
			if _, exists := nplus1[key]; !exists {
				nplus1[key] = &nplus1Entry{
					RequestID: entry.RequestID,
					Query:     entry.Query,
					Count:     entry.ExecCount,
				}
			} else if entry.ExecCount > nplus1[key].Count {
				nplus1[key].Count = entry.ExecCount
			}

		case "request_completed":
			durMs := entry.DurationMs
			if durMs == 0 && entry.Duration > 0 {
				durMs = int64(entry.Duration * 1000)
			}
			if entry.Path == "/api/forensics/dashboard" || entry.Path == "/api/forensics" {
				continue
			}
			if len(slowest) < 25 || durMs > slowestMinMs {
				slowest = append(slowest, slowRequestEntry{
					RequestID:    entry.RequestID,
					Method:       entry.Method,
					Path:         entry.Path,
					StatusCode:   entry.StatusCode,
					DurationMs:   durMs,
					DurationSec:  entry.Duration,
					ResponseSize: entry.ResponseSize,
					Timestamp:    entry.Timestamp,
				})
				sort.Slice(slowest, func(i, j int) bool {
					return slowest[i].DurationMs > slowest[j].DurationMs
				})
				if len(slowest) > 25 {
					slowest = slowest[:25]
				}
				if len(slowest) == 25 {
					slowestMinMs = slowest[24].DurationMs
				}
			}
		}
	}

	hitRate := 0.0
	if cacheHits+cacheMisses > 0 {
		hitRate = float64(cacheHits) / float64(cacheHits+cacheMisses) * 100
	}
	report.CacheStats = cacheReport{
		Hits:    cacheHits,
		Misses:  cacheMisses,
		HitRate: hitRate,
	}

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

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].TotalMs > entries[j].TotalMs
	})

	for i := range entries {
		entries[i].Rank = i + 1
		if i >= 25 {
			entries = entries[:25]
			break
		}
	}
	report.TopSpans = entries

	for errType, agg := range errors {
		report.ErrorSummary = append(report.ErrorSummary, errorReportEntry{
			ErrorType:    errType,
			Count:        agg.Count,
			LastOccurred: agg.LastOccurred,
			Examples:     agg.Examples,
		})
	}
	sort.Slice(report.ErrorSummary, func(i, j int) bool {
		return report.ErrorSummary[i].Count > report.ErrorSummary[j].Count
	})

	for _, entry := range nplus1 {
		report.NPlus1 = append(report.NPlus1, *entry)
	}

	for i := range slowest {
		slowest[i].Rank = i + 1
	}
	report.SlowestRequests = slowest

	report.LogStats = logStatsReport{
		LinesProcessed: linesProcessed,
		LogFileSize:    fileSize,
		ParseTimeMs:    time.Since(parseStart).Milliseconds(),
	}
}

func renderDashboardHTML(r dashboardReport) string {
	html := `<!DOCTYPE html>
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
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin: 15px 0; }
  .card { background: #161b22; border: 1px solid #30363d; border-radius: 6px; padding: 15px; }
  .card .label { color: #8b949e; font-size: 12px; text-transform: uppercase; }
  .card .value { color: #f0f6fc; font-size: 28px; font-weight: bold; margin-top: 5px; }
  .card.warn .value { color: #d29922; }
  .card.crit .value { color: #f85149; }
  table { width: 100%%; border-collapse: collapse; margin: 10px 0; }
  th { text-align: left; color: #8b949e; font-size: 12px; text-transform: uppercase; padding: 8px; border-bottom: 1px solid #30363d; }
  td { padding: 8px; border-bottom: 1px solid #21262d; font-size: 14px; }
  tr:hover { background: #161b22; }
  .critical { background: #f8514922; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: bold; }
  .badge-red { background: #f8514933; color: #f85149; }
  .badge-green { background: #2ea04333; color: #3fb950; }
  .badge-yellow { background: #d2992233; color: #d29922; }
  .empty { color: #484f58; font-style: italic; padding: 20px; text-align: center; }
  .source { color: #484f58; font-size: 12px; margin-top: 5px; }
  .section-header { display: flex; align-items: center; justify-content: space-between; }
  .section-header h2 { margin: 0; }
  .btn { background: #21262d; color: #c9d1d9; border: 1px solid #30363d; border-radius: 6px; padding: 6px 14px; cursor: pointer; font-size: 12px; font-family: inherit; transition: background 0.15s; }
  .btn:hover { background: #30363d; }
  .btn-group { display: flex; gap: 8px; }
  .status-ok { color: #3fb950; }
  .status-err { color: #f85149; }
  .status-warn { color: #d29922; }
  .slow-req { background: #f8514911; }
</style>
</head>
<body>
<h1>SkyCrypt Forensic Dashboard</h1>
<div class="meta">Generated: %s &nbsp;|&nbsp; Uptime: %s &nbsp;|&nbsp; PID: %d &nbsp;|&nbsp; Auto-refresh: 10s
<br>Log: %s (%d lines parsed in %dms) — data aggregated from ALL prefork processes</div>

<h2>Runtime (this process)</h2>
<div class="grid">
  <div class="card"><div class="label">Goroutines</div><div class="value">%d</div></div>
  <div class="card"><div class="label">Heap Alloc</div><div class="value">%d MB</div></div>
  <div class="card"><div class="label">Heap Sys</div><div class="value">%d MB</div></div>
  <div class="card"><div class="label">Heap Objects</div><div class="value">%d</div></div>
  <div class="card"><div class="label">Sys Memory</div><div class="value">%d MB</div></div>
  <div class="card"><div class="label">GC Runs</div><div class="value">%d</div></div>
  <div class="card"><div class="label">Last GC Pause</div><div class="value">%d ms</div></div>
  <div class="card"><div class="label">CPUs</div><div class="value">%d</div></div>
</div>

<h2>Cache Performance (all processes)</h2>
<div class="grid">
  <div class="card"><div class="label">Hits</div><div class="value badge-green">%d</div></div>
  <div class="card"><div class="label">Misses</div><div class="value badge-red">%d</div></div>
  <div class="card"><div class="label">Hit Rate</div><div class="value">%.1f%%%%</div></div>
</div>
`

	html = fmt.Sprintf(html,
		r.GeneratedAt, r.Uptime, r.Runtime.PID,
		r.LogStats.LogFileSize, r.LogStats.LinesProcessed, r.LogStats.ParseTimeMs,
		r.Runtime.Goroutines, r.Runtime.HeapAllocMB, r.Runtime.HeapSysMB,
		r.Runtime.HeapObjects, r.Runtime.SysMemMB, r.Runtime.NumGC,
		r.Runtime.LastPauseMs, r.Runtime.NumCPU,
		r.CacheStats.Hits, r.CacheStats.Misses, r.CacheStats.HitRate,
	)

	html += `<div class="section-header"><h2>Top Operations (by total time)</h2>
<div class="btn-group"><button class="btn" onclick="exportTable('ops-table','operations.csv')">Export CSV</button></div></div>`
	if len(r.TopSpans) == 0 {
		html += `<div class="empty">No operations recorded yet. Make some requests first.</div>`
	} else {
		html += `<table id="ops-table">
<tr><th>#</th><th>Operation</th><th>Calls</th><th>Total (ms)</th><th>Avg (ms)</th><th>Min (ms)</th><th>Max (ms)</th><th>%% Total</th><th>Status</th></tr>`
		for _, s := range r.TopSpans {
			rowClass := ""
			badge := `<span class="badge badge-green">OK</span>`
			if s.Critical {
				rowClass = ` class="critical"`
				badge = `<span class="badge badge-red">BOTTLENECK</span>`
			} else if s.PctOfTotal > 5 {
				badge = `<span class="badge badge-yellow">WATCH</span>`
			}
			html += fmt.Sprintf(`<tr%s><td>%d</td><td>%s</td><td>%d</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.1f%%</td><td>%s</td></tr>`,
				rowClass, s.Rank, s.Operation, s.CallCount, s.TotalMs, s.AvgMs, s.MinMs, s.MaxMs, s.PctOfTotal, badge)
		}
		html += `</table>`
	}

	// Errors table
	html += `<div class="section-header"><h2>Error Summary</h2>
<div class="btn-group"><button class="btn" onclick="exportTable('err-table','errors.csv')">Export CSV</button></div></div>`
	if len(r.ErrorSummary) == 0 {
		html += `<div class="empty">No errors recorded. Nice!</div>`
	} else {
		html += `<table id="err-table">
<tr><th>Error Type</th><th>Count</th><th>Last Seen</th><th>Example</th></tr>`
		for _, e := range r.ErrorSummary {
			example := ""
			if len(e.Examples) > 0 {
				example = e.Examples[0]
				if len(example) > 120 {
					example = example[:120] + "..."
				}
			}
			html += fmt.Sprintf(`<tr><td>%s</td><td>%d</td><td>%s</td><td>%s</td></tr>`,
				e.ErrorType, e.Count, e.LastOccurred, example)
		}
		html += `</table>`
	}

	html += `<div class="section-header"><h2>N+1 Query Patterns</h2></div>`
	if len(r.NPlus1) == 0 {
		html += `<div class="empty">No N+1 patterns detected.</div>`
	} else {
		html += `<table>
<tr><th>Request ID</th><th>Query Pattern</th><th>Repetitions</th></tr>`
		for _, n := range r.NPlus1 {
			html += fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%d</td></tr>`,
				n.RequestID, n.Query, n.Count)
		}
		html += `</table>`
	}

	// Slowest requests table
	html += `<div class="section-header"><h2>Slowest Requests (top 25)</h2>
<div class="btn-group"><button class="btn" onclick="exportTable('slow-table','slowest_requests.csv')">Export CSV</button></div></div>`
	if len(r.SlowestRequests) == 0 {
		html += `<div class="empty">No requests recorded yet.</div>`
	} else {
		html += `<table id="slow-table">
<tr><th>#</th><th>Method</th><th>Path</th><th>Status</th><th>Duration (ms)</th><th>Duration (s)</th><th>Size</th><th>Time</th><th>Request ID</th></tr>`
		for _, req := range r.SlowestRequests {
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
			html += fmt.Sprintf(`<tr%s><td>%d</td><td>%s</td><td>%s</td><td class="%s">%d</td><td>%d</td><td>%.3f</td><td>%d</td><td>%s</td><td style="font-size:11px;color:#484f58">%s</td></tr>`,
				rowClass, req.Rank, req.Method, req.Path, statusClass, req.StatusCode, req.DurationMs, req.DurationSec, req.ResponseSize, req.Timestamp, req.RequestID)
		}
		html += `</table>`
	}

	// Export all data as JSON button + JS helpers
	html += `
<div style="margin-top:30px;padding-top:15px;border-top:1px solid #30363d;">
  <div class="btn-group">
    <button class="btn" onclick="exportJSON()">Export Full Report (JSON)</button>
    <button class="btn" onclick="window.location.href='/api/forensics'">Raw JSON API</button>
  </div>
</div>
<script>
function exportTable(tableId, filename) {
  const table = document.getElementById(tableId);
  if (!table) return;
  let csv = [];
  for (const row of table.rows) {
    let cols = [];
    for (const cell of row.cells) {
      let text = cell.innerText.replace(/"/g, '""');
      cols.push('"' + text + '"');
    }
    csv.push(cols.join(','));
  }
  downloadFile(csv.join('\n'), filename, 'text/csv');
}
function exportJSON() {
  fetch('/api/forensics').then(r => r.text()).then(data => {
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
</script>`

	html += `</body></html>`
	return html
}
