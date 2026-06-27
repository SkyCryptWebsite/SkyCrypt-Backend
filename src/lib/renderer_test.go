package lib

import (
	"path/filepath"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"testing"
)

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

	previousRenderer := SkyCryptRender
	previousItems := constants.ItemsSnapshot()
	previousCache := ITEM_TEXTURE_CACHE

	SkyCryptRender = nil
	constants.SetItems(map[string]models.ProcessedHypixelItem{})
	ITEM_TEXTURE_CACHE = map[string]AppliedItemTexture{}

	t.Cleanup(func() {
		SkyCryptRender = previousRenderer
		constants.SetItems(previousItems)
		ITEM_TEXTURE_CACHE = previousCache
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
	ITEM_TEXTURE_CACHE["TEST_SKYBLOCK_ITEM"] = AppliedItemTexture{
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

	ITEM_TEXTURE_CACHE["chest"] = AppliedItemTexture{
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
	withRealRenderer(t)

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
	withRealRenderer(t)

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
	withRealRenderer(t)

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
