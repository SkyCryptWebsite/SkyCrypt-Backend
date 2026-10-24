package routes

import (
	"context"
	"os"
	"skycrypt/src/models"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

const sourceRepository = "https://github.com/SkyCryptWebsite/SkyCrypt-Backend"

// bodyOutput is a response whose JSON body is T.
type bodyOutput[T any] struct {
	Body T
}

func registerSource(humaAPI huma.API) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "getSourceInfo",
		Method:      fiber.MethodGet,
		Path:        "/api/source",
		Summary:     "Get source information",
		Description: "Returns repository, license, commit, and notice information for the running SkyCrypt Backend service.",
		Tags:        []string{"Source"},
		Responses:   map[string]*huma.Response{"200": {Description: "Source and license information returned successfully."}},
	}, func(context.Context, *struct{}) (*bodyOutput[models.SourceInfo], error) {
		commit := os.Getenv("SOURCE_COMMIT")
		sourceURL := sourceRepository
		if commit != "" {
			sourceURL = sourceRepository + "/tree/" + commit
		}
		return &bodyOutput[models.SourceInfo]{Body: models.SourceInfo{
			Name:        "SkyCrypt Backend",
			License:     "GNU AGPLv3",
			LicenseSPDX: "AGPL-3.0-only",
			Repository:  sourceRepository,
			Source:      sourceURL,
			Commit:      commit,
			Notice:      "This software is provided without warranty. Third-party assets and dependencies retain their own licenses.",
		}}, nil
	})
}
