package minecraftblockrenderer

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
)

type expectedChestFace struct {
	uv          data.Vector4
	rotation    int
	hasRotation bool
}

func TestSpecialChestElementsMatchReferenceModel(t *testing.T) {
	elements := specialChestElements()
	if len(elements) != 3 {
		t.Fatalf("element count = %d, want 3", len(elements))
	}

	assertChestElement(t, elements[0],
		data.Vector3{X: 1, Y: 0, Z: 1},
		data.Vector3{X: 15, Y: 10, Z: 15},
		map[data.BlockFaceDirection]expectedChestFace{
			data.Down:  {uv: data.Vector4{X: 3.5, Y: 4.75, Z: 7, W: 8.25}, rotation: 180, hasRotation: true},
			data.North: {uv: data.Vector4{X: 10.5, Y: 8.25, Z: 14, W: 10.75}, rotation: 180, hasRotation: true},
			data.East:  {uv: data.Vector4{X: 0, Y: 8.25, Z: 3.5, W: 10.75}, rotation: 180, hasRotation: true},
			data.South: {uv: data.Vector4{X: 3.5, Y: 8.25, Z: 7, W: 10.75}, rotation: 180, hasRotation: true},
			data.West:  {uv: data.Vector4{X: 7, Y: 8.25, Z: 10.5, W: 10.75}, rotation: 180, hasRotation: true},
		})

	assertChestElement(t, elements[1],
		data.Vector3{X: 1, Y: 10, Z: 1},
		data.Vector3{X: 15, Y: 14, Z: 15},
		map[data.BlockFaceDirection]expectedChestFace{
			data.Up:    {uv: data.Vector4{X: 3.5, Y: 4.75, Z: 7, W: 8.25}},
			data.North: {uv: data.Vector4{X: 10.5, Y: 3.75, Z: 14, W: 4.75}, rotation: 180, hasRotation: true},
			data.East:  {uv: data.Vector4{X: 0, Y: 3.75, Z: 3.5, W: 4.75}, rotation: 180, hasRotation: true},
			data.South: {uv: data.Vector4{X: 3.5, Y: 3.75, Z: 7, W: 4.75}, rotation: 180, hasRotation: true},
			data.West:  {uv: data.Vector4{X: 7, Y: 3.75, Z: 10.5, W: 4.75}, rotation: 180, hasRotation: true},
		})

	assertChestElement(t, elements[2],
		data.Vector3{X: 7, Y: 7, Z: 0},
		data.Vector3{X: 9, Y: 11, Z: 1},
		map[data.BlockFaceDirection]expectedChestFace{
			data.Down:  {uv: data.Vector4{X: 0.25, Y: 0, Z: 0.75, W: 0.25}, rotation: 180, hasRotation: true},
			data.Up:    {uv: data.Vector4{X: 0.75, Y: 0, Z: 1.25, W: 0.25}, rotation: 180, hasRotation: true},
			data.North: {uv: data.Vector4{X: 1, Y: 0.25, Z: 1.5, W: 1.25}, rotation: 180, hasRotation: true},
			data.West:  {uv: data.Vector4{X: 0.75, Y: 0.25, Z: 1, W: 1.25}, rotation: 180, hasRotation: true},
			data.East:  {uv: data.Vector4{X: 0, Y: 0.25, Z: 0.25, W: 1.25}, rotation: 180, hasRotation: true},
		})
}

func TestRenderSpecialChestItemsUseEntityTextures(t *testing.T) {
	renderer, err := CreateFromDataDirectory(createMinimalAssets(t))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		item            string
		entityTexture   string
		particleTexture string
	}{
		{"chest", "minecraft:entity/chest/normal", "minecraft:block/oak_planks"},
		{"ender_chest", "minecraft:entity/chest/ender", "minecraft:block/obsidian"},
	}

	for _, test := range tests {
		t.Run(test.item, func(t *testing.T) {
			options := &BlockRenderOptions{Size: 64}
			rendered := renderer.RenderGuiItemWithResourceId(test.item, options)
			if rendered == nil || rendered.Image == nil || !hasOpaquePixels(rendered.Image) {
				t.Fatalf("%s render did not produce visible pixels", test.item)
			}
			if !contains(rendered.ResourceId.Textures, test.entityTexture) {
				t.Fatalf("resource textures = %v, want %s", rendered.ResourceId.Textures, test.entityTexture)
			}
			if contains(rendered.ResourceId.Textures, test.particleTexture) {
				t.Fatalf("resource textures = %v, must not use particle texture %s", rendered.ResourceId.Textures, test.particleTexture)
			}
			if rendered.ResourceId.Model == nil || !strings.HasPrefix(*rendered.ResourceId.Model, "special:minecraft:chest:") {
				t.Fatalf("resource model = %v, want special chest marker", rendered.ResourceId.Model)
			}

			flatParticle, err := renderer.RenderGuiItemFromTextureId(test.particleTexture, options)
			if err != nil {
				t.Fatal(err)
			}
			if imagesAreIdentical(rendered.Image, flatParticle) {
				t.Fatalf("%s special render matched flat particle render", test.item)
			}
		})
	}
}

