package routes

import (
	"encoding/json"
	"fmt"
	"os"
	"skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"

	"time"

	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
)

// InventorySearchHandler godoc
//
//	@Summary		Get searched inventory items for a specified player and search parameter
//	@Description	Returns inventory items that match the search parameter for the given user and profile ID. Searches across all inventories and returns items that contain the search parameter in their name or lore.
//	@Tags			inventory
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Param			searchParam	path		string	true	"Search parameter to filter inventory items"
//	@Success		200			{object}	[]models.StrippedItem
//	@Failure		400			{object}	models.ProcessingError
//	@Failure		500			{object}	models.ProcessingError
//	@Router			/api/inventorySearch/{uuid}/{profileId} [get]
func InventorySearchHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Inventory")()
	}

	timeNow := time.Now()

	disabledPacks := []string{""}
	disabledPacksCookies := c.Cookies("disabledPacks", "FAILED")
	if disabledPacksCookies != "FAILED" {
		var parsedPacks []string
		err := json.Unmarshal([]byte(disabledPacksCookies), &parsedPacks)
		if err == nil {
			disabledPacks = append(disabledPacks, parsedPacks...)
		}
	} else if os.Getenv("DEV") == "true" {
		disabledResourcePacks := c.Query("disabledPacks", "")
		if disabledResourcePacks != "" {
			disabledPacks = strings.Split(disabledResourcePacks, ",")
		}
	}

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")
	searchParam := c.Params("searchParam")

	cache, err := db.Get(fmt.Sprintf("items:%s:%s:%s", profileId, uuid, strings.Join(disabledPacks, ",")))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get items: %v", err),
		})
	}

	var inventories []models.Inventory
	if cache != "" {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal([]byte(cache), &inventories)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to parse items: %v", err),
			})
		}
	} else {
		// Couldn't find cache to use so call InventoryHandler so it gets cached and rerun the search handler to get the data from cache
		err := InventoryHandler(c)
		if err != nil {
			return err
		}

		return InventorySearchHandler(c)
	}

	output := []models.StrippedItem{}
	for _, inventory := range inventories {
		if inventory.Items == nil {
			continue
		}

		for _, item := range inventory.Items {
			if item.DisplayName == "" || (!strings.Contains(strings.ToLower(item.DisplayName), searchParam) && !strings.Contains(strings.Join(item.Lore, " "), searchParam)) {
				continue
			}

			item.SourceTab = &models.SourceTab{
				Icon: inventory.Texture,
				Name: inventory.Name,
			}

			output = append(output, item)

			if len(output) >= 5*9 {
				break
			}
		}
	}

	utility.LogVerbose("Returning /api/inventorySearch/%s/%s/%s in %s", uuid, profileId, searchParam, time.Since(timeNow))

	return c.JSON(output)
}
