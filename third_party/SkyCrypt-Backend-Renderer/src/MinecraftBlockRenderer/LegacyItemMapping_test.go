package minecraftblockrenderer

import (
	"image/color"
	"testing"
)

func TestLegacyNumericItemIDDamageVariants(t *testing.T) {
	tests := []struct {
		id     int
		damage int
		want   string
	}{
		{54, 0, "chest"},
		{130, 0, "ender_chest"},
		{146, 0, "trapped_chest"},
	}

	for _, test := range tests {
		got, ok := legacyNumericItemID(test.id, test.damage)
		if !ok || got != test.want {
			t.Fatalf("legacyNumericItemID(%d, %d) = %q, %v; want %q, true", test.id, test.damage, got, ok, test.want)
		}
	}

	wool := []string{
		"white_wool", "orange_wool", "magenta_wool", "light_blue_wool",
		"yellow_wool", "lime_wool", "pink_wool", "gray_wool",
		"light_gray_wool", "cyan_wool", "purple_wool", "blue_wool",
		"brown_wool", "green_wool", "red_wool", "black_wool",
	}
	for damage, want := range wool {
		got, ok := legacyNumericItemID(35, damage)
		if !ok || got != want {
			t.Fatalf("legacyNumericItemID(35, %d) = %q, %v; want %q, true", damage, got, ok, want)
		}
	}

	dyes := []string{
		"ink_sac", "red_dye", "green_dye", "cocoa_beans",
		"lapis_lazuli", "purple_dye", "cyan_dye", "light_gray_dye",
		"gray_dye", "pink_dye", "lime_dye", "yellow_dye",
		"light_blue_dye", "magenta_dye", "orange_dye", "bone_meal",
	}
	for damage, want := range dyes {
		got, ok := legacyNumericItemID(351, damage)
		if !ok || got != want {
			t.Fatalf("legacyNumericItemID(351, %d) = %q, %v; want %q, true", damage, got, ok, want)
		}
	}
}

func TestLegacyStringItemIDDamageVariants(t *testing.T) {
	tests := []struct {
		id     string
		damage int
		want   string
	}{
		{"INK_SACK:10", 0, "lime_dye"},
		{"INK_SACK-10", 0, "lime_dye"},
		{"minecraft:dye", 10, "lime_dye"},
		{"minecraft:ink_sack", 10, "lime_dye"},
		{"54", 0, "chest"},
		{"130", 0, "ender_chest"},
		{"146", 0, "trapped_chest"},
	}

	for _, test := range tests {
		got, ok := legacyStringItemID(test.id, test.damage)
		if !ok || got != test.want {
			t.Fatalf("legacyStringItemID(%q, %d) = %q, %v; want %q, true", test.id, test.damage, got, ok, test.want)
		}
	}
}

func TestRenderItemObjectNormalizesLegacyDyeString(t *testing.T) {
	renderer, err := CreateFromDataDirectory(createMinimalAssets(t))
	if err != nil {
		t.Fatal(err)
	}

	rendered, err := renderer.RenderItemObjectWithResourceId(map[string]any{
		"id":     "minecraft:dye",
		"damage": 10,
	}, &BlockRenderOptions{Size: 32})
	if err != nil {
		t.Fatal(err)
	}
	if rendered == nil || rendered.Image == nil || !hasOpaquePixels(rendered.Image) {
		t.Fatal("legacy dye render did not produce visible pixels")
	}
	if !contains(rendered.ResourceId.Textures, "minecraft:item/lime_dye") {
		t.Fatalf("resource textures = %v, want lime dye texture", rendered.ResourceId.Textures)
	}
	if !imageContainsApproxColor(rendered.Image, color.RGBA{R: 90, G: 220, B: 60, A: 255}, 5) {
		t.Fatal("legacy dye render does not contain the lime dye test texture")
	}
}
