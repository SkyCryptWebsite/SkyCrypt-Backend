package forensics

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
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
	analysis.TopHeapSites = profileSites("heap")
	analysis.TopAllocationSites = profileSites("allocs")
	return analysis
}

func profileSites(name string) []memoryProfileSite {
	profile := pprof.Lookup(name)
	if profile == nil {
		return nil
	}
	var output bytes.Buffer
	if err := profile.WriteTo(&output, 1); err != nil {
		return nil
	}

	sitesByStack := make(map[string]*memoryProfileSite)
	var pending *memoryProfileSite
	var stackLines []string
	flush := func() {
		if pending != nil && len(stackLines) > 0 {
			pending.Stack = strings.Join(stackLines, "\n")
			if existing := sitesByStack[pending.Stack]; existing != nil {
				existing.InUseObjects += pending.InUseObjects
				existing.InUseBytes += pending.InUseBytes
				existing.CumulativeObjects += pending.CumulativeObjects
				existing.CumulativeBytes += pending.CumulativeBytes
			} else {
				sitesByStack[pending.Stack] = pending
			}
		}
		pending = nil
		stackLines = nil
	}
	for _, line := range strings.Split(output.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasSuffix(fields[0], ":") {
			if pending != nil && strings.HasPrefix(strings.TrimSpace(line), "#") {
				if frame := profileStackName(line); frame != "" {
					stackLines = append(stackLines, frame)
				}
			}
			continue
		}
		objects, objectErr := strconv.ParseInt(strings.TrimSuffix(fields[0], ":"), 10, 64)
		bytesUsed, bytesErr := strconv.ParseInt(fields[1], 10, 64)
		if objectErr != nil || bytesErr != nil {
			continue
		}
		flush()
		pending = &memoryProfileSite{InUseObjects: objects, InUseBytes: bytesUsed}
		if len(fields) >= 4 {
			cumulativeObjects := strings.TrimSuffix(strings.TrimPrefix(fields[2], "["), ":")
			pending.CumulativeObjects, _ = strconv.ParseInt(cumulativeObjects, 10, 64)
			pending.CumulativeBytes, _ = strconv.ParseInt(strings.TrimSuffix(fields[3], "]"), 10, 64)
		}
	}
	flush()
	sites := make([]memoryProfileSite, 0, len(sitesByStack))
	for _, site := range sitesByStack {
		sites = append(sites, *site)
	}
	sort.Slice(sites, func(i, j int) bool {
		return sites[i].InUseBytes > sites[j].InUseBytes
	})
	if len(sites) > 15 {
		sites = sites[:15]
	}
	return sites
}

func profileStackName(line string) string {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 3 {
		return ""
	}
	return strings.Join(fields[2:], " ")
}

func writeMemoryProfile(c *fiber.Ctx, profileName string) error {
	if profileName != "heap" && profileName != "allocs" && profileName != "goroutine" {
		return c.Status(400).SendString("unsupported profile; use heap, allocs, or goroutine")
	}
	profile := pprof.Lookup(profileName)
	if profile == nil {
		return c.Status(404).SendString(fmt.Sprintf("profile %q is unavailable", profileName))
	}
	var output bytes.Buffer
	if err := profile.WriteTo(&output, 0); err != nil {
		return c.Status(500).SendString("unable to write profile")
	}
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.pprof", profileName))
	return c.Send(output.Bytes())
}
