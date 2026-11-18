package notenoughupdates

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseMaxEnchants(t *testing.T) {
	dir := t.TempDir()
	itemsDir := filepath.Join(dir, "NotEnoughUpdates-REPO", "items")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	book := func(id string, lore ...string) {
		data, err := json.Marshal(map[string]any{"itemid": "minecraft:enchanted_book", "internalname": id, "lore": lore})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(itemsDir, id+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	book("SHARPNESS;1", "§8Combinable in Anvil", "", "§9Sharpness I")
	book("EXPERTISE;1", "§8Combinable in Anvil", "", "§9Expertise I", "§8Kill 50 Sea Creatures for Tier II", "§8Kill 2,5k Sea Creatures for Tier X")
	book("EFFICIENCY;5", "§8Combinable in Anvil", "", "§9Efficiency V", " §7- Enchantment Table (§aI-V§7)", " §7- §5Silex§7 (§aVI-X§7)")
	book("SCUBA;1", "§8Combinable in Anvil", "", "§9Scuba I")
	book("SCUBA;5", "§8Combinable in Anvil", "", "§9Scuba 5")
	book("ULTIMATE_LEGION;5", "§8Combinable in Anvil", "", "§d§lLegion V")
	if err := os.WriteFile(filepath.Join(itemsDir, "GOLDEN_DRAGON;4.json"), []byte(`{"itemid":"minecraft:skull","lore":["a","b","c"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)
	got, err := parseMaxEnchants(map[string][]string{
		"SHARPNESS;1":    {"SHARPNESS;2", "SHARPNESS;7"},
		"ADAPTIVE_BOOTS": {"STARRED_ADAPTIVE_BOOTS"},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"Efficiency X", "Expertise X", "Scuba V", "Sharpness VII"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseMaxEnchants() = %#v, want %#v", got, want)
	}
}
