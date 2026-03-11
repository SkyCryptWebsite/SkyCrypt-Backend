package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

// MiscHandler godoc
//
//	@Summary		Get misc stats of a specified player
//	@Description	Returns misc stats for the given user and profile ID
//	@Tags			misc
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.MiscOutput
//	@Failure		400			{object}	models.ProcessingError
//	@Router			/api/misc/{uuid}/{profileId} [get]
func MiscHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Misc")()
	}
	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")

	var profile *skycrypttypes.Profile
	var player *skycrypttypes.Player

	var g errgroup.Group
	g.Go(func() error {
		var err error
		profile, err = api.GetProfile(uuid, profileId)
		return err
	})
	g.Go(func() error {
		var err error
		player, err = api.GetPlayer(uuid)
		return err
	})
	if err := g.Wait(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get profile/player: %v", err),
		})
	}

	userProfileValue := profile.Members[uuid]
	userProfile := &userProfileValue

	output := stats.GetMisc(userProfile, profile, player)

	utility.LogVerbose("Returning /api/misc/%s in %s", profileId, time.Since(timeNow))

	return c.JSON(output)
}
