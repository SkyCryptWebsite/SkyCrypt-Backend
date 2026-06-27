package lib

import (
	"encoding/base64"
	"os"
	"path/filepath"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"
	"testing"

	mr "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer"
	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func TestPublicCacheTextureURLNormalizesAbsoluteCachePath(t *testing.T) {
	got := publicCacheTextureURL("/home/duckysolucky/Desktop/SkyCrypt-Backend/cache/rendered/skyblock=HYPERION.webp")
	want := testDomain() + "/cache/rendered/skyblock=HYPERION.webp"
	if got != want {
		t.Fatalf("publicCacheTextureURL() = %q, want %q", got, want)
	}
}

func TestPublicCacheTextureURLNormalizesRelativeCachePath(t *testing.T) {
	got := publicCacheTextureURL("cache/rendered/skyblock=HYPERION.webp")
	want := testDomain() + "/cache/rendered/skyblock=HYPERION.webp"
	if got != want {
		t.Fatalf("publicCacheTextureURL() = %q, want %q", got, want)
	}
}

func TestPublicCacheTextureURLKeepsPublicURL(t *testing.T) {
	got := publicCacheTextureURL("https://cdn.example/cache/rendered/skyblock=HYPERION.webp")
	want := "https://cdn.example/cache/rendered/skyblock=HYPERION.webp"
	if got != want {
		t.Fatalf("publicCacheTextureURL() = %q, want %q", got, want)
	}
}

func TestPreRenderSkyBlockItemIDsIncludesCachedNEUItems(t *testing.T) {
	notenoughupdates.CACHED_NEU_ITEMS.Store("TEST_NEU_CACHE_KEY", models.NEUItem{
		NEUId: "TEST_NEU_ID",
		NBT: skycrypttypes.Tag{
			ExtraAttributes: &skycrypttypes.ExtraAttributes{Id: "TEST_NEU_NBT_ID"},
		},
	})
	t.Cleanup(func() {
		notenoughupdates.CACHED_NEU_ITEMS.Delete("TEST_NEU_CACHE_KEY")
	})

	ids := preRenderSkyBlockItemIDs()
	for _, want := range []string{"TEST_NEU_CACHE_KEY", "TEST_NEU_ID", "TEST_NEU_NBT_ID"} {
		found := false
		for _, id := range ids {
			if id == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("preRenderSkyBlockItemIDs() missing %q", want)
		}
	}
}

func TestPreRenderSkyBlockItemIDsIncludesNEURepoFilenames(t *testing.T) {
	ids := preRenderSkyBlockItemIDs()
	want := "ABICASE_SUMSUNG_1"

	for _, id := range ids {
		if id == want {
			return
		}
	}

	t.Fatalf("preRenderSkyBlockItemIDs() missing NEU repo filename %q", want)
}

func testSkinValue(hash string) string {
	payload := `{"textures":{"SKIN":{"url":"http://textures.minecraft.net/texture/` + hash + `"}}}`
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

func testDomain() string {
	return utility.GetDomain()
}

func withNoRenderer(t *testing.T) {
	t.Helper()
	previous := SkyCryptRender
	SkyCryptRender = nil
	t.Cleanup(func() {
		SkyCryptRender = previous
	})
}

func withTextureCache(t *testing.T, cache map[string]AppliedItemTexture) {
	t.Helper()
	previous := ITEM_TEXTURE_CACHE
	ITEM_TEXTURE_CACHE = cache
	t.Cleanup(func() {
		ITEM_TEXTURE_CACHE = previous
	})
}

func withRenderedSkyBlockIndex(t *testing.T, index map[string]struct{}) {
	t.Helper()
	previous := renderedSkyBlockIndex
	renderedSkyBlockIndex = index
	t.Cleanup(func() {
		renderedSkyBlockIndex = previous
	})
}

func withRealRenderer(t *testing.T) {
	t.Helper()

	appRoot, err := appRootDir()
	if err != nil {
		t.Fatalf("appRootDir returned error: %v", err)
	}

	renderer, err := mr.NewRendererWithOptions(mr.Options{
		AssetsRoot:        filepath.Join(appRoot, "assets", "resourcepacks", "Vanilla", "assets", "minecraft"),
		ResourcePacksRoot: filepath.Join(appRoot, "assets", "resourcepacks"),
		PackIDs:           []string{"hplus", "fsr"},
		CacheDir:          filepath.Join(appRoot, "cache"),
	})
	if err != nil {
		t.Fatalf("NewRendererWithOptions returned error: %v", err)
	}

	previous := SkyCryptRender
	SkyCryptRender = renderer
	t.Cleanup(func() {
		SkyCryptRender = previous
	})
}

func TestLoadRenderedTextureIndexPopulatesDefaultCacheWithoutRenderer(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})
	withRenderedSkyBlockIndex(t, map[string]struct{}{})

	cacheDir := t.TempDir()
	renderedDir := filepath.Join(cacheDir, "rendered")
	if err := os.MkdirAll(renderedDir, 0755); err != nil {
		t.Fatal(err)
	}
	fileName := "skyblock=INDEXED_ITEM__pack=fsr__hash=abc.webp"
	if err := os.WriteFile(filepath.Join(renderedDir, fileName), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRenderedTextureIndex(cacheDir)
	if err != nil {
		t.Fatalf("LoadRenderedTextureIndex returned error: %v", err)
	}
	if loaded != 1 {
		t.Fatalf("loaded = %d, want 1", loaded)
	}

	texture := ApplyTextureInput(ItemTextureInput{SkyBlockID: "INDEXED_ITEM"}, NewTextureApplyContext())
	want := testDomain() + "/cache/rendered/" + fileName
	if texture.Texture != want || texture.TexturePack != "fsr" {
		t.Fatalf("ApplyTextureInput() = %#v, want texture %q pack fsr", texture, want)
	}
}

func TestApplyTextureInputReturnsHotCachedSkyBlockTexture(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	setCachedTextureForStableKey(defaultPackSignature, "skyblock:FAST_ITEM", AppliedItemTexture{
		Texture:     testDomain() + "/cache/rendered/skyblock=FAST_ITEM.webp",
		TexturePack: "hplus",
	})

	texture := ApplyTextureInput(ItemTextureInput{SkyBlockID: "FAST_ITEM"}, NewTextureApplyContext())
	want := testDomain() + "/cache/rendered/skyblock=FAST_ITEM.webp"
	if texture.Texture != want || texture.TexturePack != "hplus" {
		t.Fatalf("ApplyTextureInput() = %#v, want texture %q pack hplus", texture, want)
	}
}

func TestApplyTextureInputAllPacksDisabledSkipsCustomCache(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	setCachedTextureForStableKey(defaultPackSignature, "skyblock:DISABLED_ITEM", AppliedItemTexture{
		Texture:     testDomain() + "/cache/rendered/skyblock=DISABLED_ITEM.webp",
		TexturePack: "fsr",
	})

	texture := ApplyTextureInput(ItemTextureInput{
		ID:         "minecraft:player_head",
		SkyBlockID: "DISABLED_ITEM",
		Texture:    "fallback-disabled-hash",
	}, NewTextureApplyContext([]string{"hplus", "fsr"}))

	want := testDomain() + "/api/head/fallback-disabled-hash"
	if texture.Texture != want {
		t.Fatalf("ApplyTextureInput() = %q, want %q", texture.Texture, want)
	}
}

func TestRendererNotReadyFallbackDoesNotPoisonLaterCacheHit(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	first := ApplyTextureInput(ItemTextureInput{
		ID:         "minecraft:player_head",
		SkyBlockID: "LATE_RENDER",
		Texture:    "fallback-late-hash",
	}, NewTextureApplyContext())
	if first.Texture != testDomain()+"/api/head/fallback-late-hash" {
		t.Fatalf("first ApplyTextureInput() = %q", first.Texture)
	}

	setCachedTextureForStableKey(defaultPackSignature, "skyblock:LATE_RENDER", AppliedItemTexture{
		Texture:     testDomain() + "/cache/rendered/skyblock=LATE_RENDER.webp",
		TexturePack: "hplus",
	})

	second := ApplyTextureInput(ItemTextureInput{
		ID:         "minecraft:player_head",
		SkyBlockID: "LATE_RENDER",
		Texture:    "fallback-late-hash",
	}, NewTextureApplyContext())
	want := testDomain() + "/cache/rendered/skyblock=LATE_RENDER.webp"
	if second.Texture != want {
		t.Fatalf("second ApplyTextureInput() = %q, want %q", second.Texture, want)
	}
}

func TestApplyTextureInputDoesNotReuseGenericPlayerHeadCacheForDifferentSkulls(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	firstOwner := &skycrypttypes.SkullOwner{
		ID: "first-skull-id",
		Properties: skycrypttypes.Properties{
			Textures: []skycrypttypes.Texture{{Value: testSkinValue("first-skull-hash")}},
		},
	}
	secondOwner := &skycrypttypes.SkullOwner{
		ID: "second-skull-id",
		Properties: skycrypttypes.Properties{
			Textures: []skycrypttypes.Texture{{Value: testSkinValue("second-skull-hash")}},
		},
	}

	textureCtx := NewTextureApplyContext()
	setCachedTextureForInput(ItemTextureInput{
		ID:         "minecraft:player_head",
		ItemModel:  "minecraft:player_head",
		SkullOwner: firstOwner,
	}, textureCtx, AppliedItemTexture{
		Texture:     testDomain() + "/cache/rendered/first-skull.webp",
		TexturePack: "hplus",
	})

	second := ApplyTextureInput(ItemTextureInput{
		ID:         "minecraft:player_head",
		ItemModel:  "minecraft:player_head",
		SkullOwner: secondOwner,
	}, textureCtx)

	want := testDomain() + "/api/head/second-skull-hash"
	if second.Texture != want {
		t.Fatalf("second skull texture = %q, want %q", second.Texture, want)
	}
}

func TestApplyTextureInputSkullOwnerIgnoresSkyBlockAliasCache(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	setCachedItemTexture("SHARED_SKULL_ITEM", AppliedItemTexture{
		Texture:     testDomain() + "/cache/rendered/wrong-shared-skull.webp",
		TexturePack: "hplus",
	})

	texture := ApplyTextureInput(ItemTextureInput{
		ID:         "minecraft:player_head",
		ItemModel:  "minecraft:player_head",
		SkyBlockID: "SHARED_SKULL_ITEM",
		SkullOwner: &skycrypttypes.SkullOwner{
			ID: "specific-skull-id",
			Properties: skycrypttypes.Properties{
				Textures: []skycrypttypes.Texture{{Value: testSkinValue("specific-skull-hash")}},
			},
		},
	}, NewTextureApplyContext())

	want := testDomain() + "/api/head/specific-skull-hash"
	if texture.Texture != want {
		t.Fatalf("skull texture = %q, want %q", texture.Texture, want)
	}
}

func TestApplyTextureInputSkullOwnerPrefersHeadFallbackOverVanillaRender(t *testing.T) {
	withRealRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	texture := ApplyTextureInput(ItemTextureInput{
		ID:        "minecraft:player_head",
		ItemModel: "minecraft:player_head",
		SkullOwner: &skycrypttypes.SkullOwner{
			ID: "vanilla-render-skull-id",
			Properties: skycrypttypes.Properties{
				Textures: []skycrypttypes.Texture{{Value: testSkinValue("vanilla-render-skull-hash")}},
			},
		},
	}, NewTextureApplyContext())

	want := testDomain() + "/api/head/vanilla-render-skull-hash"
	if texture.Texture != want {
		t.Fatalf("skull texture = %q, want %q", texture.Texture, want)
	}
}

func TestApplyTextureMapSkullIgnoresGenericPlayerHeadCache(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	setCachedTextureForStableKey(defaultPackSignature, "itemmodel:minecraft:player_head", AppliedItemTexture{
		Texture:     testDomain() + "/cache/rendered/generic-player-head.webp",
		TexturePack: "hplus",
	})

	texture := ApplyTexture(map[string]any{
		"id":        "minecraft:player_head",
		"ItemModel": "minecraft:player_head",
		"tag": map[string]any{
			"SkullOwner": map[string]any{
				"Id": "map-skull-id",
				"Properties": map[string]any{
					"textures": []any{map[string]any{"Value": testSkinValue("map-skull-hash")}},
				},
			},
		},
	})

	want := testDomain() + "/api/head/map-skull-hash"
	if texture.Texture != want {
		t.Fatalf("ApplyTexture() = %q, want %q", texture.Texture, want)
	}
}

func TestApplyTextureFallsBackToHeadFromStructuredSkullOwner(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	hash := "structured-skull-hash"
	texture := ApplyTexture(map[string]any{
		"id":          "minecraft:player_head",
		"skyblock_id": "CUSTOM_HEAD",
		"tag": skycrypttypes.TextureItemExtraAttributes{
			ExtraAttributes: map[string]any{"id": "CUSTOM_HEAD"},
			SkullOwner: &skycrypttypes.SkullOwner{
				Properties: skycrypttypes.Properties{
					Textures: []skycrypttypes.Texture{{Value: testSkinValue(hash)}},
				},
			},
		},
	})

	want := testDomain() + "/api/head/" + hash
	if texture.Texture != want {
		t.Fatalf("ApplyTexture() = %q, want %q", texture.Texture, want)
	}
}

func TestApplyTextureFallsBackToHeadFromLowercaseTexturesMap(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	hash := "lowercase-skull-hash"
	texture := ApplyTexture(map[string]any{
		"id": "minecraft:player_head",
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{"id": "CUSTOM_HEAD"},
			"SkullOwner": map[string]any{
				"Properties": map[string]any{
					"textures": []any{
						map[string]any{"value": testSkinValue(hash)},
					},
				},
			},
		},
	})

	want := testDomain() + "/api/head/" + hash
	if texture.Texture != want {
		t.Fatalf("ApplyTexture() = %q, want %q", texture.Texture, want)
	}
}

