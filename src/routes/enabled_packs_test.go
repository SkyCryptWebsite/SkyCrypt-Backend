package routes

import (
	"context"
	"io"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
)

// packsTestApp answers GET path with the cache part of the packs that packs reads from the request's input.
func packsTestApp[I any](path string, packs func(*I) []string) *fiber.App {
	app := fiber.New()
	humaAPI := humafiber.New(app, huma.DefaultConfig("test", "1"))
	huma.Get(humaAPI, path, func(_ context.Context, in *I) (*struct{ Body []byte }, error) {
		return &struct{ Body []byte }{Body: []byte(enabledPacksCachePart(packs(in)))}, nil
	})
	return app
}

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
	// Handlers embed PacksInput, so check the cookie is still read through an embedded field.
	app := packsTestApp("/", func(in *struct{ PacksInput }) []string { return in.enabledPacks })

	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Cookie", "enabledPacks=%5B%22HYPIXEL_PACK%22%2C%22HYPIXEL_PLUS%22%2C%22FSR%22%5D")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	})
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

func TestEncodedEmptyEnabledPacksCookieUsesVanillaOnly(t *testing.T) {
	t.Setenv("DEV", "false")
	app := packsTestApp("/", func(in *PacksInput) []string { return in.enabledPacks })

	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Cookie", "enabledPacks=%5B%5D")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	})
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "enabled-v7:" {
		t.Fatalf("enabled packs = %q, want vanilla-only signature", body)
	}
}

func TestEnabledPacksFromRequestDefaultsAndIgnoresLegacyCookie(t *testing.T) {
	t.Setenv("DEV", "false")
	want := enabledPacksCachePart(nil)
	app := packsTestApp("/", func(in *PacksInput) []string { return in.enabledPacks })

	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Cookie", "disabledPacks=[\"FSR\"]")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	})
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
	app := packsTestApp("/", func(in *PacksInput) []string { return in.enabledPacks })

	request := httptest.NewRequest("GET", "/?enabledPacks=FSR,HPLUS", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	})
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
	app := packsTestApp("/", func(in *PacksInput) []string { return in.enabledPacks })

	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Cookie", "enabledPacks=['FSR','HPLUS','HYPIXEL_PACK']")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	})
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
	app := packsTestApp("/api/item/{itemId}/resolve", (*ResolveInput).packs)

	request := httptest.NewRequest("GET", "/api/item/HYPERION/resolve?enabledPacks=HYPIXEL_PACK,FSR", nil)
	request.Header.Set("Cookie", `enabledPacks=["HYPIXEL_PLUS","FSR"]`)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	})
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "enabled-v7:HYPIXEL_PACK,FSR" {
		t.Fatalf("enabled packs = %q", body)
	}
}
