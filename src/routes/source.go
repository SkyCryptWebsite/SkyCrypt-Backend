package routes

import (
	"os"
	"skycrypt/src/models"

	"github.com/gofiber/fiber/v2"
)

const sourceRepository = "https://github.com/SkyCryptWebsite/SkyCrypt-Backend"

// SourceHandler godoc
//
//	@Summary		Get source information
//	@Description	Returns repository, license, commit, and notice information for the running SkyCrypt Backend service.
//	@ID				getSourceInfo
//	@Tags			Source
//	@Produce		json
//	@Success		200	{object}	models.SourceInfo	"Source and license information returned successfully."
//	@Router			/api/source [get]
func SourceHandler(c *fiber.Ctx) error {
	commit := os.Getenv("SOURCE_COMMIT")
	sourceURL := sourceRepository
	if commit != "" {
		sourceURL = sourceRepository + "/tree/" + commit
	}

	return c.JSON(models.SourceInfo{
		Name:        "SkyCrypt Backend",
		License:     "GNU AGPLv3",
		LicenseSPDX: "AGPL-3.0-only",
		Repository:  sourceRepository,
		Source:      sourceURL,
		Commit:      commit,
		Notice:      "This software is provided without warranty. Third-party assets and dependencies retain their own licenses.",
	})
}
