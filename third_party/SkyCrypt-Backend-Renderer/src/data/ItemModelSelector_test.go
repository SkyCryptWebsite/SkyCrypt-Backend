package data

import (
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	"testing"
)

func TestUnsupportedCatharsisSelectFallsBackToFirstCase(t *testing.T) {
	selector := mustParseSelector(t, `{
		"model": {
			"type": "select",
			"property": "catharsis:data_type",
			"data_type": "midas_weapon_paid",
			"cases": [
				{"when": "first", "model": {"type": "model", "model": "minecraft:item/first_case"}},
				{"when": "second", "model": {"type": "model", "model": "minecraft:item/second_case"}}
			],
			"fallback": {"type": "model", "model": "minecraft:item/fallback_case"}
		}
	}`)

	resolved := selector.Resolve(ItemModelContext{ItemName: "golden_sword", DisplayContext: "gui"})
	if resolved == nil || *resolved != "minecraft:item/first_case" {
		t.Fatalf("resolved = %v, want first case", resolved)
	}
}

func TestUnsupportedCatharsisRangeDispatchFallsBackToFirstEntry(t *testing.T) {
	selector := mustParseSelector(t, `{
		"model": {
			"type": "range_dispatch",
			"property": "catharsis:data_type",
			"data_type": "midas_weapon_paid",
			"entries": [
				{"threshold": 0, "model": {"type": "model", "model": "minecraft:item/entry_zero"}},
				{"threshold": 1000000, "model": {"type": "model", "model": "minecraft:item/entry_million"}}
			],
			"fallback": {"type": "model", "model": "minecraft:item/fallback_case"}
		}
	}`)

	resolved := selector.Resolve(ItemModelContext{ItemName: "golden_sword", DisplayContext: "gui"})
	if resolved == nil || *resolved != "minecraft:item/entry_zero" {
		t.Fatalf("resolved = %v, want first entry", resolved)
	}
}

func TestCatharsisSelectDataTypeReadsKnownCustomDataValue(t *testing.T) {
	selector := mustParseSelector(t, `{
		"model": {
			"type": "select",
			"property": "catharsis:data_type",
			"data_type": "modifier",
			"cases": [
				{"when": "heroic", "model": {"type": "model", "model": "minecraft:item/heroic"}},
				{"when": "spicy", "model": {"type": "model", "model": "minecraft:item/spicy"}}
			],
			"fallback": {"type": "model", "model": "minecraft:item/base"}
		}
	}`)
	itemData := &ItemRenderData{CustomData: nbt.NewNbtCompound(map[string]nbt.NbtTag{
		"modifier": nbt.NewNbtString("HEROIC"),
	})}

	resolved := selector.Resolve(ItemModelContext{ItemData: itemData, ItemName: "diamond_sword", DisplayContext: "gui"})
	if resolved == nil || *resolved != "minecraft:item/heroic" {
		t.Fatalf("resolved = %v, want heroic", resolved)
	}
}

func TestCatharsisNumericDataTypeSupportsMidasPaidValue(t *testing.T) {
	selector := mustParseSelector(t, `{
		"model": {
			"type": "range_dispatch",
			"property": "catharsis:data_type",
			"data_type": "midas_weapon_paid",
			"entries": [
				{"threshold": 0, "model": {"type": "model", "model": "minecraft:item/base_midas"}},
				{"threshold": 1000000, "model": {"type": "model", "model": "minecraft:item/rich_midas"}}
			],
			"fallback": {"type": "model", "model": "minecraft:item/base_midas"}
		}
	}`)
	itemData := &ItemRenderData{CustomData: nbt.NewNbtCompound(map[string]nbt.NbtTag{
		"winning_bid": nbt.NewNbtLong(2500000),
	})}

	resolved := selector.Resolve(ItemModelContext{ItemData: itemData, ItemName: "golden_sword", DisplayContext: "gui"})
	if resolved == nil || *resolved != "minecraft:item/rich_midas" {
		t.Fatalf("resolved = %v, want rich midas", resolved)
	}
}

func TestComponentItemModelMatchesExplicitItemModel(t *testing.T) {
	selector := mustParseSelector(t, `{
		"model": {
			"type": "select",
			"property": "component",
			"component": "item_model",
			"cases": [
				{"when": "minecraft:diamond_sword", "model": {"type": "model", "model": "item_melee:item/atomsplit_katana"}},
				{"when": "minecraft:golden_sword", "model": {"type": "model", "model": "item_melee:item/atomsplit_katana_ability"}}
			]
		}
	}`)

	resolved := selector.Resolve(ItemModelContext{ItemName: "firmskyblock:item/atomsplit_katana", ItemModel: "minecraft:diamond_sword", DisplayContext: "gui"})
	if resolved == nil || *resolved != "item_melee:item/atomsplit_katana" {
		t.Fatalf("resolved = %v, want atomsplit katana", resolved)
	}
}

