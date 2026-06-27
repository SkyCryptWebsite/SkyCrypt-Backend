package minecraftblockrenderer

import (
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	texturepacks "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/TexturePacks"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"strings"
	"testing"
)

func TestHPlusPlayerHeadSelectorRendersPackModel(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	packRoot := requireTexturePack(t, "hplus")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"hplus"})
	options := &BlockRenderOptions{
		Size:    64,
		PackIds: []string{"hplus"},
		ItemData: &data.ItemRenderData{CustomData: nbt.NewNbtCompound(map[string]nbt.NbtTag{
			"id": nbt.NewNbtString("AATROX_BATPHONE"),
		})},
	}

	rendered := renderer.RenderGuiItemWithResourceId("minecraft:player_head", options)
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("HPLUS player head render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	if rendered.ResourceId.SourcePackId != "hplus" || !strings.Contains(strings.ToLower(model), "aatrox") {
		t.Fatalf("resource did not resolve to HPLUS Aatrox model: source=%s model=%s textures=%v", rendered.ResourceId.SourcePackId, model, rendered.ResourceId.Textures)
	}
}

func TestRenderSkyBlockItemIDUsesHPlusSelector(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	packRoot := requireTexturePack(t, "hplus")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"hplus"})

	rendered, err := renderer.RenderSkyBlockItemID("AATROX_BATPHONE", &BlockRenderOptions{
		Size:    64,
		PackIds: []string{"hplus"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("SkyBlock item ID render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	if rendered.ResourceId.SourcePackId != "hplus" || !strings.Contains(strings.ToLower(model), "aatrox") {
		t.Fatalf("resource did not resolve to HPLUS Aatrox model: source=%s model=%s textures=%v", rendered.ResourceId.SourcePackId, model, rendered.ResourceId.Textures)
	}
}

func TestReloadResourcePacksClearsSkyblockItemDefinitionCache(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	packRoot := requireTexturePack(t, "hplus")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"hplus"})
	itemData := &data.ItemRenderData{CustomData: nbt.NewNbtCompound(map[string]nbt.NbtTag{
		"id": nbt.NewNbtString("AATROX_BATPHONE"),
	})}

	if model := renderer.ResolveSkyblockItemModelFromPackProviders("aatrox_batphone", "minecraft:player_head", itemData, "gui"); model == nil {
		t.Fatal("model did not resolve")
	}

	renderer._skyblockItemDefinitionsMu.RLock()
	cacheSize := len(renderer._skyblockItemDefinitions)
	renderer._skyblockItemDefinitionsMu.RUnlock()
	if cacheSize == 0 {
		t.Fatal("skyblock item definition cache was not populated")
	}

	if err := renderer.ReloadResourcePacks(); err != nil {
		t.Fatal(err)
	}

	renderer._skyblockItemDefinitionsMu.RLock()
	cacheSize = len(renderer._skyblockItemDefinitions)
	renderer._skyblockItemDefinitionsMu.RUnlock()
	if cacheSize != 0 {
		t.Fatalf("skyblock item definition cache was not cleared, len=%d", cacheSize)
	}
}

func TestFsrMidasSwordUsesPackSelector(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	packRoot := requireTexturePack(t, "fsr")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"fsr"})
	options := &BlockRenderOptions{
		Size:    64,
		PackIds: []string{"fsr"},
		ItemData: &data.ItemRenderData{CustomData: nbt.NewNbtCompound(map[string]nbt.NbtTag{
			"id": nbt.NewNbtString("MIDAS_SWORD"),
		})},
	}

	rendered := renderer.RenderGuiItemWithResourceId("minecraft:golden_sword", options)
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("FSR Midas sword render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	if rendered.ResourceId.SourcePackId != "fsr" || !strings.Contains(strings.ToLower(model), "midas") {
		t.Fatalf("resource did not resolve to FSR Midas model: source=%s model=%s textures=%v", rendered.ResourceId.SourcePackId, model, rendered.ResourceId.Textures)
	}
}

