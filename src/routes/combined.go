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

	"github.com/gofiber/fiber/v2"
)

// CombinedHandler godoc
//
//	@Summary		Get combined profile data
//	@Description	Returns the combined SkyCrypt payload for a player's SkyBlock profile, including gear, saved loadouts, accessories, pets, skills, dungeons, slayers, collections, minions, bestiary, and other profile sections.
//	@Description	Resource pack preferences supplied by cookie can affect rendered item texture URLs in the response.
//	@ID				getCombinedProfileStats
//	@Tags			Stats
//	@Produce		json
//	@Security		ApiKeyHeader
//	@Param			uuid		path		string	true	"Minecraft UUID or username"	example(4855c53ee4fb4100997600a92fc50984)
//	@Param			profileId	path		string	true	"Hypixel SkyBlock profile UUID or cute name"	example(00912956-3fd6-42ee-a166-3f649ceaf559)
//	@Success		200			{object}	models.CombinedOutput	"Combined profile data returned successfully."
//	@Failure		401			{object}	models.APIError	"The x-api-key header is missing, or the key is invalid, disabled, or expired."
//	@Failure		404			{object}	models.APIError	"PLAYER_NOT_FOUND or PROFILE_NOT_FOUND: no such player or profile. NOT_CACHED: the player isn't cached, and the key lacks the cache-bypass scope."
//	@Failure		429			{object}	models.APIError	"RATE_LIMITED: the key's rate limit is used up. See the Retry-After header."
//	@Failure		500			{object}	models.APIError	"INTERNAL_ERROR: Player, profile, museum, or combined data could not be loaded."
//	@Failure		502			{object}	models.APIError	"UPSTREAM_ERROR: Hypixel or Mojang failed or sent an unusable response."
//	@Failure		503			{object}	models.APIError	"API keys can't be verified right now."
//	@Failure		504			{object}	models.APIError	"UPSTREAM_TIMEOUT: Hypixel or Mojang didn't answer in time."
//	@Router			/api/combined/{uuid}/{profileId} [get]
func CombinedHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Combined")()
	}

	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")
	if len(profileId) > 0 && profileId[0] == '/' {
		profileId = profileId[1:]
	}

	enabledPacks := enabledPacksFromRequest(c)

	reqCtx := c.UserContext()
	// Check the cache under the request as given before resolving the player: a hit then needs no identity lookup,
	// so a cached response still loads when Mojang is down.
	rawCacheKey := responseCacheKey("combined", uuid, profileId, enabledPacksCachePart(enabledPacks))
	if ok, err := sendCachedJSON(c, rawCacheKey); ok || err != nil {
		return err
	}

	mowojang, err := api.ResolvePlayerContext(reqCtx, uuid)
	if err != nil {
		return sendLoadError(c, "failed to resolve player", err)
	}

	cacheKey := responseCacheKey("combined", mowojang.UUID, profileId, enabledPacksCachePart(enabledPacks))
	if cacheKey.key != rawCacheKey.key {
		if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
			return err
		}
	}

	body, err := sharedResponse(reqCtx, cacheKey, 5*60, func(ctx context.Context) ([]byte, error) {
		result, err := computeCombinedContext(ctx, mowojang, profileId, enabledPacks)
		if err != nil {
			return nil, failedTo("failed to load data", err)
		}
		return marshalBody(result)
	})
	if err != nil {
		return sendBuildError(c, err)
	}

	utility.LogVerbose("Returning /api/combined/%s in %s pid=%d", uuid, time.Since(timeNow), os.Getpid())
	// The shared build cached it under the resolved key; keep a copy under the request as given too.
	if cacheKey.key != rawCacheKey.key {
		cacheBody(reqCtx, rawCacheKey, body, 5*60)
	}
	return sendBody(c, reqCtx, cacheKey, body)
}

func computeCombinedContext(ctx context.Context, mowojang *models.MowojangResponse, profileId string, enabledPacks []string) (*models.CombinedOutput, error) {
	data, err := loadProfileData(ctx, mowojang, profileId, profileParts{player: true, museum: true})
	if err != nil {
		return nil, err
	}

	members, err := stats.FormatMembersContext(ctx, data.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to format members: %v", err)
	}

	return stats.GetCombinedContext(ctx, mowojang, data.Profiles, data.Profile, data.Player, data.Member(), data.Museum(), members, enabledPacks)
}
