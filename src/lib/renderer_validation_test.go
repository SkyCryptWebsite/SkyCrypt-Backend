package lib

import (
	"bytes"
	"errors"
	"image"
	"math"
	"math/rand"
	"testing"
)

func TestRenderRejectsUnknownTextures(t *testing.T) {
	cases := map[string]func() error{
		"head not hex":   func() error { _, err := RenderHead("../../etc/passwd"); return err },
		"head too short": func() error { _, err := RenderHead("abc123"); return err },
		"head uppercase": func() error {
			_, err := RenderHead("93A1B830399AB432A5178FDAF3939B24BF25C724A66BE947296C503352BC380D")
			return err
		},
		"potion type":         func() error { _, err := RenderPotion("lingering", "ff0000"); return err },
		"potion color":        func() error { _, err := RenderPotion("splash", "red"); return err },
		"leather type":        func() error { _, err := RenderArmor("../x", "ff0000"); return err },
		"leather short color": func() error { _, err := RenderArmor("helmet", "fff"); return err },
	}
	for name, render := range cases {
		if err := render(); !errors.Is(err, ErrTextureNotFound) {
			t.Errorf("%s: err = %v, want ErrTextureNotFound", name, err)
		}
	}
}

// warpImageReference is warpImage as it was before it read pixels directly, kept to check the output is unchanged.
// The comparison runs on the same machine because the float32 math rounds differently across CPUs (FMA on arm64).
func warpImageReference(src *image.RGBA, matrix Matrix, outputSize uint32) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, int(outputSize), int(outputSize)))

	det := matrix[0]*(matrix[4]*matrix[8]-matrix[5]*matrix[7]) -
		matrix[1]*(matrix[3]*matrix[8]-matrix[5]*matrix[6]) +
		matrix[2]*(matrix[3]*matrix[7]-matrix[4]*matrix[6])

	if math.Abs(float64(det)) < 1e-10 {
		return dst
	}

	invMatrix := Matrix{
		(matrix[4]*matrix[8] - matrix[5]*matrix[7]) / det,
		(matrix[2]*matrix[7] - matrix[1]*matrix[8]) / det,
		(matrix[1]*matrix[5] - matrix[2]*matrix[4]) / det,
		(matrix[5]*matrix[6] - matrix[3]*matrix[8]) / det,
		(matrix[0]*matrix[8] - matrix[2]*matrix[6]) / det,
		(matrix[2]*matrix[3] - matrix[0]*matrix[5]) / det,
		(matrix[3]*matrix[7] - matrix[4]*matrix[6]) / det,
		(matrix[1]*matrix[6] - matrix[0]*matrix[7]) / det,
		(matrix[0]*matrix[4] - matrix[1]*matrix[3]) / det,
	}

	srcBounds := src.Bounds()

	for y := 0; y < int(outputSize); y++ {
		for x := 0; x < int(outputSize); x++ {
			srcPoint := invMatrix.Transform(Point2D{X: float32(x), Y: float32(y)})

			srcX := int(math.Round(float64(srcPoint.X)))
			srcY := int(math.Round(float64(srcPoint.Y)))

			if srcX >= srcBounds.Min.X && srcX < srcBounds.Max.X &&
				srcY >= srcBounds.Min.Y && srcY < srcBounds.Max.Y {
				dst.Set(x, y, src.At(srcX, srcY))
			}
		}
	}

	return dst
}

func TestWarpImageMatchesReference(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	src := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for i := range src.Pix {
		src.Pix[i] = uint8(random.Intn(256))
	}
	matrices := []Matrix{
		{1, 0, 0, 0, 1, 0, 0, 0, 1},
		{8, 0, 4, 0, 8, 2, 0, 0, 1},
		{6.928, -6.928, 60, 4, 4, 20, 0, 0, 1},
		{-5.5, 3.2, 90.7, 2.1, 7.3, -3.4, 0, 0, 1},
		{0, 0, 0, 0, 0, 0, 0, 0, 1}, // singular: both return an empty image
	}
	for i, m := range matrices {
		if got, want := warpImage(src, m, 128), warpImageReference(src, m, 128); !bytes.Equal(got.Pix, want.Pix) {
			t.Errorf("matrix %d: warpImage output differs from the reference", i)
		}
	}
	// To3DHead runs every warp the real renderer does; it must still produce an image.
	if head := To3DHead(src); head == nil || head.Bounds().Empty() {
		t.Fatal("To3DHead returned no image")
	}
}
