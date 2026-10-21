package routes

import (
	"context"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"time"

	"github.com/gofiber/fiber/v2"
)

// PlayerStatsHandler godoc
//
//	@Summary		Get raw player stats
//	@Description	Returns normalized player statistics for a specific SkyBlock profile.
//	@Description	The player identifier can be a Minecraft UUID or username. The profile identifier can be a Hypixel profile UUID or profile cute name.
//	@ID				getPlayerStats
//	@Tags			Stats
//	@Produce		json
//	@Security		ApiKeyHeader
//	@Param			uuid		path		string	true	"Minecraft UUID or username"	example(4855c53ee4fb4100997600a92fc50984)
//	@Param			profileId	path		string	true	"Hypixel SkyBlock profile UUID or cute name"	example(00912956-3fd6-42ee-a166-3f649ceaf559)
//	@Success		200			{object}	models.Stats			"Player stats returned successfully."
//	@Failure		401			{object}	models.APIError	"The x-api-key header is missing, or the key is invalid, disabled, or expired."
//	@Failure		404			{object}	models.APIError	"PLAYER_NOT_FOUND or PROFILE_NOT_FOUND: no such player or profile. NOT_CACHED: the player isn't cached, and the key lacks the cache-bypass scope."
//	@Failure		429			{object}	models.APIError	"RATE_LIMITED: the key's rate limit is used up. See the Retry-After header."
//	@Failure		500			{object}	models.APIError	"INTERNAL_ERROR: Unexpected failure."
//	@Failure		502			{object}	models.APIError	"UPSTREAM_ERROR: Hypixel or Mojang failed or sent an unusable response."
//	@Failure		503			{object}	models.APIError	"API keys can't be verified right now."
//	@Failure		504			{object}	models.APIError	"UPSTREAM_TIMEOUT: Hypixel or Mojang didn't answer in time."
//	@Router			/api/playerStats/{uuid}/{profileId} [get]
func PlayerStatsHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.PlayerStats")()
	}

	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")
	reqCtx := c.UserContext()

	// Check the cache under the request as given before resolving the player: a hit then needs no identity lookup,
	// so a cached response still loads when Mojang is down.
	rawCacheKey := responseCacheKey("playerStats", uuid, profileId)
	if ok, err := sendCachedJSON(c, rawCacheKey); ok || err != nil {
		return err
	}

	resolvedPlayer, err := api.ResolvePlayerContext(reqCtx, uuid)
	if err != nil {
		return sendLoadError(c, "Failed to resolve player", err)
	}

	resolvedUUID := resolvedPlayer.UUID
	cacheKey := responseCacheKey("playerStats", resolvedUUID, profileId)
	if cacheKey.key != rawCacheKey.key {
		if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
			return err
		}
	}

	body, err := sharedResponse(reqCtx, cacheKey, 5*60, func(ctx context.Context) ([]byte, error) {
		profile, err := api.GetProfileContext(ctx, resolvedUUID, profileId)
		if err != nil {
			return nil, failedTo("Failed to get profile", err)
		}

		userProfileValue := profile.Members[resolvedUUID]
		output := stats.GetPlayerStats(&userProfileValue, profile, profile.ProfileID, resolvedUUID)
		return marshalBody(models.Stats{Stats: output})
	})
	if err != nil {
		return sendBuildError(c, err)
	}

	utility.LogVerbose("Returning /api/playerStats/%s in %s", profileId, time.Since(timeNow))

	// The shared build cached it under the resolved key; keep a copy under the request as given too.
	if cacheKey.key != rawCacheKey.key {
		cacheBody(reqCtx, rawCacheKey, body, 5*60)
	}
	return sendBody(c, reqCtx, cacheKey, body)
}
