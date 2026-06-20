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
//	@Summary		Search cached profile inventory
//	@Description	Searches cached inventory tabs for items whose display name or lore contains the search parameter.
//	@Description	Call GET /api/inventory/{uuid}/{profileId} first to refresh the short-lived cache for the same player, profile, and resource pack preferences. At most 45 matching items are returned.
//	@ID				searchProfileInventory
//	@Tags			Stats
//	@Produce		json
//	@Security		ApiTokenHeader
//	@Param			uuid		path		string	true	"Minecraft UUID of the profile member"	example(4855c53ee4fb4100997600a92fc50984)
//	@Param			profileId	path		string	true	"Hypixel SkyBlock profile UUID or cute name"	example(00912956-3fd6-42ee-a166-3f649ceaf559)
//	@Param			searchParam	path		string	true	"Case-sensitive search text matched against item display names and lore"	example(hyperion)
//	@Success		200			{array}		models.StrippedItem		"Matching inventory items returned successfully."
//	@Failure		401			{object}	models.ProcessingError	"X-API-Token is missing or invalid."
//	@Failure		500			{object}	models.ProcessingError	"Cached inventory data is missing or could not be parsed."
//	@Router			/api/inventory/search/{uuid}/{profileId}/{searchParam} [get]
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
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "No cached items found for this player and profile. Please try again later.",
		})
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

	utility.LogVerbose("Returning /api/inventory/search/%s/%s/%s in %s", uuid, profileId, searchParam, time.Since(timeNow))

	return c.JSON(output)
}
