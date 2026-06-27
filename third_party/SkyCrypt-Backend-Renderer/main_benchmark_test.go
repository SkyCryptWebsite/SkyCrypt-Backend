package renderer

import (
	"context"
	"runtime"
	"testing"
)

func BenchmarkPreRenderSkyBlockItemIDs(b *testing.B) {
	renderer := newTestRenderer(b)
	ids := []string{"HYPERION", "INFERNO_HELMET", "DIRT", "CROWN_OF_AVARICE"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := renderer.PreRenderSkyBlockItemIDs(context.Background(), ids, PreRenderOptions{
			Workers:   runtime.GOMAXPROCS(0),
			Overwrite: true,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
