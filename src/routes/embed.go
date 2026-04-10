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

	player, err := api.GetPlayer(mowojang.UUID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get player data: %v", err),
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
	formattedNetworth := map[string]float64{
		"normal":      networth.Networth,
		"nonCosmetic": nonCosmeticNetworth.Networth,
	}

	output := stats.GetEmbedData(mowojang, player, userProfile, profile, formattedNetworth)

	utility.LogVerbose("Returning /api/embed/%s in %s", profileId, time.Since(timeNow))

	return c.JSON(output)
}
