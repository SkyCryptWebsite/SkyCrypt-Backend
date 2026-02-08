package forensics

import (
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CriticalPathAnalyzer records operation span durations and identifies
// which operations consume the most total time (bottleneck ranking).
type CriticalPathAnalyzer struct {
	logger *zap.Logger
	spans  map[string]*SpanStats
	mu     sync.RWMutex
}

// SpanStats accumulates timing data for a named operation.
type SpanStats struct {
	Count     uint64
	TotalTime time.Duration
	MinTime   time.Duration
	MaxTime   time.Duration
}

var GlobalCPAnalyzer *CriticalPathAnalyzer

func InitCriticalPathAnalyzer() {
	GlobalCPAnalyzer = &CriticalPathAnalyzer{
		logger: Logger,
		spans:  make(map[string]*SpanStats),
	}
}

// RecordSpan records a single span measurement.
func (cpa *CriticalPathAnalyzer) RecordSpan(name string, duration time.Duration) {
	cpa.mu.Lock()
	defer cpa.mu.Unlock()

	if _, exists := cpa.spans[name]; !exists {
		cpa.spans[name] = &SpanStats{
			MinTime: duration,
			MaxTime: duration,
		}
	}

	stats := cpa.spans[name]
	stats.Count++
	stats.TotalTime += duration

	if duration < stats.MinTime {
		stats.MinTime = duration
	}
	if duration > stats.MaxTime {
		stats.MaxTime = duration
	}
}

// TrackSpan returns a stop function. Usage: defer GlobalCPAnalyzer.TrackSpan("name")()
func (cpa *CriticalPathAnalyzer) TrackSpan(name string) func() {
	start := time.Now()
	return func() {
		duration := time.Since(start)
		cpa.RecordSpan(name, duration)

		// Log to file so all prefork children's data ends up in the shared log
		cpa.logger.Info("span_completed",
			zap.String("span", name),
			zap.Int64("duration_us", duration.Microseconds()),
		)
	}
}

type spanReport struct {
	Name    string
	Stats   *SpanStats
	AvgTime time.Duration
	Percent float64
}

// GenerateReport logs the top 20 slowest operations by total time.
func (cpa *CriticalPathAnalyzer) GenerateReport() {
	cpa.mu.RLock()
	defer cpa.mu.RUnlock()

	if len(cpa.spans) == 0 {
		cpa.logger.Info("critical_path_report", zap.String("status", "no spans recorded"))
		return
	}

	var totalTime time.Duration
	for _, stats := range cpa.spans {
		totalTime += stats.TotalTime
	}

	reports := make([]spanReport, 0, len(cpa.spans))
	for name, stats := range cpa.spans {
		percent := 0.0
		if totalTime > 0 {
			percent = float64(stats.TotalTime) / float64(totalTime) * 100
		}

		avgTime := stats.TotalTime
		if stats.Count > 0 {
			avgTime = stats.TotalTime / time.Duration(stats.Count)
		}

		reports = append(reports, spanReport{
			Name:    name,
			Stats:   stats,
			AvgTime: avgTime,
			Percent: percent,
		})
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Stats.TotalTime > reports[j].Stats.TotalTime
	})

	cpa.logger.Info("critical_path_analysis_start",
		zap.Int("total_operations_tracked", len(reports)),
		zap.Duration("total_cumulative_time", totalTime),
	)

	for i, report := range reports {
		if i >= 20 {
			break
		}

		cpa.logger.Info("operation_timing",
			zap.Int("rank", i+1),
			zap.String("operation", report.Name),
			zap.Uint64("call_count", report.Stats.Count),
			zap.Duration("total_time", report.Stats.TotalTime),
			zap.Duration("avg_time", report.AvgTime),
			zap.Duration("min_time", report.Stats.MinTime),
			zap.Duration("max_time", report.Stats.MaxTime),
			zap.Float64("percent_of_total", report.Percent),
		)

		if report.Percent > 10 {
			cpa.logger.Error("critical_bottleneck_detected",
				zap.String("operation", report.Name),
				zap.Float64("percent_of_total_time", report.Percent),
				zap.Duration("total_time", report.Stats.TotalTime),
				zap.Uint64("call_count", report.Stats.Count),
				zap.String("action_required", "This operation is a critical bottleneck consuming >10% of total time"),
			)
		}
	}
}

// StartPeriodicReport generates reports every 10 minutes. Run in a goroutine.
func (cpa *CriticalPathAnalyzer) StartPeriodicReport() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cpa.GenerateReport()
	}
}

// RecordSpan is a package-level convenience.
func RecordSpan(name string, duration time.Duration) {
	if GlobalCPAnalyzer != nil {
		GlobalCPAnalyzer.RecordSpan(name, duration)
	}
}

// TrackSpan is a package-level convenience. Usage: defer forensics.TrackSpan("op")()
func TrackSpan(name string) func() {
	if GlobalCPAnalyzer != nil {
		return GlobalCPAnalyzer.TrackSpan(name)
	}
	return func() {}
}
