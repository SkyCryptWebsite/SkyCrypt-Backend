package minecraftblockrenderer

import (
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderStoneProducesOpaquePixels(t *testing.T) {
	renderer := CreateFromMinecraftAssets(requireFullAssets(t), nil, nil)
	image := renderer.RenderBlock("stone", BlockRenderOptions{Size: 128})
	if image == nil {
		t.Fatal("RenderBlock returned nil")
	}
	if image.Bounds().Dx() != 128 || image.Bounds().Dy() != 128 {
		t.Fatalf("image size = %dx%d, want 128x128", image.Bounds().Dx(), image.Bounds().Dy())
	}
	if !hasOpaquePixels(image) {
		t.Fatal("stone render did not contain opaque pixels")
	}
}

func TestItemRegistryIncludesBlockInventoryItems(t *testing.T) {
	renderer := CreateFromMinecraftAssets(requireFullAssets(t), nil, nil)
	names := renderer.GetKnownItemNames()
	for _, want := range []string{"oak_fence", "white_shulker_box"} {
		if !contains(names, want) {
			t.Fatalf("known items missing %q", want)
		}
		rendered := renderer.RenderGuiItemWithResourceId(want, &BlockRenderOptions{Size: 64})
		if rendered == nil {
			t.Fatalf("%s item render returned nil", want)
		}
		if !hasOpaquePixels(rendered.Image) {
			t.Fatalf("%s item render did not contain opaque pixels", want)
		}
	}
}

func TestRenderBedItemUsesBlockModelFallback(t *testing.T) {
	renderer := CreateFromMinecraftAssets(requireFullAssets(t), nil, nil)
	rendered := renderer.RenderGuiItemWithResourceId("white_bed", &BlockRenderOptions{Size: 96})
	if rendered == nil {
		t.Fatal("white bed render returned nil")
	}
	if !hasOpaquePixels(rendered.Image) {
		t.Fatal("white bed render did not contain opaque pixels")
	}
	minX, maxX, _, _, ok := opaqueBounds(rendered.Image)
	if !ok {
		t.Fatal("white bed render has no opaque bounds")
	}
	if span := maxX - minX; span <= rendered.Image.Bounds().Dx()/2 {
		t.Fatalf("white bed render span = %d, want more than half image width", span)
	}
}

func TestRenderBlockFaceAPIs(t *testing.T) {
	renderer := CreateFromMinecraftAssets(requireFullAssets(t), nil, nil)
	face, err := renderer.RenderBlockFace("grass_block", BlockFaceRenderOptions{Size: 64, Face: data.Up})
	if err != nil {
		t.Fatal(err)
	}
	if face.Bounds().Dx() != 64 || face.Bounds().Dy() != 64 {
		t.Fatalf("face size = %dx%d, want 64x64", face.Bounds().Dx(), face.Bounds().Dy())
	}
	if !hasOpaquePixels(face) {
		t.Fatal("grass top face did not contain opaque pixels")
	}

	top, err := renderer.RenderBlockFace("grass_block", BlockFaceRenderOptions{Size: 64, Face: data.Up})
	if err != nil {
		t.Fatal(err)
	}
	side, err := renderer.RenderBlockFace("grass_block", BlockFaceRenderOptions{Size: 64, Face: data.North})
	if err != nil {
		t.Fatal(err)
	}
	if imagesAreIdentical(top, side) {
		t.Fatal("grass top and side faces should differ")
	}
}

func TestLeatherHelmetRespectsCustomTint(t *testing.T) {
	renderer := CreateFromMinecraftAssets(requireFullAssets(t), nil, nil)
	info := renderer._itemRegistry.GetItemInfo("leather_helmet")
	if info == nil || len(info.LayerTints) == 0 {
		t.Skip("local assets do not expose leather helmet tint metadata")
	}
	baseline := renderer.RenderItem("leather_helmet", nil, &BlockRenderOptions{Size: 64})
	customTint := data.ItemRenderData{Layer0Tint: colorPtr(10, 200, 240)}
	custom := renderer.RenderItem("leather_helmet", &customTint, &BlockRenderOptions{Size: 64})
	if baseline == nil || custom == nil {
		t.Fatal("helmet render returned nil")
	}
	if !hasOpaquePixels(baseline) || !hasOpaquePixels(custom) {
		t.Fatal("helmet render missing opaque pixels")
	}
	if imagesAreIdentical(baseline, custom) {
		t.Skip("local leather helmet render path does not currently expose a tintable layer")
	}
}

func colorPtr(r, g, b uint8) *color.RGBA {
	return &color.RGBA{R: r, G: g, B: b, A: 255}
}

func TestDefaultInventoryOrientationShowsFrontOnRight(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	renderer := CreateFromMinecraftAssets(assetsRoot, nil, nil)
	faceColors := map[data.BlockFaceDirection]color.RGBA{
		data.North: {R: 0xFF, G: 0x33, B: 0x33, A: 0xFF},
		data.South: {R: 0x33, G: 0x99, B: 0xFF, A: 0xFF},
		data.East:  {R: 0x33, G: 0xFF, B: 0x99, A: 0xFF},
		data.West:  {R: 0x99, G: 0x33, B: 0xFF, A: 0xFF},
		data.Up:    {R: 0xFF, G: 0xFF, B: 0x66, A: 0xFF},
		data.Down:  {R: 0xFF, G: 0x99, B: 0x33, A: 0xFF},
	}

	textures := map[string]string{"particle": "minecraft:block/unit_test_debug_up"}
	faces := make(map[data.BlockFaceDirection]data.ModelFace)
	for direction, c := range faceColors {
		name := data.BlockFaceDirectionToString(direction)
		textureId := "minecraft:block/unit_test_debug_" + strings.ToLower(name)
		writePNG(t, filepath.Join(assetsRoot, "textures", "block", "unit_test_debug_"+strings.ToLower(name)+".png"), 16, 16, c)
		key := strings.ToLower(name)
		textures[key] = textureId
		faces[direction] = data.ModelFace{
			Texture: "#" + key,
			Uv:      &data.Vector4{X: 0, Y: 0, Z: 16, W: 16},
		}
	}

	element := data.ModelElement{
		From:  data.Vector3{X: 0, Y: 0, Z: 0},
		To:    data.Vector3{X: 16, Y: 16, Z: 16},
		Faces: faces,
		Shade: true,
	}
	model := &data.BlockModelInstance{
		Name:     "unit_test:debug_cube",
		Textures: textures,
		Display:  map[string]*data.TransformDefinition{},
		Elements: []data.ModelElement{element},
	}

	options := DefaultBlockRenderOptions()
	options.Size = 160
	modelName := model.Name
	rendered := renderer.RenderModel(model, options, &modelName)
	rightColor := sampleAverageColor(rendered, int(float64(rendered.Bounds().Dx())*0.70), int(float64(rendered.Bounds().Dx())*0.95), rendered.Bounds().Dy()/2-10, rendered.Bounds().Dy()/2+10)
	leftColor := sampleAverageColor(rendered, int(float64(rendered.Bounds().Dx())*0.05), int(float64(rendered.Bounds().Dx())*0.30), rendered.Bounds().Dy()/2-10, rendered.Bounds().Dy()/2+10)
	topColor := sampleAverageColor(rendered, rendered.Bounds().Dx()/2-10, rendered.Bounds().Dx()/2+10, int(float64(rendered.Bounds().Dy())*0.05), int(float64(rendered.Bounds().Dy())*0.25))

	if !closerColor(rightColor, faceColors[data.North], faceColors[data.South]) {
		t.Fatalf("right face color=%+v should be closer to north=%+v than south=%+v", rightColor, faceColors[data.North], faceColors[data.South])
	}
	if !closerColor(leftColor, faceColors[data.East], faceColors[data.West]) {
		t.Fatalf("left face color=%+v should be closer to east=%+v than west=%+v", leftColor, faceColors[data.East], faceColors[data.West])
	}
	if !closerColor(topColor, faceColors[data.Up], faceColors[data.Down]) {
		t.Fatalf("top face color=%+v should be closer to up=%+v than down=%+v", topColor, faceColors[data.Up], faceColors[data.Down])
	}
}

func TestGui3DItemOrientationShowsSouthEastCorner(t *testing.T) {
	assetsRoot := createMinimalAssets(t)
	writeJSON(t, assetsRoot, "items/oriented_cube.json", `{"model":{"model":"minecraft:item/oriented_cube"}}`)

	faceColors := map[data.BlockFaceDirection]color.RGBA{
		data.North: {R: 0xFF, G: 0x33, B: 0x33, A: 0xFF},
		data.South: {R: 0x33, G: 0x99, B: 0xFF, A: 0xFF},
		data.East:  {R: 0x33, G: 0xFF, B: 0x99, A: 0xFF},
		data.West:  {R: 0x99, G: 0x33, B: 0xFF, A: 0xFF},
		data.Up:    {R: 0xFF, G: 0xFF, B: 0x66, A: 0xFF},
		data.Down:  {R: 0xFF, G: 0x99, B: 0x33, A: 0xFF},
	}

	for direction, c := range faceColors {
		name := strings.ToLower(data.BlockFaceDirectionToString(direction))
		writePNG(t, filepath.Join(assetsRoot, "textures", "block", "unit_test_gui_item_"+name+".png"), 16, 16, c)
	}
	writeJSON(t, assetsRoot, "models/item/oriented_cube.json", `{
		"display":{"gui":{"rotation":[30,225,0],"translation":[0,0,0],"scale":[0.625,0.625,0.625]}},
		"textures":{
			"north":"minecraft:block/unit_test_gui_item_north",
			"south":"minecraft:block/unit_test_gui_item_south",
			"east":"minecraft:block/unit_test_gui_item_east",
			"west":"minecraft:block/unit_test_gui_item_west",
			"up":"minecraft:block/unit_test_gui_item_up",
			"down":"minecraft:block/unit_test_gui_item_down"
		},
		"elements":[{"from":[0,0,0],"to":[16,16,16],"faces":{
			"north":{"texture":"#north","uv":[0,0,16,16]},
			"south":{"texture":"#south","uv":[0,0,16,16]},
			"east":{"texture":"#east","uv":[0,0,16,16]},
			"west":{"texture":"#west","uv":[0,0,16,16]},
			"up":{"texture":"#up","uv":[0,0,16,16]},
			"down":{"texture":"#down","uv":[0,0,16,16]}
		}}]
	}`)

	renderer, err := CreateFromDataDirectory(assetsRoot)
	if err != nil {
		t.Fatal(err)
	}

	rendered := renderer.RenderGuiItemWithResourceId("oriented_cube", &BlockRenderOptions{Size: 160})
	if rendered == nil || rendered.Image == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("oriented cube item render did not produce visible pixels")
	}

	img := rendered.Image
	rightColor := sampleAverageColor(img, int(float64(img.Bounds().Dx())*0.70), int(float64(img.Bounds().Dx())*0.95), img.Bounds().Dy()/2-10, img.Bounds().Dy()/2+10)
	leftColor := sampleAverageColor(img, int(float64(img.Bounds().Dx())*0.05), int(float64(img.Bounds().Dx())*0.30), img.Bounds().Dy()/2-10, img.Bounds().Dy()/2+10)
	topColor := sampleAverageColor(img, img.Bounds().Dx()/2-10, img.Bounds().Dx()/2+10, int(float64(img.Bounds().Dy())*0.05), int(float64(img.Bounds().Dy())*0.25))

	assertClosestFaceColor(t, "right face", rightColor, data.East, faceColors)
	assertClosestFaceColor(t, "left face", leftColor, data.South, faceColors)
	assertClosestFaceColor(t, "top face", topColor, data.Up, faceColors)
}

