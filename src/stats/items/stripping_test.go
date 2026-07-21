package stats

import (
	"encoding/json"
	"strings"
	"testing"

	"skycrypt/src/models"
)

func TestStripItemPreservesTexturePack(t *testing.T) {
	stripped := StripItem(&models.ProcessedItem{
		Texture:     "/cache/rendered/skyblock=TEST__pack=HYPIXEL_PLUS__hash=test.webp",
		TexturePack: "HYPIXEL_PLUS",
	})

	if stripped.TexturePack != "HYPIXEL_PLUS" {
		t.Fatalf("TexturePack = %q, want HYPIXEL_PLUS", stripped.TexturePack)
	}

	encoded, err := json.Marshal(stripped)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"texture_pack":"HYPIXEL_PLUS"`) {
		t.Fatalf("serialized stripped item = %s, want texture_pack", encoded)
	}
}

func TestStripItemOmitsEmptyTexturePack(t *testing.T) {
	encoded, err := json.Marshal(StripItem(&models.ProcessedItem{Texture: "/api/head/test"}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "texture_pack") {
		t.Fatalf("serialized stripped item = %s, want texture_pack omitted", encoded)
	}
}

func TestStripItemsPreservesNestedTexturePacks(t *testing.T) {
	items := []models.ProcessedItem{
		{
			TexturePack: "HYPIXEL_PLUS",
			ContainsItems: []models.ProcessedItem{
				{TexturePack: "FSR"},
			},
		},
	}

	stripped := StripItems(&items)
	if stripped[0].TexturePack != "HYPIXEL_PLUS" {
		t.Fatalf("parent TexturePack = %q, want HYPIXEL_PLUS", stripped[0].TexturePack)
	}
	if len(stripped[0].ContainsItems) != 1 || stripped[0].ContainsItems[0].TexturePack != "FSR" {
		t.Fatalf("nested items = %#v, want FSR texture pack", stripped[0].ContainsItems)
	}
}
