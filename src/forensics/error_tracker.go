package forensics

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

type ErrorTracker struct {
	logger     *zap.Logger
	errorCount map[string]*ErrorStats
	mu         sync.RWMutex
}

type ErrorStats struct {
	Count        uint64
	LastOccurred time.Time
	Examples     []string
}

var GlobalErrorTracker *ErrorTracker

func InitErrorTracker() {
	GlobalErrorTracker = &ErrorTracker{
		logger:     Logger,
		errorCount: make(map[string]*ErrorStats),
	}
}

func (et *ErrorTracker) RecordError(errType string, err error, context map[string]interface{}) {
	et.mu.Lock()
	defer et.mu.Unlock()

	if _, exists := et.errorCount[errType]; !exists {
		et.errorCount[errType] = &ErrorStats{
			Examples: make([]string, 0, 10),
		}
	}

	stats := et.errorCount[errType]
	stats.Count++
	stats.LastOccurred = time.Now()

	if len(stats.Examples) < 10 {
		stats.Examples = append(stats.Examples, err.Error())
	}

	fields := []zap.Field{
		zap.String("error_type", errType),
		zap.Error(err),
		zap.Uint64("occurrence_count", stats.Count),
	}

	for key, value := range context {
		fields = append(fields, zap.Any(key, value))
	}

	et.logger.Error("error_recorded", fields...)
}

func (et *ErrorTracker) DumpStats() {
	et.mu.RLock()
	defer et.mu.RUnlock()

	if len(et.errorCount) == 0 {
		et.logger.Info("error_statistics_summary", zap.String("status", "no errors recorded"))
		return
	}

	for errType, stats := range et.errorCount {
		et.logger.Info("error_statistics",
			zap.String("error_type", errType),
			zap.Uint64("total_count", stats.Count),
			zap.Time("last_occurred", stats.LastOccurred),
			zap.Strings("examples", stats.Examples),
		)
	}
}

func (et *ErrorTracker) StartPeriodicSummary() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		et.DumpStats()
	}
}

func RecordError(errType string, err error, context map[string]interface{}) {
	if GlobalErrorTracker != nil {
		GlobalErrorTracker.RecordError(errType, err, context)
	}
}
