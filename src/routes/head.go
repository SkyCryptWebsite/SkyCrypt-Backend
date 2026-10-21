package routes

import (
	"errors"
	"skycrypt/src/constants"
	"skycrypt/src/lib"

	"github.com/gofiber/fiber/v2"
)

// notFoundImageCacheControl lets clients and the CDN absorb repeats of a bad image URL without pinning the miss for long.
const notFoundImageCacheControl = "public, max-age=300"

// HeadHandlers godoc
//
//	@Summary		Render a player head
//	@Description	Renders a Minecraft player head texture as PNG bytes.
//	@ID				renderHeadImage
//	@Tags			Rendering
//	@Produce		png
//	@Param			textureId	path		string	true	"Minecraft skin texture identifier"	example(93a1b830399ab432a5178fdaf3939b24bf25c724a66be947296c503352bc380d)
//	@Success		200			{file}		binary					"PNG image bytes returned successfully."
//	@Failure		400			{object}	models.ProcessingError	"Texture identifier is missing."
//	@Failure		404			{string}	string					"Texture identifier is malformed or unknown."
//	@Failure		500			{string}	string					"Head image could not be rendered."
//	@Router			/api/head/{textureId} [get]
func HeadHandlers(c *fiber.Ctx) error {
	// timeNow := time.Now()
	textureId := c.Params("textureId")
	if textureId == "" {
		c.Status(400)
		return c.JSON(constants.InvalidItemProvidedError)
	}

	textureBytes, err := lib.RenderHead(textureId)
	if errors.Is(err, lib.ErrTextureNotFound) {
		c.Set(fiber.HeaderCacheControl, notFoundImageCacheControl)
		return c.Status(fiber.StatusNotFound).SendString("Head texture not found")
	}
	if err != nil {
		c.Status(500)
		return c.SendString("Failed to render head")
	}

	c.Type("png")
	c.Set(fiber.HeaderCacheControl, renderedImageCacheControl)
	// fmt.Printf("Returning /api/head/%s in %s\n", textureId, time.Since(timeNow))
	return c.Send(textureBytes)
}
