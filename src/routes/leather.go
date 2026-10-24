package routes

import (
	"errors"
	"fmt"
	"skycrypt/src/constants"
	"skycrypt/src/lib"

	"github.com/gofiber/fiber/v3"
)

// LeatherHandlers godoc
//
//	@Summary		Render leather armor
//	@Description	Renders a dyed leather armor piece as PNG bytes for the requested armor piece and color.
//	@ID				renderLeatherArmorImage
//	@Tags			Rendering
//	@Produce		png
//	@Param			type	path		string	true	"Leather armor piece identifier"	example(helmet)
//	@Param			color	path		string	true	"Armor color value"				example(ff0000)
//	@Success		200		{file}		binary					"PNG image bytes returned successfully."
//	@Failure		400		{object}	models.ProcessingError	"Armor type or color is missing or invalid."
//	@Failure		404		{object}	models.ProcessingError	"Armor piece is unknown or color is not 6-digit hex."
//	@Failure		500		{object}	models.ProcessingError	"Leather armor image could not be rendered."
//	@Router			/api/leather/{type}/{color} [get]
func LeatherHandlers(c fiber.Ctx) error {
	// timeNow := time.Now()
	armorType := c.Params("type")
	armorColor := c.Params("color")
	if armorType == "" || armorColor == "" {
		c.Status(400)
		return c.JSON(constants.InvalidItemProvidedError)
	}

	imageBytes, err := lib.RenderArmor(armorType, armorColor)
	if errors.Is(err, lib.ErrTextureNotFound) {
		c.Set(fiber.HeaderCacheControl, notFoundImageCacheControl)
		return c.Status(fiber.StatusNotFound).JSON(constants.InvalidItemProvidedError)
	}
	if err != nil {
		fmt.Printf("Error rendering armor: %v\n", err)
		c.Status(500)
		return c.JSON(constants.InvalidItemProvidedError)
	}

	c.Type("png")
	c.Set(fiber.HeaderCacheControl, renderedImageCacheControl)
	// fmt.Printf("Returning /api/leather/%s/%s in %s\n", armorType, armorColor, time.Since(timeNow))
	return c.Send(imageBytes)
}