func TestApplyTextureDoesNotPanicWhenRendererIsNil(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	texture := ApplyTexture(map[string]any{"id": "minecraft:apple"})
	want := testDomain() + "/assets/resourcepacks/Vanilla/assets/minecraft/textures/item/apple.png"
	if texture.Texture != want {
		t.Fatalf("ApplyTexture() = %q, want %q", texture.Texture, want)
	}
}

func TestApplyTextureHandlesPointerHeavyLeatherInput(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	itemID := 298
	damage := 0
	color := 0x112233
	texture := ApplyTexture(map[string]any{
		"id":      "minecraft:leather_helmet",
		"item_id": &itemID,
		"damage":  &damage,
		"tag": map[string]any{
			"display": map[string]any{"color": &color},
		},
	})

	want := testDomain() + "/api/leather/helmet/112233"
	if texture.Texture != want {
		t.Fatalf("ApplyTexture() = %q, want %q", texture.Texture, want)
	}
}

func TestApplyTextureSkipsDisabledCachedTexture(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{
		"CACHED_HEAD": {Texture: testDomain() + "/cache/rendered/skyblock=CACHED_HEAD.webp", TexturePack: "fsr"},
	})

	texture := ApplyTexture(map[string]any{
		"id":          "minecraft:player_head",
		"skyblock_id": "CACHED_HEAD",
		"texture":     "fallback-head-hash",
	}, []string{"fsr"})

	want := testDomain() + "/api/head/fallback-head-hash"
	if texture.Texture != want {
		t.Fatalf("ApplyTexture() = %q, want %q", texture.Texture, want)
	}
}

