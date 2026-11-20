package db

import (
	"runtime"
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

func TestCompressValuePooledReusesBuffers(t *testing.T) {
	first := strings.Repeat(`{"profiles":[{"profile_id":"first"}]}`, 100)
	second := strings.Repeat(`{"member":"second","coins":12345}`, 200)

	compressed, release := compressValuePooled(first)
	got, err := decompressValue(string(compressed.([]byte)))
	if err != nil || got != first {
		t.Fatalf("first round trip failed: %v", err)
	}
	release()

	// The second value may land in the first one's buffer; it must not keep any of its bytes.
	compressed, release = compressValuePooled([]byte(second))
	defer release()
	got, err = decompressValue(string(compressed.([]byte)))
	if err != nil || got != second {
		t.Fatalf("second round trip failed: %v", err)
	}

	small := "4855c53ee4fb4100997600a92fc50984"
	value, releaseSmall := compressValuePooled(small)
	releaseSmall()
	if value != small {
		t.Fatalf("small value should be stored as-is, got %v", value)
	}
}

func TestCompressBuffersSurviveGC(t *testing.T) {
	for len(compressBuffers) > 0 {
		<-compressBuffers
	}
	value := strings.Repeat(`{"profiles":[{"profile_id":"kept"}]}`, 100)

	compressed, release := compressValuePooled(value)
	first := compressed.([]byte)
	release()
	runtime.GC()
	runtime.GC()

	compressed, release = compressValuePooled(value)
	defer release()
	if second := compressed.([]byte); &first[0] != &second[0] {
		t.Fatal("buffer was not reused after a GC")
	}
}
