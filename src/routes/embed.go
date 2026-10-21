package routes

import (
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"time"

	"github.com/gofiber/fiber/v2"
)

// EmbedHandler godoc
//
//	@Summary		Get profile embed data
//	@Description	Returns compact profile data intended for embeds and previews, including rank, SkyBlock level, skills, dungeons, slayers, purse, bank, and networth summary.
//	@Description	The player identifier can be a Minecraft UUID or username. The profile identifier can be a Hypixel profile UUID or profile cute name.
//	@ID				getProfileEmbed
//	@Tags			Discord Embed
//	@Produce		json
//	@Security		ApiKeyHeader
//	@Param			uuid		path		string	true	"Minecraft UUID or username"	example(4855c53ee4fb4100997600a92fc50984)
//	@Param			profileId	path		string	true	"Hypixel SkyBlock profile UUID or cute name"	example(00912956-3fd6-42ee-a166-3f649ceaf559)
//	@Success		200			{object}	models.EmbedData		"Profile embed data returned successfully."
//	@Failure		401			{object}	models.APIError	"The x-api-key header is missing, or the key is invalid, disabled, or expired."
//	@Failure		404			{object}	models.APIError	"PLAYER_NOT_FOUND or PROFILE_NOT_FOUND: no such player or profile. NOT_CACHED: the player isn't cached, and the key lacks the cache-bypass scope."
//	@Failure		429			{object}	models.APIError	"RATE_LIMITED: the key's rate limit is used up. See the Retry-After header."
//	@Failure		500			{object}	models.APIError	"INTERNAL_ERROR: Networth calculation failed."
//	@Failure		502			{object}	models.APIError	"UPSTREAM_ERROR: Hypixel or Mojang failed or sent an unusable response."
//	@Failure		503			{object}	models.APIError	"API keys can't be verified right now, or PRICES_UNAVAILABLE: item prices haven't loaded yet."
//	@Failure		504			{object}	models.APIError	"UPSTREAM_TIMEOUT: Hypixel or Mojang didn't answer in time."
//	@Router			/api/embed/{uuid}/{profileId} [get]
func EmbedHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Embed")()
	}

	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")
	if len(profileId) > 0 && profileId[0] == '/' {
		profileId = profileId[1:]
	}
	reqCtx := c.UserContext()

	mowojang, err := api.ResolvePlayerContext(reqCtx, uuid)
	if err != nil {
		return sendLoadError(c, "Failed to resolve player", err)
	}

	cacheKey := responseCacheKey("embed", mowojang.UUID, profileId)
	if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
		return err
	}

	data, err := loadProfileData(reqCtx, mowojang, profileId, profileParts{player: true, museum: true})
	if err != nil {
		return sendLoadError(c, "Failed to fetch data", err)
	}

	_, totals, err := profileNetworth(reqCtx, data)
	if err != nil {
		return sendLoadError(c, "Failed to create networth calculator", err)
	}
	formattedNetworth := map[string]float64{
		"normal":      totals.Normal,
		"nonCosmetic": totals.NonCosmetic,
	}

	output := stats.GetEmbedData(mowojang, data.Player, data.Member(), data.Profile, formattedNetworth)

	utility.LogVerbose("Returning /api/embed/%s/%s in %s", uuid, profileId, time.Since(timeNow))

	return sendAndCacheJSON(c, reqCtx, cacheKey, output, 60*60)
}

// SelectedProfileEmbedHandler godoc
//
//	@Summary		Get selected profile embed data
//	@Description	Returns compact profile data intended for embeds and previews for a player's selected SkyBlock profile.
//	@Description	If Hypixel does not mark a selected profile, the first available profile is used.
//	@ID				getSelectedProfileEmbed
//	@Tags			Discord Embed
//	@Produce		json
//	@Security		ApiKeyHeader
//	@Param			uuid	path		string	true	"Minecraft UUID or username"	example(4855c53ee4fb4100997600a92fc50984)
//	@Success		200		{object}	models.EmbedData		"Selected profile embed data returned successfully."
//	@Failure		401		{object}	models.APIError	"The x-api-key header is missing, or the key is invalid, disabled, or expired."
//	@Failure		404		{object}	models.APIError	"PLAYER_NOT_FOUND or PROFILE_NOT_FOUND: no such player or profile. NOT_CACHED: the player isn't cached, and the key lacks the cache-bypass scope."
//	@Failure		429		{object}	models.APIError	"RATE_LIMITED: the key's rate limit is used up. See the Retry-After header."
//	@Failure		500		{object}	models.APIError	"INTERNAL_ERROR: Networth calculation failed."
//	@Failure		502		{object}	models.APIError	"UPSTREAM_ERROR: Hypixel or Mojang failed or sent an unusable response."
//	@Failure		503		{object}	models.APIError	"API keys can't be verified right now, or PRICES_UNAVAILABLE: item prices haven't loaded yet."
//	@Failure		504		{object}	models.APIError	"UPSTREAM_TIMEOUT: Hypixel or Mojang didn't answer in time."
//	@Router			/api/embed/{uuid} [get]
func SelectedProfileEmbedHandler(c *fiber.Ctx) error {
	return EmbedHandler(c)
}
