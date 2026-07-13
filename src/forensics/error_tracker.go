package forensics

import (
	"fmt"
	"skycrypt/src/security"
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

	redactedErr := security.RedactError(err)
	redactedErrorType := security.RedactString(errType)

	if _, exists := et.errorCount[redactedErrorType]; !exists {
		et.errorCount[redactedErrorType] = &ErrorStats{
			Examples: make([]string, 0, 10),
		}
	}

	stats := et.errorCount[redactedErrorType]
	stats.Count++
	stats.LastOccurred = time.Now()

	if len(stats.Examples) < 10 {
		stats.Examples = append(stats.Examples, redactedErr.Error())
	}

	fields := []zap.Field{
		zap.String("error_type", redactedErrorType),
		zap.Error(redactedErr),
		zap.Uint64("occurrence_count", stats.Count),
	}

	for key, value := range context {
		fields = append(fields, zap.Any(security.RedactString(key), redactLogValue(value)))
	}

	et.logger.Error("error_recorded", fields...)
}

func redactLogValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		return security.RedactString(typed)
	case []byte:
		return security.RedactString(string(typed))
	case error:
		return security.RedactError(typed)
	case fmt.Stringer:
		return security.RedactString(typed.String())
	default:
		return value
	}
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
