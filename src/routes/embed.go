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
//	@Summary		Get profile embed data
//	@Description	Returns compact profile data intended for embeds and previews, including rank, SkyBlock level, skills, dungeons, slayers, purse, bank, and networth summary.
//	@Description	The player identifier can be a Minecraft UUID or username. The profile identifier can be a Hypixel profile UUID or profile cute name.
//	@ID				getProfileEmbed
//	@Tags			Discord Embed
//	@Produce		json
//	@Security		ApiTokenHeader
//	@Param			uuid		path		string	true	"Minecraft UUID or username"	example(4855c53ee4fb4100997600a92fc50984)
//	@Param			profileId	path		string	true	"Hypixel SkyBlock profile UUID or cute name"	example(00912956-3fd6-42ee-a166-3f649ceaf559)
//	@Success		200			{object}	models.EmbedData		"Profile embed data returned successfully."
//	@Failure		400			{object}	models.ProcessingError	"Player, profile, or museum data could not be resolved."
//	@Failure		401			{object}	models.ProcessingError	"X-API-Token is missing or invalid."
//	@Failure		500			{object}	models.ProcessingError	"Networth calculation failed."
//	@Router			/api/embed/{uuid}/{profileId} [get]
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
	reqCtx := c.UserContext()
	cacheKey := responseCacheKey("embed", uuid, profileId)
	if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
		return err
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
	g, groupCtx := errgroup.WithContext(reqCtx)
	type museumResult struct {
		profileID string
		museum    map[string]*skycrypttypes.Museum
		err       error
	}
	var selectedMuseumCh chan museumResult

	if isProfileUUID {
		g.Go(func() error {
			var err error
			profileMuseum, err = api.GetMuseumContext(groupCtx, profileId)
			return err
		})
	}

	mowojang, err = api.ResolvePlayerContext(reqCtx, uuid)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to resolve player: %v", err),
		})
	}

	g.Go(func() error {
		var err error
		player, err = api.GetPlayerContext(groupCtx, mowojang.UUID)
		return err
	})

	if profileId == "" {
		if cachedProfileID := getCachedSelectedProfileID(reqCtx, mowojang.UUID); cachedProfileID != "" {
			selectedMuseumCh = make(chan museumResult, 1)
			g.Go(func() error {
				museum, err := api.GetMuseumContext(groupCtx, cachedProfileID)
				selectedMuseumCh <- museumResult{profileID: cachedProfileID, museum: museum, err: err}
				return nil
			})
		}
	}

	g.Go(func() error {
		var err error
		profiles, err = api.GetProfilesContext(groupCtx, mowojang.UUID)
		if err != nil {
			return err
		}

		profile, err = stats.GetProfile(profiles, profileId)
		if err != nil {
			return err
		}
		if profileId == "" {
			cacheSelectedProfileID(groupCtx, mowojang.UUID, profile.ProfileID)
		}

		if !isProfileUUID {
			if selectedMuseumCh != nil {
				result := <-selectedMuseumCh
				if result.err == nil && result.profileID == profile.ProfileID {
					profileMuseum = result.museum
					return nil
				}
			}
			profileMuseum, err = api.GetMuseumContext(groupCtx, profile.ProfileID)
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

	return sendAndCacheJSON(c, reqCtx, cacheKey, output, 5*60)
}

// SelectedProfileEmbedHandler godoc
//
//	@Summary		Get selected profile embed data
//	@Description	Returns compact profile data intended for embeds and previews for a player's selected SkyBlock profile.
//	@Description	If Hypixel does not mark a selected profile, the first available profile is used.
//	@ID				getSelectedProfileEmbed
//	@Tags			Discord Embed
//	@Produce		json
//	@Security		ApiTokenHeader
//	@Param			uuid	path		string	true	"Minecraft UUID or username"	example(4855c53ee4fb4100997600a92fc50984)
//	@Success		200		{object}	models.EmbedData		"Selected profile embed data returned successfully."
//	@Failure		400		{object}	models.ProcessingError	"Player, profile, or museum data could not be resolved."
//	@Failure		401		{object}	models.ProcessingError	"X-API-Token is missing or invalid."
//	@Failure		500		{object}	models.ProcessingError	"Networth calculation failed."
//	@Router			/api/embed/{uuid} [get]
func SelectedProfileEmbedHandler(c *fiber.Ctx) error {
	return EmbedHandler(c)
}