func TestApplyTextureReturnsVanillaAsset(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	texture := ApplyTexture(map[string]any{"id": "minecraft:apple"})
	want := testDomain() + "/assets/resourcepacks/Vanilla/assets/minecraft/textures/item/apple.png"
	if texture.Texture != want {
		t.Fatalf("ApplyTexture() = %q, want %q", texture.Texture, want)
	}
}

func TestApplyTextureSkipsStaleVanillaChestParticleCache(t *testing.T) {
	withRealRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	stale := AppliedItemTexture{
		Texture:     testDomain() + "/cache/rendered/skyblock=CHEST__mc=minecraft_chest__itemmodel=minecraft_chest__pack=vanilla__model=minecraft_item_chest__tex1=minecraft_block_oak_planks__hash=old.webp",
		TexturePack: "vanilla",
	}
	setCachedTextureForStableKey(defaultPackSignature, "skyblock:CHEST", stale)
	setCachedTextureForStableKey(defaultPackSignature, "itemmodel:minecraft:chest", stale)

	texture := ApplyTextureInput(ItemTextureInput{
		ID:         "minecraft:chest",
		NumericID:  54,
		ItemModel:  "minecraft:chest",
		SkyBlockID: "CHEST",
		Tag:        map[string]any{"ExtraAttributes": map[string]any{"id": "CHEST"}},
	}, NewTextureApplyContext())

	if texture.Texture == stale.Texture {
		t.Fatal("ApplyTextureInput reused stale chest particle render")
	}
	if !strings.Contains(texture.Texture, "model=special_minecraft_chest_minecraft_normal_minecraft_item_chest") ||
		!strings.Contains(texture.Texture, "tex1=minecraft_entity_chest_normal") {
		t.Fatalf("ApplyTextureInput() = %q, want special chest entity texture render", texture.Texture)
	}
}

