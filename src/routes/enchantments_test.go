package routes

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"reflect"
	"testing"

	notenoughupdates "skycrypt/src/NotEnoughUpdates"

	"github.com/gofiber/fiber/v3"
)

func TestEnchantmentsConstantsHandler(t *testing.T) {
	app := fiber.New()
	NewAPI(app, "")

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
	want := notenoughupdates.Constants().MaxEnchants
	if want == nil {
		want = []string{}
	}
	if got == nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("enchantments = %#v, want %#v", got, want)
	}
}
