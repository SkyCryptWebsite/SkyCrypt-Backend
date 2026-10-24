package routes

import (
	"context"
	"skycrypt/src/lib"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

func registerPotion(humaAPI huma.API) {
	responses := errorResponses(humaAPI, map[int]string{
		fiber.StatusNotFound:            "Potion type is not normal or splash, or color is not 6-digit hex.",
		fiber.StatusInternalServerError: "Potion image could not be rendered.",
	})
	responses["200"] = imageResponse("PNG image bytes returned successfully.", "image/png")
	huma.Register(humaAPI, huma.Operation{
		OperationID: "renderPotionImage",
		Method:      fiber.MethodGet,
		Path:        "/api/potion/{type}/{color}",
		Summary:     "Render a potion",
		Description: "Renders a potion image for the requested potion type and color.",
		Tags:        []string{"Rendering"},
		Responses:   responses,
	}, func(_ context.Context, in *struct {
		Type  string `path:"type" doc:"Potion type identifier" example:"splash"`
		Color string `path:"color" doc:"Potion color value" example:"ff0000"`
	}) (*imageOutput, error) {
		return renderedImage(lib.RenderPotion(in.Type, in.Color))
	})
}
