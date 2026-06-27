package imagecache

import (
	"image"
	"image/color"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HugoSmits86/nativewebp"
)

func TestWriteWebPAtomicPreservesPremultipliedRGBAColors(t *testing.T) {
	path := t.TempDir() + "/premultiplied-rgba.webp"
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 40, G: 180, B: 220, A: 128})

	if err := WriteWebPAtomic(path, img); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	decoded, err := nativewebp.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	got := color.NRGBAModel.Convert(decoded.At(0, 0)).(color.NRGBA)
	want := color.NRGBA{R: 40, G: 180, B: 220, A: 128}
	if !closeNRGBA(got, want, 1) {
		t.Fatalf("decoded color = %#v, want %#v", got, want)
	}
}

func TestWriteWebPAtomicPreservesColorsWithTransparentBackground(t *testing.T) {
	path := t.TempDir() + "/transparent-background.webp"
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 4; y < 12; y++ {
		for x := 4; x < 12; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 40, G: 180, B: 220, A: 255})
		}
	}

	if err := WriteWebPAtomic(path, img); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	decoded, err := nativewebp.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	got := color.NRGBAModel.Convert(decoded.At(8, 8)).(color.NRGBA)
	want := color.NRGBA{R: 40, G: 180, B: 220, A: 255}
	if got != want {
		t.Fatalf("decoded center color = %#v, want %#v", got, want)
	}
	transparent := color.NRGBAModel.Convert(decoded.At(0, 0)).(color.NRGBA)
	if transparent.A != 0 {
		t.Fatalf("decoded transparent color = %#v, want alpha 0", transparent)
	}
}

func TestWriteWebPAtomicPreservesColorsForExternalDecoders(t *testing.T) {
	if _, err := exec.LookPath("magick"); err != nil {
		t.Skip("magick not available")
	}

	dir := t.TempDir()
	webpPath := filepath.Join(dir, "transparent-background.webp")
	pngPath := filepath.Join(dir, "decoded.png")
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 4; y < 12; y++ {
		for x := 4; x < 12; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 40, G: 180, B: 220, A: 255})
		}
	}

	if err := WriteWebPAtomic(webpPath, img); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("magick", webpPath, pngPath).CombinedOutput(); err != nil {
		t.Fatalf("magick decode failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}

	file, err := os.Open(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	got := color.NRGBAModel.Convert(decoded.At(8, 8)).(color.NRGBA)
	want := color.NRGBA{R: 40, G: 180, B: 220, A: 255}
	if got != want {
		t.Fatalf("external decoded center color = %#v, want %#v", got, want)
	}
}

func TestWriteWebPAtomicPreservesSemiTransparentRGBAColorsForExternalDecoders(t *testing.T) {
	if _, err := exec.LookPath("magick"); err != nil {
		t.Skip("magick not available")
	}

	dir := t.TempDir()
	webpPath := filepath.Join(dir, "semi-transparent.webp")
	pngPath := filepath.Join(dir, "decoded.png")
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 4; y < 12; y++ {
		for x := 4; x < 12; x++ {
			img.Set(x, y, color.NRGBA{R: 40, G: 180, B: 220, A: 128})
		}
	}

	if err := WriteWebPAtomic(webpPath, img); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("magick", webpPath, pngPath).CombinedOutput(); err != nil {
		t.Fatalf("magick decode failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}

	file, err := os.Open(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	got := color.NRGBAModel.Convert(decoded.At(8, 8)).(color.NRGBA)
	want := color.NRGBA{R: 40, G: 180, B: 220, A: 128}
	if !closeNRGBA(got, want, 1) {
		t.Fatalf("external decoded semi-transparent color = %#v, want %#v", got, want)
	}
}

func TestWriteAnimatedWebPAtomicPreservesColorsForExternalDecoders(t *testing.T) {
	if _, err := exec.LookPath("magick"); err != nil {
		t.Skip("magick not available")
	}

	dir := t.TempDir()
	webpPath := filepath.Join(dir, "animated.webp")
	pngPath := filepath.Join(dir, "decoded.png")
	first := image.NewRGBA(image.Rect(0, 0, 16, 16))
	second := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 4; y < 12; y++ {
		for x := 4; x < 12; x++ {
			first.SetRGBA(x, y, color.RGBA{R: 40, G: 180, B: 220, A: 255})
			second.SetRGBA(x, y, color.RGBA{R: 220, G: 40, B: 80, A: 255})
		}
	}

	if err := WriteAnimatedWebPAtomic(webpPath, []image.Image{first, second}, []uint{50, 50}); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("magick", webpPath+"[0]", pngPath).CombinedOutput(); err != nil {
		t.Fatalf("magick decode failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}

	file, err := os.Open(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	got := color.NRGBAModel.Convert(decoded.At(8, 8)).(color.NRGBA)
	want := color.NRGBA{R: 40, G: 180, B: 220, A: 255}
	if got != want {
		t.Fatalf("external decoded animated center color = %#v, want %#v", got, want)
	}
}

func TestEnsureCacheVersionPurgesManagedCategoriesOnVersionChange(t *testing.T) {
	root := t.TempDir()
	managedPath := filepath.Join(root, "rendered", "old.webp")
	unmanagedPath := filepath.Join(root, "other", "keep.txt")
	writeTestFile(t, managedPath, "old")
	writeTestFile(t, unmanagedPath, "keep")

	if err := EnsureCacheVersion(root, "test-v1", "rendered", "derived", "player_skins"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(managedPath); !os.IsNotExist(err) {
		t.Fatalf("managed cache file was not purged, err=%v", err)
	}
	if _, err := os.Stat(unmanagedPath); err != nil {
		t.Fatalf("unmanaged file was removed: %v", err)
	}

	currentPath := filepath.Join(root, "rendered", "current.webp")
	writeTestFile(t, currentPath, "current")
	if err := EnsureCacheVersion(root, "test-v1", "rendered", "derived", "player_skins"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(currentPath); err != nil {
		t.Fatalf("matching cache version purged current file: %v", err)
	}

	if err := EnsureCacheVersion(root, "test-v2", "rendered", "derived", "player_skins"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(currentPath); !os.IsNotExist(err) {
		t.Fatalf("version change did not purge managed cache file, err=%v", err)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func closeNRGBA(got color.NRGBA, want color.NRGBA, tolerance int) bool {
	return channelDelta(got.R, want.R) <= tolerance &&
		channelDelta(got.G, want.G) <= tolerance &&
		channelDelta(got.B, want.B) <= tolerance &&
		channelDelta(got.A, want.A) <= tolerance
}

func channelDelta(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
