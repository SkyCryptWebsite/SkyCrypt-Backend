package routes

import (
	"skycrypt/src/lib"
	"skycrypt/src/models"

	"github.com/gofiber/fiber/v2"
)

var RESOURCE_PACKS = []models.ResourcePackConfig{}

// ResourcePackHandler godoc
//
//	@Summary		List resource packs
//	@Description	Returns toggleable resource packs available to SkyCrypt clients.
//	@Description	The vanilla resource pack is intentionally omitted because it is the default and cannot be disabled.
//	@ID				listResourcePacks
//	@Tags			Rendering
//	@Produce		json
//	@Success		200	{array}		models.ResourcePackConfig	"Resource packs returned successfully."
//	@Failure		500	{object}	models.ProcessingError		"Resource pack metadata could not be read."
//	@Router			/api/resourcepacks [get]
func ResourcePackHandler(c *fiber.Ctx) error {
	// timeNow := time.Now()

	configs, err := lib.ResourcePackConfigs()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read resource packs directory",
		})
	}

	// utility.LogVerbose("Returning /api/resourcepacks in %s", time.Since(timeNow))

	return c.JSON(configs)
}
