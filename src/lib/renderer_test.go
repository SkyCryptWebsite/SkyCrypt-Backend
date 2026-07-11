package lib

import (
	"path/filepath"
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
