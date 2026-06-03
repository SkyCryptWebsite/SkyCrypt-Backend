package routes

import (
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

// GardenHandler godoc
//
//	@Summary		Get garden stats of a specified profile
//	@Description	Returns garden data for the given profile ID
//	@Tags			garden
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//
// @Success		200			{object}	models.Garden
// @Failure		400			{object}	models.ProcessingError
// @Router			/api/garden/{uuid}/{profileId} [get]
func GardenHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Garden")()
	}

	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")

	var (
		profiles *models.HypixelProfilesResponse
		profile  *skycrypttypes.Profile
		garden   *skycrypttypes.Garden
	)

	isProfileUUID := utility.IsUUID(profileId)
	g, _ := errgroup.WithContext(c.Context())

	if isProfileUUID {
		g.Go(func() error {
			var err error
			garden, err = api.GetGarden(profileId)
			return err
		})
	}

	g.Go(func() error {
		var err error
		profiles, err = api.GetProfiles(uuid)
		if err != nil {
			return err
		}

		profile, err = stats.GetProfile(profiles, profileId)
		if err != nil {
			return err
		}

		if !isProfileUUID {
			garden, err = api.GetGarden(profile.ProfileID)
			return err
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}

	userProfileValue := profile.Members[uuid]
	userProfile := &userProfileValue

	output := stats.GetGarden(userProfile, garden)

	utility.LogVerbose("Returning /api/garden/%s in %s", profileId, time.Since(timeNow))

	return c.JSON(output)
}