func TestComponentItemModelMatchesItemRenderDataModel(t *testing.T) {
	selector := mustParseSelector(t, `{
		"model": {
			"type": "select",
			"property": "component",
			"component": "minecraft:item_model",
			"cases": [
				{"when": "minecraft:diamond_sword", "model": {"type": "model", "model": "item_melee:item/atomsplit_katana"}},
				{"when": "minecraft:golden_sword", "model": {"type": "model", "model": "item_melee:item/atomsplit_katana_ability"}}
			]
		}
	}`)

	resolved := selector.Resolve(ItemModelContext{
		ItemData:       &ItemRenderData{ItemModel: "minecraft:golden_sword"},
		ItemName:       "firmskyblock:item/atomsplit_katana",
		DisplayContext: "gui",
	})
	if resolved == nil || *resolved != "item_melee:item/atomsplit_katana_ability" {
		t.Fatalf("resolved = %v, want atomsplit ability", resolved)
	}
}

func TestComponentItemModelFallsBackToFirstCaseWhenItemModelMissing(t *testing.T) {
	selector := mustParseSelector(t, `{
		"model": {
			"type": "select",
			"property": "component",
			"component": "item_model",
			"cases": [
				{"when": "minecraft:diamond_sword", "model": {"type": "model", "model": "item_melee:item/atomsplit_katana"}},
				{"when": "minecraft:golden_sword", "model": {"type": "model", "model": "item_melee:item/atomsplit_katana_ability"}}
			]
		}
	}`)

	resolved := ResolveAllItemModelSelector(selector, ItemModelContext{ItemName: "firmskyblock:item/atomsplit_katana", DisplayContext: "gui"})
	if len(resolved) != 1 || resolved[0] != "item_melee:item/atomsplit_katana" {
		t.Fatalf("resolved = %v, want first item_model case", resolved)
	}
}

func TestComponentItemModelDoesNotDefaultWhenExplicitItemModelMismatches(t *testing.T) {
	selector := mustParseSelector(t, `{
		"model": {
			"type": "select",
			"property": "component",
			"component": "item_model",
			"cases": [
				{"when": "minecraft:diamond_sword", "model": {"type": "model", "model": "item_melee:item/atomsplit_katana"}},
				{"when": "minecraft:golden_sword", "model": {"type": "model", "model": "item_melee:item/atomsplit_katana_ability"}}
			]
		}
	}`)

	resolved := ResolveAllItemModelSelector(selector, ItemModelContext{ItemName: "firmskyblock:item/atomsplit_katana", ItemModel: "minecraft:stick", DisplayContext: "gui"})
	if len(resolved) != 0 {
		t.Fatalf("resolved = %v, want no model for explicit mismatch", resolved)
	}
}

func TestResolveAllFallsBackToSingularSelectResolution(t *testing.T) {
	selector := mustParseSelector(t, `{
		"model": {
			"type": "select",
			"property": "display_context",
			"cases": [
				{"when": "ignored", "model": {"type": "model", "model": "minecraft:item/ignored"}}
			],
			"fallback": {
				"type": "range_dispatch",
				"property": "catharsis:data_type",
				"data_type": "midas_weapon_paid",
				"entries": [
					{"threshold": 0, "model": {"type": "model", "model": "item_melee:item/midas_sword_0"}}
				]
			}
		}
	}`)

	resolved := ResolveAllItemModelSelector(selector, ItemModelContext{ItemName: "golden_sword", DisplayContext: "gui"})
	if len(resolved) != 1 || resolved[0] != "item_melee:item/midas_sword_0" {
		t.Fatalf("resolved = %v, want midas fallback model", resolved)
	}
}

func TestResolveAllCompositePreservesAllModels(t *testing.T) {
	selector := mustParseSelector(t, `{
		"model": {
			"type": "composite",
			"models": [
				{"type": "model", "model": "minecraft:item/base"},
				{"type": "model", "model": "minecraft:item/overlay"},
				{"type": "model", "model": "minecraft:item/base"}
			]
		}
	}`)

	resolved := ResolveAllItemModelSelector(selector, ItemModelContext{ItemName: "layered", DisplayContext: "gui"})
	want := []string{"minecraft:item/base", "minecraft:item/overlay"}
	if len(resolved) != len(want) {
		t.Fatalf("resolved = %v, want %v", resolved, want)
	}
	for i := range want {
		if resolved[i] != want[i] {
			t.Fatalf("resolved = %v, want %v", resolved, want)
		}
	}
}

func TestComponentConditionMatchesNestedCustomData(t *testing.T) {
	selector := mustParseSelector(t, `{
		"model": {
			"type": "condition",
			"property": "component",
			"predicate": "custom_data",
			"value": {
				"id": "nested_head_test",
				"runes": {"AXE_FADING_GREEN": 2}
			},
			"on_true": {"type": "model", "model": "minecraft:item/matched"},
			"on_false": {"type": "model", "model": "minecraft:item/fallback"}
		}
	}`)
	itemData := &ItemRenderData{CustomData: nbt.NewNbtCompound(map[string]nbt.NbtTag{
		"id": nbt.NewNbtString("nested_head_test"),
		"runes": nbt.NewNbtCompound(map[string]nbt.NbtTag{
			"AXE_FADING_GREEN": nbt.NewNbtInt(2),
		}),
	})}

	resolved := selector.Resolve(ItemModelContext{ItemData: itemData, ItemName: "player_head", DisplayContext: "gui"})
	if resolved == nil || *resolved != "minecraft:item/matched" {
		t.Fatalf("resolved = %v, want matched", resolved)
	}
}

func mustParseSelector(t *testing.T, raw string) ItemModelSelector {
	t.Helper()
	selector, err := ParseItemModelSelectorFromJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if selector == nil {
		t.Fatal("selector is nil")
	}
	return selector
}
