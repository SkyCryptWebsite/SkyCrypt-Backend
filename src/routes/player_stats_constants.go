package routes

import (
	"skycrypt/src/constants"

	"github.com/gofiber/fiber/v2"
)

// PlayerStatsConstantsHandler godoc
//
//	@Summary		List player stats constants
//	@Description	Returns player stats constants.
//	@ID				listPlayerStatsConstants
//	@Tags			Stats
//	@Produce		json
//	@Success		200	{array}		[]constants.StatData	"Player stats constants returned successfully."
//	@Router			/api/constants/stats [get]
func PlayerStatsConstantsHandler(c *fiber.Ctx) error {
	return c.JSON(constants.STATS_DATA)
}
