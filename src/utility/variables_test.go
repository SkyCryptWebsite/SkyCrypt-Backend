package utility

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
)

// The regular expression ReplaceVariables used to run, kept here as the reference it must agree with.
var replaceVarRegex = regexp.MustCompile(`\{(\w+)\}`)

func replaceVariablesWithRegex(template string, variables map[string]float64) string {
	return replaceVarRegex.ReplaceAllStringFunc(template, func(match string) string {
		value, exists := variables[strings.Trim(match, "{}")]
		if !exists {
			return match
		}
		return fmt.Sprintf("%.0f", value)
	})
}

func TestReplaceVariablesMatchesRegex(t *testing.T) {
	variables := map[string]float64{"a": 1.5, "b_2": -3.49, "LONG_name": 1234567.8, "z": 0}
	cases := []string{
		"", "plain", "{a}", "§7Damage: §c+{a}", "{a}{b_2}", "{missing}", "{a", "a}", "{}", "{{a}}", "{a{b_2}",
		"{ a}", "{a }", "x{LONG_name}y{z}", "{é}", "{a}{missing}{z}", "}{a}{",
	}
	random := rand.New(rand.NewPCG(1, 2))
	alphabet := []string{"{", "}", "a", "b", "_", "2", "z", " ", "é", "LONG_name", "missing"}
	for range 20000 {
		var b strings.Builder
		for range random.IntN(12) {
			b.WriteString(alphabet[random.IntN(len(alphabet))])
		}
		cases = append(cases, b.String())
	}
	for _, template := range cases {
		if got, want := ReplaceVariables(template, variables), replaceVariablesWithRegex(template, variables); got != want {
			t.Fatalf("ReplaceVariables(%q) = %q, want %q", template, got, want)
		}
	}
}

func TestGetSkinHashRemembers(t *testing.T) {
	// {"textures":{"SKIN":{"url":"http://textures.minecraft.net/texture/abc123"}}}
	texture := "eyJ0ZXh0dXJlcyI6eyJTS0lOIjp7InVybCI6Imh0dHA6Ly90ZXh0dXJlcy5taW5lY3JhZnQubmV0L3RleHR1cmUvYWJjMTIzIn19fQ=="
	for range 3 {
		if got := GetSkinHash(texture); got != "abc123" {
			t.Fatalf("GetSkinHash = %q, want abc123", got)
		}
	}
	if got := GetSkinHash("not base64!"); got != "" {
		t.Fatalf("GetSkinHash(bad) = %q", got)
	}
}
