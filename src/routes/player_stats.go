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

func registerPlayerStats(humaAPI huma.API) {
	responses := profileErrors(humaAPI, "Unexpected failure.")
	responses["200"] = jsonResponse[models.Stats](humaAPI, "Player stats returned successfully.")
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "getPlayerStats",
		Method:      fiber.MethodGet,
		Path:        "/api/playerStats/{uuid}/{profileId}",
		Summary:     "Get raw player stats",
		Description: "Returns normalized player statistics for a specific SkyBlock profile.\n" +
			"The player identifier can be a Minecraft UUID or username. The profile identifier can be a Hypixel profile UUID or profile cute name.",
		Tags:      []string{"Stats"},
		Responses: responses,
	}), func(ctx context.Context, in *ProfileInput) (*jsonOutput, error) {
		return playerStatsResponse(ctx, in.UUID, in.ProfileID)
	})
}

func playerStatsResponse(reqCtx context.Context, uuid string, profileId string) (*jsonOutput, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.PlayerStats")()
	}

	timeNow := time.Now()

	// Check the cache under the request as given before resolving the player: a hit then needs no identity lookup,
	// so a cached response still loads when Mojang is down.
	rawCacheKey := responseCacheKey("playerStats", uuid, profileId)
	if cached := cachedJSON(reqCtx, rawCacheKey); cached != nil {
		return cached, nil
	}

	// Player stats only need the UUID, so skip the Mojang lookup when that is what was given.
	resolvedUUID := uuid
	if !utility.IsUUID(uuid) {
		resolvedPlayer, err := api.ResolvePlayerContext(reqCtx, uuid)
		if err != nil {
			return nil, loadError("Failed to resolve player", err)
		}
		resolvedUUID = resolvedPlayer.UUID
	}
	cacheKey := responseCacheKey("playerStats", resolvedUUID, profileId)
	if cacheKey.key != rawCacheKey.key {
		if cached := cachedJSON(reqCtx, cacheKey); cached != nil {
			return cached, nil
		}
	}

	body, err := sharedResponse(reqCtx, cacheKey, rawCacheKey, 5*60, func(ctx context.Context) ([]byte, error) {
		profile, err := api.GetProfileContext(ctx, resolvedUUID, profileId)
		if err != nil {
			return nil, failedTo("Failed to get profile", err)
		}

		userProfileValue := profile.Members[resolvedUUID]
		output := stats.GetPlayerStats(&userProfileValue, profile, profile.ProfileID, resolvedUUID)
		return marshalBody(models.Stats{Stats: output})
	})
	if err != nil {
		return nil, buildFailure(err)
	}

	utility.LogVerbose("Returning /api/playerStats/%s in %s", profileId, time.Since(timeNow))
	return jsonBody(reqCtx, cacheKey, body), nil
}