func TestApplyTextureSkipsStaleVanillaEnderChestParticleCache(t *testing.T) {
	withRealRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	stale := AppliedItemTexture{
		Texture:     testDomain() + "/cache/rendered/skyblock=ENDER_CHEST__mc=minecraft_ender_chest__itemmodel=minecraft_ender_chest__pack=vanilla__model=minecraft_item_ender_chest__tex1=minecraft_block_obsidian__hash=old.webp",
		TexturePack: "vanilla",
	}
	setCachedTextureForStableKey(defaultPackSignature, "skyblock:ENDER_CHEST", stale)
	setCachedTextureForStableKey(defaultPackSignature, "itemmodel:minecraft:ender_chest", stale)

	texture := ApplyTextureInput(ItemTextureInput{
		ID:         "minecraft:ender_chest",
		NumericID:  130,
		ItemModel:  "minecraft:ender_chest",
		SkyBlockID: "ENDER_CHEST",
		Tag:        map[string]any{"ExtraAttributes": map[string]any{"id": "ENDER_CHEST"}},
	}, NewTextureApplyContext())

	if texture.Texture == stale.Texture {
		t.Fatal("ApplyTextureInput reused stale ender chest particle render")
	}
	if !strings.Contains(texture.Texture, "model=special_minecraft_chest_minecraft_ender_minecraft_item_ender_chest") ||
		!strings.Contains(texture.Texture, "tex1=minecraft_entity_chest_ender") {
		t.Fatalf("ApplyTextureInput() = %q, want special ender chest entity texture render", texture.Texture)
	}
}

