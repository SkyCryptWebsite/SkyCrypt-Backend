package routes

import (
	"context"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

func registerEmbed(humaAPI huma.API) {
	responses := profileErrors(humaAPI, "Networth calculation failed.")
	responses["200"] = jsonResponse[models.EmbedData](humaAPI, "Profile embed data returned successfully.")
	responses["503"] = errorResponse(humaAPI, pricesUnavailableDescription)
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "getProfileEmbed",
		Method:      fiber.MethodGet,
		Path:        "/api/embed/{uuid}/{profileId}",
		Summary:     "Get profile embed data",
		Description: "Returns compact profile data intended for embeds and previews, including rank, SkyBlock level, skills, dungeons, slayers, purse, bank, and networth summary.\n" +
			"The player identifier can be a Minecraft UUID or username. The profile identifier can be a Hypixel profile UUID or profile cute name.",
		Tags:      []string{"Discord Embed"},
		Responses: responses,
	}), func(ctx context.Context, in *ProfileInput) (*jsonOutput, error) {
		return embedResponse(ctx, in.UUID, in.ProfileID)
	})

	responses = profileErrors(humaAPI, "Networth calculation failed.")
	responses["200"] = jsonResponse[models.EmbedData](humaAPI, "Selected profile embed data returned successfully.")
	responses["503"] = errorResponse(humaAPI, pricesUnavailableDescription)
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "getSelectedProfileEmbed",
		Method:      fiber.MethodGet,
		Path:        "/api/embed/{uuid}",
		Summary:     "Get selected profile embed data",
		Description: "Returns compact profile data intended for embeds and previews for a player's selected SkyBlock profile.\n" +
			"If Hypixel does not mark a selected profile, the first available profile is used.",
		Tags:      []string{"Discord Embed"},
		Responses: responses,
	}), func(ctx context.Context, in *PlayerInput) (*jsonOutput, error) {
		return embedResponse(ctx, in.UUID, "")
	})
}

func embedResponse(reqCtx context.Context, uuid string, profileId string) (*jsonOutput, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Embed")()
	}

	timeNow := time.Now()

	// Check the cache under the request as given before resolving the player: a hit then needs no identity lookup,
	// so a cached response still loads when Mojang is down.
	rawCacheKey := responseCacheKey("embed", uuid, profileId)
	if cached := cachedJSON(reqCtx, rawCacheKey); cached != nil {
		return cached, nil
	}

	// A UUID is all the cache key and the profile fetch need, so its Mojang lookup runs alongside the fetch instead.
	mowojang := &models.MowojangResponse{UUID: uuid}
	if !utility.IsUUID(uuid) {
		resolved, err := api.ResolvePlayerContext(reqCtx, uuid)
		if err != nil {
			return nil, loadError("Failed to resolve player", err)
		}
		mowojang = resolved
	}

	cacheKey := responseCacheKey("embed", mowojang.UUID, profileId)
	if cacheKey.key != rawCacheKey.key {
		if cached := cachedJSON(reqCtx, cacheKey); cached != nil {
			return cached, nil
		}
	}

	body, err := sharedResponse(reqCtx, cacheKey, rawCacheKey, 60*60, func(ctx context.Context) ([]byte, error) {
		data, err := loadProfileData(ctx, mowojang, profileId, profileParts{player: true, museum: true, name: true})
		if err != nil {
			return nil, failedTo("Failed to fetch data", err)
		}

		_, totals, err := profileNetworth(ctx, data)
		if err != nil {
			return nil, failedTo("Failed to create networth calculator", err)
		}
		formattedNetworth := map[string]float64{
			"normal":      totals.Normal,
			"nonCosmetic": totals.NonCosmetic,
		}

		return marshalBody(stats.GetEmbedData(data.Mowojang, data.Player, data.Member(), data.Profile, formattedNetworth))
	})
	if err != nil {
		return nil, buildFailure(err)
	}

	utility.LogVerbose("Returning /api/embed/%s/%s in %s", uuid, profileId, time.Since(timeNow))
	return jsonBody(reqCtx, cacheKey, body), nil
}
