package routes

import (
	"skycrypt/src/constants"
	"skycrypt/src/lib"

	"github.com/gofiber/fiber/v2"
)

// HeadHandlers godoc
//
//	@Summary		Render and return a head image
//	@Description	Returns a PNG image of a head for the given texture ID
//	@Tags			head
//	@Produce		png
//	@Param			textureId	path		string	true	"Texture ID"
//	@Success		200			{file}		binary	"PNG image of the head"
//	@Failure		400			{object}	models.ProcessingError
//	@Failure		500			{string}	string	"Failed to render head"
//	@Router			/api/head/{textureId} [get]
func HeadHandlers(c *fiber.Ctx) error {
	// timeNow := time.Now()
	textureId := c.Params("textureId")
	if textureId == "" {
		c.Status(400)
		return c.JSON(constants.InvalidItemProvidedError)
	}

	textureBytes := lib.RenderHead(textureId)
	if textureBytes == nil {
		c.Status(500)
		return c.SendString("Failed to render head")
	}

	c.Type("png")
	// fmt.Printf("Returning /api/head/%s in %s\n", textureId, time.Since(timeNow))
	return c.Send(textureBytes)
}
