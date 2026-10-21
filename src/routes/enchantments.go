package routes

import (
	"skycrypt/src/constants"

	"github.com/gofiber/fiber/v2"
)

// EnchantmentsConstantsHandler godoc
//
//	@Summary		List max-level enchantments
//	@Description	Returns the lore strings for enchantments at their maximum level.
//	@ID				listEnchantments
//	@Tags			Constants
//	@Produce		json
//	@Success		200	{array}		string	"Max-level enchantments returned successfully."
//	@Router			/api/constants/enchantments [get]
func EnchantmentsConstantsHandler(c *fiber.Ctx) error {
	return c.JSON(constants.MAX_ENCHANTS)
}
