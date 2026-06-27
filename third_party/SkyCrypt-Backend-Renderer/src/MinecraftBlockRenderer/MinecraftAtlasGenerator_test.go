package minecraftblockrenderer

import "testing"

func TestGenerateItemAtlasProducesImagesAndManifest(t *testing.T) {
	renderer := CreateFromMinecraftAssets(requireFullAssets(t), nil, nil)
	generator := NewMinecraftAtlasGenerator(renderer)

	result, err := generator.GenerateItemAtlas(MinecraftAtlasOptions{
		Names: []string{"missing_item", "diamond_sword"},
		Size:  32,
		Cols:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Image.Bounds().Dx() != 32 || result.Image.Bounds().Dy() != 64 {
		t.Fatalf("atlas size = %dx%d, want 32x64", result.Image.Bounds().Dx(), result.Image.Bounds().Dy())
	}
	if result.Manifest.Width != 32 || result.Manifest.Height != 64 {
		t.Fatalf("manifest size = %dx%d, want 32x64", result.Manifest.Width, result.Manifest.Height)
	}
	if len(result.Manifest.Entries) != 2 {
		t.Fatalf("manifest entries = %d, want 2", len(result.Manifest.Entries))
	}
	if result.Manifest.Entries[0].Name != "diamond_sword" || result.Manifest.Entries[1].Name != "missing_item" {
		t.Fatalf("unexpected sorted entry order: %#v", result.Manifest.Entries)
	}
	if result.Manifest.Entries[0].ResourceId == "" {
		t.Fatal("successful atlas entry is missing resource id")
	}
	if !hasOpaquePixels(result.Image) {
		t.Fatal("atlas image did not contain opaque pixels")
	}
}
