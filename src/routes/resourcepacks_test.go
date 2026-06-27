package routes

import (
	"encoding/json"
	"net/http/httptest"
	"skycrypt/src/models"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestResourcePackHandlerReturnsMetaBackedPacks(t *testing.T) {
	previous := RESOURCE_PACKS
	RESOURCE_PACKS = nil
	t.Cleanup(func() {
		RESOURCE_PACKS = previous
	})

	app := fiber.New()
	app.Get("/api/resourcepacks", ResourcePackHandler)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/resourcepacks", nil))
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var packs []models.ResourcePackConfig
	if err := json.NewDecoder(resp.Body).Decode(&packs); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(packs) < 2 {
		t.Fatalf("got %d packs, want at least 2", len(packs))
	}
	if packs[0].Id != "FSR" || packs[0].Priority != 100 {
		t.Fatalf("first pack = %#v, want FSR priority 100", packs[0])
	}
	if packs[1].Id != "HYPIXEL_PLUS" || packs[1].Priority != 50 {
		t.Fatalf("second pack = %#v, want HYPIXEL_PLUS priority 50", packs[1])
	}
	if packs[0].Url == "" || packs[0].Author == "" {
		t.Fatalf("resource pack fields missing: %#v", packs[0])
	}
	for _, pack := range packs {
		if strings.Contains(pack.Icon, "FurSky_Reborn") || strings.Contains(pack.Icon, "Hypixel_Plus") {
			t.Fatalf("pack %s returned invalid metadata icon path %q", pack.Id, pack.Icon)
		}
	}
	if packs[0].Icon != "/assets/resourcepacks/FurSky%20Reborn/pack.png" {
		t.Fatalf("fsr icon = %q, want corrected assets path", packs[0].Icon)
	}
	if packs[1].Icon != "/assets/resourcepacks/Hypixel%20Plus/pack.png" {
		t.Fatalf("hplus icon = %q, want corrected assets path", packs[1].Icon)
	}
}
