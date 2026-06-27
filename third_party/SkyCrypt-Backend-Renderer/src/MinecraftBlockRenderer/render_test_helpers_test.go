package minecraftblockrenderer

import (
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func testRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			t.Fatal("could not locate repository root")
		}
		cwd = parent
	}
}

func testAssetsRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(testRepoRoot(t), "packs", "assets", "minecraft")
}

func testTexturePacksRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(testRepoRoot(t), "texturepacks")
}

func requireFullAssets(t *testing.T) string {
	t.Helper()
	root := testAssetsRoot(t)
	required := []string{
		filepath.Join(root, "blockstates", "stone.json"),
		filepath.Join(root, "models", "block", "stone.json"),
		filepath.Join(root, "textures", "block", "stone.png"),
		filepath.Join(root, "items", "diamond_sword.json"),
	}
	for _, path := range required {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("full vanilla assets are not available: missing %s", path)
		}
	}
	return root
}

func requireTexturePack(t *testing.T, packName string) string {
	t.Helper()
	root := filepath.Join(testTexturePacksRoot(t), packName)
	if _, err := os.Stat(filepath.Join(root, "meta.json")); err != nil {
		t.Skipf("texture pack %q is not available at %s", packName, root)
	}
	return root
}

func hasOpaquePixels(img *image.RGBA) bool {
	if img == nil {
		return false
	}
	b := img.Bounds()
	stepX := maxInt(1, b.Dx()/64)
	stepY := maxInt(1, b.Dy()/64)
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			if img.RGBAAt(x, y).A > 10 {
				return true
			}
		}
	}
	return false
}

func imagesAreIdentical(a, b *image.RGBA) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Bounds().Dx() != b.Bounds().Dx() || a.Bounds().Dy() != b.Bounds().Dy() {
		return false
	}
	ab := a.Bounds()
	bb := b.Bounds()
	for y := 0; y < ab.Dy(); y++ {
		for x := 0; x < ab.Dx(); x++ {
			if a.RGBAAt(ab.Min.X+x, ab.Min.Y+y) != b.RGBAAt(bb.Min.X+x, bb.Min.Y+y) {
				return false
			}
		}
	}
	return true
}

func sampleOpaquePixel(t *testing.T, img *image.RGBA) color.RGBA {
	t.Helper()
	if img == nil {
		t.Fatal("image is nil")
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.A > 10 {
				return p
			}
		}
	}
	t.Fatal("image does not contain opaque pixels")
	return color.RGBA{}
}

func opaqueBounds(img *image.RGBA) (minX, maxX, minY, maxY int, ok bool) {
	if img == nil {
		return 0, 0, 0, 0, false
	}
	b := img.Bounds()
	minX, minY = math.MaxInt, math.MaxInt
	maxX, maxY = math.MinInt, math.MinInt
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.RGBAAt(x, y).A <= 10 {
				continue
			}
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
		}
	}
	return minX, maxX, minY, maxY, minX != math.MaxInt
}

func averageColor(img *image.RGBA) color.RGBA {
	if img == nil {
		return color.RGBA{}
	}
	var r, g, b, a, count uint64
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.A == 0 {
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

func imageContainsApproxColor(img *image.RGBA, expected color.RGBA, tolerance uint8) bool {
	if img == nil {
		return false
	}
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.A <= 10 {
				continue
			}
			if channelWithinTolerance(p.R, expected.R, tolerance) &&
				channelWithinTolerance(p.G, expected.G, tolerance) &&
				channelWithinTolerance(p.B, expected.B, tolerance) {
				return true
			}
		}
	}
	return false
}

func channelWithinTolerance(actual uint8, expected uint8, tolerance uint8) bool {
	if actual > expected {
		return actual-expected <= tolerance
	}
	return expected-actual <= tolerance
}

func lowerHalfLuminanceBySide(img *image.RGBA) (left, right uint64) {
	if img == nil {
		return 0, 0
	}

	bounds := img.Bounds()
	midX := bounds.Min.X + bounds.Dx()/2
	midY := bounds.Min.Y + bounds.Dy()/2
	for y := midY; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.A <= 10 {
				continue
			}
			luminance := uint64(p.R)*299 + uint64(p.G)*587 + uint64(p.B)*114
			if x < midX {
				left += luminance
			} else {
				right += luminance
			}
		}
	}
	return left, right
}

func pixelDistance(a, b color.RGBA) float64 {
	dr := float64(a.R) - float64(b.R)
	dg := float64(a.G) - float64(b.G)
	db := float64(a.B) - float64(b.B)
	da := float64(a.A) - float64(b.A)
	return math.Sqrt(dr*dr + dg*dg + db*db + da*da)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
