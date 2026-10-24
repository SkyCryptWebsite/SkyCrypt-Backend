package routes

import "testing"

func TestItemMatchesSearch(t *testing.T) {
	lore := []string{"§7Ability: Wither Impact"}
	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"training weights", true},
		{"wither impact", true},
		{"hyperion", false},
	} {
		if got := itemMatchesSearch("§aTraining Weights", lore, tc.query); got != tc.want {
			t.Errorf("itemMatchesSearch(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}
