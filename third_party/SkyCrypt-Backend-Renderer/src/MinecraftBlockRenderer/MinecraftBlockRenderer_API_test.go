package minecraftblockrenderer

import (
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/assets"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type skycryptItem struct {
	ID     any            `json:"id"`
	Damage int            `json:"Damage"`
	Tag    map[string]any `json:"tag"`
}

func TestRendererPublicAPIsWithMinimalAssets(t *testing.T) {
	assetsRoot := createMinimalAssets(t)

	renderer, err := CreateFromDataDirectory(assetsRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := renderer.GetKnownBlockNames(), []string{"stone"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("known blocks = %#v, want %#v", got, want)
	}
	if got := renderer.GetKnownItemNames(); !contains(got, "diamond_sword") || !contains(got, "red_wool") {
		t.Fatalf("known items missing expected entries: %#v", got)
	}

	block := renderer.RenderBlock("stone", BlockRenderOptions{Size: 32})
	assertImage(t, block, 32, 1)

	face, err := renderer.RenderBlockFace("stone", BlockFaceRenderOptions{Size: 32, Face: 0})
	if err != nil {
		t.Fatal(err)
	}
	assertImage(t, face, 32, 1)

	textureItem, err := renderer.RenderGuiItemFromTextureId("minecraft:item/diamond_sword", &BlockRenderOptions{Size: 32})
	if err != nil {
		t.Fatal(err)
	}
	assertImage(t, textureItem, 32, 1)

	rendered, err := renderer.RenderItemObjectWithResourceId(map[string]any{
		"id":     float64(35),
		"Damage": float64(14),
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{"id": "RED_WOOL_TEST"},
		},
	}, &BlockRenderOptions{Size: 32})
	if err != nil {
		t.Fatal(err)
	}
	assertImage(t, rendered.Image, 32, 1)
	if rendered.ResourceId.ResourceId == "" {
		t.Fatal("empty resource id")
	}

	again, err := renderer.ComputeResourceIdFromItemObject(skycryptItem{
		ID:     35,
		Damage: 14,
		Tag: map[string]any{
			"ExtraAttributes": map[string]any{"id": "RED_WOOL_TEST"},
		},
	}, &BlockRenderOptions{Size: 32})
	if err != nil {
		t.Fatal(err)
	}
	if again.ResourceId != rendered.ResourceId.ResourceId {
		t.Fatalf("resource id changed: %s != %s", again.ResourceId, rendered.ResourceId.ResourceId)
	}
}

func TestCreateFromResourceProviderAndAnimatedRender(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	provider, err := assets.NewDirectoryResourceProvider(assetsRoot)
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := CreateFromResourceProvider(provider)
	if err != nil {
		t.Fatal(err)
	}

	animated, err := renderer.RenderAnimatedGuiItemWithResourceId("clock", &BlockRenderOptions{Size: 32})
	if err != nil {
		t.Fatal(err)
	}
	assertImage(t, animated.Image, 32, 1)
	if len(animated.Frames) != 2 {
		t.Fatalf("animated frames = %d, want 2", len(animated.Frames))
	}
	for _, frame := range animated.Frames {
		assertImage(t, frame.Image, 32, 1)
		if frame.ResourceId.ResourceId == "" {
			t.Fatal("frame resource id is empty")
		}
	}
}

