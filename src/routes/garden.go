package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/forensics"
	"skycrypt/src/stats"
	"time"

	"github.com/gofiber/fiber/v2"
)

// GardenHandler godoc
//
//	@Summary		Get garden stats of a specified profile
//	@Description	Returns garden data for the given profile ID
//	@Tags			garden
//	@Produce		json
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.Garden
//	@Failure		400			{object}	models.ProcessingError
//	@Router			/api/garden/{profileId} [get]
func GardenHandler(c *fiber.Ctx) error {
	defer forensics.TrackSpan("handler.Garden")()
	timeNow := time.Now()

	profileId := c.Params("profileId")
	garden, err := api.GetGarden(profileId)
	if err != nil {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}

	output := stats.GetGarden(garden)

	fmt.Printf("Returning /api/garden/%s in %s\n", profileId, time.Since(timeNow))

	return c.JSON(output)
}
