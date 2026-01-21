package routes

import (
	"fmt"
	"skycrypt/src/constants"
	"skycrypt/src/lib"

	"github.com/gofiber/fiber/v2"
)

// LeatherHandlers godoc
//
//	@Summary		Render and return a leather armor image
//	@Description	Returns a PNG image of leather armor for the given type and color
//	@Tags			leather
//	@Produce		png
//	@Param			type	path		string	true	"Armor Type"
//	@Param			color	path		string	true	"Armor Color"
//	@Success		200		{file}		binary	"PNG image of the leather armor"
//	@Failure		400		{object}	models.ProcessingError
//	@Failure		500		{object}	models.ProcessingError
//	@Router			/api/leather/{type}/{color} [get]
func LeatherHandlers(c *fiber.Ctx) error {
	// timeNow := time.Now()
	armorType := c.Params("type")
	armorColor := c.Params("color")
	if armorType == "" || armorColor == "" {
		c.Status(400)
		return c.JSON(constants.InvalidItemProvidedError)
	}

	imageBytes, err := lib.RenderArmor(armorType, armorColor)
	if err != nil {
		fmt.Printf("Error rendering armor: %v\n", err)
		c.Status(500)
		return c.JSON(constants.InvalidItemProvidedError)
	}

	c.Type("png")
	// fmt.Printf("Returning /api/leather/%s/%s in %s\n", armorType, armorColor, time.Since(timeNow))
	return c.Send(imageBytes)
}