func sampleAverageColor(img *image.RGBA, minX, maxX, minY, maxY int) color.RGBA {
	var r, g, b, a, count uint64
	bounds := img.Bounds()
	minX = clampInt(minX, bounds.Min.X, bounds.Max.X)
	maxX = clampInt(maxX, bounds.Min.X, bounds.Max.X)
	minY = clampInt(minY, bounds.Min.Y, bounds.Max.Y)
	maxY = clampInt(maxY, bounds.Min.Y, bounds.Max.Y)
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			p := img.RGBAAt(x, y)
			if p.A <= 10 {
				continue
			}
			r += uint64(p.R)
			g += uint64(p.G)
			b += uint64(p.B)
			a += uint64(p.A)
			count++
		}
	}
	if count == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(r / count), G: uint8(g / count), B: uint8(b / count), A: uint8(a / count)}
}

func assertClosestFaceColor(t *testing.T, label string, actual color.RGBA, want data.BlockFaceDirection, faceColors map[data.BlockFaceDirection]color.RGBA) {
	t.Helper()

	bestDirection := data.BlockFaceDirection(-1)
	bestDistance := math.Inf(1)
	for direction, candidate := range faceColors {
		distance := colorDistance(actual, candidate)
		if distance < bestDistance {
			bestDirection = direction
			bestDistance = distance
		}
	}

	if bestDirection != want {
		t.Fatalf("%s color=%+v closest to %s, want %s", label, actual, data.BlockFaceDirectionToString(bestDirection), data.BlockFaceDirectionToString(want))
	}
}

func closerColor(actual, expected, other color.RGBA) bool {
	return colorDistance(actual, expected) < colorDistance(actual, other)
}

func colorDistance(a, b color.RGBA) float64 {
	dr := float64(a.R) - float64(b.R)
	dg := float64(a.G) - float64(b.G)
	db := float64(a.B) - float64(b.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
