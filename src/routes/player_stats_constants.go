package routes

import (
	"context"
	"skycrypt/src/constants"
	"skycrypt/src/models"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

func registerStatsConstants(humaAPI huma.API) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "getStatsConstants",
		Method:      fiber.MethodGet,
		Path:        "/api/constants/stats",
		Summary:     "List player stats constants",
		Description: "Returns player stats constants.",
		Tags:        []string{"Stats"},
		Responses:   map[string]*huma.Response{"200": {Description: "Player stats constants returned successfully."}},
	}, func(context.Context, *struct{}) (*bodyOutput[[]models.StatData], error) {
		return &bodyOutput[[]models.StatData]{Body: constants.STATS_DATA}, nil
	})
}
