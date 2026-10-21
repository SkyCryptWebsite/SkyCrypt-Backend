package utility

import (
	"regexp"
	"testing"
)

func TestIsUUIDMatchesRegex(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9a-f]{32}$`)
	for _, input := range []string{
		"4855c53ee4fb4100997600a92fc50984",
		"4855C53EE4FB4100997600A92FC50984",
		"4855c53e-e4fb-4100-9976-00a92fc50984",
		"4855c53ee4fb4100997600a92fc5098",
		"4855c53ee4fb4100997600a92fc50984a",
		"g855c53ee4fb4100997600a92fc50984",
		"",
		"Refraction",
	} {
		if got, want := IsUUID(input), pattern.MatchString(input); got != want {
			t.Errorf("IsUUID(%q) = %v, regex says %v", input, got, want)
		}
	}
}