func TestApplyTextureUsesVanillaModelFallbacks(t *testing.T) {
	withRealRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	for _, item := range []map[string]any{
		{
			"id":          "minecraft:polished_diorite",
			"item_id":     1,
			"damage":      4,
			"skyblock_id": "STONE:4",
			"tag":         map[string]any{"ExtraAttributes": map[string]any{"id": "STONE:4"}},
		},
		{
			"id":          "minecraft:clock",
			"item_id":     347,
			"damage":      0,
			"skyblock_id": "ENCHANTED_TIME_CLOCK",
			"tag":         map[string]any{"ExtraAttributes": map[string]any{"id": "ENCHANTED_TIME_CLOCK"}},
		},
	} {
		texture := ApplyTexture(item)
		if texture.Texture == "" || strings.HasSuffix(texture.Texture, "/barrier.png") {
			t.Fatalf("ApplyTexture(%v) = %q, want rendered vanilla texture", item["id"], texture.Texture)
		}
		if !strings.Contains(texture.Texture, "/cache/rendered/") {
			t.Fatalf("ApplyTexture(%v) = %q, want rendered cache URL", item["id"], texture.Texture)
		}
	}
}

func TestVanillaItemResourceExistsForModelOnlyBlockItem(t *testing.T) {
	if !vanillaItemResourceExists("polished_diorite") {
		t.Fatal("vanillaItemResourceExists(polished_diorite) = false, want true")
	}
}

func TestApplyTextureRendersVanillaSpecialHeadModel(t *testing.T) {
	withRealRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	texture := ApplyTexture(map[string]any{
		"id":          "minecraft:zombie_head",
		"item_id":     397,
		"damage":      2,
		"skyblock_id": "ZOMBIE_HAT",
		"tag":         map[string]any{"ExtraAttributes": map[string]any{"id": "ZOMBIE_HAT"}},
	})

	if texture.Texture == "" || strings.HasSuffix(texture.Texture, "/barrier.png") {
		t.Fatalf("ApplyTexture() = %q, want rendered zombie head", texture.Texture)
	}
	if strings.Contains(texture.Texture, "/textures/entity/zombie/zombie.png") {
		t.Fatalf("ApplyTexture() = %q, want skull model render, not flat entity texture", texture.Texture)
	}
	if !strings.Contains(texture.Texture, "/cache/rendered/") {
		t.Fatalf("ApplyTexture() = %q, want rendered cache URL", texture.Texture)
	}
}

