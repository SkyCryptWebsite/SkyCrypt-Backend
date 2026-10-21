package routes

import (
	"context"
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
)

// InventoryHandler godoc
//
//	@Summary		Get profile inventory
//	@Description	Returns rendered inventory tabs and stripped item data for a specific SkyBlock profile.
//	@Description	This endpoint refreshes the short-lived cache used by the inventory search endpoint. Resource pack preferences supplied by cookie can affect rendered item texture URLs in the response.
//	@ID				getProfileInventory
//	@Tags			Stats
//	@Produce		json
//	@Security		ApiKeyHeader
//	@Param			uuid		path		string	true	"Minecraft UUID of the profile member"	example(4855c53ee4fb4100997600a92fc50984)
//	@Param			profileId	path		string	true	"Hypixel SkyBlock profile UUID or cute name"	example(00912956-3fd6-42ee-a166-3f649ceaf559)
//	@Success		200			{array}		models.Inventory		"Profile inventory returned successfully."
//	@Failure		401			{object}	models.APIError	"The x-api-key header is missing, or the key is invalid, disabled, or expired."
//	@Failure		404			{object}	models.APIError	"PLAYER_NOT_FOUND or PROFILE_NOT_FOUND: no such player or profile. NOT_CACHED: the player isn't cached, and the key lacks the cache-bypass scope."
//	@Failure		429			{object}	models.APIError	"RATE_LIMITED: the key's rate limit is used up. See the Retry-After header."
//	@Failure		500			{object}	models.APIError	"INTERNAL_ERROR: Unexpected failure."
//	@Failure		502			{object}	models.APIError	"UPSTREAM_ERROR: Hypixel or Mojang failed or sent an unusable response."
//	@Failure		503			{object}	models.APIError	"API keys can't be verified right now."
//	@Failure		504			{object}	models.APIError	"UPSTREAM_TIMEOUT: Hypixel or Mojang didn't answer in time."
//	@Router			/api/inventory/{uuid}/{profileId} [get]
func InventoryHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Inventory")()
	}

	timeNow := time.Now()

	enabledPacks := enabledPacksFromRequest(c)

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")
	reqCtx := c.UserContext()
	cacheKey := inventoryCacheKey(uuid, profileId, enabledPacks)
	if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
		return err
	}

	body, err := sharedResponse(reqCtx, cacheKey, cacheKey, 5*60, func(ctx context.Context) ([]byte, error) {
		return buildInventory(ctx, uuid, profileId, enabledPacks, cacheKey)
	})
	if err != nil {
		return sendBuildError(c, err)
	}

	utility.LogVerbose("Returning /api/inventory/%s/%s in %s pid=%d", uuid, profileId, time.Since(timeNow), os.Getpid())
	return sendBody(c, reqCtx, cacheKey, body)
}

func buildInventory(ctx context.Context, uuid string, profileId string, enabledPacks []string, cacheKey responseCacheHandle) ([]byte, error) {
	// The route takes a UUID, so the loader gets it as is rather than through a Mojang lookup.
	data, err := loadProfileData(ctx, &models.MowojangResponse{UUID: uuid}, profileId, profileParts{museum: true})
	if err != nil {
		return nil, failedTo("Failed to fetch data", err)
	}
	profile := data.Profile
	profileMuseum := data.Museums

	userProfile := profile.Members[uuid]
	if userProfile.Inventory == nil {
		userProfile.Inventory = &skycrypttypes.Inventory{}
	}

	output := []models.Inventory{}
	var itemProcessingStats *statsItems.ItemProcessingStats
	if statsItems.ItemProcessingDebugEnabled() {
		itemProcessingStats = statsItems.NewItemProcessingStats("inventory", uuid, profile.ProfileID, enabledPacks)
	}
	neuItemCache := map[string]models.NEUItem{}

	// The museum tab is built separately; run it alongside decoding the other inventories, which happens in one
	// batched call.
	museumCh := make(chan []models.ProcessedItem, 1)
	go func() {
		var museumItems []models.ProcessedItem
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[INVENTORY] museum tab failed: %v\n", r)
				api.SkipCaching(ctx)
			}
			museumCh <- museumItems
		}()
		museumItems = statsItems.GetMuseum(profileMuseum[uuid], enabledPacks)
	}()
	decodedInventories := stats.GetInventories(&userProfile, constants.INVENTORY_ORDER)

	for _, inventoryId := range constants.INVENTORY_ORDER {
		inventoryData := constants.INVENTORY[inventoryId]
		if inventoryId == "museum" {
			museumItems := <-museumCh
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
			itemSlice := decodedInventories["sacks"]
			sackItems := userProfile.Inventory.Sacks

			parsedSacks := statsItems.ProcessItemsWithNEUCacheAndStats(itemSlice, "sacks", neuItemCache, itemProcessingStats, enabledPacks)
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

		inventoryItems := decodedInventories[inventoryId]
		processedItems := statsItems.ProcessItemsWithNEUCacheAndStats(inventoryItems, inventoryId, neuItemCache, itemProcessingStats, enabledPacks)
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
	body, err := marshalBody(output)
	if err != nil {
		return nil, err
	}

	// Inventory search reads this key, so write it even when the response cache is off.
	if !processedResponseCacheEnabled() {
		cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := db.SetContext(cacheCtx, cacheKey.key, body, 5*60); err != nil {
			return nil, fmt.Errorf("failed to cache inventory for search: %v", err)
		}
	}
	return body, nil
}
