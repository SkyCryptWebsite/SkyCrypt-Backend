package routes

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"reflect"
	"testing"

	"skycrypt/src/constants"

	"github.com/gofiber/fiber/v2"
)

func TestEnchantmentsConstantsHandler(t *testing.T) {
	app := fiber.New()
	app.Get("/api/constants/enchantments", EnchantmentsConstantsHandler)

	response, err := app.Test(httptest.NewRequest("GET", "/api/constants/enchantments", nil))
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

	var got []string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !reflect.DeepEqual(got, constants.MAX_ENCHANTS) {
		t.Fatalf("enchantments = %#v, want %#v", got, constants.MAX_ENCHANTS)
	}
}
