package src

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestCacheStaticCacheControl(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"rendered/resourcepacks-manifest.json":       "public, no-cache",
		"rendered/item__pack=vanilla__hash=ABC.webp": "public, max-age=604800",
		"heads/abc.png": "public, max-age=604800",
	}
	for name := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	app := fiber.New()
	app.Static("/cache", dir, fiber.Static{MaxAge: 7 * 24 * 60 * 60, ModifyResponse: cacheStaticResponse})

	for name, want := range files {
		response, err := app.Test(httptest.NewRequest("GET", "/cache/"+name, nil))
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != fiber.StatusOK {
			t.Fatalf("%s: status %d", name, response.StatusCode)
		}
		if got := response.Header.Get(fiber.HeaderCacheControl); got != want {
			t.Errorf("%s: Cache-Control = %q, want %q", name, got, want)
		}
	}
}
