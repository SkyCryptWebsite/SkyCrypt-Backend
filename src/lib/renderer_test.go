package lib

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"
	"testing"
)

func testDomain() string {
	return utility.GetDomain()
}

func TestLocalStaticTexturePath(t *testing.T) {
	appRoot, err := appRootDir()
	if err != nil {
		t.Fatalf("appRootDir returned error: %v", err)
	}

	tests := []struct {
		name    string
		texture string
		want    string
		wantOK  bool
	}{
		{
			name:    "relative cache path",
			texture: "cache/rendered/skyblock=HYPERION.webp",
			want:    filepath.Join(appRoot, "cache", "rendered", "skyblock=HYPERION.webp"),
			wantOK:  true,
		},
		{
			name:    "absolute local domain assets URL",
			texture: testDomain() + "/assets/resourcepacks/Vanilla/pack.png",
			want:    filepath.Join(appRoot, "assets", "resourcepacks", "Vanilla", "pack.png"),
			wantOK:  true,
		},
		{
			name:    "filesystem path containing cache root",
			texture: "/tmp/project/cache/rendered/skyblock=HYPERION.webp",
			want:    filepath.Clean("/tmp/project/cache/rendered/skyblock=HYPERION.webp"),
			wantOK:  true,
		},
		{
			name:    "external URL stays unresolved",
			texture: "https://cdn.example/cache/rendered/skyblock=HYPERION.webp",
			wantOK:  false,
		},
		{
			name:    "path traversal escapes static roots",
			texture: "/cache/../../.env",
			wantOK:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok, err := localStaticTexturePath(test.texture)
			if err != nil {
				t.Fatalf("localStaticTexturePath returned error: %v", err)
			}
			if ok != test.wantOK {
				t.Fatalf("ok = %v, want %v", ok, test.wantOK)
			}
			if got != test.want {
				t.Fatalf("path = %q, want %q", got, test.want)
			}
		})
	}
}

func withRenderItemGlobals(t *testing.T) {
	t.Helper()

	previousRenderer := customResourceRenderer
	previousItems := constants.ItemsSnapshot()
	previousCache := itemTextureCache

	customResourceRenderer = nil
	constants.SetItems(map[string]models.ProcessedHypixelItem{})
	itemTextureCache = map[string]AppliedItemTexture{}

	t.Cleanup(func() {
		customResourceRenderer = previousRenderer
		constants.SetItems(previousItems)
		itemTextureCache = previousCache
	})
}

func TestRenderItemHandlesKnownSkyBlockItem(t *testing.T) {
	withRenderItemGlobals(t)

	constants.SetItems(map[string]models.ProcessedHypixelItem{
		"TEST_SKYBLOCK_ITEM": {
			SkyblockID: "TEST_SKYBLOCK_ITEM",
			Material:   "APPLE",
			ItemId:     constants.BUKKIT_TO_ID["APPLE"],
		},
	})
	itemTextureCache["TEST_SKYBLOCK_ITEM"] = AppliedItemTexture{
		Texture: testDomain() + "/assets/resourcepacks/Vanilla/assets/minecraft/textures/item/apple.png",
	}

	textureBytes, err := RenderItem("TEST_SKYBLOCK_ITEM", nil, false)
	if err != nil {
		t.Fatalf("RenderItem returned error: %v", err)
	}
	if len(textureBytes) == 0 {
		t.Fatal("RenderItem returned empty texture bytes")
	}
}

func TestPreferredItemModelUsesHypixelBeforeNEU(t *testing.T) {
	got := preferredItemModel(" minecraft:bamboo ", "hypixel_skyblock:item/combat_1/arack")
	if got != "minecraft:bamboo" {
		t.Fatalf("preferredItemModel = %q, want minecraft:bamboo", got)
	}

	got = preferredItemModel("", " hypixel_skyblock:item/combat_1/arack ")
	if got != "hypixel_skyblock:item/combat_1/arack" {
		t.Fatalf("preferredItemModel NEU fallback = %q", got)
	}
}

