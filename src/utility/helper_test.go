package utility

import "testing"

func TestGetHexColor(t *testing.T) {
	tests := map[string]string{
		"231,65,60": "E7413C",
		"0,0,0":     "000000",
		"":          "",
		"red":       "",
		"1,2":       "",
	}
	for input, want := range tests {
		if got := GetHexColor(input); got != want {
			t.Errorf("GetHexColor(%q) = %q, want %q", input, got, want)
		}
	}
}
