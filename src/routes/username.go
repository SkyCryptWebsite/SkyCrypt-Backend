package routes

import (
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/db"
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
//	@Failure		400		{object}	models.ProcessingError	"UUID is invalid or the player could not be found."
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
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}

	utility.LogVerbose("Returning /api/username/%s in %s", username, time.Since(timeNow))

	return sendAndCacheJSON(c, reqCtx, cacheKey, fiber.Map{
		"displayName": db.GetDisplayName(username, uuid),
		"username":    username,
		"uuid":        uuid,
	}, 24*60*60)
}
