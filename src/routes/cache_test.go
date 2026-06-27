package routes

import "testing"

func TestDisabledPacksCachePartNormalizesInput(t *testing.T) {
	got := disabledPacksCachePart([]string{"HPLUS", "fsr", "", "unknown", "hplus"})
	want := "FSR,HYPIXEL_PLUS"
	if got != want {
		t.Fatalf("disabledPacksCachePart() = %q, want %q", got, want)
	}
}
