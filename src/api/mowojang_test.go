package api

import (
	"skycrypt/src/models"
	"testing"
)

func TestUniqueUnresolvedUUIDs(t *testing.T) {
	resolved := map[string]*models.MowojangResponse{
		"resolved": {UUID: "resolved", Name: "Resolved"},
	}

	got := uniqueUnresolvedUUIDs([]string{"", "a", "resolved", "a", "b", "b"}, resolved)
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q: %v", i, got[i], want[i], got)
		}
	}
}

func TestUniqueUnresolvedUsernames(t *testing.T) {
	usernames := map[string]string{
		"resolved": "Resolved",
	}

	got := uniqueUnresolvedUsernames([]string{"", "a", "resolved", "a", "b", "b"}, usernames)
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q: %v", i, got[i], want[i], got)
		}
	}
}
