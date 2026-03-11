package forensics

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

type NPlusOneDetector struct {
	logger        *zap.Logger
	queryPatterns map[string]*QueryPattern
	mu            sync.RWMutex
}

type QueryPattern struct {
	Query     string
	Count     uint64
	Timestamp time.Time
	RequestID string
}

var GlobalNPlus1Detector *NPlusOneDetector

func InitNPlus1Detector() {
	GlobalNPlus1Detector = &NPlusOneDetector{
		logger:        Logger,
		queryPatterns: make(map[string]*QueryPattern),
	}
}

func (npd *NPlusOneDetector) RecordQuery(requestID, collection, operation string) {
	npd.mu.Lock()
	defer npd.mu.Unlock()

	queryKey := fmt.Sprintf("%s:%s", collection, operation)
	key := fmt.Sprintf("%s:%s", requestID, queryKey)

	if _, exists := npd.queryPatterns[key]; !exists {
		npd.queryPatterns[key] = &QueryPattern{
			Query:     queryKey,
			Timestamp: time.Now(),
			RequestID: requestID,
		}
	}

	npd.queryPatterns[key].Count++

	if npd.queryPatterns[key].Count == 6 {
		npd.logger.Error("nplus1_query_detected",
			zap.String("request_id", requestID),
			zap.String("query", queryKey),
			zap.Uint64("execution_count", npd.queryPatterns[key].Count),
			zap.String("action_required", "N+1 query pattern detected. Use batch loading or aggregation"),
		)
	}
}

func (npd *NPlusOneDetector) CleanupOldPatterns() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		npd.mu.Lock()
		cutoff := time.Now().Add(-5 * time.Minute)

		cleaned := 0
		for key, pattern := range npd.queryPatterns {
			if pattern.Timestamp.Before(cutoff) {
				delete(npd.queryPatterns, key)
				cleaned++
			}
		}
		npd.mu.Unlock()

		if cleaned > 0 {
			npd.logger.Debug("nplus1_cleanup",
				zap.Int("entries_cleaned", cleaned),
			)
		}
	}
}

func RecordQuery(requestID, collection, operation string) {
	if GlobalNPlus1Detector != nil {
		GlobalNPlus1Detector.RecordQuery(requestID, collection, operation)
	}
}
