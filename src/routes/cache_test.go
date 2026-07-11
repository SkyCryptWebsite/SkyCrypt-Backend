package routes

import "testing"

func TestEnabledPacksCachePartPreservesNormalizedOrder(t *testing.T) {
	got := enabledPacksCachePart([]string{"fsr", "HPLUS", "", "unknown", "hplus"})
	want := "enabled-v7:FSR,HYPIXEL_PLUS"
	if got != want {
		t.Fatalf("enabledPacksCachePart() = %q, want %q", got, want)
	}
}

func TestEnabledPacksCachePartIsOrderSensitive(t *testing.T) {
	first := enabledPacksCachePart([]string{"FSR", "HYPIXEL_PLUS"})
	second := enabledPacksCachePart([]string{"HYPIXEL_PLUS", "FSR"})
	if first == second {
		t.Fatalf("enabled pack cache parts should differ by order: %q", first)
	}
}
