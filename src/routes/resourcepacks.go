package routes

import (
	"skycrypt/src/lib"

	"github.com/gofiber/fiber/v2"
)

// ResourcePackHandler godoc
//
//	@Summary		List resource packs
//	@Description	Returns toggleable resource packs sorted by descending priority for the recommended default order.
//	@Description	The enabledPacks cookie controls per-request rendering from highest to lowest priority and does not change this response order.
//	@Description	A missing preference uses the default order, while an explicit empty array uses vanilla textures only.
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
