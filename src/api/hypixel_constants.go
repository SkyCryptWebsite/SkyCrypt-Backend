package api

import (
	"fmt"
	"io"
	"net/http"
	"skycrypt/src/constants"
	redis "skycrypt/src/db"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

func getSkyBlockItems() ([]models.HypixelItem, error) {
	cachedData, err := redis.Get("skyblock_items")
	if err == nil && cachedData != "" {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		var data models.HypixelItemsResponse
		err = json.Unmarshal([]byte(cachedData), &data)
		if err == nil {
			return data.Items, nil
		}
	}

	resp, err := http.Get("https://api.hypixel.net/v2/resources/skyblock/items")
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("received empty response from API")
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	var data models.HypixelItemsResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	_ = redis.Set("skyblock_items", string(body), 12*60*60) // Cache for 12 hours

	return data.Items, nil
}

func GetSkyBlockCollections() (map[string]models.HypixelCollection, error) {
	cachedData, err := redis.Get("skyblock_collections")
	if err == nil && cachedData != "" {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		var data models.HypixelCollectionsResponse
		err = json.Unmarshal([]byte(cachedData), &data)
		if err == nil {
			return data.Collections, nil
		}
	}

	resp, err := http.Get("https://api.hypixel.net/v2/resources/skyblock/collections")
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("received empty response from API")
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	var data models.HypixelCollectionsResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	_ = redis.Set("skyblock_collections", string(body), 12*60*60) // Cache for 12 hours

	return data.Collections, nil
}

func processItems(items *[]models.HypixelItem) map[string]models.ProcessedHypixelItem {
	processed := make(map[string]models.ProcessedHypixelItem)
	for _, item := range *items {
		if item.Rarity == "" {
			item.Rarity = "common"
		}

		texture := fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), item.SkyBlockID)

		processed[item.SkyBlockID] = models.ProcessedHypixelItem{
			SkyblockID:        item.SkyBlockID,
			Name:              item.Name,
			ItemId:            constants.BUKKIT_TO_ID[item.Material],
			Rarity:            strings.ToLower(item.Rarity),
			Damage:            item.Damage,
			Texture:           texture,
			TextureId:         utility.GetSkinHash(item.Skin.Value),
			Category:          strings.ToLower(item.Category),
			Origin:            item.Origin,
			RiftTransferrable: item.RiftTransferrable,
			MuseumData:        item.MuseumData,
			Color:             utility.GetHexColor(item.Color),
			GemstoneSlots:     item.GemstoneSlots,
		}
	}

	return processed
}

func processCollections(collections map[string]models.HypixelCollection) models.ProcessedHypixelCollection {
	output := models.ProcessedHypixelCollection{}
	for categoryId, categoryData := range collections {
		category := models.ProcessedHypixelCollectionCategory{
			Name:        categoryData.Name,
			Texture:     fmt.Sprintf("%s%s", utility.GetDomain(), constants.COLLECTION_ICONS[strings.ToLower(categoryId)]),
			Collections: []models.ProcessedHypixelCollectionItem{},
		}

		for collectionId, collectionData := range categoryData.Items {
			processedItem := models.ProcessedHypixelCollectionItem{
				Id:      collectionId,
				Name:    collectionData.Name,
				Texture: fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), collectionId),
				MaxTier: collectionData.MaxTiers,
				Tiers:   collectionData.Tiers,
			}

			category.Collections = append(category.Collections, processedItem)
		}

		output[categoryId] = category
	}

	return output
}

func LoadSkyBlockItems() error {
	// timeNow := time.Now()
	items, err := getSkyBlockItems()
	if err != nil {
		return fmt.Errorf("failed to get SkyBlock items: %v", err)
	}

	constants.ITEMS = processItems(&items)

	// fmt.Printf("[ITEMS] Loaded %d items in %s\n", len(constants.ITEMS), time.Since(timeNow))

	// timeNow = time.Now()
	collections, err := GetSkyBlockCollections()
	if err != nil {
		return fmt.Errorf("failed to get SkyBlock collections: %v", err)
	}

	constants.COLLECTIONS = processCollections(collections)

	// fmt.Printf("[COLLECTIONS] Loaded %d collections in %s\n", len(constants.COLLECTIONS), time.Since(timeNow))

	return nil
}
