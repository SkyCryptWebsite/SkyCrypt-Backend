package db

import (
	"strings"
	"testing"
)

func TestCompressValueRoundTrip(t *testing.T) {
	large := strings.Repeat(`{"profiles":[{"profile_id":"abc"}]}`, 100)

	compressed, ok := compressValue(large).([]byte)
	if !ok || len(compressed) >= len(large) {
		t.Fatalf("large value was not compressed")
	}
	got, err := decompressValue(string(compressed))
	if err != nil || got != large {
		t.Fatalf("round trip failed: %v", err)
	}

	if small := compressValue("4855c53ee4fb4100997600a92fc50984"); small != "4855c53ee4fb4100997600a92fc50984" {
		t.Fatalf("small value should be stored as-is, got %v", small)
	}
	if got, err := decompressValue(large); err != nil || got != large {
		t.Fatalf("plain values written before compression must pass through")
	}
}
