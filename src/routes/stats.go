package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

// StatsHandler godoc
//
//	@Summary		Get stats of a specified player
//	@Description	Returns stats for the given user and profile ID
//	@Tags			stats
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.StatsOutput
//	@Failure		400			{object}	models.ProcessingError
//	@Failure		500			{object}	models.ProcessingError
//	@Router			/api/stats/{uuid}/{profileId} [get]
func StatsHandler(c *fiber.Ctx) error {
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

	var profiles *models.HypixelProfilesResponse
	var player *skycrypttypes.Player

	var g errgroup.Group
	g.Go(func() error {
		var err error
		profiles, err = api.GetProfiles(mowojang.UUID)
		return err
	})
	g.Go(func() error {
		var err error
		player, err = api.GetPlayer(mowojang.UUID)
		return err
	})
	if err := g.Wait(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get player data: %v", err),
		})
	}

	// Now GetMuseum and FormatMembers can also run in parallel
	profile, err := stats.GetProfile(profiles, profileId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get profile: %v", err),
		})
	}

	var profileMuseum map[string]*skycrypttypes.Museum
	var members []*models.MemberStats

	g2, _ := errgroup.WithContext(c.Context())
	g2.Go(func() error {
		var err error
		profileMuseum, err = api.GetMuseum(profile.ProfileID)
		return err
	})
	g2.Go(func() error {
		var err error
		members, err = stats.FormatMembers(profile)
		return err
	})
	if err := g2.Wait(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get profile museum or members: %v", err),
		})
	}

	userProfileValue := profile.Members[mowojang.UUID]
	museum := profileMuseum[mowojang.UUID]
	userProfile := &userProfileValue

	output, err := stats.GetStats(mowojang, profiles, profile, player, userProfile, museum, members)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get stats: %v", err),
		})
	}

	fmt.Printf("Returning /api/stats/%s in %s\n", uuid, time.Since(timeNow))

	return c.JSON(output)
}
