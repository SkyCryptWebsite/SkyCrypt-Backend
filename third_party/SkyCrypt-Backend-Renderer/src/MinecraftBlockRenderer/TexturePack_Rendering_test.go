package minecraftblockrenderer

import (
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	texturepacks "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/TexturePacks"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputeResourceIdIncludesPackOverrides(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	packRoot := createOverridePack(t, "overridepack", color.RGBA{R: 20, G: 220, B: 80, A: 255})

	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, nil)
	options := &BlockRenderOptions{Size: 32, PackIds: []string{"overridepack"}}

	rendered := renderer.RenderGuiItemWithResourceId("diamond_sword", options)
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("pack override render did not produce visible pixels")
	}
	if rendered.ResourceId.SourcePackId != "overridepack" {
		t.Fatalf("source pack = %q, want overridepack; model=%v textures=%v", rendered.ResourceId.SourcePackId, rendered.ResourceId.Model, rendered.ResourceId.Textures)
	}

	resolvedRenderer, forwarded := renderer.ResolveRendererForOptions(*options)
	recomputed := resolvedRenderer.ComputeResourceIdInternal("diamond_sword", forwarded, nil)
	if recomputed.ResourceId != rendered.ResourceId.ResourceId {
		t.Fatalf("recomputed resource id = %q, want %q", recomputed.ResourceId, rendered.ResourceId.ResourceId)
	}
}

func TestMissingSkyBlockCustomTextureReturnsError(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	packRoot := createEmptyPack(t, "emptypack")

	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, nil)

	rendered, err := renderer.RenderItemObjectWithResourceId(map[string]any{
		"id": "minecraft:diamond_sword",
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id": "MISSING_CUSTOM_SWORD",
			},
		},
	}, &BlockRenderOptions{Size: 32, PackIds: []string{"emptypack"}})
	if err == nil {
		t.Fatalf("expected missing custom texture render to fail, got rendered=%+v", rendered)
	}
}

func TestVanillaSkyBlockItemWithoutPackOverrideRendersVanillaItemObject(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	packRoot := createEmptyPack(t, "emptypack")

	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, nil)

	rendered, err := renderer.RenderItemObjectWithResourceId(map[string]any{
		"id":      "EYE_OF_ENDER",
		"item_id": "minecraft:eye_of_ender",
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id": "EYE_OF_ENDER",
			},
		},
	}, &BlockRenderOptions{Size: 32, PackIds: []string{"emptypack"}})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("vanilla SkyBlock item render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	if rendered.ResourceId.SourcePackId != "vanilla" || !strings.Contains(strings.ToLower(model), "eye_of_ender") {
		t.Fatalf("resource did not resolve to vanilla eye of ender: source=%s model=%s textures=%v", rendered.ResourceId.SourcePackId, model, rendered.ResourceId.Textures)
	}
}

func TestVanillaSkyBlockItemIDWithoutPackOverrideRendersVanilla(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	packRoot := createEmptyPack(t, "emptypack")

	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, nil)

	rendered, err := renderer.RenderSkyBlockItemID("EYE_OF_ENDER", &BlockRenderOptions{Size: 32, PackIds: []string{"emptypack"}})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("vanilla SkyBlock item ID render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	if rendered.ResourceId.SourcePackId != "vanilla" || !strings.Contains(strings.ToLower(model), "eye_of_ender") {
		t.Fatalf("resource did not resolve to vanilla eye of ender: source=%s model=%s textures=%v", rendered.ResourceId.SourcePackId, model, rendered.ResourceId.Textures)
	}
}

