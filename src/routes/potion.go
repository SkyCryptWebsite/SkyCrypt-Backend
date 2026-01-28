package routes

import (
	"skycrypt/src/constants"
	"skycrypt/src/lib"

	"github.com/gofiber/fiber/v2"
)

// PotionHandlers godoc
//
//	@Summary		Render and return a potion image
//	@Description	Returns a PNG image of a potion for the given type and color
//	@Tags			potion
//	@Produce		png
//	@Param			type	path		string	true	"Potion Type"
//	@Param			color	path		string	true	"Potion Color"
//	@Success		200		{file}		binary	"PNG image of the potion"
//	@Failure		400		{object}	models.ProcessingError
//	@Failure		500		{object}	models.ProcessingError
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
