package lib

import (
	"encoding/base64"
	"reflect"
	"testing"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"

	"skycrypt/src/models"
)

func testSkullTextureValue(hash string) string {
	payload := `{"textures":{"SKIN":{"url":"http://textures.minecraft.net/texture/` + hash + `"}}}`
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

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

func TestRendererPackIDsPreserveExplicitEmptyPreference(t *testing.T) {
	got := packIDsForRenderer([]string{})
	if got == nil || len(got) != 0 {
		t.Fatalf("packIDsForRenderer([]string{}) = %#v, want non-nil empty slice", got)
	}
	if got := packIDsForRenderer(nil); got != nil {
		t.Fatalf("packIDsForRenderer(nil) = %#v, want nil", got)
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

func TestApplyTextureInputReturnsCachedCustomSkullWithoutRendering(t *testing.T) {
	previousCache := itemTextureCache
	context := NewTextureApplyContext([]string{"FSR"})
	itemTextureCache = map[string]AppliedItemTexture{
		textureCacheKey(context.PackSignature, "skyblock:PROCESS_CACHED_SKULL"): {
			Texture:     "/cache/process-cached-skull.png",
			TexturePack: "FSR",
		},
	}
	t.Cleanup(func() { itemTextureCache = previousCache })
	context.DisableRuntimeRender = true

	texture := ApplyTextureInput(ItemTextureInput{
		ID:         "minecraft:player_head",
		ItemModel:  "minecraft:player_head",
		SkyBlockID: "PROCESS_CACHED_SKULL",
		Texture:    "process-cached-skull-hash",
		SkullOwner: &skycrypttypes.SkullOwner{
			Properties: skycrypttypes.Properties{
				Textures: []skycrypttypes.Texture{{Value: testSkullTextureValue("process-cached-skull-hash")}},
			},
		},
	}, context)

	if texture.Texture != "/cache/process-cached-skull.png" || texture.TexturePack != "FSR" {
		t.Fatalf("texture = %#v, want cached FSR texture", texture)
	}
	if context.Stats.RenderAttempts != 0 || context.Stats.CacheHits != 1 {
		t.Fatalf("render attempts/cache hits = %d/%d, want 0/1", context.Stats.RenderAttempts, context.Stats.CacheHits)
	}
}

func TestApplyTextureInputIgnoresDisabledPackSkullCache(t *testing.T) {
	previousCache := itemTextureCache
	disabledContext := NewTextureApplyContext([]string{"FSR"})
	itemTextureCache = map[string]AppliedItemTexture{
		textureCacheKey(disabledContext.PackSignature, "skyblock:PROCESS_DISABLED_SKULL"): {
			Texture:     "/cache/process-disabled-skull.png",
			TexturePack: "FSR",
		},
	}
	t.Cleanup(func() { itemTextureCache = previousCache })
	context := NewTextureApplyContext([]string{"HYPIXEL_PLUS"})
	context.DisableRuntimeRender = true

	texture := ApplyTextureInput(ItemTextureInput{
		ID:         "minecraft:player_head",
		ItemModel:  "minecraft:player_head",
		SkyBlockID: "PROCESS_DISABLED_SKULL",
		Texture:    "process-disabled-skull-hash",
	}, context)

	want := context.Domain + "/api/head/process-disabled-skull-hash"
	if texture.Texture != want || texture.TexturePack != "" {
		t.Fatalf("texture = %#v, want head fallback %q", texture, want)
	}
}

func TestApplyTextureInputRejectsGenericPackedSkullCache(t *testing.T) {
	previousCache := itemTextureCache
	context := NewTextureApplyContext([]string{"FSR"})
	itemTextureCache = map[string]AppliedItemTexture{
		textureCacheKey(context.PackSignature, "skyblock:PROCESS_PLACEHOLDER_SKULL"): {
			Texture:     "/cache/model=minecraft_item_template_skull&tex1=block_soul_sand.png",
			TexturePack: "FSR",
		},
	}
	t.Cleanup(func() { itemTextureCache = previousCache })
	context.DisableRuntimeRender = true

	texture := ApplyTextureInput(ItemTextureInput{
		ID:         "minecraft:player_head",
		ItemModel:  "minecraft:player_head",
		SkyBlockID: "PROCESS_PLACEHOLDER_SKULL",
		Texture:    "process-real-skull-hash",
	}, context)

	want := context.Domain + "/api/head/process-real-skull-hash"
	if texture.Texture != want || texture.TexturePack != "" {
		t.Fatalf("texture = %#v, want real head fallback %q", texture, want)
	}
}

func TestApplyTextureInputRejectsVanillaCustomSkullCache(t *testing.T) {
	tests := []struct {
		name         string
		enabledPacks []string
		cacheKey     string
		texture      AppliedItemTexture
	}{
		{
			name:         "vanilla only",
			enabledPacks: []string{},
			cacheKey:     "vanilla",
			texture: AppliedItemTexture{
				Texture:     "/cache/rendered/skyblock=BURNING_KUUDRA_CORE__pack=vanilla__mc=minecraft_player_head__itemmodel=minecraft_player_head__model=minecraft_item_template_skull__hash=test.webp",
				TexturePack: "vanilla",
			},
		},
		{
			name:         "Hypixel Pack only",
			enabledPacks: []string{"HYPIXEL_PACK"},
			cacheKey:     "enabled-v7:HYPIXEL_PACK",
			texture: AppliedItemTexture{
				Texture:     "/cache/rendered/skyblock=BURNING_KUUDRA_CORE__pack=vanilla__mc=minecraft_player_head__itemmodel=minecraft_player_head__model=minecraft_item_template_skull__hash=test.webp",
				TexturePack: "vanilla",
			},
		},
		{
			name:         "legacy path without metadata",
			enabledPacks: []string{"HYPIXEL_PACK"},
			cacheKey:     "enabled-v7:HYPIXEL_PACK",
			texture: AppliedItemTexture{
				Texture: "/cache/rendered/skyblock=BURNING_KUUDRA_CORE__pack=vanilla__mc=minecraft_player_head__itemmodel=minecraft_player_head__model=minecraft_item_template_skull__hash=test.webp",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previousCache := itemTextureCache
			itemTextureCache = map[string]AppliedItemTexture{
				textureCacheKey(test.cacheKey, "skyblock:BURNING_KUUDRA_CORE"): test.texture,
			}
			t.Cleanup(func() { itemTextureCache = previousCache })

			context := NewTextureApplyContext(test.enabledPacks)
			context.DisableRuntimeRender = true
			texture := ApplyTextureInput(ItemTextureInput{
				ID:         "minecraft:player_head",
				ItemModel:  "minecraft:player_head",
				SkyBlockID: "BURNING_KUUDRA_CORE",
				Texture:    "burning-kuudra-core-hash",
			}, context)

			want := context.Domain + "/api/head/burning-kuudra-core-hash"
			if texture.Texture != want || texture.TexturePack != "" {
				t.Fatalf("texture = %#v, want head fallback %q", texture, want)
			}
		})
	}
}

func TestGenericPackedSkullTexturePreservesCustomPackAndNonSkulls(t *testing.T) {
	customSkullInput := ItemTextureInput{
		ID:         "minecraft:player_head",
		ItemModel:  "minecraft:player_head",
		SkyBlockID: "CUSTOM_PACK_SKULL",
		Texture:    "custom-pack-skull-hash",
	}
	customPackTexture := AppliedItemTexture{
		Texture:     "/cache/rendered/skyblock=CUSTOM_PACK_SKULL__pack=HYPIXEL_PACK__model=minecraft_item_template_skull__hash=test.webp",
		TexturePack: "HYPIXEL_PACK",
	}
	if isGenericPackedSkullTexture(customSkullInput, customPackTexture) {
		t.Fatal("custom pack skull texture was classified as a generic vanilla skull")
	}

	nonSkullInput := ItemTextureInput{
		ID:         "minecraft:iron_sword",
		ItemModel:  "minecraft:iron_sword",
		SkyBlockID: "NON_SKULL_ITEM",
	}
	vanillaTexture := AppliedItemTexture{
		Texture:     "/cache/rendered/skyblock=NON_SKULL_ITEM__pack=vanilla__hash=test.webp",
		TexturePack: "vanilla",
	}
	if isGenericPackedSkullTexture(nonSkullInput, vanillaTexture) {
		t.Fatal("non-skull vanilla texture was classified as a generic skull")
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