func TestNBTItemModelSelectorTakesPriorityOverFirmamentFallback(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	packRoot := createEmptyPack(t, "testpack")
	writeJSON(t, packRoot, "assets/skyblock/items/selector_3d.json", `{"model":{"type":"select","property":"component","component":"item_model","cases":[
		{"when":"minecraft:diamond_sword","model":{"type":"model","model":"firmskyblock:item/selector_3d_inventory"}}
	]}}`)
	writeJSON(t, packRoot, "assets/firmskyblock/models/item/selector_3d.json", `{
		"textures":{"all":"firmskyblock:item/selector_3d_wrong"},
		"elements":[{"from":[2,2,2],"to":[14,14,14],"faces":{
			"north":{"texture":"#all"},"south":{"texture":"#all"},"east":{"texture":"#all"},
			"west":{"texture":"#all"},"up":{"texture":"#all"},"down":{"texture":"#all"}
		}}],
		"display":{"gui":{"rotation":[0,0,0],"scale":[1,1,1]}}
	}`)
	writeJSON(t, packRoot, "assets/firmskyblock/models/item/selector_3d_inventory.json", `{
		"textures":{"all":"firmskyblock:item/selector_3d_inventory"},
		"elements":[{"from":[2,2,2],"to":[14,14,14],"faces":{
			"north":{"texture":"#all"},"south":{"texture":"#all"},"east":{"texture":"#all"},
			"west":{"texture":"#all"},"up":{"texture":"#all"},"down":{"texture":"#all"}
		}}],
		"display":{"gui":{"rotation":[30,225,0],"scale":[0.8,0.8,0.8]}}
	}`)
	writePNG(t, filepath.Join(packRoot, "assets", "firmskyblock", "textures", "item", "selector_3d_wrong.png"), 16, 16, color.RGBA{R: 220, G: 30, B: 30, A: 255})
	writePNG(t, filepath.Join(packRoot, "assets", "firmskyblock", "textures", "item", "selector_3d_inventory.png"), 16, 16, color.RGBA{R: 30, G: 220, B: 80, A: 255})

	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, nil)

	rendered, err := renderer.RenderItemNBT(map[string]any{
		"id":      "minecraft:diamond_sword",
		"item_id": "minecraft:diamond_sword",
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id": "SELECTOR_3D",
			},
		},
	}, &BlockRenderOptions{Size: 48, PackIds: []string{"testpack"}})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("NBT selector 3D render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	if !strings.Contains(strings.ToLower(model), "selector_3d_inventory") {
		t.Fatalf("resource model = %q, want inventory selector model; textures=%v", model, rendered.ResourceId.Textures)
	}
	if !imageContainsApproxColor(rendered.Image, color.RGBA{R: 30, G: 220, B: 80, A: 255}, 20) {
		t.Fatalf("NBT selector render did not use inventory texture; resource=%+v", rendered.ResourceId)
	}
}

func TestMissingSkyBlockCustomSkullReturnsError(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	packRoot := createEmptyPack(t, "emptypack")

	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, nil)

	var sawResolver bool
	texture := "minecraft:item/diamond_sword"
	rendered, err := renderer.RenderItemObjectWithResourceId(map[string]any{
		"id": "minecraft:player_head",
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id": "MISSING_CUSTOM_HEAD",
			},
		},
	}, &BlockRenderOptions{
		Size:    32,
		PackIds: []string{"emptypack"},
		SkullTextureResolver: func(context SkullResolverContext) *string {
			sawResolver = true
			if context.CustomDataId == nil || *context.CustomDataId != "MISSING_CUSTOM_HEAD" {
				t.Fatalf("custom data id = %#v", context.CustomDataId)
			}
			return &texture
		},
	})
	if err == nil {
		t.Fatalf("expected missing custom skull render to fail, got rendered=%+v", rendered)
	}
	_ = sawResolver
}

func TestTryResolveHeadSkinIgnoresMissingTextureOverride(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	registry := texturepacks.NewTexturePackRegistry()
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, nil)

	options := DefaultBlockRenderOptions()
	options.ItemData = &data.ItemRenderData{CustomData: nbt.NewNbtCompound(map[string]nbt.NbtTag{
		"texture": nbt.NewNbtString("minecraft:item/not_a_real_head_skin"),
	})}

	skin := renderer.TryResolveHeadSkin("minecraft:player_head", options)
	if skin != nil && renderer._textureRepository.IsMissingTexture(skin) {
		t.Fatal("missing texture placeholder was accepted as a resolved head skin")
	}
}

