package lib

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/png"
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

// The golden hash was taken from the At()/Set() implementation before warpImage read pixels directly.
func TestTo3DHeadOutputUnchanged(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	skin := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for i := range skin.Pix {
		skin.Pix[i] = uint8(random.Intn(256))
	}
	// Some fully opaque and fully transparent pixels hit blendRGBA's fast paths.
	for y := 0; y < 8; y++ {
		skin.SetRGBA(8+y, 8, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		skin.SetRGBA(40+y, 8, color.RGBA{})
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, To3DHead(skin)); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf.Bytes())
	if got, want := hex.EncodeToString(sum[:]), "b53f45f0ce7e9388a70ae44a4585f1edf1ce2785072b1651fff39fa76f209685"; got != want {
		t.Fatalf("head PNG hash = %s, want %s", got, want)
	}
}
