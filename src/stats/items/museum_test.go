package stats

import (
	"testing"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func TestDecodeMuseumEntriesKeepsOrder(t *testing.T) {
	entries := make([]skycrypttypes.MuseumItem, 64)
	for i := range entries {
		if i%3 == 0 {
			entries[i].Items.Data = "not base64!"
		}
	}

	decoded := decodeMuseumEntries(entries)
	if len(decoded) != len(entries) {
		t.Fatalf("got %d results, want %d", len(decoded), len(entries))
	}
	for i, inventory := range decoded {
		if failed := inventory == nil; failed != (i%3 == 0) {
			t.Errorf("entry %d: decode failed = %v", i, failed)
		}
	}
}
