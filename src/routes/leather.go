package routes

import (
	"context"
	"errors"
	"fmt"
	"skycrypt/src/lib"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

func registerLeather(humaAPI huma.API) {
	responses := errorResponses(humaAPI, map[int]string{
		fiber.StatusNotFound:            "Armor piece is unknown or color is not 6-digit hex.",
		fiber.StatusInternalServerError: "Leather armor image could not be rendered.",
	})
	responses["200"] = imageResponse("PNG image bytes returned successfully.", "image/png")
	huma.Register(humaAPI, huma.Operation{
		OperationID: "renderLeatherArmorImage",
		Method:      fiber.MethodGet,
		Path:        "/api/leather/{type}/{color}",
		Summary:     "Render leather armor",
		Description: "Renders a dyed leather armor piece as PNG bytes for the requested armor piece and color.",
		Tags:        []string{"Rendering"},
		Responses:   responses,
	}, func(_ context.Context, in *struct {
		Type  string `path:"type" doc:"Leather armor piece identifier" example:"helmet"`
		Color string `path:"color" doc:"Armor color value" example:"ff0000"`
	}) (*imageOutput, error) {
		imageBytes, err := lib.RenderArmor(in.Type, in.Color)
		if err != nil && !errors.Is(err, lib.ErrTextureNotFound) {
			fmt.Printf("Error rendering armor: %v\n", err)
		}
		return renderedImage(imageBytes, err)
	})
}
