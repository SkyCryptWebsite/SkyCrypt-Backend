package utility

import "testing"

func TestGetRawLoreMatchesRegex(t *testing.T) {
	for _, input := range []string{
		"",
		"plain text",
		"§aGreen §lbold§r normal",
		"§§a doubled",
		"§A uppercase is not a code",
		"ends with §",
		"§zunknown code",
		"§6§l§ka§r §6§lLEGENDARY SWORD §6§l§ka",
		"multi-byte ✪✪✪ §5stars",
		"§",
		"§k",
	} {
		want := colorCodeRegex.ReplaceAllString(input, "")
		if got := GetRawLore(input); got != want {
			t.Errorf("GetRawLore(%q) = %q, regex gives %q", input, got, want)
		}
	}
}

func TestTitleCaseUnchanged(t *testing.T) {
	for input, want := range map[string]string{
		"golden_dragon": "Golden Dragon",
		"sheep":         "Sheep",
		"MITHRIL-GOLEM": "Mithril Golem",
	} {
		if got := TitleCase(input); got != want {
			t.Errorf("TitleCase(%q) = %q, want %q", input, got, want)
		}
	}
}
