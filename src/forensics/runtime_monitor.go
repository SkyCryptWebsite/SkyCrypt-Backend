package forensics

import (
	"runtime"
	"time"

	"go.uber.org/zap"
)

type RuntimeMonitor struct {
	logger *zap.Logger
}

func NewRuntimeMonitor() *RuntimeMonitor {
	return &RuntimeMonitor{logger: Logger}
}

func (rm *RuntimeMonitor) Start() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	var lastNumGoroutine int
	var lastAlloc uint64

	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		numGoroutine := runtime.NumGoroutine()

		rm.logger.Info("runtime_stats",
			zap.Int("num_goroutines", numGoroutine),
			zap.Uint64("alloc_mb", m.Alloc/1024/1024),
			zap.Uint64("total_alloc_mb", m.TotalAlloc/1024/1024),
			zap.Uint64("sys_mb", m.Sys/1024/1024),
			zap.Uint32("num_gc", m.NumGC),
			zap.Uint64("heap_alloc_mb", m.HeapAlloc/1024/1024),
			zap.Uint64("heap_sys_mb", m.HeapSys/1024/1024),
			zap.Uint64("heap_idle_mb", m.HeapIdle/1024/1024),
			zap.Uint64("heap_inuse_mb", m.HeapInuse/1024/1024),
			zap.Uint64("heap_released_mb", m.HeapReleased/1024/1024),
			zap.Uint64("heap_objects", m.HeapObjects),
			zap.Uint64("stack_inuse_bytes", m.StackInuse),
			zap.Uint64("mallocs", m.Mallocs),
			zap.Uint64("frees", m.Frees),
		)

		if lastNumGoroutine > 0 && numGoroutine > lastNumGoroutine+100 {
			rm.logger.Error("goroutine_leak_detected",
				zap.Int("current_goroutines", numGoroutine),
				zap.Int("previous_goroutines", lastNumGoroutine),
				zap.Int("increase", numGoroutine-lastNumGoroutine),
				zap.String("action_required", "Goroutines increasing rapidly. Check for leaked goroutines"),
			)
		}

		if numGoroutine > 10000 {
			rm.logger.Error("goroutine_count_critical",
				zap.Int("num_goroutines", numGoroutine),
				zap.String("action_required", "Goroutine count extremely high"),
			)
		} else if numGoroutine > 1000 {
			rm.logger.Warn("goroutine_count_high",
				zap.Int("num_goroutines", numGoroutine),
			)
		}

		if lastAlloc > 0 && m.Alloc > lastAlloc+100*1024*1024 {
			rm.logger.Error("memory_leak_suspected",
				zap.Uint64("current_alloc_mb", m.Alloc/1024/1024),
				zap.Uint64("previous_alloc_mb", lastAlloc/1024/1024),
				zap.Uint64("increase_mb", (m.Alloc-lastAlloc)/1024/1024),
				zap.String("action_required", "Memory increasing rapidly. Check for leaks"),
			)
		}

		if m.Alloc > 1*1024*1024*1024 { // 1GB
			rm.logger.Warn("high_memory_usage",
				zap.Uint64("alloc_mb", m.Alloc/1024/1024),
				zap.String("suggestion", "Memory usage exceeds 1GB. Consider optimization"),
			)
		}

		if m.NumGC > 0 {
			lastPauseNs := m.PauseNs[(m.NumGC+255)%256]
			lastPauseMs := lastPauseNs / 1_000_000

			if lastPauseMs > 10 {
				rm.logger.Warn("gc_pause_time_high",
					zap.Uint64("pause_ms", lastPauseMs),
					zap.Uint32("num_gc", m.NumGC),
					zap.String("suggestion", "GC pauses >10ms. Reduce heap allocations"),
				)
			}

			if lastPauseMs > 50 {
				rm.logger.Error("gc_pause_time_critical",
					zap.Uint64("pause_ms", lastPauseMs),
					zap.String("action_required", "GC stop-the-world pauses >50ms. Investigate allocation hot paths"),
				)
			}
		}

		lastNumGoroutine = numGoroutine
		lastAlloc = m.Alloc
	}
}

func (rm *RuntimeMonitor) DumpGoroutines() {
	buf := make([]byte, 1<<20) // 1MB buffer
	stackSize := runtime.Stack(buf, true)

	rm.logger.Error("goroutine_stack_dump",
		zap.Int("num_goroutines", runtime.NumGoroutine()),
		zap.String("stack_trace", string(buf[:stackSize])),
	)
}
