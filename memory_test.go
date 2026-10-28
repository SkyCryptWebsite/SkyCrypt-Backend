package main

import "testing"

func TestParseBytes(t *testing.T) {
	cases := map[string]int64{"7GiB": 7 << 30, "512MiB": 512 << 20, "1.5GiB": 3 << 29, "1048576": 1 << 20}
	for input, want := range cases {
		if got, ok := parseBytes(input); !ok || got != want {
			t.Errorf("parseBytes(%q) = %d %v, want %d", input, got, ok, want)
		}
	}
	for _, input := range []string{"", "-1GiB", "lots", "0"} {
		if _, ok := parseBytes(input); ok {
			t.Errorf("parseBytes(%q) should fail", input)
		}
	}
}