func TestItemTextureCandidatesFallBackToNEU(t *testing.T) {
	withRenderItemGlobals(t)
	previousWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	appRoot, err := appRootDir()
	if err != nil {
		t.Fatalf("appRootDir returned error: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("Chdir returned error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousWorkingDirectory) })

	customItem, modelItem, err := itemTextureCandidates("KUUDRA_TIER_KEY", 0)
	if err != nil {
		t.Fatalf("itemTextureCandidates returned error: %v", err)
	}
	if modelItem == nil {
		t.Fatal("expected KUUDRA_TIER_KEY model candidate from NEU")
	}

	customInput := itemTextureInputFromMap(customItem)
	if customInput.SkyBlockID != "KUUDRA_TIER_KEY" || customInput.ID != "minecraft:player_head" {
		t.Fatalf("custom NEU input = %#v", customInput)
	}
	if skullIdentityFromInput(customInput) == "" {
		t.Fatal("custom NEU input lost SkullOwner data")
	}

	modelInput := itemTextureInputFromMap(modelItem)
	if modelInput.ItemModel != "minecraft:player_head" || modelInput.SkyBlockID != "KUUDRA_TIER_KEY" {
		t.Fatalf("model NEU input = %#v", modelInput)
	}
	fallback := skullFallbackTexture(modelInput, NewTextureApplyContext([]string{"FSR"}))
	if !strings.Contains(fallback.Texture, "/api/head/bfd3e71838c0e76f890213120b4ce7449577736604338a8d28b4c86db2547e71") {
		t.Fatalf("KUUDRA_TIER_KEY fallback = %#v", fallback)
	}
}

func TestNEUMobHeadItemsFallBackToRenderedVanillaHeads(t *testing.T) {
	withRenderItemGlobals(t)
	previousWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	appRoot, err := appRootDir()
	if err != nil {
		t.Fatalf("appRootDir returned error: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("Chdir returned error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousWorkingDirectory) })

	previousCacheDir := CACHE_DIR
	CACHE_DIR = filepath.Join(t.TempDir(), "cache")
	t.Cleanup(func() { CACHE_DIR = previousCacheDir })

	tests := map[string]string{
		"ZOMBIE_HAT":   "zombie_head",
		"SKELETON_HAT": "skeleton_skull",
		"CREEPER_HAT":  "creeper_head",
	}
	for skyBlockID, minecraftID := range tests {
		t.Run(skyBlockID, func(t *testing.T) {
			_, modelItem, err := itemTextureCandidates(skyBlockID, 0)
			if err != nil {
				t.Fatalf("itemTextureCandidates returned error: %v", err)
			}
			if modelItem == nil {
				t.Fatal("expected model candidate")
			}

			context := NewTextureApplyContext([]string{"FSR"})
			context.DisableRuntimeRender = true
			texture := ApplyTextureInput(itemTextureInputFromMap(modelItem), context)
			wantSuffix := "/cache/heads/minecraft_" + minecraftID + ".png"
			if !strings.HasSuffix(texture.Texture, wantSuffix) || texture.TexturePack != "" {
				t.Fatalf("texture = %#v, want suffix %q", texture, wantSuffix)
			}

			file, err := os.Open(filepath.Join(CACHE_DIR, "heads", "minecraft_"+minecraftID+".png"))
			if err != nil {
				t.Fatalf("open rendered head: %v", err)
			}
			img, err := png.Decode(file)
			_ = file.Close()
			if err != nil {
				t.Fatalf("decode rendered head: %v", err)
			}
			hasVisiblePixel := false
			for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y && !hasVisiblePixel; y++ {
				for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
					_, _, _, alpha := img.At(x, y).RGBA()
					if alpha != 0 {
						hasVisiblePixel = true
						break
					}
				}
			}
			if !hasVisiblePixel {
				t.Fatal("rendered head has no visible pixels")
			}
		})
	}
}

