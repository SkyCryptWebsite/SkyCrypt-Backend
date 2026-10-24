package routes

import (
	"context"
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

// ProfileInput names a player and one of their SkyBlock profiles. Input types are exported because Huma skips
// embedded structs of unexported types, and handlers embed them.
type ProfileInput struct {
	UUID      string `path:"uuid" doc:"Minecraft UUID or username" example:"4855c53ee4fb4100997600a92fc50984"`
	ProfileID string `path:"profileId" doc:"Hypixel SkyBlock profile UUID or cute name" example:"00912956-3fd6-42ee-a166-3f649ceaf559"`
}

// MemberInput names a profile member by UUID, for the endpoints that read the profile without a Mojang lookup.
type MemberInput struct {
	UUID      string `path:"uuid" doc:"Minecraft UUID of the profile member" example:"4855c53ee4fb4100997600a92fc50984"`
	ProfileID string `path:"profileId" doc:"Hypixel SkyBlock profile UUID or cute name" example:"00912956-3fd6-42ee-a166-3f649ceaf559"`
}

// PlayerInput names a player, for the endpoints that use their selected profile.
type PlayerInput struct {
	UUID string `path:"uuid" doc:"Minecraft UUID or username" example:"4855c53ee4fb4100997600a92fc50984"`
}

// profileErrors documents the failures of an endpoint that loads a profile; internal describes its 500.
func profileErrors(humaAPI huma.API, internal string) map[string]*huma.Response {
	return errorResponses(humaAPI, map[int]string{
		fiber.StatusNotFound:            "PLAYER_NOT_FOUND or PROFILE_NOT_FOUND: no such player or profile. NOT_CACHED: the player isn't cached, and the key lacks the cache-bypass scope.",
		fiber.StatusInternalServerError: "INTERNAL_ERROR: " + internal,
		fiber.StatusBadGateway:          "UPSTREAM_ERROR: Hypixel or Mojang failed or sent an unusable response.",
		fiber.StatusGatewayTimeout:      "UPSTREAM_TIMEOUT: Hypixel or Mojang didn't answer in time.",
	})
}

func registerStats(humaAPI huma.API) {
	responses := profileErrors(humaAPI, "Player, profile, museum, or stats data could not be loaded.")
	responses["200"] = jsonResponse[models.StatsOutput](humaAPI, "Profile stats returned successfully.")
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "getProfileStats",
		Method:      fiber.MethodGet,
		Path:        "/api/stats/{uuid}/{profileId}",
		Summary:     "Get profile stats",
		Description: "Returns the complete SkyCrypt stats payload for a player on a specific SkyBlock profile.\n" +
			"The player identifier can be a Minecraft UUID or username. The profile identifier can be a Hypixel profile UUID or profile cute name.",
		Tags:      []string{"Stats"},
		Responses: responses,
	}), func(ctx context.Context, in *ProfileInput) (*jsonOutput, error) {
		return statsResponse(ctx, in.UUID, in.ProfileID)
	})

	responses = profileErrors(humaAPI, "Player, profile, museum, or stats data could not be loaded.")
	responses["200"] = jsonResponse[models.StatsOutput](humaAPI, "Selected profile stats returned successfully.")
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "getSelectedProfileStats",
		Method:      fiber.MethodGet,
		Path:        "/api/stats/{uuid}",
		Summary:     "Get selected profile stats",
		Description: "Returns the complete SkyCrypt stats payload for a player's selected SkyBlock profile.\n" +
			"If Hypixel does not mark a selected profile, the first available profile is used.",
		Tags:      []string{"Stats"},
		Responses: responses,
	}), func(ctx context.Context, in *PlayerInput) (*jsonOutput, error) {
		return statsResponse(ctx, in.UUID, "")
	})
}

func statsResponse(reqCtx context.Context, uuid string, profileId string) (*jsonOutput, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Stats")()
	}

	timeNow := time.Now()

	// Check the cache under the request as given before resolving the player: a hit then needs no identity lookup,
	// so a cached response still loads when Mojang is down.
	rawCacheKey := responseCacheKey("stats", uuid, profileId)
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

	cacheKey := responseCacheKey("stats", mowojang.UUID, profileId)
	if cacheKey.key != rawCacheKey.key {
		if cached := cachedJSON(reqCtx, cacheKey); cached != nil {
			return cached, nil
		}
	}

	body, err := sharedResponse(reqCtx, cacheKey, rawCacheKey, 5*60, func(ctx context.Context) ([]byte, error) {
		output, err := computeStatsContext(ctx, mowojang, profileId)
		if err != nil {
			return nil, failedTo("failed to load data", err)
		}
		return marshalBody(output)
	})
	if err != nil {
		return nil, buildFailure(err)
	}

	utility.LogVerbose("Returning /api/stats/%s in %s", uuid, time.Since(timeNow))
	return jsonBody(reqCtx, cacheKey, body), nil
}

func computeStatsContext(ctx context.Context, mowojang *models.MowojangResponse, profileId string) (*models.StatsOutput, error) {
	data, err := loadProfileData(ctx, mowojang, profileId, profileParts{player: true, museum: true, name: true})
	if err != nil {
		return nil, fmt.Errorf("failed to load profile: %w", err)
	}

	members, err := stats.FormatMembersContext(ctx, data.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to format members: %v", err)
	}

	return stats.GetStats(data.Mowojang, data.Profiles, data.Profile, data.Player, data.Member(), data.Museum(), members)
}
