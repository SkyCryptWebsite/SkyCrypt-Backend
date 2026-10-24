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
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

func registerInventory(humaAPI huma.API) {
	responses := profileErrors(humaAPI, "Unexpected failure.")
	responses["200"] = jsonResponse[[]models.Inventory](humaAPI, "Profile inventory returned successfully.")
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "getProfileInventory",
		Method:      fiber.MethodGet,
		Path:        "/api/inventory/{uuid}/{profileId}",
		Summary:     "Get profile inventory",
		Description: "Returns rendered inventory tabs and stripped item data for a specific SkyBlock profile.\n" +
			"This endpoint refreshes the short-lived cache used by the inventory search endpoint. Resource pack preferences supplied by cookie can affect rendered item texture URLs in the response.",
		Tags:      []string{"Stats"},
		Responses: responses,
	}), func(ctx context.Context, in *struct {
		MemberInput
		PacksInput
	}) (*jsonOutput, error) {
		return inventoryResponse(ctx, in.UUID, in.ProfileID, in.enabledPacks)
	})
}

func inventoryResponse(reqCtx context.Context, uuid string, profileId string, enabledPacks []string) (*jsonOutput, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Inventory")()
	}

	timeNow := time.Now()

	// Members is keyed by the lowercase UUID, and the cache key is lowercased too.
	uuid = strings.ToLower(uuid)
	cacheKey := inventoryCacheKey(uuid, profileId, enabledPacks)
	if cached := cachedJSON(reqCtx, cacheKey); cached != nil {
		return cached, nil
	}

	body, err := sharedResponse(reqCtx, cacheKey, cacheKey, 5*60, func(ctx context.Context) ([]byte, error) {
		return buildInventory(ctx, uuid, profileId, enabledPacks, cacheKey)
	})
	if err != nil {
		return nil, buildFailure(err)
	}

	utility.LogVerbose("Returning /api/inventory/%s/%s in %s pid=%d", uuid, profileId, time.Since(timeNow), os.Getpid())
	return jsonBody(reqCtx, cacheKey, body), nil
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