func TestRenderItemUsesVanillaHypixelItemModel(t *testing.T) {
	withRenderItemGlobals(t)

	constants.SetItems(map[string]models.ProcessedHypixelItem{
		"TEST_BAMBOO_MODEL": {
			SkyblockID: "TEST_BAMBOO_MODEL",
			Material:   "SKULL_ITEM",
			ItemModel:  "minecraft:bamboo",
			ItemId:     constants.BUKKIT_TO_ID["SKULL_ITEM"],
			Damage:     3,
		},
	})
	itemTextureCache["bamboo"] = AppliedItemTexture{
		Texture: testDomain() + "/assets/resourcepacks/Vanilla/assets/minecraft/textures/item/bamboo.png",
	}

	textureBytes, err := RenderItem("TEST_BAMBOO_MODEL", nil, false)
	if err != nil {
		t.Fatalf("RenderItem returned error: %v", err)
	}
	if len(textureBytes) == 0 {
		t.Fatal("RenderItem returned empty texture bytes")
	}
}

func TestRenderItemUsesCustomHypixelItemModel(t *testing.T) {
	withRenderItemGlobals(t)

	constants.SetItems(map[string]models.ProcessedHypixelItem{
		"TEST_ARACK_MODEL": {
			SkyblockID: "TEST_ARACK_MODEL",
			Material:   "IRON_SWORD",
			ItemModel:  "hypixel_skyblock:item/combat_1/arack",
			ItemId:     constants.BUKKIT_TO_ID["IRON_SWORD"],
		},
	})
	itemTextureCache["hypixel_skyblock:item/combat_1/arack"] = AppliedItemTexture{
		Texture:     testDomain() + "/assets/resourcepacks/Hypixel_Pack/assets/hypixel_skyblock/textures/item/combat_1/arack.png",
		TexturePack: "HYPIXEL_PACK",
	}

	textureBytes, err := RenderItem("TEST_ARACK_MODEL", nil, false)
	if err != nil {
		t.Fatalf("RenderItem returned error: %v", err)
	}
	if len(textureBytes) == 0 {
		t.Fatal("RenderItem returned empty texture bytes")
	}
}

func TestRenderItemFallsBackWhenHypixelItemModelIsMissing(t *testing.T) {
	withRenderItemGlobals(t)

	constants.SetItems(map[string]models.ProcessedHypixelItem{
		"TEST_MISSING_MODEL": {
			SkyblockID: "TEST_MISSING_MODEL",
			Material:   "APPLE",
			ItemModel:  "missing_namespace:item/does_not_exist",
			ItemId:     constants.BUKKIT_TO_ID["APPLE"],
		},
	})
	itemTextureCache["TEST_MISSING_MODEL"] = AppliedItemTexture{
		Texture: testDomain() + "/assets/resourcepacks/Vanilla/assets/minecraft/textures/item/apple.png",
	}

	textureBytes, err := RenderItem("TEST_MISSING_MODEL", nil, false)
	if err != nil {
		t.Fatalf("RenderItem returned error: %v", err)
	}
	if len(textureBytes) == 0 {
		t.Fatal("RenderItem returned empty texture bytes")
	}
}

func TestRenderItemMissingModelUsesSkullFallback(t *testing.T) {
	withRenderItemGlobals(t)

	constants.SetItems(map[string]models.ProcessedHypixelItem{
		"TEST_MODEL_SKULL": {
			SkyblockID: "TEST_MODEL_SKULL",
			Material:   "SKULL_ITEM",
			ItemModel:  "missing_namespace:item/does_not_exist",
			ItemId:     constants.BUKKIT_TO_ID["SKULL_ITEM"],
			Damage:     3,
			TextureId:  "test-head-hash",
		},
	})

	_, err := RenderItem("TEST_MODEL_SKULL", nil, false)
	redirectErr, ok := err.(RedirectError)
	if !ok {
		t.Fatalf("RenderItem error = %v, want RedirectError", err)
	}
	if !strings.Contains(redirectErr.URL, "/api/head/test-head-hash") {
		t.Fatalf("redirect URL = %q", redirectErr.URL)
	}
}