func TestRenderSpecialChestUsesReferenceUVLayout(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	writePatternedChestPNG(t, filepath.Join(assetsRoot, "textures", "entity", "chest", "normal.png"))

	renderer, err := CreateFromDataDirectory(assetsRoot)
	if err != nil {
		t.Fatal(err)
	}

	actual := renderer.RenderGuiItemWithResourceId("chest", &BlockRenderOptions{Size: 96})
	if actual == nil || actual.Image == nil || !hasOpaquePixels(actual.Image) {
		t.Fatal("special chest render did not produce visible pixels")
	}

	generic := renderGenericChestItemForRegression(t, renderer, 96)
	if imagesAreIdentical(actual.Image, generic) {
		t.Fatal("special chest render matched generic default-UV chest render")
	}
}

func TestRenderSpecialChestKeepsFrontLockVisible(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	writePatternedChestPNG(t, filepath.Join(assetsRoot, "textures", "entity", "chest", "normal.png"))
	writePatternedChestPNG(t, filepath.Join(assetsRoot, "textures", "entity", "chest", "ender.png"))

	renderer, err := CreateFromDataDirectory(assetsRoot)
	if err != nil {
		t.Fatal(err)
	}

	lockFrontColor := color.RGBA{R: 255, G: 240, B: 80, A: 255}
	for _, item := range []string{"chest", "ender_chest"} {
		t.Run(item, func(t *testing.T) {
			rendered := renderer.RenderGuiItemWithResourceId(item, &BlockRenderOptions{Size: 128})
			if rendered == nil || rendered.Image == nil || !hasOpaquePixels(rendered.Image) {
				t.Fatalf("%s render did not produce visible pixels", item)
			}
			if !imageContainsApproxColor(rendered.Image, lockFrontColor, 85) {
				t.Fatalf("%s render does not show the front lock face", item)
			}
			minX, maxX, _, _, ok := approxColorBounds(rendered.Image, lockFrontColor, 85)
			if !ok {
				t.Fatalf("%s render does not expose measurable front lock pixels", item)
			}
			if maxX >= rendered.Image.Bounds().Dx()/2 {
				t.Fatalf("%s front lock is on the wrong side: lock x=%d..%d image width=%d", item, minX, maxX, rendered.Image.Bounds().Dx())
			}
		})
	}
}

func TestPackedSkyBlockChestFallbackUsesSpecialEntityTexture(t *testing.T) {
	renderer, err := CreateFromDataDirectory(createMinimalAssets(t))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		item          map[string]any
		entityTexture string
	}{
		{
			name: "chest",
			item: map[string]any{
				"id":          "minecraft:chest",
				"item_id":     54,
				"ItemModel":   "minecraft:chest",
				"skyblock_id": "CHEST",
				"tag":         map[string]any{"ExtraAttributes": map[string]any{"id": "CHEST"}},
			},
			entityTexture: "minecraft:entity/chest/normal",
		},
		{
			name: "ender_chest",
			item: map[string]any{
				"id":          "minecraft:ender_chest",
				"item_id":     130,
				"ItemModel":   "minecraft:ender_chest",
				"skyblock_id": "ENDER_CHEST",
				"tag":         map[string]any{"ExtraAttributes": map[string]any{"id": "ENDER_CHEST"}},
			},
			entityTexture: "minecraft:entity/chest/ender",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rendered, err := renderer.RenderItemObjectWithResourceId(test.item, &BlockRenderOptions{Size: 64, PackIds: []string{"unused-pack-id"}})
			if err != nil {
				t.Fatal(err)
			}
			if rendered == nil || rendered.Image == nil || !hasOpaquePixels(rendered.Image) {
				t.Fatal("render did not produce visible pixels")
			}
			if !contains(rendered.ResourceId.Textures, test.entityTexture) {
				t.Fatalf("resource textures = %v, want %s; model=%v", rendered.ResourceId.Textures, test.entityTexture, rendered.ResourceId.Model)
			}
		})
	}
}

