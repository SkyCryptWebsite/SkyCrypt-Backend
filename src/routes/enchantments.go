package routes

import (
	"context"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

func registerEnchantments(humaAPI huma.API) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "listEnchantments",
		Method:      fiber.MethodGet,
		Path:        "/api/constants/enchantments",
		Summary:     "List max-level enchantments",
		Description: "Returns the lore strings for enchantments at their maximum level, read from the NEU repository.",
		Tags:        []string{"Constants"},
		Responses:   map[string]*huma.Response{"200": {Description: "Max-level enchantments returned successfully."}},
	}, func(context.Context, *struct{}) (*bodyOutput[[]string], error) {
		enchants := notenoughupdates.Constants().MaxEnchants
		if enchants == nil {
			enchants = []string{}
		}
		return &bodyOutput[[]string]{Body: enchants}, nil
	})
}