func TestRenderItemHandlesBukkitItemWithDamage(t *testing.T) {
	withRenderItemGlobals(t)

	textureBytes, err := RenderItem("INK_SACK:10", nil, false)
	if err != nil {
		t.Fatalf("RenderItem returned error: %v", err)
	}
	if len(textureBytes) == 0 {
		t.Fatal("RenderItem returned empty texture bytes")
	}
}

func TestRenderItemHandlesLowercaseMinecraftItemID(t *testing.T) {
	withRenderItemGlobals(t)

	itemTextureCache["chest"] = AppliedItemTexture{
		Texture: testDomain() + "/assets/resourcepacks/Vanilla/assets/minecraft/textures/item/apple.png",
	}

	textureBytes, err := RenderItem("chest", nil, false)
	if err != nil {
		t.Fatalf("RenderItem returned error: %v", err)
	}
	if len(textureBytes) == 0 {
		t.Fatal("RenderItem returned empty texture bytes")
	}
}

func TestRenderItemHandlesModelOnlyMinecraftItemID(t *testing.T) {
	withRenderItemGlobals(t)

	textureBytes, err := RenderItem("minecraft:polished_diorite", nil, false)
	if err != nil {
		t.Fatalf("RenderItem returned error: %v", err)
	}
	if len(textureBytes) == 0 {
		t.Fatal("RenderItem returned empty texture bytes")
	}
}

