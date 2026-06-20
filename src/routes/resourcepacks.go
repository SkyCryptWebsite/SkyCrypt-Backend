package routes

import (
	"encoding/json"
	"fmt"
	"os"
	"skycrypt/src/models"
	"skycrypt/src/utility"

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

	if len(RESOURCE_PACKS) == 0 {
		filePath := "assets/resourcepacks/"
		files, err := os.ReadDir(filePath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to read resource packs directory",
			})
		}

		for _, file := range files {
			if !file.IsDir() {
				continue
			}

			configPath := fmt.Sprintf("%s/%s/config.json", filePath, file.Name())
			configFile, err := os.Open(configPath)
			if err != nil {
				continue
			}

			defer func() {
				_ = configFile.Close()
			}()

			var configData models.ResourcePackConfig
			if err := json.NewDecoder(configFile).Decode(&configData); err != nil {
				continue
			}

			// We don't want to include the vanilla resource pack in the list because it's not toggleable
			if configData.Id == "VANILLA" {
				continue
			}

			configData.Icon = fmt.Sprintf("%s/assets/resourcepacks/%s/pack.png", utility.GetDomain(), file.Name())

			RESOURCE_PACKS = append(RESOURCE_PACKS, configData)
		}
	}

	// utility.LogVerbose("Returning /api/resourcepacks in %s", time.Since(timeNow))

	return c.JSON(RESOURCE_PACKS)
}
