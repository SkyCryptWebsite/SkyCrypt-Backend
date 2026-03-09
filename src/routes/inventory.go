package routes

import (
	"encoding/json"
	"fmt"
	"os"
	"skycrypt/src/api"
	redis "skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	statsItems "skycrypt/src/stats/items"
	"skycrypt/src/utility"
	"strings"

	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
)

var ICONS map[string]string = map[string]string{
	"backpack":        "/api/item/chest",
	"enderchest":      "/api/item/ender_chest",
	"personal_vault":  "/api/head/f7aadff9ddc546fdcec6ed5919cc39dfa8d0c07ff4bc613a19f2e6d7f2593",
	"talisman_bag":    "/api/head/961a918c0c49ba8d053e522cb91abc74689367b4d8aa06bfc1ba9154730985ff",
	"potion_bag":      "/api/head/9f8b82427b260d0a61e6483fc3b2c35a585851e08a9a9df372548b4168cc817c",
	"fishing_bag":     "/api/head/eb8e297df6b8dffcf135dba84ec792d420ad8ecb458d144288572a84603b1631",
	"quiver":          "/api/head/4cb3acdc11ca747bf710e59f4c8e9b3d949fdd364c6869831ca878f0763d1787",
	"museum":          "/api/head/438cf3f8e54afc3b3f91d20a49f324dca1486007fe545399055524c17941f4dc",
	"rift_inventory":  "/api/head/445240fcf1a9796327dda5593985343af9121a7156bc76e3d6b341b02e6a6e52",
	"rift_enderchest": "/api/head/a6cc486c2be1cb9dfcb2e53dd9a3e9a883bfadb27cb956f1896d602b4067",
}

func getIcon(source string, uuid string) string {
	if icon, exists := ICONS[source]; exists {
		return fmt.Sprintf("%s%s", utility.GetDomain(), icon)
	}

	return fmt.Sprintf(`https://crafatar.com/renders/head/%s?overlay`, uuid)
}

// InventoryHandler godoc
//
//	@Summary		Get inventory items for a specified player
//	@Description	Returns inventory items for the given user, profile ID, and inventory ID. Supports museum, search, and other inventories.
//	@Tags			inventory
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Param			inventoryId	path		string	true	"Inventory ID (e.g., museum, search, or other inventory types)"
//	@Param			query		query		string	false	"Search query (required when inventoryId is 'search')"
//	@Success		200			{object}	[]models.StrippedItem
//	@Failure		400			{object}	models.ProcessingError
//	@Failure		500			{object}	models.ProcessingError
//	@Router			/api/inventory/{uuid}/{profileId}/{inventoryId} [get]
func InventoryHandler(c *fiber.Ctx) error {
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
	inventoryId := c.Params("inventoryId")
	if inventoryId == "museum" {
		profileMuseum, err := api.GetMuseum(profileId)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to get museum: %v", err),
			})
		}

		museum := profileMuseum[uuid]
		if museum == nil {
			return c.JSON([]models.StrippedItem{})
		}

		museumItems := statsItems.GetMuseum(museum, disabledPacks)

		output := statsItems.StripItems(&museumItems)

		utility.LogVerbose("Returning /api/inventory/%s/%s in %s", uuid, inventoryId, time.Since(timeNow))

		return c.JSON(output)
	}

	profile, err := api.GetProfile(uuid, profileId)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get profile: %v", err),
		})
	}

	userProfileValue := profile.Members[uuid]
	userProfile := &userProfileValue

	if inventoryId == "sacks" {
		itemSlice := stats.GetInventory(userProfile, inventoryId)
		parsedSacks := statsItems.ProcessItems(itemSlice, inventoryId, disabledPacks)

		sackItems := userProfile.Inventory.Sacks
		processedSacks := statsItems.ProcessSacks(parsedSacks, sackItems)

		strippedItems := statsItems.StripItems(&processedSacks)

		return c.JSON(strippedItems)
	}

	if inventoryId == "search" {
		var items map[string][]*skycrypttypes.Item
		cache, err := redis.Get(fmt.Sprintf("items:%s", profileId))
		if err == nil && cache != "" {
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err = json.Unmarshal([]byte(cache), &items)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("Failed to parse items: %v", err),
				})
			}
		} else {
			items, err = stats.GetItems(userProfile, profile.ProfileID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("Failed to get items: %v", err),
				})
			}
		}

		searchString := c.Query("query")
		if searchString == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Search string cannot be empty",
			})
		}

		formattedItems := make([]models.ProcessedItem, 0, 5*9)
		for inventoryId, inventory := range items {
			for _, item := range inventory {
				if item.Tag == nil || item.Tag.Display.Name == "" {
					continue
				}

				if strings.Contains(strings.ToLower(item.Tag.Display.Name), searchString) || strings.Contains(strings.Join(item.Tag.Display.Lore, " "), searchString) {
					item := statsItems.ProcessItem(item, inventoryId, disabledPacks)

					formattedItems = append(formattedItems, item)
				}

				if len(formattedItems) >= 5*9 {
					break
				}

			}
		}

		strippedItems := statsItems.StripItems(&formattedItems, true)

		searchResults := make([]models.StrippedItem, len(strippedItems))
		for i, item := range strippedItems {
			item.SourceTab = &models.SourceTab{
				Icon: getIcon(item.Source, uuid),
				Name: utility.TitleCase(item.Source),
			}

			searchResults[i] = item
		}

		utility.LogVerbose("Returning /api/inventory/%s/%s/%s in %s", uuid, inventoryId, searchString, time.Since(timeNow))

		return c.JSON(searchResults)
	}

	itemSlice := stats.GetInventory(userProfile, inventoryId)
	output := statsItems.ProcessItems(itemSlice, inventoryId, disabledPacks)

	strippedItems := statsItems.StripItems(&output)
	if strings.HasSuffix(inventoryId, "inventory") {
		if len(strippedItems) > 9 {
			strippedItems = append(strippedItems[9:], strippedItems[:9]...)
		}
	}

	utility.LogVerbose("Returning /api/inventory/%s/%s in %s", uuid, inventoryId, time.Since(timeNow))

	return c.JSON(strippedItems)
}