func TestSpecialChestItemsWithFullAssets(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	renderer := CreateFromMinecraftAssets(assetsRoot, nil, nil)

	tests := []struct {
		item          string
		entityTexture string
	}{
		{"chest", "minecraft:entity/chest/normal"},
		{"ender_chest", "minecraft:entity/chest/ender"},
		{"trapped_chest", "minecraft:entity/chest/trapped"},
	}

	for _, test := range tests {
		t.Run(test.item, func(t *testing.T) {
			rendered := renderer.RenderGuiItemWithResourceId(test.item, &BlockRenderOptions{Size: 64})
			if rendered == nil || rendered.Image == nil || !hasOpaquePixels(rendered.Image) {
				t.Fatalf("%s render did not produce visible pixels", test.item)
			}
			if !contains(rendered.ResourceId.Textures, test.entityTexture) {
				t.Fatalf("%s resource textures = %v, want %s", test.item, rendered.ResourceId.Textures, test.entityTexture)
			}
		})
	}

	copperCandidates := []struct {
		item          string
		entityTexture string
	}{
		{"copper_chest", "minecraft:entity/chest/copper"},
		{"exposed_copper_chest", "minecraft:entity/chest/copper_exposed"},
		{"weathered_copper_chest", "minecraft:entity/chest/copper_weathered"},
		{"oxidized_copper_chest", "minecraft:entity/chest/copper_oxidized"},
	}
	for _, candidate := range copperCandidates {
		if renderer._itemRegistry.GetItemInfo(candidate.item) == nil {
			continue
		}
		rendered := renderer.RenderGuiItemWithResourceId(candidate.item, &BlockRenderOptions{Size: 64})
		if rendered == nil || rendered.Image == nil || !hasOpaquePixels(rendered.Image) {
			t.Fatalf("%s render did not produce visible pixels", candidate.item)
		}
		if !contains(rendered.ResourceId.Textures, candidate.entityTexture) {
			t.Fatalf("%s resource textures = %v, want %s", candidate.item, rendered.ResourceId.Textures, candidate.entityTexture)
		}
		return
	}
	t.Skip("full assets do not include a copper chest item variant")
}

func assertChestElement(t *testing.T, got data.ModelElement, from data.Vector3, to data.Vector3, faces map[data.BlockFaceDirection]expectedChestFace) {
	t.Helper()
	if got.From != from {
		t.Fatalf("from = %+v, want %+v", got.From, from)
	}
	if got.To != to {
		t.Fatalf("to = %+v, want %+v", got.To, to)
	}
	if !got.Shade {
		t.Fatal("shade = false, want true")
	}
	if len(got.Faces) != len(faces) {
		t.Fatalf("face count = %d, want %d; faces=%v", len(got.Faces), len(faces), got.Faces)
	}

	for direction, want := range faces {
		face, ok := got.Faces[direction]
		if !ok {
			t.Fatalf("missing face %s", data.BlockFaceDirectionToString(direction))
		}
		if face.Texture != "#chest" {
			t.Fatalf("%s texture = %q, want #chest", data.BlockFaceDirectionToString(direction), face.Texture)
		}
		if face.Uv == nil {
			t.Fatalf("%s uv = nil, want %+v", data.BlockFaceDirectionToString(direction), want.uv)
		}
		if *face.Uv != want.uv {
			t.Fatalf("%s uv = %+v, want %+v", data.BlockFaceDirectionToString(direction), *face.Uv, want.uv)
		}
		if !want.hasRotation {
			if face.Rotation != nil {
				t.Fatalf("%s rotation = %d, want nil", data.BlockFaceDirectionToString(direction), *face.Rotation)
			}
			continue
		}
		if face.Rotation == nil {
			t.Fatalf("%s rotation = nil, want %d", data.BlockFaceDirectionToString(direction), want.rotation)
		}
		if *face.Rotation != want.rotation {
			t.Fatalf("%s rotation = %d, want %d", data.BlockFaceDirectionToString(direction), *face.Rotation, want.rotation)
		}
	}
}

func renderGenericChestItemForRegression(t *testing.T, renderer *MinecraftBlockRenderer, size int) *image.RGBA {
	t.Helper()

	templateChest := "item/template_chest"
	template := renderer.ResolveModelOrNull(&templateChest)
	if template == nil {
		t.Fatal("template_chest model was not resolved")
	}

	display := renderer.CloneDisplayDictionary(template)
	modelName := "generic_default_uv_chest"
	model := data.BlockModelInstance{
		Name: modelName,
		Textures: map[string]string{
			"chest":    "minecraft:entity/chest/normal",
			"particle": "minecraft:entity/chest/normal",
		},
		Display: display,
		Elements: []data.ModelElement{
			genericChestCuboid(data.Vector3{X: 1, Y: 0, Z: 1}, data.Vector3{X: 15, Y: 10, Z: 15}),
			genericChestCuboid(data.Vector3{X: 1, Y: 10, Z: 1}, data.Vector3{X: 15, Y: 15, Z: 15}),
			genericChestCuboid(data.Vector3{X: 7, Y: 7, Z: 0}, data.Vector3{X: 9, Y: 11, Z: 1}),
		},
	}

	options := MergeBlockRenderOptions(&BlockRenderOptions{Size: size})
	if guiTransform, ok := display["gui"]; ok {
		options.OverrideGuiTransform = guiTransform
	}

	rendered := renderer.RenderModel(&model, options, &modelName)
	if rendered == nil || !hasOpaquePixels(rendered) {
		t.Fatal("generic chest render did not produce visible pixels")
	}
	return rendered
}

