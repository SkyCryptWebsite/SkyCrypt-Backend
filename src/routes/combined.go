package routes

import (
	"context"
	"fmt"
	"os"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

func registerCombined(humaAPI huma.API) {
	responses := profileErrors(humaAPI, "Player, profile, museum, or combined data could not be loaded.")
	responses["200"] = jsonResponse[models.CombinedOutput](humaAPI, "Combined profile data returned successfully.")
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "getCombinedProfileStats",
		Method:      fiber.MethodGet,
		Path:        "/api/combined/{uuid}/{profileId}",
		Summary:     "Get combined profile data",
		Description: "Returns the combined SkyCrypt payload for a player's SkyBlock profile, including gear, saved loadouts, accessories, pets, skills, dungeons, slayers, collections, minions, bestiary, and other profile sections.\n" +
			"Resource pack preferences supplied by cookie can affect rendered item texture URLs in the response.",
		Tags:      []string{"Stats"},
		Responses: responses,
	}), func(ctx context.Context, in *struct {
		ProfileInput
		PacksInput
	}) (*jsonOutput, error) {
		return combinedResponse(ctx, in.UUID, in.ProfileID, in.enabledPacks)
	})
}

func combinedResponse(reqCtx context.Context, uuid string, profileId string, enabledPacks []string) (*jsonOutput, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Combined")()
	}

	timeNow := time.Now()

	// Check the cache under the request as given before resolving the player: a hit then needs no identity lookup,
	// so a cached response still loads when Mojang is down.
	rawCacheKey := responseCacheKey("combined", uuid, profileId, enabledPacksCachePart(enabledPacks))
	if cached := cachedJSON(reqCtx, rawCacheKey); cached != nil {
		return cached, nil
	}

	// A UUID is all the cache key and the profile fetch need, so its Mojang lookup runs alongside the fetch instead.
	mowojang := &models.MowojangResponse{UUID: uuid}
	if !utility.IsUUID(uuid) {
		resolved, err := api.ResolvePlayerContext(reqCtx, uuid)
		if err != nil {
			return nil, loadError("failed to resolve player", err)
		}
		mowojang = resolved
	}

	cacheKey := responseCacheKey("combined", mowojang.UUID, profileId, enabledPacksCachePart(enabledPacks))
	if cacheKey.key != rawCacheKey.key {
		if cached := cachedJSON(reqCtx, cacheKey); cached != nil {
			return cached, nil
		}
	}

	body, err := sharedResponse(reqCtx, cacheKey, rawCacheKey, 5*60, func(ctx context.Context) ([]byte, error) {
		result, err := computeCombinedContext(ctx, mowojang, profileId, enabledPacks)
		if err != nil {
			return nil, failedTo("failed to load data", err)
		}
		return marshalBody(result)
	})
	if err != nil {
		return nil, buildFailure(err)
	}

	utility.LogVerbose("Returning /api/combined/%s in %s pid=%d", uuid, time.Since(timeNow), os.Getpid())
	return jsonBody(reqCtx, cacheKey, body), nil
}

func computeCombinedContext(ctx context.Context, mowojang *models.MowojangResponse, profileId string, enabledPacks []string) (*models.CombinedOutput, error) {
	data, err := loadProfileData(ctx, mowojang, profileId, profileParts{player: true, museum: true, name: true})
	if err != nil {
		return nil, err
	}

	members, err := stats.FormatMembersContext(ctx, data.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to format members: %v", err)
	}

	return stats.GetCombinedContext(ctx, data.Mowojang, data.Profiles, data.Profile, data.Player, data.Member(), data.Museum(), members, enabledPacks)
}