func TestFirmamentCompositeSkyBlockItemRendersAllLayers(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	packRoot := createEmptyPack(t, "testpack")
	writeJSON(t, packRoot, "assets/skyblock/items/dark_claymore.json", `{"model":{"type":"composite","models":[
		{"type":"model","model":"firmskyblock:item/dark_claymore_base"},
		{"type":"model","model":"firmskyblock:item/dark_claymore_overlay"}
	]}}`)
	writeJSON(t, packRoot, "assets/firmskyblock/models/item/dark_claymore_base.json", `{"parent":"builtin/generated","textures":{"layer0":"firmskyblock:item/dark_claymore_base"}}`)
	writeJSON(t, packRoot, "assets/firmskyblock/models/item/dark_claymore_overlay.json", `{"parent":"builtin/generated","textures":{"layer0":"firmskyblock:item/dark_claymore_overlay"}}`)
	baseColor := color.RGBA{R: 44, G: 80, B: 220, A: 255}
	overlayColor := color.RGBA{R: 230, G: 210, B: 40, A: 255}
	writePNG(t, filepath.Join(packRoot, "assets", "firmskyblock", "textures", "item", "dark_claymore_base.png"), 16, 16, baseColor)
	writeCenterPatchPNG(t, filepath.Join(packRoot, "assets", "firmskyblock", "textures", "item", "dark_claymore_overlay.png"), 16, 16, overlayColor)

	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, nil)

	rendered, err := renderer.RenderSkyBlockItemID("DARK_CLAYMORE", &BlockRenderOptions{
		Size:    64,
		PackIds: []string{"testpack"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || rendered.Image == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("Firmament composite render did not produce visible pixels")
	}
	if !imageContainsApproxColor(rendered.Image, baseColor, 8) {
		t.Fatalf("Firmament composite render does not contain base color; resource=%+v", rendered.ResourceId)
	}
	if !imageContainsApproxColor(rendered.Image, overlayColor, 8) {
		t.Fatalf("Firmament composite render does not contain overlay color; resource=%+v", rendered.ResourceId)
	}
	if rendered.ResourceId.SourcePackId != "testpack" {
		t.Fatalf("source pack = %q, want testpack; model=%v textures=%v", rendered.ResourceId.SourcePackId, rendered.ResourceId.Model, rendered.ResourceId.Textures)
	}
	if rendered.ResourceId.Model == nil || !strings.HasPrefix(*rendered.ResourceId.Model, "composite:") {
		t.Fatalf("resource model = %v, want composite model", rendered.ResourceId.Model)
	}
	packRenderer, _ := renderer.ResolveRendererForOptions(BlockRenderOptions{PackIds: []string{"testpack"}})
	for _, textureID := range rendered.ResourceId.Textures {
		if packRenderer.TextureIsMissing(textureID) {
			t.Fatalf("resource texture %q is missing; textures=%v", textureID, rendered.ResourceId.Textures)
		}
	}
}

func createOverridePack(t *testing.T, id string, c color.RGBA) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), id)
	writeJSON(t, root, "meta.json", `{"id":"`+id+`","name":"Override Pack","version":"test"}`)
	writeJSON(t, root, "pack.mcmeta", `{"pack":{"pack_format":99,"description":"test"}}`)
	writeJSON(t, root, "assets/minecraft/items/diamond_sword.json", `{"model":{"type":"model","model":"minecraft:item/diamond_sword"}}`)
	writeJSON(t, root, "assets/minecraft/models/item/diamond_sword.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/diamond_sword"}}`)
	writePNG(t, filepath.Join(root, "assets", "minecraft", "textures", "item", "diamond_sword.png"), 16, 16, c)
	if err := os.WriteFile(filepath.Join(root, "pack.png"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func createEmptyPack(t *testing.T, id string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), id)
	writeJSON(t, root, "meta.json", `{"id":"`+id+`","name":"Empty Pack","version":"test"}`)
	writeJSON(t, root, "pack.mcmeta", `{"pack":{"pack_format":99,"description":"test"}}`)
	if err := os.MkdirAll(filepath.Join(root, "assets", "minecraft"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pack.png"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
