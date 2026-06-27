package minecraftblockrenderer

import (
	"encoding/base64"
	"strings"
	"testing"
)

func encodeTestTextureDescriptor(raw string) string {
	return strings.TrimRight(base64.StdEncoding.EncodeToString([]byte(raw)), "=")
}

func TestRenderGuiItemFromSkyblockTextureIdFallsBackToBaseItem(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	renderer := CreateFromMinecraftAssets(assetsRoot, nil, nil)
	options := &BlockRenderOptions{Size: 64}

	textureID := encodeTestTextureDescriptor("skyblock:fungi_cutter?base=minecraft:golden_hoe&numeric=294")
	actual, err := renderer.RenderGuiItemFromTextureId(textureID, options)
	if err != nil {
		t.Fatal(err)
	}
	expectedResource := renderer.RenderGuiItemWithResourceId("minecraft:golden_hoe", options)
	if expectedResource == nil || expectedResource.Image == nil {
		t.Fatal("expected fallback item render returned nil")
	}
	if !hasOpaquePixels(actual) {
		t.Fatal("fallback texture id render did not produce visible pixels")
	}
	if !imagesAreIdentical(expectedResource.Image, actual) {
		t.Fatal("skyblock texture id fallback does not match base item render")
	}
}

func TestRenderGuiItemFromPlainTextureIdStillRendersTexture(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	renderer := CreateFromMinecraftAssets(assetsRoot, nil, nil)

	img, err := renderer.RenderGuiItemFromTextureId("minecraft:item/diamond_sword", &BlockRenderOptions{Size: 64})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("image size = %dx%d, want 64x64", img.Bounds().Dx(), img.Bounds().Dy())
	}
	if !hasOpaquePixels(img) {
		t.Fatal("plain texture render did not produce visible pixels")
	}
}
