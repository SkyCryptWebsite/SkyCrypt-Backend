package forensics

import (
	"runtime"
	"strings"
	"testing"
)

func testStack(t *testing.T) [32]uintptr {
	t.Helper()
	var stack [32]uintptr
	runtime.Callers(1, stack[:])
	return stack
}

func TestTopMemorySitesRanksAndSymbolizes(t *testing.T) {
	first, second := testStack(t), [32]uintptr{}
	copy(second[:], first[1:])
	sites := map[[32]uintptr]*memoryProfileSite{
		first:  {InUseBytes: 100, CumulativeBytes: 900},
		second: {InUseBytes: 500, CumulativeBytes: 600},
	}

	heap := topMemorySites(sites, func(site *memoryProfileSite) int64 { return site.InUseBytes })
	if len(heap) != 2 || heap[0].InUseBytes != 500 || heap[1].InUseBytes != 100 {
		t.Fatalf("heap order = %+v", heap)
	}
	allocs := topMemorySites(sites, func(site *memoryProfileSite) int64 { return site.CumulativeBytes })
	if allocs[0].CumulativeBytes != 900 {
		t.Fatalf("allocs order = %+v", allocs)
	}
	if !strings.Contains(allocs[0].Stack, "forensics.testStack") || !strings.Contains(allocs[0].Stack, "memory_analysis_test.go:") {
		t.Fatalf("stack = %q", allocs[0].Stack)
	}
}

func TestTopMemorySitesSkipsEmptyAndCaps(t *testing.T) {
	sites := map[[32]uintptr]*memoryProfileSite{}
	for i := range maxProfileSites + 5 {
		sites[[32]uintptr{uintptr(i + 1)}] = &memoryProfileSite{InUseBytes: int64(i)}
	}
	heap := topMemorySites(sites, func(site *memoryProfileSite) int64 { return site.InUseBytes })
	if len(heap) != maxProfileSites {
		t.Fatalf("len = %d, want %d", len(heap), maxProfileSites)
	}
	for _, site := range heap {
		if site.InUseBytes == 0 {
			t.Fatal("kept a site with no live bytes")
		}
	}
}

func TestMemoryProfileSitesFindsLiveAllocation(t *testing.T) {
	previous := runtime.MemProfileRate
	runtime.MemProfileRate = 1
	t.Cleanup(func() { runtime.MemProfileRate = previous })

	kept := make([][]byte, 0, 64)
	for range 64 {
		kept = append(kept, make([]byte, 64<<10))
	}
	runtime.GC()
	runtime.GC()

	heap, allocs := memoryProfileSites()
	runtime.KeepAlive(kept)
	if len(heap) == 0 || len(allocs) == 0 {
		t.Fatalf("heap = %d sites, allocs = %d sites", len(heap), len(allocs))
	}
	found := false
	for _, site := range heap {
		if strings.Contains(site.Stack, "TestMemoryProfileSitesFindsLiveAllocation") {
			found = true
		}
	}
	if !found {
		t.Fatalf("test allocation missing from heap sites: %+v", heap)
	}
}
