package imagecache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"strings"

	"github.com/HugoSmits86/nativewebp"
)

const CacheFormatVersion = "8"

func WriteWebPAtomic(targetPath string, img image.Image) error {
	if img == nil {
		return fmt.Errorf("image is nil")
	}
	encodedImage := imageForWebP(img)
	return writeAtomic(targetPath, func(file *os.File) error {
		return nativewebp.Encode(file, encodedImage, &nativewebp.Options{
			CompressionLevel: nativewebp.DefaultCompression,
		})
	})
}

func WriteAnimatedWebPAtomic(targetPath string, frames []image.Image, durations []uint) error {
	if len(frames) == 0 {
		return fmt.Errorf("animated webp requires at least one frame")
	}
	if len(frames) == 1 {
		return WriteWebPAtomic(targetPath, frames[0])
	}

	normalizedFrames := make([]image.Image, len(frames))
	normalizedDurations := make([]uint, len(frames))
	disposals := make([]uint, len(frames))
	for i := range frames {
		if frames[i] == nil {
			return fmt.Errorf("animated webp frame %d is nil", i)
		}
		normalizedFrames[i] = imageForWebP(frames[i])
		if i < len(durations) && durations[i] > 0 {
			normalizedDurations[i] = durations[i]
		} else {
			normalizedDurations[i] = 50
		}
	}

	animation := nativewebp.Animation{
		Images:          normalizedFrames,
		Durations:       normalizedDurations,
		Disposals:       disposals,
		LoopCount:       0,
		BackgroundColor: 0x00000000,
	}

	return writeAtomic(targetPath, func(file *os.File) error {
		return nativewebp.EncodeAll(file, &animation, &nativewebp.Options{
			CompressionLevel: nativewebp.DefaultCompression,
		})
	})
}

func ReadRGBA(path string) (*image.RGBA, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoded, err := nativewebp.Decode(file)
	if err != nil {
		return nil, err
	}

	bounds := decoded.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgba, rgba.Bounds(), decoded, bounds.Min, draw.Src)
	return rgba, nil
}

func KeyedPath(root string, category string, namespace string, key string) string {
	return filepath.Join(root, sanitizeCategory(category), HashKey(namespace, key)+".webp")
}

func KeyedDir(root string, category string, namespace string, key string) string {
	return filepath.Join(root, sanitizeCategory(category), HashKey(namespace, key))
}

func HashKey(namespace string, key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(namespace) + "|" + key))
	return hex.EncodeToString(sum[:])
}

func EnsureCacheVersion(root string, version string, managedCategories ...string) error {
	root = strings.TrimSpace(root)
	version = strings.TrimSpace(version)
	if root == "" {
		return fmt.Errorf("cache root is empty")
	}
	if version == "" {
		return fmt.Errorf("cache version is empty")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	versionPath := filepath.Join(root, ".renderer-cache-version")
	if data, err := os.ReadFile(versionPath); err == nil && strings.TrimSpace(string(data)) == version {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	for _, category := range managedCategories {
		category = strings.TrimSpace(category)
		if category == "" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, sanitizeCategory(category))); err != nil {
			return err
		}
	}

	return writeAtomic(versionPath, func(file *os.File) error {
		_, err := file.WriteString(version + "\n")
		return err
	})
}

func imageForWebP(img image.Image) image.Image {
	if typed, ok := img.(*image.NRGBA); ok && typed.Bounds().Min == image.Pt(0, 0) {
		return typed
	}

	bounds := img.Bounds()
	nrgba := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(nrgba, nrgba.Bounds(), img, bounds.Min, draw.Src)
	return nrgba
}

func writeAtomic(targetPath string, write func(*os.File) error) error {
	if strings.TrimSpace(targetPath) == "" {
		return fmt.Errorf("target path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".imagecache-*.webp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := write(tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return err
	}
	removeTmp = false
	return nil
}

func sanitizeCategory(category string) string {
	normalized := strings.Trim(filepath.ToSlash(strings.TrimSpace(category)), "/")
	if normalized == "" || normalized == "." {
		return "default"
	}
	parts := strings.Split(normalized, "/")
	out := parts[:0]
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return "default"
	}
	return filepath.Join(out...)
}
