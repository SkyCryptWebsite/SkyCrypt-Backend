package routes

import (
	"context"
	"fmt"
	"skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"

	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	jsoniter "github.com/json-iterator/go"
)

func registerInventorySearch(humaAPI huma.API) {
	responses := errorResponses(humaAPI, map[int]string{
		fiber.StatusNotFound:            "NOT_CACHED: No inventory is cached for this player, profile and packs; call the inventory endpoint first.",
		fiber.StatusInternalServerError: "INTERNAL_ERROR: Cached inventory data could not be read or parsed.",
	})
	responses["200"] = &huma.Response{Description: "Matching inventory items returned successfully."}
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "searchProfileInventory",
		Method:      fiber.MethodGet,
		Path:        "/api/inventory/search/{uuid}/{profileId}/{searchParam}",
		Summary:     "Search cached profile inventory",
		Description: "Searches cached inventory tabs for items whose display name or lore contains the search parameter.\n" +
			"Call GET /api/inventory/{uuid}/{profileId} first to refresh the short-lived cache for the same player, profile, and resource pack preferences. At most 45 matching items are returned.",
		Tags:      []string{"Stats"},
		Responses: responses,
	}), func(ctx context.Context, in *struct {
		MemberInput
		SearchParam string `path:"searchParam" doc:"Search text matched against item display names and lore, ignoring case" example:"hyperion"`
		PacksInput
	}) (*bodyOutput[[]models.StrippedItem], error) {
		return searchInventory(ctx, in.UUID, in.ProfileID, in.SearchParam, in.enabledPacks)
	})
}

func searchInventory(ctx context.Context, uuid string, profileId string, searchParam string, enabledPacks []string) (*bodyOutput[[]models.StrippedItem], error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Inventory")()
	}

	timeNow := time.Now()

	// Members is keyed by the lowercase UUID, and the cache key is lowercased too.
	uuid = strings.ToLower(uuid)
	searchParam = strings.ToLower(searchParam)

	cache, err := db.GetBytesContext(ctx, inventoryCacheKey(uuid, profileId, enabledPacks).key)
	if err != nil {
		return nil, NewAPIError(fiber.StatusInternalServerError, codeInternalError, fmt.Sprintf("Failed to get items: %v", err))
	}
	if len(cache) == 0 {
		return nil, NewAPIError(fiber.StatusNotFound, codeNotCached, "No cached items found for this player and profile. Load their inventory first.")
	}

	var inventories []models.Inventory
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(cache, &inventories); err != nil {
		return nil, NewAPIError(fiber.StatusInternalServerError, codeInternalError, fmt.Sprintf("Failed to parse items: %v", err))
	}

	output := []models.StrippedItem{}
search:
	for _, inventory := range inventories {
		if inventory.Items == nil {
			continue
		}

		for _, item := range inventory.Items {
			if item.DisplayName == "" || !itemMatchesSearch(item.DisplayName, item.Lore, searchParam) {
				continue
			}

			item.SourceTab = &models.SourceTab{
				Icon: inventory.Texture,
				Name: inventory.Name,
			}

			output = append(output, item)

			if len(output) >= 5*9 {
				break search
			}
		}
	}

	utility.LogVerbose("Returning /api/inventory/search/%s/%s/%s in %s", uuid, profileId, searchParam, time.Since(timeNow))

	return &bodyOutput[[]models.StrippedItem]{Body: output}, nil
}

// itemMatchesSearch reports whether the name or lore contains query, ignoring case. query must already be lowercase.
func itemMatchesSearch(displayName string, lore []string, query string) bool {
	return strings.Contains(strings.ToLower(displayName), query) || strings.Contains(strings.ToLower(strings.Join(lore, " ")), query)
}
