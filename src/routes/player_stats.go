package routes

import (
	"fmt"
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
//	@Security		ApiTokenHeader
//	@Param			uuid		path		string	true	"Minecraft UUID or username"	example(4855c53ee4fb4100997600a92fc50984)
//	@Param			profileId	path		string	true	"Hypixel SkyBlock profile UUID or cute name"	example(00912956-3fd6-42ee-a166-3f649ceaf559)
//	@Success		200			{object}	models.Stats			"Player stats returned successfully."
//	@Failure		400			{object}	models.ProcessingError	"Player or profile could not be resolved."
//	@Failure		401			{object}	models.ProcessingError	"X-API-Token is missing or invalid."
//	@Router			/api/playerStats/{uuid}/{profileId} [get]
func PlayerStatsHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.PlayerStats")()
	}

	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")
	reqCtx := c.UserContext()

	cacheKey := responseCacheKey("playerStats", uuid, profileId)

	if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
		return err
	}

	resolvedPlayer, err := api.ResolvePlayerContext(reqCtx, uuid)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to resolve player: %v", err),
		})
	}

	resolvedUUID := resolvedPlayer.UUID

	profile, err := api.GetProfileContext(
		reqCtx,
		resolvedUUID,
		profileId,
	)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get profile: %v", err),
		})
	}

	userProfileValue := profile.Members[resolvedUUID]
	userProfile := &userProfileValue

	output := stats.GetPlayerStats(
		userProfile,
		profile,
		profile.ProfileID,
		resolvedUUID,
	)

	utility.LogVerbose(
		"Returning /api/playerStats/%s in %s",
		profileId,
		time.Since(timeNow),
	)

	formattedStats := models.Stats{
		Stats: output,
	}

	return sendAndCacheJSON(
		c,
		reqCtx,
		cacheKey,
		formattedStats,
		5*60,
	)
}
