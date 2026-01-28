package routes

import (
	"encoding/json"
	"fmt"
	"os"
	"skycrypt/src/api"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	statsItems "skycrypt/src/stats/items"
	"strings"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
	"github.com/gofiber/fiber/v2"
)

// RiftHandler godoc
//
//	@Summary		Get rift stats of a specified player
//	@Description	Returns rift data for the given user and profile ID
//	@Tags			rift
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.RiftOutput
//	@Failure		400			{object}	models.ProcessingError
//	@Failure		500			{object}	models.ProcessingError
//	@Router			/api/rift/{uuid}/{profileId} [get]
func RiftHandler(c *fiber.Ctx) error {
	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")

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

	profile, err := api.GetProfile(uuid, profileId)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get profile: %v", err),
		})
	}

	userProfileValue := profile.Members[uuid]
	userProfile := &userProfileValue

	specifiedInventories := skyhelpernetworthgo.SpecifiedInventory{
		"rift_armor":     profile.Members[uuid].Rift.Inventory.Armor,
		"rift_equipment": profile.Members[uuid].Rift.Inventory.Equipment,
	}

	decodedItems, err := skyhelpernetworthgo.CalculateFromSpecifiedInventories(specifiedInventories, skyhelpernetworthgo.NetworthOptions{
		IncludeItemData:  true,
		KeepInvalidItems: true,
	}.ToInternal())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to calculate items: %v", err),
		})
	}

	processedItems := map[string][]models.ProcessedItem{}
	for inventoryId := range specifiedInventories {
		if decodedItems.Types[inventoryId] == nil {
			continue
		}

		inventoryData := decodedItems.Types[inventoryId].Items
		if len(inventoryData) == 0 {
			continue
		}

		combinedItems := make([]*skycrypttypes.Item, len(inventoryData))
		for i, item := range inventoryData {
			combinedItems[i] = item.ItemData
			if combinedItems[i] == nil {
				continue
			}

			combinedItems[i].Price = item.Price
		}

		processedItems[inventoryId] = statsItems.ProcessItems(combinedItems, inventoryId, disabledPacks)
	}

	output := stats.GetRift(userProfile, processedItems)

	fmt.Printf("Returning /api/rift/%s in %s\n", profileId, time.Since(timeNow))

	return c.JSON(output)
}
