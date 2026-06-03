package routes

import (
	"encoding/json"
	"fmt"
	"os"
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/db"
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
	"golang.org/x/sync/errgroup"
)

// InventoryHandler godoc
//
//	@Summary		Get inventory items for a specified player
//	@Description	Returns inventory items for the given user, profile ID
//	@Tags			inventory
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Param			query		query		string	false	"Search query (required when inventoryId is 'search')"
//	@Success		200			{object}	[]models.Inventory
//	@Failure		400			{object}	models.ProcessingError
//	@Failure		500			{object}	models.ProcessingError
//	@Router			/api/inventory/{uuid}/{profileId} [get]
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

	var (
		profiles      *models.HypixelProfilesResponse
		profile       *skycrypttypes.Profile
		profileMuseum map[string]*skycrypttypes.Museum
	)

	isProfileUUID := utility.IsUUID(profileId)
	g, _ := errgroup.WithContext(c.Context())

	if isProfileUUID {
		g.Go(func() error {
			var err error
			profileMuseum, err = api.GetMuseum(profileId)
			return err
		})
	}

	g.Go(func() error {
		var err error
		profiles, err = api.GetProfiles(uuid)
		if err != nil {
			return err
		}

		profile, err = stats.GetProfile(profiles, profileId)
		if err != nil {
			return err
		}

		if !isProfileUUID {
			profileMuseum, err = api.GetMuseum(profile.ProfileID)
			return err
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to fetch data: %v", err),
		})
	}

	userProfile := profile.Members[uuid]
	if userProfile.Inventory == nil {
		userProfile.Inventory = &skycrypttypes.Inventory{}
	}

	output := []models.Inventory{}
	for _, inventoryId := range constants.INVENTORY_ORDER {
		inventoryData := constants.INVENTORY[inventoryId]
		if inventoryId == "museum" {
			museumItems := statsItems.GetMuseum(profileMuseum[uuid], disabledPacks)

			output = append(output, models.Inventory{
				Name:           inventoryData.Name,
				Texture:        fmt.Sprintf("%s%s", utility.GetDomain(), inventoryData.Texture),
				SeparatorAfter: inventoryData.SeparatorAfter,
				Items:          statsItems.StripItems(&museumItems),
			})
			continue
		} else if inventoryId == "sacks" {
			itemSlice := stats.GetInventory(&userProfile, "sacks")
			sackItems := userProfile.Inventory.Sacks

			parsedSacks := statsItems.ProcessItems(itemSlice, "sacks", disabledPacks)
			processedSacks := statsItems.ProcessSacks(parsedSacks, sackItems)

			output = append(output, models.Inventory{
				Name:           inventoryData.Name,
				Texture:        fmt.Sprintf("%s%s", utility.GetDomain(), inventoryData.Texture),
				SeparatorAfter: inventoryData.SeparatorAfter,
				Items: statsItems.StripItems(&processedSacks, models.StripOptions{
					Nested: true,
				}),
			})
			continue
		}

		texture := fmt.Sprintf("%s%s", utility.GetDomain(), inventoryData.Texture)
		if inventoryId == "inventory" {
			texture = fmt.Sprintf(`https://nmsr.nickac.dev/headiso/%s?noshading&no=shadow`, uuid)
		}

		inventoryItems := stats.GetInventory(&userProfile, inventoryId)
		processedItems := statsItems.ProcessItems(inventoryItems, inventoryId, disabledPacks)
		strippedItems := statsItems.StripItems(&processedItems)
		if strings.HasSuffix(inventoryId, "inventory") { // Move hotbar to end of inventory
			if len(strippedItems) > 9 {
				strippedItems = append(strippedItems[9:], strippedItems[:9]...)
			}
		}

		output = append(output, models.Inventory{
			Name:           inventoryData.Name,
			Texture:        texture,
			SeparatorAfter: inventoryData.SeparatorAfter,
			Items:          strippedItems,
		})
	}

	utility.LogVerbose("Returning /api/inventory/%s/%s in %s", uuid, profileId, time.Since(timeNow))

	// Cache the full inventory for search functionality
	go func() {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		jsonData, err := json.Marshal(output)
		if err != nil {
			fmt.Printf("Error marshaling items for caching: %v\n", err)
		} else {
			_ = db.Set(fmt.Sprintf("items:%s:%s:%s", profileId, uuid, strings.Join(disabledPacks, ",")), string(jsonData), 5*60) // Cache for 5 minutes
		}
	}()

	return c.JSON(output)
}