func TestApplyTexturePlayerHeadDoesNotFallBackToSoulSand(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	hash := "abicase-head-hash"
	texture := ApplyTexture(map[string]any{
		"id":          "minecraft:player_head",
		"ItemModel":   "minecraft:player_head",
		"item_id":     88,
		"damage":      3,
		"skyblock_id": "ABICASE_SUMSUNG_1",
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{"id": "ABICASE_SUMSUNG_1", "model": "SUMSUNG_1"},
			"ItemModel":       "minecraft:player_head",
			"SkullOwner": map[string]any{
				"Properties": map[string]any{
					"textures": []any{map[string]any{"Value": testSkinValue(hash)}},
				},
			},
		},
	})

	want := testDomain() + "/api/head/" + hash
	if texture.Texture != want {
		t.Fatalf("ApplyTexture() = %q, want %q", texture.Texture, want)
	}
	if strings.Contains(texture.Texture, "soul_sand") {
		t.Fatalf("ApplyTexture() = %q, must not use legacy soul sand fallback", texture.Texture)
	}
}

func TestApplyTexturePlayerHeadDoesNotFallBackToOakPlanks(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	hash := "backpack-head-hash"
	texture := ApplyTexture(map[string]any{
		"id":          "minecraft:player_head",
		"ItemModel":   "minecraft:player_head",
		"item_id":     5,
		"damage":      0,
		"skyblock_id": "SMALL_BACKPACK",
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{"id": "SMALL_BACKPACK"},
			"ItemModel":       "minecraft:player_head",
			"SkullOwner": map[string]any{
				"Properties": map[string]any{
					"textures": []any{map[string]any{"Value": testSkinValue(hash)}},
				},
			},
		},
	})

	want := testDomain() + "/api/head/" + hash
	if texture.Texture != want {
		t.Fatalf("ApplyTexture() = %q, want %q", texture.Texture, want)
	}
	if strings.Contains(texture.Texture, "planks") {
		t.Fatalf("ApplyTexture() = %q, must not use legacy planks fallback", texture.Texture)
	}
}

func TestApplyTextureReturnsBarrierForInvalidItem(t *testing.T) {
	withNoRenderer(t)
	withTextureCache(t, map[string]AppliedItemTexture{})

	texture := ApplyTexture(map[string]any{"id": ""})
	if !strings.HasSuffix(texture.Texture, "/barrier.png") {
		t.Fatalf("ApplyTexture() = %q, want barrier", texture.Texture)
	}
}

func BenchmarkApplyTextureInputHotCache(b *testing.B) {
	previousRenderer := SkyCryptRender
	previousCache := ITEM_TEXTURE_CACHE
	SkyCryptRender = nil
	ITEM_TEXTURE_CACHE = map[string]AppliedItemTexture{}
	defer func() {
		SkyCryptRender = previousRenderer
		ITEM_TEXTURE_CACHE = previousCache
	}()

	setCachedTextureForStableKey(defaultPackSignature, "skyblock:BENCH_ITEM", AppliedItemTexture{
		Texture:     testDomain() + "/cache/rendered/skyblock=BENCH_ITEM.webp",
		TexturePack: "hplus",
	})
	input := ItemTextureInput{SkyBlockID: "BENCH_ITEM"}
	textureCtx := NewTextureApplyContext()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		texture := ApplyTextureInput(input, textureCtx)
		if texture.Texture == "" {
			b.Fatal("empty texture")
		}
	}
}

func BenchmarkApplyTextureMapCompatibility(b *testing.B) {
	previousRenderer := SkyCryptRender
	previousCache := ITEM_TEXTURE_CACHE
	SkyCryptRender = nil
	ITEM_TEXTURE_CACHE = map[string]AppliedItemTexture{}
	defer func() {
		SkyCryptRender = previousRenderer
		ITEM_TEXTURE_CACHE = previousCache
	}()

	setCachedTextureForStableKey(defaultPackSignature, "skyblock:BENCH_MAP_ITEM", AppliedItemTexture{
		Texture:     testDomain() + "/cache/rendered/skyblock=BENCH_MAP_ITEM.webp",
		TexturePack: "hplus",
	})
	item := map[string]any{
		"id":          "minecraft:player_head",
		"skyblock_id": "BENCH_MAP_ITEM",
		"tag":         map[string]any{"ExtraAttributes": map[string]any{"id": "BENCH_MAP_ITEM"}},
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		texture := ApplyTexture(item)
		if texture.Texture == "" {
			b.Fatal("empty texture")
		}
	}
}

func BenchmarkVanillaResourceExistsCached(b *testing.B) {
	if !vanillaItemResourceExists("apple") {
		b.Fatal("apple should exist")
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !vanillaItemResourceExists("apple") {
			b.Fatal("apple should exist")
		}
	}
}
