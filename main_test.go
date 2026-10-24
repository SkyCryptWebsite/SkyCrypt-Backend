package main

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skycrypt/src/forensics"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"go.uber.org/zap"
)

func TestRecoverHidesPanicDetail(t *testing.T) {
	// utility loads .env on init, which may enable forensics; its logger is only built in main.
	forensics.Logger = zap.NewNop()

	app := fiber.New()
	app.Use(newRecoverMiddleware())
	app.Get("/", func(fiber.Ctx) error { panic("secret panic detail") })

	res, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("status = %d, want 500", res.StatusCode)
	}
	if strings.Contains(string(body), "secret") || !strings.Contains(string(body), `"code":"INTERNAL_ERROR"`) {
		t.Errorf("body = %s", body)
	}
}

func TestAppConfigDecodesPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "item__hash=AB==.webp"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	app := fiber.New(newAppConfig())
	app.Get("/search/:searchParam", func(c fiber.Ctx) error { return c.SendString(c.Params("searchParam")) })
	app.Get("/cache*", static.New(dir))

	for path, want := range map[string]string{
		"/search/giant%20sword":             "giant sword",
		"/cache/item__hash%3DAB%3D%3D.webp": "x",
	} {
		res, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != fiber.StatusOK || string(body) != want {
			t.Errorf("%s: %d %q, want 200 %q", path, res.StatusCode, body, want)
		}
	}
}
