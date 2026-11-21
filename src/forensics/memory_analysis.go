package forensics

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/pprof"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type memoryAnalysis struct {
	HeapInuseMB        uint64              `json:"heap_inuse_mb"`
	HeapAllocMB        uint64              `json:"heap_alloc_mb"`
	HeapIdleMB         uint64              `json:"heap_idle_mb"`
	HeapReleasedMB     uint64              `json:"heap_released_mb"`
	SysMB              uint64              `json:"sys_mb"`
	StackInuseMB       uint64              `json:"stack_inuse_mb"`
	OtherSysMB         uint64              `json:"other_sys_mb"`
	MSpanInuseMB       uint64              `json:"mspan_inuse_mb"`
	MCacheInuseMB      uint64              `json:"mcache_inuse_mb"`
	GCSysMB            uint64              `json:"gc_sys_mb"`
	BuckHashSysMB      uint64              `json:"buck_hash_sys_mb"`
	HeapObjects        uint64              `json:"heap_objects"`
	TotalAllocMB       uint64              `json:"total_alloc_mb"`
	Mallocs            uint64              `json:"mallocs"`
	Frees              uint64              `json:"frees"`
	NextGCMB           uint64              `json:"next_gc_mb"`
	PauseTotalMs       uint64              `json:"pause_total_ms"`
	LastGC             string              `json:"last_gc"`
	NumForcedGC        uint32              `json:"num_forced_gc"`
	EnableGC           bool                `json:"enable_gc"`
	GCCPUPercent       float64             `json:"gc_cpu_percent"`
	SizeClasses        []memorySizeClass   `json:"size_classes"`
	TopHeapSites       []memoryProfileSite `json:"top_heap_sites"`
	TopAllocationSites []memoryProfileSite `json:"top_allocation_sites"`
}

type memorySizeClass struct {
	ObjectSize int    `json:"object_size_bytes"`
	Live       uint64 `json:"live_objects"`
	LiveBytes  uint64 `json:"live_bytes"`
}

type memoryProfileSite struct {
	InUseObjects      int64  `json:"inuse_objects"`
	InUseBytes        int64  `json:"inuse_bytes"`
	CumulativeObjects int64  `json:"cumulative_objects"`
	CumulativeBytes   int64  `json:"cumulative_bytes"`
	Stack             string `json:"stack"`
}

func collectMemoryAnalysis() memoryAnalysis {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	analysis := memoryAnalysis{
		HeapInuseMB:    stats.HeapInuse / 1024 / 1024,
		HeapAllocMB:    stats.HeapAlloc / 1024 / 1024,
		HeapIdleMB:     stats.HeapIdle / 1024 / 1024,
		HeapReleasedMB: stats.HeapReleased / 1024 / 1024,
		SysMB:          stats.Sys / 1024 / 1024,
		StackInuseMB:   stats.StackInuse / 1024 / 1024,
		OtherSysMB:     stats.OtherSys / 1024 / 1024,
		MSpanInuseMB:   stats.MSpanInuse / 1024 / 1024,
		MCacheInuseMB:  stats.MCacheInuse / 1024 / 1024,
		GCSysMB:        stats.GCSys / 1024 / 1024,
		BuckHashSysMB:  stats.BuckHashSys / 1024 / 1024,
		HeapObjects:    stats.HeapObjects,
		TotalAllocMB:   stats.TotalAlloc / 1024 / 1024,
		Mallocs:        stats.Mallocs,
		Frees:          stats.Frees,
		NextGCMB:       stats.NextGC / 1024 / 1024,
		PauseTotalMs:   stats.PauseTotalNs / 1_000_000,
		NumForcedGC:    stats.NumForcedGC,
		EnableGC:       stats.EnableGC,
		GCCPUPercent:   stats.GCCPUFraction * 100,
	}
	if stats.LastGC > 0 {
		analysis.LastGC = time.Unix(0, int64(stats.LastGC)).Format(time.RFC3339)
	}
	for _, class := range stats.BySize {
		if class.Size == 0 {
			continue
		}
		live := class.Mallocs - class.Frees
		if live == 0 {
			continue
		}
		analysis.SizeClasses = append(analysis.SizeClasses, memorySizeClass{
			ObjectSize: int(class.Size),
			Live:       live,
			LiveBytes:  live * uint64(class.Size),
		})
	}
	sort.Slice(analysis.SizeClasses, func(i, j int) bool {
		return analysis.SizeClasses[i].LiveBytes > analysis.SizeClasses[j].LiveBytes
	})
	if len(analysis.SizeClasses) > 12 {
		analysis.SizeClasses = analysis.SizeClasses[:12]
	}
	analysis.TopHeapSites, analysis.TopAllocationSites = memoryProfileSites()
	return analysis
}

const maxProfileSites = 15

