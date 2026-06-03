package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

// EmbedHandler godoc
//
//	@Summary		Get embed data for a specified player
//	@Description	Returns embed data for the given user (UUID or username) and optional profile ID
//	@Tags			embed
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID or username"
//	@Param			profileId	query		string	false	"Profile ID (optional, defaults to selected profile)"
//	@Success		200			{object}	models.EmbedData
//	@Failure		400			{object}	models.ProcessingError
//	@Failure		500			{object}	models.ProcessingError
//	@Router			/api/embed/{uuid} [get]
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

	var (
		mowojang      *models.MowojangResponse
		profiles      *models.HypixelProfilesResponse
		profile       *skycrypttypes.Profile
		player        *skycrypttypes.Player
		profileMuseum map[string]*skycrypttypes.Museum
		err           error
	)

	isProfileUUID := utility.IsUUID(profileId)
	g, _ := errgroup.WithContext(c.Context())

	if isProfileUUID {
		g.Go(func() error {
			var err error
			profileMuseum, err = api.GetMuseum(profileId)
			return err
		})
	}

	mowojang, err = api.ResolvePlayer(uuid)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to resolve player: %v", err),
		})
	}

	g.Go(func() error {
		var err error
		player, err = api.GetPlayer(mowojang.UUID)
		return err
	})

	g.Go(func() error {
		var err error
		profiles, err = api.GetProfiles(mowojang.UUID)
		if err != nil {
			return err
		}

		profile, err = stats.GetProfile(profiles, profileId)
		if err != nil {
			return err
		}

		if !isProfileUUID {
			profileMuseum, err = api.GetMuseum(profile.ProfileID)
			return err
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to fetch data: %v", err),
		})
	}

	userProfileValue := profile.Members[mowojang.UUID]
	museum := profileMuseum[mowojang.UUID]
	userProfile := &userProfileValue

	var bankBalance float64
	if profile.Banking != nil && profile.Banking.Balance != nil {
		bankBalance = *profile.Banking.Balance
	} else {
		bankBalance = 0.0
	}

	calculator, err := skyhelpernetworthgo.NewProfileNetworthCalculator(userProfile, museum, bankBalance)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to create networth calculator: %v", err),
		})
	}

	networth := calculator.GetNetworth(skyhelpernetworthgo.NetworthOptions{OnlyNetworth: true})
	nonCosmeticNetworth := calculator.GetNonCosmeticNetworth(skyhelpernetworthgo.NetworthOptions{OnlyNetworth: true})
	formattedNetworth := map[string]float64{
		"normal":      networth.Networth,
		"nonCosmetic": nonCosmeticNetworth.Networth,
	}

	output := stats.GetEmbedData(mowojang, player, userProfile, profile, formattedNetworth)

	utility.LogVerbose("Returning /api/embed/%s/%s in %s", uuid, profileId, time.Since(timeNow))

	return c.JSON(output)
}
