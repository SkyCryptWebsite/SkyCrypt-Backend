package forensics

import (
	"bufio"
	"bytes"
	"fmt"
	"hash/fnv"
	"io"
	"runtime"
	"runtime/pprof"
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
	analysis.TopHeapSites = profileSites("heap")
	analysis.TopAllocationSites = profileSites("allocs")
	return analysis
}

func profileSites(name string) []memoryProfileSite {
	profile := pprof.Lookup(name)
	if profile == nil {
		return nil
	}
	sites, err := topProfileSites(func() (io.ReadCloser, error) {
		reader, writer := io.Pipe()
		go func() {
			_ = writer.CloseWithError(profile.WriteTo(writer, 1))
		}()
		return reader, nil
	})
	if err != nil {
		return nil
	}
	return sites
}

const maxProfileSites = 15

type siteCounts struct {
	inUseObjects, inUseBytes, cumulativeObjects, cumulativeBytes int64
}

// topProfileSites returns the profile's largest sites by live bytes, adding up records whose stacks name the same
// functions. It reads the profile twice: once summing counts per stack hash, then once more for the winners'
// stack text. Keeping every stack as a string instead held a few hundred MB for a long-running process's
// allocation profile, each time the dashboard was built.
func topProfileSites(open func() (io.ReadCloser, error)) ([]memoryProfileSite, error) {
	counts := map[uint64]*siteCounts{}
	err := scanProfileFile(open, func(record siteCounts, frames []string) {
		key := hashFrames(frames)
		total := counts[key]
		if total == nil {
			total = &siteCounts{}
			counts[key] = total
		}
		total.inUseObjects += record.inUseObjects
		total.inUseBytes += record.inUseBytes
		total.cumulativeObjects += record.cumulativeObjects
		total.cumulativeBytes += record.cumulativeBytes
	})
	if err != nil || len(counts) == 0 {
		return nil, err
	}

	keys := make([]uint64, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return counts[keys[i]].inUseBytes > counts[keys[j]].inUseBytes })
	if len(keys) > maxProfileSites {
		keys = keys[:maxProfileSites]
	}
	stacks := make(map[uint64]string, len(keys))
	for _, key := range keys {
		stacks[key] = ""
	}
	err = scanProfileFile(open, func(_ siteCounts, frames []string) {
		key := hashFrames(frames)
		if stack, wanted := stacks[key]; wanted && stack == "" {
			stacks[key] = strings.Join(frames, "\n")
		}
	})
	if err != nil {
		return nil, err
	}

	sites := make([]memoryProfileSite, 0, len(keys))
	for _, key := range keys {
		total := counts[key]
		sites = append(sites, memoryProfileSite{
			InUseObjects: total.inUseObjects, InUseBytes: total.inUseBytes,
			CumulativeObjects: total.cumulativeObjects, CumulativeBytes: total.cumulativeBytes,
			Stack: stacks[key],
		})
	}
	return sites, nil
}

func scanProfileFile(open func() (io.ReadCloser, error), visit func(siteCounts, []string)) error {
	reader, err := open()
	if err != nil {
		return err
	}
	err = scanProfile(reader, visit)
	_ = reader.Close()
	return err
}

// scanProfile calls visit once per record of a text (debug=1) heap profile, with its counts and frame names. The
// frames slice is reused between records.
func scanProfile(reader io.Reader, visit func(siteCounts, []string)) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	var pending *siteCounts
	var frames []string
	flush := func() {
		if pending != nil && len(frames) > 0 {
			visit(*pending, frames)
		}
		pending = nil
		frames = frames[:0]
	}
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasSuffix(fields[0], ":") {
			if pending != nil && strings.HasPrefix(strings.TrimSpace(line), "#") {
				if frame := profileStackName(line); frame != "" {
					frames = append(frames, frame)
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
		pending = &siteCounts{inUseObjects: objects, inUseBytes: bytesUsed}
		if len(fields) >= 4 {
			cumulativeObjects := strings.TrimSuffix(strings.TrimPrefix(fields[2], "["), ":")
			pending.cumulativeObjects, _ = strconv.ParseInt(cumulativeObjects, 10, 64)
			pending.cumulativeBytes, _ = strconv.ParseInt(strings.TrimSuffix(fields[3], "]"), 10, 64)
		}
	}
	flush()
	return scanner.Err()
}

// hashFrames identifies a stack by its frame names.
func hashFrames(frames []string) uint64 {
	h := fnv.New64a()
	for _, frame := range frames {
		_, _ = h.Write([]byte(frame))
		_, _ = h.Write([]byte{'\n'})
	}
	return h.Sum64()
}

func profileStackName(line string) string {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 3 {
		return ""
	}
	return strings.Join(fields[2:], " ")
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