// memoryProfileSites returns the largest sites by live bytes and by bytes allocated since start, adding up the
// records of each stack. It reads the runtime's records directly and only symbolizes the stacks it returns:
// writing the heap profile as text symbolized every stack, twice per list, and took most of a core for the many
// seconds it ran, long enough that the dashboard timed out behind the proxy.
func memoryProfileSites() (heap, allocs []memoryProfileSite) {
	sites := map[[32]uintptr]*memoryProfileSite{}
	for _, record := range memProfileRecords() {
		site := sites[record.Stack0]
		if site == nil {
			site = &memoryProfileSite{}
			sites[record.Stack0] = site
		}
		site.InUseObjects += record.InUseObjects()
		site.InUseBytes += record.InUseBytes()
		site.CumulativeObjects += record.AllocObjects
		site.CumulativeBytes += record.AllocBytes
	}
	heap = topMemorySites(sites, func(site *memoryProfileSite) int64 { return site.InUseBytes })
	allocs = topMemorySites(sites, func(site *memoryProfileSite) int64 { return site.CumulativeBytes })
	return heap, allocs
}

func memProfileRecords() []runtime.MemProfileRecord {
	n, _ := runtime.MemProfile(nil, true)
	for {
		// Room for records added between the two calls.
		records := make([]runtime.MemProfileRecord, n+64)
		count, ok := runtime.MemProfile(records, true)
		if ok {
			return records[:count]
		}
		n = count
	}
}

func topMemorySites(sites map[[32]uintptr]*memoryProfileSite, value func(*memoryProfileSite) int64) []memoryProfileSite {
	stacks := make([][32]uintptr, 0, len(sites))
	for stack, site := range sites {
		if value(site) > 0 {
			stacks = append(stacks, stack)
		}
	}
	sort.Slice(stacks, func(i, j int) bool { return value(sites[stacks[i]]) > value(sites[stacks[j]]) })
	if len(stacks) > maxProfileSites {
		stacks = stacks[:maxProfileSites]
	}
	out := make([]memoryProfileSite, 0, len(stacks))
	for _, stack := range stacks {
		site := *sites[stack]
		site.Stack = symbolizeStack(stack)
		out = append(out, site)
	}
	return out
}

// symbolizeStack writes one "function file:line" line per frame, as the text heap profile did.
func symbolizeStack(stack [32]uintptr) string {
	pcs := stack[:]
	if end := slices.Index(pcs, 0); end >= 0 {
		pcs = pcs[:end]
	}
	var out strings.Builder
	frames := runtime.CallersFrames(pcs)
	for {
		frame, more := frames.Next()
		if frame.Function != "" {
			if out.Len() > 0 {
				out.WriteByte('\n')
			}
			fmt.Fprintf(&out, "%s %s:%d", frame.Function, frame.File, frame.Line)
		}
		if !more {
			return out.String()
		}
	}
}

// Sampling rates for the contention profiles, set only while forensics is on. One in mutexProfileFraction
// contended lock waits is recorded; a blocking wait of blockProfileRate or longer always is, shorter ones in
// proportion, so the overhead stays small.
const (
	mutexProfileFraction = 100
	blockProfileRate     = int(time.Millisecond)
)

// EnableContentionProfiles turns on the mutex and block profiles the dashboard serves, which show requests
// waiting on each other's locks and channels in one process.
func EnableContentionProfiles() {
	runtime.SetMutexProfileFraction(mutexProfileFraction)
	runtime.SetBlockProfileRate(blockProfileRate)
}

// CPU profiles are taken one at a time, for this long unless the request asks otherwise.
const (
	defaultCPUProfileSeconds = 30
	maxCPUProfileSeconds     = 120
)

// writeProfile sends a profile for pprof: heap, allocs, goroutine, mutex and block as they stand, and cpu sampled
// for ?seconds=N (default 30).
func writeProfile(c fiber.Ctx, profileName string) error {
	var output bytes.Buffer
	switch profileName {
	case "cpu":
		seconds := defaultCPUProfileSeconds
		if raw := c.Query("seconds"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				seconds = min(n, maxCPUProfileSeconds)
			}
		}
		if err := pprof.StartCPUProfile(&output); err != nil {
			return c.Status(409).SendString("a CPU profile is already being taken; try again when it finishes")
		}
		time.Sleep(time.Duration(seconds) * time.Second)
		pprof.StopCPUProfile()
	case "heap", "allocs", "goroutine", "mutex", "block":
		profile := pprof.Lookup(profileName)
		if profile == nil {
			return c.Status(404).SendString(fmt.Sprintf("profile %q is unavailable", profileName))
		}
		if err := profile.WriteTo(&output, 0); err != nil {
			return c.Status(500).SendString("unable to write profile")
		}
	default:
		return c.Status(400).SendString("unsupported profile; use heap, allocs, goroutine, mutex, block or cpu")
	}
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.pprof", profileName))
	return c.Send(output.Bytes())
}