func genericChestCuboid(from data.Vector3, to data.Vector3) data.ModelElement {
	return data.ModelElement{
		From:  from,
		To:    to,
		Shade: true,
		Faces: map[data.BlockFaceDirection]data.ModelFace{
			data.North: {Texture: "#chest"},
			data.South: {Texture: "#chest"},
			data.East:  {Texture: "#chest"},
			data.West:  {Texture: "#chest"},
			data.Up:    {Texture: "#chest"},
			data.Down:  {Texture: "#chest"},
		},
	}
}

func writePatternedChestPNG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 18, G: 18, B: 18, A: 255})
		}
	}

	fillUVRect(img, data.Vector4{X: 3.5, Y: 4.75, Z: 7, W: 8.25}, color.RGBA{R: 235, G: 30, B: 40, A: 255})
	fillUVRect(img, data.Vector4{X: 10.5, Y: 8.25, Z: 14, W: 10.75}, color.RGBA{R: 35, G: 170, B: 240, A: 255})
	fillUVRect(img, data.Vector4{X: 0, Y: 8.25, Z: 3.5, W: 10.75}, color.RGBA{R: 245, G: 180, B: 35, A: 255})
	fillUVRect(img, data.Vector4{X: 3.5, Y: 8.25, Z: 7, W: 10.75}, color.RGBA{R: 30, G: 210, B: 90, A: 255})
	fillUVRect(img, data.Vector4{X: 7, Y: 8.25, Z: 10.5, W: 10.75}, color.RGBA{R: 180, G: 80, B: 230, A: 255})
	fillUVRect(img, data.Vector4{X: 10.5, Y: 3.75, Z: 14, W: 4.75}, color.RGBA{R: 255, G: 95, B: 110, A: 255})
	fillUVRect(img, data.Vector4{X: 0, Y: 3.75, Z: 3.5, W: 4.75}, color.RGBA{R: 70, G: 220, B: 255, A: 255})
	fillUVRect(img, data.Vector4{X: 3.5, Y: 3.75, Z: 7, W: 4.75}, color.RGBA{R: 80, G: 255, B: 145, A: 255})
	fillUVRect(img, data.Vector4{X: 7, Y: 3.75, Z: 10.5, W: 4.75}, color.RGBA{R: 230, G: 125, B: 255, A: 255})
	fillUVRect(img, data.Vector4{X: 0.25, Y: 0, Z: 0.75, W: 0.25}, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	fillUVRect(img, data.Vector4{X: 0.75, Y: 0, Z: 1.25, W: 0.25}, color.RGBA{R: 210, G: 210, B: 210, A: 255})
	fillUVRect(img, data.Vector4{X: 1, Y: 0.25, Z: 1.5, W: 1.25}, color.RGBA{R: 255, G: 240, B: 80, A: 255})
	fillUVRect(img, data.Vector4{X: 0.75, Y: 0.25, Z: 1, W: 1.25}, color.RGBA{R: 170, G: 170, B: 170, A: 255})
	fillUVRect(img, data.Vector4{X: 0, Y: 0.25, Z: 0.25, W: 1.25}, color.RGBA{R: 120, G: 120, B: 120, A: 255})

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func fillUVRect(img *image.RGBA, uv data.Vector4, c color.RGBA) {
	x0 := int(uv.X * 4)
	y0 := int(uv.Y * 4)
	x1 := int(uv.Z * 4)
	y1 := int(uv.W * 4)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func approxColorBounds(img *image.RGBA, expected color.RGBA, tolerance uint8) (minX, maxX, minY, maxY int, ok bool) {
	if img == nil {
		return 0, 0, 0, 0, false
	}

	bounds := img.Bounds()
	minX = bounds.Max.X
	minY = bounds.Max.Y
	maxX = bounds.Min.X
	maxY = bounds.Min.Y
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.A <= 10 {
				continue
			}
			if channelWithinTolerance(p.R, expected.R, tolerance) &&
				channelWithinTolerance(p.G, expected.G, tolerance) &&
				channelWithinTolerance(p.B, expected.B, tolerance) {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
				ok = true
			}
		}
	}
	return minX, maxX, minY, maxY, ok
}
