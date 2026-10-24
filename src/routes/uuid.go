package routes

import (
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/utility"
	"time"

	"github.com/gofiber/fiber/v3"
)

// UUIDHandler godoc
//
//	@Summary		Resolve UUID by username
//	@Description	Resolves a Minecraft username to its UUID and returns the normalized player identity payload.
//	@ID				resolveUuidByUsername
//	@Tags			Mojang
//	@Produce		json
//	@Security		ApiKeyHeader
//	@Param			username	path		string	true	"Minecraft username"	minlength(3)	maxlength(16)	example(duckysolucky)
//	@Success		200			{object}	models.PlayerResolve		"UUID resolved successfully."
//	@Failure		400			{object}	models.ProcessingError	"The username is malformed."
//	@Failure		401			{object}	models.APIError	"The x-api-key header is missing, or the key is invalid, disabled, or expired."
//	@Failure		404			{object}	models.APIError	"PLAYER_NOT_FOUND: no player has that name or UUID."
//	@Failure		429			{object}	models.APIError	"RATE_LIMITED: the key's rate limit is used up. See the Retry-After header."
//	@Failure		500			{object}	models.APIError	"INTERNAL_ERROR: Unexpected failure."
//	@Failure		502			{object}	models.APIError	"UPSTREAM_ERROR: Hypixel or Mojang failed or sent an unusable response."
//	@Failure		503			{object}	models.APIError	"API keys can't be verified right now."
//	@Failure		504			{object}	models.APIError	"UPSTREAM_TIMEOUT: Hypixel or Mojang didn't answer in time."
//	@Router			/api/uuid/{username} [get]
func UUIDHandler(c fiber.Ctx) error {
	timeNow := time.Now()
	username := c.Params("username")
	if username == "" || len(username) < 3 || len(username) > 16 {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}
	reqCtx := c.Context()
	cacheKey := responseCacheKey("uuid", username)
	if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
		return err
	}

	uuid, err := api.GetUUIDContext(reqCtx, username)
	if err != nil {
		return sendLoadError(c, "Failed to resolve player", err)
	}

	utility.LogVerbose("Returning /api/username/%s in %s\n", username, time.Since(timeNow))

	return sendAndCacheJSON(c, reqCtx, cacheKey, fiber.Map{
		"username": username,
		"uuid":     uuid,
	}, 24*60*60)
}