func TestApplyTextureRendersVanillaBlockModelWithNoCustomPacks(t *testing.T) {
	withRenderItemGlobals(t)

	appRoot, err := appRootDir()
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := newRendererForPackIDs(
		filepath.Join(t.TempDir(), "cache"),
		filepath.Join(appRoot, "assets", "resourcepacks"),
		filepath.Join(appRoot, "assets", "resourcepacks", "Vanilla", "assets", "minecraft"),
		[]string{"HYPIXEL_PACK"},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	customResourceRenderer = renderer

	textureCtx := NewTextureApplyContext([]string{})
	texture := ApplyTextureInput(ItemTextureInput{ID: "minecraft:stone"}, textureCtx)
	if textureCtx.Stats.RenderAttempts == 0 {
		t.Fatal("vanilla-only context skipped runtime rendering")
	}
	if texture.TexturePack != "vanilla" {
		t.Fatalf("texture pack = %q, want vanilla", texture.TexturePack)
	}
	if !strings.Contains(texture.Texture, "/cache/rendered/") {
		t.Fatalf("texture = %q, want rendered model cache path", texture.Texture)
	}
	if strings.Contains(texture.Texture, "/textures/block/") {
		t.Fatalf("texture = %q, got raw block texture", texture.Texture)
	}
	if len(itemTextureCache) != 0 {
		t.Fatalf("vanilla-only render populated legacy item cache: %#v", itemTextureCache)
	}
}

func TestOverclockerPackCacheIsolation(t *testing.T) {
	withRenderItemGlobals(t)

	appRoot, err := appRootDir()
	if err != nil {
		t.Fatal(err)
	}
	rawItemBytes, err := os.ReadFile(filepath.Join(appRoot, "NotEnoughUpdates-REPO", "items", "OVERCLOCKER_3000.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rawItem models.RawNEUItem
	if err := json.Unmarshal(rawItemBytes, &rawItem); err != nil {
		t.Fatal(err)
	}
	parsedNBT, ok := notenoughupdates.ParseNBTToItem(rawItem.NBT)
	if !ok {
		t.Fatal("failed to parse OVERCLOCKER_3000 NBT")
	}

	cacheDir := filepath.Join(t.TempDir(), "cache")
	renderer, err := newRendererForPackIDs(
		cacheDir,
		filepath.Join(appRoot, "assets", "resourcepacks"),
		filepath.Join(appRoot, "assets", "resourcepacks", "Vanilla", "assets", "minecraft"),
		defaultResourcePackIDs(),
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	customResourceRenderer = renderer

	input := ItemTextureInput{
		ID:         rawItem.MinecraftId,
		ItemModel:  "hypixel_skyblock:item/island_relevant/garden/greenhouse/overclocker_3000",
		SkyBlockID: rawItem.NEUId,
		Damage:     rawItem.Damage,
		Tag:        parsedNBT,
	}

	allPacks := ApplyTextureInput(input, NewTextureApplyContext())
	if allPacks.TexturePack != "HYPIXEL_PLUS" {
		t.Fatalf("all-packs texture pack = %q, want HYPIXEL_PLUS", allPacks.TexturePack)
	}
	allPacksBytes := readAppliedTextureBytes(t, cacheDir, allPacks)

	vanilla := ApplyTextureInput(input, NewTextureApplyContext([]string{}))
	if vanilla.TexturePack != "" && vanilla.TexturePack != "vanilla" {
		t.Fatalf("vanilla texture pack = %q, want no custom pack", vanilla.TexturePack)
	}
	if !strings.Contains(strings.ToLower(vanilla.Texture), "paper") {
		t.Fatalf("vanilla texture = %q, want paper fallback", vanilla.Texture)
	}

	hypixelPack := ApplyTextureInput(input, NewTextureApplyContext([]string{"HYPIXEL_PACK"}))
	if hypixelPack.TexturePack != "HYPIXEL_PACK" {
		t.Fatalf("Hypixel-only texture pack = %q, want HYPIXEL_PACK", hypixelPack.TexturePack)
	}
	if hypixelPack.Texture == allPacks.Texture {
		t.Fatalf("Hypixel-only texture reused H+ path %q", hypixelPack.Texture)
	}
	hypixelPackBytes := readAppliedTextureBytes(t, cacheDir, hypixelPack)
	if bytes.Equal(hypixelPackBytes, allPacksBytes) {
		t.Fatal("Hypixel-only texture reused H+ image bytes")
	}
}

func readAppliedTextureBytes(t *testing.T, cacheDir string, texture AppliedItemTexture) []byte {
	t.Helper()
	texturePath, ok, err := localStaticTexturePath(texture.Texture)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("texture %q is not a local static path", texture.Texture)
	}
	if strings.Contains(filepath.ToSlash(texturePath), "/cache/rendered/") {
		texturePath = filepath.Join(cacheDir, "rendered", filepath.Base(texturePath))
	}
	textureBytes, err := os.ReadFile(texturePath)
	if err != nil {
		t.Fatal(err)
	}
	return textureBytes
}

func TestRenderItemHandlesChestAsRenderedModel(t *testing.T) {
	withRenderItemGlobals(t)

	textureBytes, err := RenderItem("chest", nil, false)
	if err != nil {
		t.Fatalf("RenderItem returned error: %v", err)
	}
	if len(textureBytes) == 0 {
		t.Fatal("RenderItem returned empty texture bytes")
	}
}

func TestRenderItemHandlesEnderChestAsRenderedModel(t *testing.T) {
	withRenderItemGlobals(t)

	textureBytes, err := RenderItem("ender_chest", nil, false)
	if err != nil {
		t.Fatalf("RenderItem returned error: %v", err)
	}
	if len(textureBytes) == 0 {
		t.Fatal("RenderItem returned empty texture bytes")
	}
}

func TestRenderItemRejectsUnknownOrEmptyItemID(t *testing.T) {
	withRenderItemGlobals(t)

	if _, err := RenderItem("", nil, false); err == nil {
		t.Fatal("RenderItem empty ID returned nil error")
	}
	if _, err := RenderItem("NOT_A_REAL_ITEM", nil, false); err == nil {
		t.Fatal("RenderItem unknown ID returned nil error")
	}
}