func TestRenderSkyBlockItemIDUsesFsrSelector(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	packRoot := requireTexturePack(t, "fsr")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"fsr"})

	rendered, err := renderer.RenderSkyBlockItemID("MIDAS_SWORD", &BlockRenderOptions{
		Size:    64,
		PackIds: []string{"fsr"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("SkyBlock item ID render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	if rendered.ResourceId.SourcePackId != "fsr" || !strings.Contains(strings.ToLower(model), "midas") {
		t.Fatalf("resource did not resolve to FSR Midas model: source=%s model=%s textures=%v", rendered.ResourceId.SourcePackId, model, rendered.ResourceId.Textures)
	}
}

func TestRenderSkyBlockItemIDUsesFsrAtomsplitFirstItemModelCase(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	packRoot := requireTexturePack(t, "fsr")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"fsr"})

	rendered, err := renderer.RenderSkyBlockItemID("ATOMSPLIT_KATANA", &BlockRenderOptions{
		Size:    64,
		PackIds: []string{"fsr"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("FSR Atomsplit SkyBlock ID render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	lowerModel := strings.ToLower(model)
	if rendered.ResourceId.SourcePackId != "fsr" || !strings.Contains(lowerModel, "atomsplit_katana") || strings.Contains(lowerModel, "ability") {
		t.Fatalf("resource did not resolve to normal FSR Atomsplit model: source=%s model=%s textures=%v", rendered.ResourceId.SourcePackId, model, rendered.ResourceId.Textures)
	}
}

func TestRenderItemNBTUsesFsrAtomsplitBaseItemModel(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	packRoot := requireTexturePack(t, "fsr")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"fsr"})

	item := map[string]any{
		"id":      "ATOMSPLIT_KATANA",
		"item_id": "minecraft:diamond_sword",
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id": "ATOMSPLIT_KATANA",
			},
		},
	}

	rendered, err := renderer.RenderItemNBT(item, &BlockRenderOptions{
		Size:    64,
		PackIds: []string{"fsr"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("FSR Atomsplit NBT render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	lowerModel := strings.ToLower(model)
	if rendered.ResourceId.SourcePackId != "fsr" || !strings.Contains(lowerModel, "atomsplit_katana") || strings.Contains(lowerModel, "ability") {
		t.Fatalf("resource did not resolve to normal FSR Atomsplit model: source=%s model=%s textures=%v", rendered.ResourceId.SourcePackId, model, rendered.ResourceId.Textures)
	}
}

func TestRenderItemNBTUsesFsrAtomsplitAbilityBaseItemModel(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	packRoot := requireTexturePack(t, "fsr")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"fsr"})

	item := map[string]any{
		"id":      "ATOMSPLIT_KATANA",
		"item_id": "minecraft:golden_sword",
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id": "ATOMSPLIT_KATANA",
			},
		},
	}

	rendered, err := renderer.RenderItemNBT(item, &BlockRenderOptions{
		Size:    64,
		PackIds: []string{"fsr"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("FSR Atomsplit ability NBT render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	lowerModel := strings.ToLower(model)
	if rendered.ResourceId.SourcePackId != "fsr" || !strings.Contains(lowerModel, "atomsplit_katana_ability") {
		t.Fatalf("resource did not resolve to FSR Atomsplit ability model: source=%s model=%s textures=%v", rendered.ResourceId.SourcePackId, model, rendered.ResourceId.Textures)
	}
}

func TestRenderGemstoneGauntlet3DModelUsesHPlusSelector(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	packRoot := requireTexturePack(t, "hplus")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"hplus"})

	rendered, err := renderer.RenderSkyBlockItemID("GEMSTONE_GAUNTLET", &BlockRenderOptions{
		Size:    96,
		PackIds: []string{"hplus"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("Gemstone Gauntlet render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	if rendered.ResourceId.SourcePackId != "hplus" || !strings.Contains(strings.ToLower(model), "gemstone") {
		t.Fatalf("resource did not resolve to HPLUS Gemstone Gauntlet model: source=%s model=%s textures=%v", rendered.ResourceId.SourcePackId, model, rendered.ResourceId.Textures)
	}

	minX, maxX, minY, maxY, ok := opaqueBounds(rendered.Image)
	if !ok {
		t.Fatal("Gemstone Gauntlet render has no opaque bounds")
	}
	width := maxX - minX + 1
	height := maxY - minY + 1
	if width < 24 || height < 24 {
		t.Fatalf("Gemstone Gauntlet 3D model rendered too small/collapsed: bounds=(%d,%d)-(%d,%d)", minX, minY, maxX, maxY)
	}
}

func TestRenderCrownOfAvariceUsesFsrModelInsteadOfTemplateSkullParent(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	packRoot := requireTexturePack(t, "fsr")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"fsr"})

	rendered, err := renderer.RenderSkyBlockItemID("CROWN_OF_AVARICE", &BlockRenderOptions{
		Size:    96,
		PackIds: []string{"fsr"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("Crown of Avarice render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	if rendered.ResourceId.SourcePackId != "fsr" || !strings.Contains(strings.ToLower(model), "crown_of_avarice") {
		t.Fatalf("resource did not resolve to FSR Crown of Avarice model: source=%s model=%s textures=%v", rendered.ResourceId.SourcePackId, model, rendered.ResourceId.Textures)
	}

	avg := averageColor(rendered.Image)
	if avg.R <= avg.B*2 || avg.G <= avg.B {
		t.Fatalf("Crown of Avarice render does not look like the gold crown model; average color=%+v", avg)
	}

	minX, maxX, minY, maxY, ok := opaqueBounds(rendered.Image)
	if !ok {
		t.Fatal("Crown of Avarice render has no opaque bounds")
	}
	width := maxX - minX + 1
	height := maxY - minY + 1
	if width < 40 || height < 40 {
		t.Fatalf("Crown of Avarice model rendered too small/collapsed: bounds=(%d,%d)-(%d,%d)", minX, minY, maxX, maxY)
	}
	if maxX < rendered.Image.Bounds().Dx()-8 || minY > 8 {
		t.Fatalf("Crown of Avarice should face the south-east GUI side; bounds=(%d,%d)-(%d,%d)", minX, minY, maxX, maxY)
	}
	lowerLeft, lowerRight := lowerHalfLuminanceBySide(rendered.Image)
	if lowerRight <= lowerLeft {
		t.Fatalf("Crown of Avarice should show the front/right GUI side; lower-left luminance=%d lower-right luminance=%d", lowerLeft, lowerRight)
	}
}

func TestRenderItemNBTCustomSkullUsesCustomModelInsteadOfHeadRenderer(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	packRoot := requireTexturePack(t, "fsr")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"fsr"})

	item := map[string]any{
		"id":    "minecraft:player_head",
		"Count": 1,
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id": "CROWN_OF_AVARICE",
			},
			"SkullOwner": map[string]any{
				"Properties": map[string]any{
					"textures": []any{
						map[string]any{
							"Value": "dummy-test-texture-value",
						},
					},
				},
			},
		},
	}

	rendered, err := renderer.RenderItemNBT(item, &BlockRenderOptions{
		Size:    96,
		PackIds: []string{"fsr"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("Crown of Avarice NBT render did not produce visible pixels")
	}
	model := ""
	if rendered.ResourceId.Model != nil {
		model = *rendered.ResourceId.Model
	}
	if rendered.ResourceId.SourcePackId != "fsr" || !strings.Contains(strings.ToLower(model), "crown_of_avarice") {
		t.Fatalf("resource did not resolve to FSR Crown of Avarice model: source=%s model=%s textures=%v", rendered.ResourceId.SourcePackId, model, rendered.ResourceId.Textures)
	}

	avg := averageColor(rendered.Image)
	if avg.R <= avg.B*2 || avg.G <= avg.B {
		t.Fatalf("Crown of Avarice NBT render does not look like the gold crown model; average color=%+v", avg)
	}

	minX, maxX, minY, maxY, ok := opaqueBounds(rendered.Image)
	if !ok {
		t.Fatal("Crown of Avarice NBT render has no opaque bounds")
	}
	width := maxX - minX + 1
	height := maxY - minY + 1
	if width < 40 || height < 40 {
		t.Fatalf("Crown of Avarice NBT model rendered too small/collapsed: bounds=(%d,%d)-(%d,%d)", minX, minY, maxX, maxY)
	}
	if maxX < rendered.Image.Bounds().Dx()-8 || minY > 8 {
		t.Fatalf("Crown of Avarice NBT render should face the south-east GUI side; bounds=(%d,%d)-(%d,%d)", minX, minY, maxX, maxY)
	}
	lowerLeft, lowerRight := lowerHalfLuminanceBySide(rendered.Image)
	if lowerRight <= lowerLeft {
		t.Fatalf("Crown of Avarice NBT render should show the front/right GUI side; lower-left luminance=%d lower-right luminance=%d", lowerLeft, lowerRight)
	}
}
