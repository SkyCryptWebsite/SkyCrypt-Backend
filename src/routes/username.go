package routes

import (
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/utility"
	"time"

	"github.com/gofiber/fiber/v2"
)

// UsernameHandler godoc
//
//	@Summary		Resolve username by UUID
//	@Description	Resolves a Minecraft UUID to the current username and display name known to SkyCrypt.
//	@ID				resolveUsernameByUuid
//	@Tags			Mojang
//	@Produce		json
//	@Param			uuid	path		string	true	"Minecraft UUID"	example(4855c53ee4fb4100997600a92fc50984)
//	@Success		200		{object}	models.PlayerResolve		"Username resolved successfully."
//	@Failure		400		{object}	models.ProcessingError	"The UUID is malformed."
//	@Failure		404		{object}	models.APIError	"PLAYER_NOT_FOUND: no player has that name or UUID."
//	@Failure		500		{object}	models.APIError	"INTERNAL_ERROR: Unexpected failure."
//	@Failure		502		{object}	models.APIError	"UPSTREAM_ERROR: Hypixel or Mojang failed or sent an unusable response."
//	@Failure		504		{object}	models.APIError	"UPSTREAM_TIMEOUT: Hypixel or Mojang didn't answer in time."
//	@Router			/api/username/{uuid} [get]
func UsernameHandler(c *fiber.Ctx) error {
	timeNow := time.Now()

	uuid := c.Params("uuid")
	if !utility.IsUUID(uuid) {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}
	reqCtx := c.UserContext()
	cacheKey := responseCacheKey("username", uuid)
	if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
		return err
	}

	username, err := api.GetUsernameContext(reqCtx, uuid)
	if err != nil {
		return sendLoadError(c, "Failed to resolve player", err)
	}

	utility.LogVerbose("Returning /api/username/%s in %s", username, time.Since(timeNow))

	return sendAndCacheJSON(c, reqCtx, cacheKey, fiber.Map{
		"username": username,
		"uuid":     uuid,
	}, 24*60*60)
}
