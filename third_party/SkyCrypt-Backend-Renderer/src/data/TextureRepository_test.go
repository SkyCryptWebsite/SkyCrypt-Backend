package data

import (
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/assets"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestAnimatedTextureUsesFirstFrameDimensions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "assets", "minecraft")
	texturePath := filepath.Join(root, "textures", "item", "clock.png")
	writeTextureRepoAnimatedPNG(t, texturePath)
	writeTextureRepoText(t, texturePath+".mcmeta", `{"animation":{"frametime":1,"frames":[0,1],"width":16,"height":16}}`)
	cacheDir := t.TempDir()

	provider, err := assets.NewDirectoryResourceProvider(filepath.Join(root, "textures"))
	if err != nil {
		t.Fatal(err)
	}
	registry := assets.NewAssetNamespaceRegistry()
	registry.AddNamespaceWithProvider("minecraft", filepath.Join(root, "textures"), "test", true, provider)
	repository := NewTextureRepository(filepath.Join(root, "textures"), nil, nil, *registry)
	repository.SetDiskCacheDirectory(cacheDir, "test")
	texture := repository.GetTexture("minecraft:item/clock")
	if texture.Bounds().Dx() != 16 || texture.Bounds().Dy() != 16 {
		t.Fatalf("animated first frame size = %dx%d, want 16x16", texture.Bounds().Dx(), texture.Bounds().Dy())
	}
	animation, ok := repository.GetAnimation("minecraft:item/clock")
	if !ok || len(animation.Frames) != 2 {
		t.Fatalf("animation frames = %#v, ok=%v; want 2 frames", animation, ok)
	}
	matches, err := filepath.Glob(filepath.Join(cacheDir, "animations", "*", "frame-000.webp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected animation frame cache file")
	}
	assertTextureRepoWebPFile(t, matches[0])
}

func TestAnimatedTextureInfersFramesWhenMcmetaFramesOmitted(t *testing.T) {
	root := filepath.Join(t.TempDir(), "assets", "minecraft")
	texturePath := filepath.Join(root, "textures", "item", "survivor_cube.png")
	writeTextureRepoAnimatedPNG(t, texturePath)
	writeTextureRepoText(t, texturePath+".mcmeta", `{"animation":{"frametime":1,"width":16,"height":16}}`)
	cacheDir := t.TempDir()

	provider, err := assets.NewDirectoryResourceProvider(filepath.Join(root, "textures"))
	if err != nil {
		t.Fatal(err)
	}
	registry := assets.NewAssetNamespaceRegistry()
	registry.AddNamespaceWithProvider("minecraft", filepath.Join(root, "textures"), "test", true, provider)
	repository := NewTextureRepository(filepath.Join(root, "textures"), nil, nil, *registry)
	repository.SetDiskCacheDirectory(cacheDir, "test")

	texture := repository.GetTexture("minecraft:item/survivor_cube")
	if texture.Bounds().Dx() != 16 || texture.Bounds().Dy() != 16 {
		t.Fatalf("animated first frame size = %dx%d, want 16x16", texture.Bounds().Dx(), texture.Bounds().Dy())
	}

	animation, ok := repository.GetAnimation("minecraft:item/survivor_cube")
	if !ok || len(animation.Frames) != 2 {
		t.Fatalf("animation frames = %#v, ok=%v; want 2 inferred frames", animation, ok)
	}
	first := color.RGBAModel.Convert(animation.Frames[0].Image.At(0, 0)).(color.RGBA)
	second := color.RGBAModel.Convert(animation.Frames[1].Image.At(0, 0)).(color.RGBA)
	if first != (color.RGBA{R: 255, A: 255}) || second != (color.RGBA{G: 255, A: 255}) {
		t.Fatalf("unexpected inferred frame colors: first=%#v second=%#v", first, second)
	}

	matches, err := filepath.Glob(filepath.Join(cacheDir, "animations", "*", "frame-*.webp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("cached animation frames = %d, want 2", len(matches))
	}
	for _, match := range matches {
		assertTextureRepoWebPFile(t, match)
	}
}

func TestTintedTextureWritesDerivedWebP(t *testing.T) {
	root := filepath.Join(t.TempDir(), "assets", "minecraft")
	textureRoot := filepath.Join(root, "textures")
	writeTextureRepoPNG(t, filepath.Join(textureRoot, "item", "dyed.png"), 1, 1, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	cacheDir := t.TempDir()

	provider, err := assets.NewDirectoryResourceProvider(textureRoot)
	if err != nil {
		t.Fatal(err)
	}
	registry := assets.NewAssetNamespaceRegistry()
	registry.AddNamespaceWithProvider("minecraft", textureRoot, "test", true, provider)
	repository := NewTextureRepository(textureRoot, nil, nil, *registry)
	repository.SetDiskCacheDirectory(cacheDir, "test")

	tinted := repository.GetTintedTexture("minecraft:item/dyed", color.RGBA{R: 255, G: 0, B: 0, A: 255}, 1, 1)
	if tinted == nil || tinted.Bounds().Dx() != 1 || tinted.Bounds().Dy() != 1 {
		t.Fatalf("unexpected tinted texture: %#v", tinted)
	}
	matches, err := filepath.Glob(filepath.Join(cacheDir, "textures", "*.webp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected derived tinted texture cache file")
	}
	assertTextureRepoWebPFile(t, matches[0])
}

func TestGeneratedArmorTrimWritesDerivedWebP(t *testing.T) {
	root := filepath.Join(t.TempDir(), "assets", "minecraft")
	textureRoot := filepath.Join(root, "textures")
	paletteColor := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	materialColor := color.RGBA{R: 120, G: 130, B: 140, A: 255}
	writeTextureRepoPNG(t, filepath.Join(textureRoot, "trims", "color_palettes", "trim_palette.png"), 1, 1, paletteColor)
	writeTextureRepoPNG(t, filepath.Join(textureRoot, "trims", "color_palettes", "diamond.png"), 1, 1, materialColor)
	writeTextureRepoPNG(t, filepath.Join(textureRoot, "trims", "items", "coast_trim.png"), 1, 1, paletteColor)
	cacheDir := t.TempDir()

	provider, err := assets.NewDirectoryResourceProvider(textureRoot)
	if err != nil {
		t.Fatal(err)
	}
	registry := assets.NewAssetNamespaceRegistry()
	registry.AddNamespaceWithProvider("minecraft", textureRoot, "test", true, provider)
	repository := NewTextureRepository(textureRoot, nil, nil, *registry)
	repository.SetDiskCacheDirectory(cacheDir, "test")

	generated, ok := repository.TryGenerateArmorTrimTexture("trims/items/coast_trim_diamond")
	if !ok {
		t.Fatal("expected generated trim texture")
	}
	got := color.RGBAModel.Convert(generated.At(0, 0)).(color.RGBA)
	if got != materialColor {
		t.Fatalf("generated trim color = %#v, want %#v", got, materialColor)
	}
	matches, err := filepath.Glob(filepath.Join(cacheDir, "textures", "*.webp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected derived generated trim cache file")
	}
	assertTextureRepoWebPFile(t, matches[0])
}

func writeTextureRepoText(t *testing.T, path string, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTextureRepoAnimatedPNG(t *testing.T, path string) {
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

func writeTextureRepoPNG(t *testing.T, path string, width int, height int, c color.RGBA) {
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

func assertTextureRepoWebPFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		t.Fatalf("file is not a webp: %s", path)
	}
}
