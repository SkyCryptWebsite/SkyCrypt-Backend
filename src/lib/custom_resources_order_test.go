package lib

import (
	"reflect"
	"skycrypt/src/models"
	"testing"
)

func TestNormalizeEnabledPacksPreservesOrderAndAliases(t *testing.T) {
	got := NormalizeEnabledPacks([]string{"fsr", "HPLUS", "unknown", "FSR", "HYPIXEL_PACK"})
	want := []string{"FSR", "HYPIXEL_PLUS", "HYPIXEL_PACK"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeEnabledPacks() = %#v, want %#v", got, want)
	}
}

func TestNormalizeEnabledPacksUsesDefaultsWhenPreferenceIsMissing(t *testing.T) {
	want := defaultResourcePackIDs()
	if got := NormalizeEnabledPacks(nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeEnabledPacks(nil) = %#v, want %#v", got, want)
	}
}

func TestNormalizeEnabledPacksPreservesExplicitEmptyPreference(t *testing.T) {
	for _, input := range [][]string{{}, {"unknown"}} {
		got := NormalizeEnabledPacks(input)
		if got == nil || len(got) != 0 {
			t.Fatalf("NormalizeEnabledPacks(%#v) = %#v, want non-nil empty slice", input, got)
		}
	}
}

func TestTextureApplyContextPreservesEnabledOrder(t *testing.T) {
	context := NewTextureApplyContext([]string{"FSR", "HPLUS", "HYPIXEL_PACK"})
	want := []string{"FSR", "HYPIXEL_PLUS", "HYPIXEL_PACK"}
	if !reflect.DeepEqual(context.EnabledPackIDs, want) {
		t.Fatalf("EnabledPackIDs = %#v, want %#v", context.EnabledPackIDs, want)
	}
	if context.PackSignature != "enabled-v7:FSR,HYPIXEL_PLUS,HYPIXEL_PACK" {
		t.Fatalf("PackSignature = %q", context.PackSignature)
	}
}

func TestTextureApplyContextPreservesVanillaOnlyPreference(t *testing.T) {
	context := normalizeTextureApplyContext(NewTextureApplyContext([]string{}))
	if context.EnabledPackIDs == nil || len(context.EnabledPackIDs) != 0 {
		t.Fatalf("EnabledPackIDs = %#v, want non-nil empty slice", context.EnabledPackIDs)
	}
	if context.PackSignature != "enabled-v7:" {
		t.Fatalf("PackSignature = %q, want %q", context.PackSignature, "enabled-v7:")
	}
}

func TestRendererPackIDsReversePublicPriorityOrder(t *testing.T) {
	input := []string{"FSR", "HYPIXEL_PLUS", "HYPIXEL_PACK"}
	got := packIDsForRenderer(input)
	want := []string{"HYPIXEL_PACK", "HYPIXEL_PLUS", "FSR"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packIDsForRenderer() = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(input, []string{"FSR", "HYPIXEL_PLUS", "HYPIXEL_PACK"}) {
		t.Fatalf("packIDsForRenderer mutated input: %#v", input)
	}
}

func TestStableTextureKeysDoNotUseGenericKeysForSkyBlockItems(t *testing.T) {
	keys := stableTextureKeysFromInput(ItemTextureInput{
		ID:         "minecraft:iron_sword",
		ItemModel:  "minecraft:iron_sword",
		SkyBlockID: "HYPERION",
	})
	want := []string{"skyblock:HYPERION|itemmodel:minecraft:iron_sword"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("stableTextureKeysFromInput() = %#v, want %#v", keys, want)
	}
}

func TestSkyBlockCacheDoesNotReuseDifferentItemsGenericModelTexture(t *testing.T) {
	previousCache := itemTextureCache
	itemTextureCache = map[string]AppliedItemTexture{
		"FSR|itemmodel:minecraft:iron_sword": {
			Texture:     "crypt-dreadlord-texture",
			TexturePack: "FSR",
		},
	}
	t.Cleanup(func() { itemTextureCache = previousCache })

	context := NewTextureApplyContext([]string{"HYPIXEL_PLUS", "FSR"})
	_, ok, _ := cachedTextureForInputDetailed(ItemTextureInput{
		ID:         "minecraft:iron_sword",
		ItemModel:  "minecraft:iron_sword",
		SkyBlockID: "HYPERION",
	}, context)
	if ok {
		t.Fatal("HYPERION reused a generic texture cached for another SkyBlock item")
	}
}

func TestCachedTextureLookupDoesNotLeakAcrossPackOrders(t *testing.T) {
	previousCache := itemTextureCache
	itemTextureCache = map[string]AppliedItemTexture{
		"FSR,HYPIXEL_PLUS|skyblock:ORDERED_CACHE": {
			Texture:     "fsr-texture",
			TexturePack: "FSR",
		},
	}
	t.Cleanup(func() { itemTextureCache = previousCache })

	context := NewTextureApplyContext([]string{"HYPIXEL_PLUS", "FSR"})
	if texture, ok := cachedTextureForStableKey("skyblock:ORDERED_CACHE", context.PackSignature, context.EnabledPackIDs, context.EnabledPackSet); ok {
		t.Fatalf("unexpected cross-order cache hit: %#v", texture)
	}
}

func TestCachedTextureLookupUsesFirstEnabledPackWithTexture(t *testing.T) {
	previousCache := itemTextureCache
	itemTextureCache = map[string]AppliedItemTexture{
		"HYPIXEL_PLUS|skyblock:OVERLAP": {
			Texture:     "hplus-texture",
			TexturePack: "HYPIXEL_PLUS",
		},
		"FSR|skyblock:OVERLAP": {
			Texture:     "fsr-texture",
			TexturePack: "FSR",
		},
	}
	t.Cleanup(func() { itemTextureCache = previousCache })

	context := NewTextureApplyContext([]string{"HYPIXEL_PLUS", "FSR"})
	texture, ok := cachedTextureForStableKey("skyblock:OVERLAP", context.PackSignature, context.EnabledPackIDs, context.EnabledPackSet)
	if !ok {
		t.Fatal("expected cached texture")
	}
	if texture.TexturePack != "HYPIXEL_PLUS" || texture.Texture != "hplus-texture" {
		t.Fatalf("texture = %#v, want HYPIXEL_PLUS", texture)
	}
}

func TestSortResourcePackConfigsUsesPriorityThenID(t *testing.T) {
	configs := []models.ResourcePackConfig{
		{Id: "B", Priority: 10},
		{Id: "C", Priority: 20},
		{Id: "A", Priority: 10},
	}
	sortResourcePackConfigs(configs)
	want := []string{"C", "A", "B"}
	for index, id := range want {
		if configs[index].Id != id {
			t.Fatalf("configs[%d].Id = %q, want %q", index, configs[index].Id, id)
		}
	}
}
