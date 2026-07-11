package routes

import (
	"io"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestParseEnabledPacksFormats(t *testing.T) {
	tests := map[string][]string{
		"['FSR', 'HYPIXEL_PLUS']":              {"FSR", "HYPIXEL_PLUS"},
		`["FSR", "HYPIXEL_PLUS"]`:              {"FSR", "HYPIXEL_PLUS"},
		"FSR,HYPIXEL_PLUS":                     {"FSR", "HYPIXEL_PLUS"},
		"%5B%22FSR%22%2C%22HYPIXEL_PLUS%22%5D": {"FSR", "HYPIXEL_PLUS"},
		"%5B%22HYPIXEL_PACK%22%2C%22FSR%22%5D": {"HYPIXEL_PACK", "FSR"},
	}

	for input, want := range tests {
		if got := parseEnabledPacks(input); !reflect.DeepEqual(got, want) {
			t.Errorf("parseEnabledPacks(%q) = %#v, want %#v", input, got, want)
		}
	}
}

func TestEncodedEnabledPacksCookie(t *testing.T) {
	t.Setenv("DEV", "false")
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(enabledPacksCachePart(enabledPacksFromRequest(c))) })

	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Cookie", "enabledPacks=%5B%22HYPIXEL_PACK%22%2C%22HYPIXEL_PLUS%22%2C%22FSR%22%5D")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "enabled-v7:HYPIXEL_PACK,HYPIXEL_PLUS,FSR" {
		t.Fatalf("enabled packs = %q", body)
	}
	if got := response.Header.Get(fiber.HeaderVary); got != fiber.HeaderCookie {
		t.Fatalf("Vary = %q", got)
	}
	if got := response.Header.Get(fiber.HeaderCacheControl); got != "private, no-cache" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestEnabledPacksFromRequestDefaultsAndIgnoresLegacyCookie(t *testing.T) {
	t.Setenv("DEV", "false")
	want := enabledPacksCachePart(nil)
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(enabledPacksCachePart(enabledPacksFromRequest(c))) })

	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Cookie", "disabledPacks=[\"FSR\"]")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Fatalf("enabled packs = %q, want defaults %q", body, want)
	}
}

func TestEnabledPacksFromDevelopmentQuery(t *testing.T) {
	t.Setenv("DEV", "true")
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(enabledPacksCachePart(enabledPacksFromRequest(c))) })

	request := httptest.NewRequest("GET", "/?enabledPacks=FSR,HPLUS", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "enabled-v7:FSR,HYPIXEL_PLUS" {
		t.Fatalf("enabled packs = %q", body)
	}
}

func TestEnabledPacksCookiePreservesPriorityOrder(t *testing.T) {
	t.Setenv("DEV", "false")
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(enabledPacksCachePart(enabledPacksFromRequest(c))) })

	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Cookie", "enabledPacks=['FSR','HPLUS','HYPIXEL_PACK']")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "enabled-v7:FSR,HYPIXEL_PLUS,HYPIXEL_PACK" {
		t.Fatalf("enabled packs = %q", body)
	}
}

func TestResolverEnabledPacksQueryOverridesCookie(t *testing.T) {
	t.Setenv("DEV", "false")
	app := fiber.New()
	app.Get("/api/item/:itemId/resolve", func(c *fiber.Ctx) error {
		return c.SendString(enabledPacksCachePart(enabledPacksFromRequest(c)))
	})

	request := httptest.NewRequest("GET", "/api/item/HYPERION/resolve?enabledPacks=HYPIXEL_PACK,FSR", nil)
	request.Header.Set("Cookie", `enabledPacks=["HYPIXEL_PLUS","FSR"]`)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "enabled-v7:HYPIXEL_PACK,FSR" {
		t.Fatalf("enabled packs = %q", body)
	}
}