func TestPartialBlockRenderOptionsKeepDefaults(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	renderer, err := CreateFromDataDirectory(assetsRoot)
	if err != nil {
		t.Fatal(err)
	}

	defaultSize := DefaultBlockRenderOptions().Size
	rendered := renderer.RenderGuiItemWithResourceId("diamond_sword", &BlockRenderOptions{})
	if rendered == nil {
		t.Fatal("rendered resource is nil")
	}
	assertImage(t, rendered.Image, defaultSize, 1)

	textureItem, err := renderer.RenderGuiItemFromTextureId("minecraft:item/diamond_sword", &BlockRenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertImage(t, textureItem, defaultSize, 1)

	animated, err := renderer.RenderAnimatedGuiItemWithResourceId("clock", &BlockRenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertImage(t, animated.Image, defaultSize, 1)
}

func TestRenderFlatItemCompositesTransparentModelLayers(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	writeJSON(t, assetsRoot, "items/layered_item.json", `{"model":{"model":"minecraft:item/layered_item"}}`)
	writeJSON(t, assetsRoot, "models/item/layered_item.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/layer_base","layer1":"minecraft:item/layer_empty"}}`)
	writePNG(t, filepath.Join(assetsRoot, "textures", "item", "layer_base.png"), 16, 16, color.RGBA{R: 40, G: 180, B: 220, A: 255})
	writeTransparentPNG(t, filepath.Join(assetsRoot, "textures", "item", "layer_empty.png"), 16, 16)

	renderer, err := CreateFromDataDirectory(assetsRoot)
	if err != nil {
		t.Fatal(err)
	}

	rendered := renderer.RenderGuiItemWithResourceId("layered_item", &BlockRenderOptions{Size: 32})
	if rendered == nil || rendered.Image == nil {
		t.Fatal("rendered image is nil")
	}

	got := rendered.Image.RGBAAt(16, 16)
	want := color.RGBA{R: 40, G: 180, B: 220, A: 255}
	if got != want {
		t.Fatalf("transparent top layer erased base layer: center=%#v, want %#v", got, want)
	}
}

func TestRenderCompositeItemSelectorDrawsAllModelLayers(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	compositeDefinition := `{"type":"composite","models":[
		{"type":"model","model":"minecraft:item/composite_base"},
		{"type":"model","model":"minecraft:item/composite_overlay"}
	]}`
	writeJSON(t, assetsRoot, "items/composite_weapon.json", compositeDefinition)
	writeJSON(t, assetsRoot, "models/item/composite_base.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/composite_base_layer"}}`)
	writeJSON(t, assetsRoot, "models/item/composite_overlay.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/composite_overlay_layer"}}`)
	baseColor := color.RGBA{R: 210, G: 30, B: 40, A: 255}
	overlayColor := color.RGBA{R: 30, G: 220, B: 70, A: 255}
	writePNG(t, filepath.Join(assetsRoot, "textures", "item", "composite_base_layer.png"), 16, 16, baseColor)
	writeCenterPatchPNG(t, filepath.Join(assetsRoot, "textures", "item", "composite_overlay_layer.png"), 16, 16, overlayColor)

	renderer, err := CreateFromDataDirectory(assetsRoot)
	if err != nil {
		t.Fatal(err)
	}
	itemInfo := renderer._itemRegistry.GetItemInfo("composite_weapon")
	if itemInfo == nil || itemInfo.Selector == nil {
		t.Fatalf("composite_weapon item selector did not load: %#v", itemInfo)
	}
	model, _, resolvedModelName, compositeModelNames, _ := renderer.ResolveItemModel("composite_weapon", itemInfo, MergeBlockRenderOptions(&BlockRenderOptions{Size: 32}))
	if model == nil || resolvedModelName == nil || len(compositeModelNames) != 2 {
		t.Fatalf("resolved model=%#v resolved=%v composite=%v", model, resolvedModelName, compositeModelNames)
	}

	rendered := renderer.RenderGuiItemWithResourceId("composite_weapon", &BlockRenderOptions{Size: 32})
	if rendered == nil || rendered.Image == nil {
		t.Fatal("rendered image is nil")
	}
	if !imageContainsApproxColor(rendered.Image, baseColor, 8) {
		t.Fatalf("composite render does not contain base color; resource=%+v", rendered.ResourceId)
	}
	if !imageContainsApproxColor(rendered.Image, overlayColor, 8) {
		t.Fatalf("composite render does not contain overlay color; resource=%+v", rendered.ResourceId)
	}
	if rendered.ResourceId.Model == nil || !strings.HasPrefix(*rendered.ResourceId.Model, "composite:") {
		t.Fatalf("resource model = %v, want composite model", rendered.ResourceId.Model)
	}
	if !contains(rendered.ResourceId.Textures, "minecraft:item/composite_base_layer") ||
		!contains(rendered.ResourceId.Textures, "minecraft:item/composite_overlay_layer") {
		t.Fatalf("resource textures = %v, want both composite textures", rendered.ResourceId.Textures)
	}
}

func TestMinecraftAtlasGeneratorOrderingAndEntryErrors(t *testing.T) {
	renderer, err := CreateFromDataDirectory(createMinimalAssets(t))
	if err != nil {
		t.Fatal(err)
	}
	generator := NewMinecraftAtlasGenerator(renderer)

	result, err := generator.GenerateItemAtlas(MinecraftAtlasOptions{
		Names: []string{"missing_item", "diamond_sword"},
		Size:  16,
		Cols:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Image.Bounds().Dx() != 16 || result.Image.Bounds().Dy() != 32 {
		t.Fatalf("atlas image size = %dx%d, want 16x32", result.Image.Bounds().Dx(), result.Image.Bounds().Dy())
	}
	if got, want := result.Manifest.Width, 16; got != want {
		t.Fatalf("manifest width = %d, want %d", got, want)
	}
	if got, want := result.Manifest.Height, 32; got != want {
		t.Fatalf("manifest height = %d, want %d", got, want)
	}
	if got, want := []string{result.Manifest.Entries[0].Name, result.Manifest.Entries[1].Name}, []string{"diamond_sword", "missing_item"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("entry order = %#v, want %#v", got, want)
	}
}

func createMinimalAssets(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "assets", "minecraft")
	writeJSON(t, root, "blockstates/stone.json", `{"variants":{"":{"model":"minecraft:block/stone"}}}`)
	writeJSON(t, root, "models/block/stone.json", `{
		"textures":{"all":"minecraft:block/stone"},
		"elements":[{"from":[0,0,0],"to":[16,16,16],"faces":{
			"north":{"texture":"#all"},"south":{"texture":"#all"},"east":{"texture":"#all"},
			"west":{"texture":"#all"},"up":{"texture":"#all"},"down":{"texture":"#all"}
		}}]
	}`)
	writeJSON(t, root, "items/diamond_sword.json", `{"model":{"model":"minecraft:item/diamond_sword"}}`)
	writeJSON(t, root, "items/player_head.json", `{"model":{"model":"minecraft:item/player_head"}}`)
	writeJSON(t, root, "items/red_wool.json", `{"model":{"model":"minecraft:item/red_wool"}}`)
	writeJSON(t, root, "items/lime_dye.json", `{"model":{"model":"minecraft:item/lime_dye"}}`)
	writeJSON(t, root, "items/clock.json", `{"model":{"model":"minecraft:item/clock"}}`)
	writeJSON(t, root, "items/eye_of_ender.json", `{"model":{"model":"minecraft:item/eye_of_ender"}}`)
	writeJSON(t, root, "items/chest.json", `{"model":{"type":"minecraft:special","base":"minecraft:item/chest","model":{"type":"minecraft:chest","texture":"minecraft:normal"}}}`)
	writeJSON(t, root, "items/ender_chest.json", `{"model":{"type":"minecraft:special","base":"minecraft:item/ender_chest","model":{"type":"minecraft:chest","texture":"minecraft:ender"}}}`)
	writeJSON(t, root, "models/item/diamond_sword.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/diamond_sword"}}`)
	writeJSON(t, root, "models/item/player_head.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/player_head"}}`)
	writeJSON(t, root, "models/item/red_wool.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/red_wool"}}`)
	writeJSON(t, root, "models/item/lime_dye.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/lime_dye"}}`)
	writeJSON(t, root, "models/item/clock.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/clock"}}`)
	writeJSON(t, root, "models/item/eye_of_ender.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/eye_of_ender"}}`)
	writeJSON(t, root, "models/item/chest.json", `{"parent":"minecraft:item/template_chest","textures":{"particle":"minecraft:block/oak_planks"}}`)
	writeJSON(t, root, "models/item/ender_chest.json", `{"parent":"minecraft:item/template_chest","textures":{"particle":"minecraft:block/obsidian"}}`)
	writeJSON(t, root, "models/item/template_chest.json", `{"display":{"gui":{"rotation":[30,225,0],"translation":[0,0,0],"scale":[0.625,0.625,0.625]}}}`)
	writePNG(t, filepath.Join(root, "textures", "block", "stone.png"), 16, 16, color.RGBA{R: 180, G: 30, B: 30, A: 255})
	writePNG(t, filepath.Join(root, "textures", "block", "oak_planks.png"), 16, 16, color.RGBA{R: 190, G: 130, B: 70, A: 255})
	writePNG(t, filepath.Join(root, "textures", "block", "obsidian.png"), 16, 16, color.RGBA{R: 35, G: 24, B: 55, A: 255})
	writePNG(t, filepath.Join(root, "textures", "item", "diamond_sword.png"), 16, 16, color.RGBA{R: 40, G: 180, B: 220, A: 255})
	writePNG(t, filepath.Join(root, "textures", "item", "player_head.png"), 16, 16, color.RGBA{R: 90, G: 90, B: 90, A: 255})
	writePNG(t, filepath.Join(root, "textures", "item", "red_wool.png"), 16, 16, color.RGBA{R: 220, G: 40, B: 40, A: 255})
	writePNG(t, filepath.Join(root, "textures", "item", "lime_dye.png"), 16, 16, color.RGBA{R: 90, G: 220, B: 60, A: 255})
	writePNG(t, filepath.Join(root, "textures", "item", "eye_of_ender.png"), 16, 16, color.RGBA{R: 80, G: 210, B: 120, A: 255})
	writePNG(t, filepath.Join(root, "textures", "entity", "chest", "normal.png"), 64, 64, color.RGBA{R: 135, G: 80, B: 38, A: 255})
	writePNG(t, filepath.Join(root, "textures", "entity", "chest", "ender.png"), 64, 64, color.RGBA{R: 45, G: 28, B: 76, A: 255})
	writeAnimatedPNG(t, filepath.Join(root, "textures", "item", "clock.png"))
	writeJSON(t, root, "textures/item/clock.png.mcmeta", `{"animation":{"frametime":1,"frames":[0,1],"width":16,"height":16}}`)
	return root
}

func writeJSON(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writePNG(t *testing.T, path string, width int, height int, c color.RGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func writeTransparentPNG(t *testing.T, path string, width int, height int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func writeCenterPatchPNG(t *testing.T, path string, width int, height int, c color.RGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := height / 4; y < height*3/4; y++ {
		for x := width / 4; x < width*3/4; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func writeAnimatedPNG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 16, 32))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
			img.SetRGBA(x, y+16, color.RGBA{G: 255, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func assertImage(t *testing.T, img *image.RGBA, size int, minOpaque int) {
	t.Helper()
	if img == nil {
		t.Fatal("image is nil")
	}
	if img.Bounds().Dx() != size || img.Bounds().Dy() != size {
		t.Fatalf("image size = %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), size, size)
	}
	opaque := 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if img.RGBAAt(x, y).A > 0 {
				opaque++
			}
		}
	}
	if opaque < minOpaque {
		t.Fatalf("opaque pixels = %d, want at least %d", opaque, minOpaque)
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
