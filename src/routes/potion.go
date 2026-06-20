package routes

import (
	"skycrypt/src/constants"
	"skycrypt/src/lib"

	"github.com/gofiber/fiber/v2"
)

// PotionHandlers godoc
//
//	@Summary		Render a potion
//	@Description	Renders a potion image for the requested potion type and color.
//	@ID				renderPotionImage
//	@Tags			Rendering
//	@Produce		png
//	@Param			type	path		string	true	"Potion type identifier"	example(splash)
//	@Param			color	path		string	true	"Potion color value"		example(ff0000)
//	@Success		200		{file}		binary					"PNG image bytes returned successfully."
//	@Failure		400		{object}	models.ProcessingError	"Potion type or color is missing or invalid."
//	@Failure		500		{object}	models.ProcessingError	"Potion image could not be rendered."
//	@Router			/api/potion/{type}/{color} [get]
func PotionHandlers(c *fiber.Ctx) error {
	// timeNow := time.Now()
	potionType := c.Params("type")
	potionColor := c.Params("color")
	if potionType == "" || potionColor == "" {
		c.Status(400)
		return c.JSON(constants.InvalidItemProvidedError)
	}

	imageBytes, err := lib.RenderPotion(potionType, potionColor)
	if err != nil {
		c.Status(500)
		return c.JSON(constants.InvalidItemProvidedError)
	}

	c.Type("png")
	// fmt.Printf("Returning /api/potion/%s/%s in %s\n", potionType, potionColor, time.Since(timeNow))
	return c.Send(imageBytes)
}
