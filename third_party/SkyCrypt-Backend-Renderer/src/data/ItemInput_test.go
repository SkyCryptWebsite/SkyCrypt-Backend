package data

import (
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	"image/color"
	"testing"
)

type fixtureItem struct {
	Count  *int        `nbt:"Count" json:"Count,omitempty"`
	Damage *int        `nbt:"Damage" json:"Damage,omitempty"`
	ID     *int        `nbt:"id" json:"id,omitempty"`
	Tag    *fixtureTag `nbt:"tag" json:"tag,omitempty"`
}

type fixtureTag struct {
	ExtraAttributes *fixtureExtraAttributes `nbt:"ExtraAttributes" json:"ExtraAttributes,omitempty"`
	Display         fixtureDisplay          `nbt:"display" json:"display"`
	SkullOwner      *fixtureSkullOwner      `nbt:"SkullOwner" json:"SkullOwner,omitempty"`
	ItemModel       string                  `nbt:"ItemModel" json:"ItemModel,omitempty"`
}

type fixtureExtraAttributes struct {
	ID           string         `nbt:"id" json:"id,omitempty"`
	Modifier     string         `nbt:"modifier" json:"modifier,omitempty"`
	Enchantments map[string]int `nbt:"enchantments" json:"enchantments,omitempty"`
}

type fixtureDisplay struct {
	Name  string   `nbt:"Name" json:"Name,omitempty"`
	Lore  []string `nbt:"Lore" json:"Lore,omitempty"`
	Color int      `nbt:"color" json:"color,omitempty"`
}

type fixtureSkullOwner struct {
	ID         string            `nbt:"Id" json:"Id,omitempty"`
	Properties fixtureProperties `nbt:"Properties" json:"Properties"`
}

type fixtureProperties struct {
	Textures []fixtureTexture `nbt:"textures" json:"textures,omitempty"`
}

type fixtureTexture struct {
	Value     string `nbt:"Value" json:"Value,omitempty"`
	Signature string `nbt:"Signature" json:"Signature,omitempty"`
}

func TestNormalizeItemInputFromSkyCryptShapedStruct(t *testing.T) {
	count := 3
	damage := 5
	id := 397
	input := fixtureItem{
		Count:  &count,
		Damage: &damage,
		ID:     &id,
		Tag: &fixtureTag{
			ItemModel: "minecraft:player_head",
			ExtraAttributes: &fixtureExtraAttributes{
				ID:           "ASPECT_OF_THE_VOID",
				Modifier:     "heroic",
				Enchantments: map[string]int{"ultimate_wise": 5},
			},
			Display: fixtureDisplay{
				Name:  "Aspect of the Void",
				Lore:  []string{"line 1", "line 2"},
				Color: 0x3366CC,
			},
			SkullOwner: &fixtureSkullOwner{
				ID: "profile-id",
				Properties: fixtureProperties{
					Textures: []fixtureTexture{{
						Value:     "texture-value",
						Signature: "texture-signature",
					}},
				},
			},
		},
	}

	normalized, err := NormalizeItemInput(input)
	if err != nil {
		t.Fatalf("NormalizeItemInput returned error: %v", err)
	}

	if normalized.Count != count {
		t.Fatalf("Count = %d, want %d", normalized.Count, count)
	}
	if normalized.Damage != damage {
		t.Fatalf("Damage = %d, want %d", normalized.Damage, damage)
	}
	if normalized.NumericID == nil || *normalized.NumericID != id {
		t.Fatalf("NumericID = %v, want %d", normalized.NumericID, id)
	}
	if normalized.ItemModel != "minecraft:player_head" {
		t.Fatalf("ItemModel = %q", normalized.ItemModel)
	}
	if normalized.SkyblockID != "ASPECT_OF_THE_VOID" {
		t.Fatalf("SkyblockID = %q", normalized.SkyblockID)
	}
	if normalized.DisplayName != "Aspect of the Void" {
		t.Fatalf("DisplayName = %q", normalized.DisplayName)
	}
	if len(normalized.Lore) != 2 {
		t.Fatalf("Lore length = %d", len(normalized.Lore))
	}
	wantColor := color.RGBA{R: 0x33, G: 0x66, B: 0xCC, A: 0xFF}
	if normalized.DisplayColor == nil || *normalized.DisplayColor != wantColor {
		t.Fatalf("DisplayColor = %#v, want %#v", normalized.DisplayColor, wantColor)
	}
	if normalized.ExtraAttributes["id"] != "ASPECT_OF_THE_VOID" {
		t.Fatalf("ExtraAttributes[id] = %#v", normalized.ExtraAttributes["id"])
	}
	if normalized.SkullProfile["value"] != "texture-value" {
		t.Fatalf("SkullProfile[value] = %#v", normalized.SkullProfile["value"])
	}
}

