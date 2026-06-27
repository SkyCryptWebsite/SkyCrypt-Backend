package routes

import (
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
//	@Summary		Get profile inventory
//	@Description	Returns rendered inventory tabs and stripped item data for a specific SkyBlock profile.
//	@Description	This endpoint refreshes the short-lived cache used by the inventory search endpoint. Resource pack preferences supplied by cookie can affect rendered item texture URLs in the response.
//	@ID				getProfileInventory
//	@Tags			Stats
//	@Produce		json
//	@Security		ApiTokenHeader
//	@Param			uuid		path		string	true	"Minecraft UUID of the profile member"	example(4855c53ee4fb4100997600a92fc50984)
//	@Param			profileId	path		string	true	"Hypixel SkyBlock profile UUID or cute name"	example(00912956-3fd6-42ee-a166-3f649ceaf559)
//	@Success		200			{array}		models.Inventory		"Profile inventory returned successfully."
//	@Failure		400			{object}	models.ProcessingError	"Profile, museum, or inventory data could not be loaded."
//	@Failure		401			{object}	models.ProcessingError	"X-API-Token is missing or invalid."
//	@Router			/api/inventory/{uuid}/{profileId} [get]
func InventoryHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Inventory")()
	}

	timeNow := time.Now()

	disabledPacks := disabledPacksFromRequest(c)

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")
	reqCtx := c.UserContext()
	cacheKey := responseCacheKey("inventory", uuid, profileId, disabledPacksCachePart(disabledPacks))
	if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
		return err
	}

	var (
		profiles      *models.HypixelProfilesResponse
		profile       *skycrypttypes.Profile
		profileMuseum map[string]*skycrypttypes.Museum
	)

	isProfileUUID := utility.IsUUID(profileId)
	g, groupCtx := errgroup.WithContext(reqCtx)
	type museumResult struct {
		profileID string
		museum    map[string]*skycrypttypes.Museum
		err       error
	}
	var selectedMuseumCh chan museumResult

	if isProfileUUID {
		g.Go(func() error {
			var err error
			profileMuseum, err = api.GetMuseumContext(groupCtx, profileId)
			return err
		})
	}

	if profileId == "" {
		if cachedProfileID := getCachedSelectedProfileID(reqCtx, uuid); cachedProfileID != "" {
			selectedMuseumCh = make(chan museumResult, 1)
			g.Go(func() error {
				museum, err := api.GetMuseumContext(groupCtx, cachedProfileID)
				selectedMuseumCh <- museumResult{profileID: cachedProfileID, museum: museum, err: err}
				return nil
			})
		}
	}

	g.Go(func() error {
		var err error
		profiles, err = api.GetProfilesContext(groupCtx, uuid)
		if err != nil {
			return err
		}

		profile, err = stats.GetProfile(profiles, profileId)
		if err != nil {
			return err
		}
		if profileId == "" {
			cacheSelectedProfileID(groupCtx, uuid, profile.ProfileID)
		}

		if !isProfileUUID {
			if selectedMuseumCh != nil {
				result := <-selectedMuseumCh
				if result.err == nil && result.profileID == profile.ProfileID {
					profileMuseum = result.museum
					return nil
				}
			}
			profileMuseum, err = api.GetMuseumContext(groupCtx, profile.ProfileID)
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
	var itemProcessingStats *statsItems.ItemProcessingStats
	if statsItems.ItemProcessingDebugEnabled() {
		itemProcessingStats = statsItems.NewItemProcessingStats("inventory", uuid, profile.ProfileID, disabledPacks)
	}
	neuItemCache := map[string]models.NEUItem{}
	for _, inventoryId := range constants.INVENTORY_ORDER {
		inventoryData := constants.INVENTORY[inventoryId]
		if inventoryId == "museum" {
			museumItems := statsItems.GetMuseum(profileMuseum[uuid], disabledPacks)
			stripStart := time.Now()
			strippedItems := statsItems.StripItems(&museumItems)
			itemProcessingStats.RecordStripDuration(time.Since(stripStart))

			output = append(output, models.Inventory{
				Name:           inventoryData.Name,
				Texture:        fmt.Sprintf("%s%s", utility.GetDomain(), inventoryData.Texture),
				SeparatorAfter: inventoryData.SeparatorAfter,
				Items:          strippedItems,
			})
			continue
		} else if inventoryId == "sacks" {
			itemSlice := stats.GetInventory(&userProfile, "sacks")
			sackItems := userProfile.Inventory.Sacks

			parsedSacks := statsItems.ProcessItemsWithNEUCacheAndStats(itemSlice, "sacks", neuItemCache, itemProcessingStats, disabledPacks)
			processedSacks := statsItems.ProcessSacks(parsedSacks, sackItems)
			stripStart := time.Now()
			strippedItems := statsItems.StripItems(&processedSacks, models.StripOptions{
				Nested: true,
			})
			itemProcessingStats.RecordStripDuration(time.Since(stripStart))

			output = append(output, models.Inventory{
				Name:           inventoryData.Name,
				Texture:        fmt.Sprintf("%s%s", utility.GetDomain(), inventoryData.Texture),
				SeparatorAfter: inventoryData.SeparatorAfter,
				Items:          strippedItems,
			})
			continue
		}

		texture := fmt.Sprintf("%s%s", utility.GetDomain(), inventoryData.Texture)
		if inventoryId == "inventory" {
			texture = fmt.Sprintf(`https://nmsr.nickac.dev/headiso/%s?noshading&no=shadow`, uuid)
		}

		inventoryItems := stats.GetInventory(&userProfile, inventoryId)
		processedItems := statsItems.ProcessItemsWithNEUCacheAndStats(inventoryItems, inventoryId, neuItemCache, itemProcessingStats, disabledPacks)
		stripStart := time.Now()
		strippedItems := statsItems.StripItems(&processedItems)
		itemProcessingStats.RecordStripDuration(time.Since(stripStart))
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

	if itemProcessingStats != nil {
		itemProcessingStats.LogIfEnabled(time.Since(itemProcessingStats.Started))
	}
	utility.LogVerbose("Returning /api/inventory/%s/%s in %s pid=%d", uuid, profileId, time.Since(timeNow), os.Getpid())

	// Cache the full inventory for search functionality
	go func() {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		jsonData, err := json.Marshal(output)
		if err != nil {
			fmt.Printf("Error marshaling items for caching: %v\n", err)
		} else {
			_ = db.Set(fmt.Sprintf("items:%s:%s:%s", profileId, uuid, disabledPacksCachePart(disabledPacks)), string(jsonData), 5*60) // Cache for 5 minutes
		}
	}()

	return sendAndCacheJSON(c, reqCtx, cacheKey, output, 5*60)
}
