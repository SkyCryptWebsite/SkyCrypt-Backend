package routes

import (
	"context"
	"skycrypt/src/lib"
	"skycrypt/src/models"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

func registerResourcePacks(humaAPI huma.API) {
	responses := errorResponses(humaAPI, map[int]string{
		fiber.StatusInternalServerError: "INTERNAL_ERROR: Resource pack metadata could not be read.",
	})
	responses["200"] = &huma.Response{Description: "Resource packs returned successfully."}
	huma.Register(humaAPI, huma.Operation{
		OperationID: "listResourcePacks",
		Method:      fiber.MethodGet,
		Path:        "/api/constants/packs",
		Summary:     "List resource packs",
		Description: "Returns toggleable resource packs sorted by descending priority for the recommended default order.\n" +
			"The enabledPacks cookie controls per-request rendering from highest to lowest priority and does not change this response order.\n" +
			"A missing preference uses the default order, while an explicit empty array uses vanilla textures only.\n" +
			"The vanilla resource pack is intentionally omitted because it is the default and cannot be disabled.",
		Tags:      []string{"Rendering"},
		Responses: responses,
	}, func(context.Context, *struct{}) (*bodyOutput[[]models.ResourcePackConfig], error) {
		configs, err := lib.ResourcePackConfigs()
		if err != nil {
			return nil, NewAPIError(fiber.StatusInternalServerError, codeInternalError, "Failed to read resource packs directory")
		}
		return &bodyOutput[[]models.ResourcePackConfig]{Body: configs}, nil
	})
}