func TestNormalizeItemInputFromMap(t *testing.T) {
	input := map[string]any{
		"Count":  float64(1),
		"Damage": float64(12),
		"id":     float64(35),
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id":    "WOOL_ITEM",
				"model": "custom_model",
			},
			"display": map[string]any{
				"Name":  "Wool",
				"color": float64(0x112233),
			},
		},
	}

	normalized, err := NormalizeItemInput(input)
	if err != nil {
		t.Fatalf("NormalizeItemInput returned error: %v", err)
	}

	if normalized.Damage != 12 {
		t.Fatalf("Damage = %d", normalized.Damage)
	}
	if normalized.NumericID == nil || *normalized.NumericID != 35 {
		t.Fatalf("NumericID = %v", normalized.NumericID)
	}
	if normalized.SkyblockID != "WOOL_ITEM" {
		t.Fatalf("SkyblockID = %q", normalized.SkyblockID)
	}
	if normalized.DisplayColor == nil || *normalized.DisplayColor != (color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xFF}) {
		t.Fatalf("DisplayColor = %#v", normalized.DisplayColor)
	}
}

func TestNormalizeItemInputReadsRootItemIDAlias(t *testing.T) {
	input := map[string]any{
		"item_id": "minecraft:diamond_sword",
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id": "ATOMSPLIT_KATANA",
			},
		},
	}

	normalized, err := NormalizeItemInput(input)
	if err != nil {
		t.Fatalf("NormalizeItemInput returned error: %v", err)
	}

	if normalized.ItemID != "minecraft:diamond_sword" {
		t.Fatalf("ItemID = %q", normalized.ItemID)
	}
	if normalized.SkyblockID != "ATOMSPLIT_KATANA" {
		t.Fatalf("SkyblockID = %q", normalized.SkyblockID)
	}
}

func TestNormalizeItemInputRootItemIDOverridesNonVanillaRootID(t *testing.T) {
	input := map[string]any{
		"id":      "ATOMSPLIT_KATANA",
		"item_id": "minecraft:diamond_sword",
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id": "ATOMSPLIT_KATANA",
			},
		},
	}

	normalized, err := NormalizeItemInput(input)
	if err != nil {
		t.Fatalf("NormalizeItemInput returned error: %v", err)
	}

	if normalized.ItemID != "minecraft:diamond_sword" {
		t.Fatalf("ItemID = %q", normalized.ItemID)
	}
	if normalized.SkyblockID != "ATOMSPLIT_KATANA" {
		t.Fatalf("SkyblockID = %q", normalized.SkyblockID)
	}
}

func TestNormalizeItemInputUsesNbtTagsWhenJsonTagsAreAbsent(t *testing.T) {
	type nbtOnlyExtra struct {
		ID string `nbt:"id"`
	}
	type nbtOnlyTag struct {
		Extra nbtOnlyExtra `nbt:"ExtraAttributes"`
		Model string       `nbt:"ItemModel"`
	}
	type nbtOnlyItem struct {
		ID  int        `nbt:"id"`
		Tag nbtOnlyTag `nbt:"tag"`
	}

	normalized, err := NormalizeItemInput(nbtOnlyItem{
		ID: 397,
		Tag: nbtOnlyTag{
			Model: "minecraft:player_head",
			Extra: nbtOnlyExtra{
				ID: "CUSTOM_HEAD",
			},
		},
	})
	if err != nil {
		t.Fatalf("NormalizeItemInput returned error: %v", err)
	}

	if normalized.NumericID == nil || *normalized.NumericID != 397 {
		t.Fatalf("NumericID = %v", normalized.NumericID)
	}
	if normalized.ItemModel != "minecraft:player_head" {
		t.Fatalf("ItemModel = %q", normalized.ItemModel)
	}
	if normalized.SkyblockID != "CUSTOM_HEAD" {
		t.Fatalf("SkyblockID = %q", normalized.SkyblockID)
	}
}

func TestDecodedMapToNbtCompound(t *testing.T) {
	compound := DecodedMapToNbtCompound(map[string]any{
		"id":       "ASPECT_OF_THE_VOID",
		"modifier": "heroic",
		"stats": map[string]any{
			"damage": 42,
		},
		"lore": []any{"one", "two"},
	})

	if compound == nil {
		t.Fatal("compound is nil")
	}

	idTag, ok := compound.Get("id")
	if !ok {
		t.Fatal("missing id tag")
	}
	idString, ok := idTag.(nbt.NbtString)
	if !ok || idString.Value != "ASPECT_OF_THE_VOID" {
		t.Fatalf("id tag has unexpected type %T", idTag)
	}

	if _, ok := compound.Get("stats"); !ok {
		t.Fatal("missing nested stats compound")
	}
	if _, ok := compound.Get("lore"); !ok {
		t.Fatal("missing lore list")
	}
}
