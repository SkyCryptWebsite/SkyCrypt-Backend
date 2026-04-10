package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"time"

	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
	"github.com/gofiber/fiber/v2"
)

// NetworthHandler godoc
//
//	@Summary		Get networth of a specified player
//	@Description	Returns networth for the given user and profile ID
//	@Tags			networth
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.Networth
//	@Failure		400			{object}	models.ProcessingError
//	@Failure		500			{object}	models.ProcessingError
//	@Router			/api/networth/{uuid}/{profileId} [get]
func NetworthHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Networth")()
	}

	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")
	if len(profileId) > 0 && profileId[0] == '/' {
		profileId = profileId[1:]
	}

	mowojang, err := api.ResolvePlayer(uuid)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to resolve player: %v", err),
		})
	}

	profiles, err := api.GetProfiles(mowojang.UUID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get profile: %v", err),
		})
	}

	profile, err := stats.GetProfile(profiles, profileId)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get profile: %v", err),
		})
	}

	profileMuseum, err := api.GetMuseum(profile.ProfileID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get museum: %v", err),
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

	utility.LogVerbose("Returning /api/networth/%s in %s", uuid, time.Since(timeNow))

	return c.JSON(fiber.Map{
		"normal":      networth,
		"nonCosmetic": nonCosmeticNetworth,
	})
}
