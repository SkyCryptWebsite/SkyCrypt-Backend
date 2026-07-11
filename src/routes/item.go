package routes

import (
	"skycrypt/src/constants"
	"skycrypt/src/lib"

	"github.com/gofiber/fiber/v2"
)

type itemTextureResolution struct {
	Texture     string `json:"texture"`
	TexturePack string `json:"texture_pack,omitempty"`
}

// ItemResolveHandler godoc
//
// \t@Summary\t\tResolve an item texture
// \t@Description\tReturns the final texture URL and the resource pack that supplied it.
// \t@ID\t\t\tresolveItemImage
// \t@Tags\t\t\tRendering
// \t@Produce\t\tjson
// \t@Param\t\t\titemId\tpath\t\tstring\ttrue\t"SkyBlock item ID or Minecraft item identifier"
// \t@Success\t\t200\t\t{object}\titemTextureResolution
// \t@Failure\t\t400\t\t{object}\tmodels.ProcessingError
// \t@Failure\t\t500\t\t{object}\tmodels.ProcessingError
// \t@Router\t\t\t/api/item/{itemId}/resolve [get]
func ItemResolveHandler(c *fiber.Ctx) error {
	textureID := c.Params("itemId")
	if textureID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(constants.InvalidItemProvidedError)
	}

	texture, err := lib.ResolveItemTexture(textureID, enabledPacksFromRequest(c), false)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(constants.InvalidItemProvidedError)
	}

	return c.JSON(itemTextureResolution{Texture: texture.Texture, TexturePack: texture.TexturePack})
}

// ItemHandlers godoc
//
//	@Summary		Render an item
//	@Description	Renders a SkyBlock or Minecraft item as PNG bytes.
//	@Description	Resource pack preferences supplied by cookie can affect which texture is rendered. Some renderer results redirect to a canonical asset URL.
//	@ID				renderItemImage
//	@Tags			Rendering
//	@Produce		png
//	@Param			itemId	path		string	true	"SkyBlock item ID or Minecraft item identifier"	example(ASPECT_OF_THE_END)
//	@Success		200		{file}		binary					"PNG image bytes returned successfully."
//	@Success		302		{string}	string					"Redirects to a canonical rendered asset URL."
//	@Header			302		{string}	Location				"Redirect target URL."
//	@Failure		400		{object}	models.ProcessingError	"Item identifier is missing or invalid."
//	@Failure		500		{object}	models.ProcessingError	"Item image could not be rendered."
//	@Router			/api/item/{itemId} [get]
func ItemHandlers(c *fiber.Ctx) error {
	// timeNow := time.Now()
	textureId := c.Params("itemId")
	if textureId == "" {
		c.Status(400)
		return c.JSON(constants.InvalidItemProvidedError)
	}

	enabledPacks := enabledPacksFromRequest(c)

	textureBytes, err := lib.RenderItem(textureId, enabledPacks, false)
	if err != nil {
		if redirectErr, ok := err.(lib.RedirectError); ok {
			return c.Redirect(redirectErr.URL, 302)
		}

		c.Status(500)
		return c.JSON(constants.InvalidItemProvidedError)
	}

	c.Type("png")
	// fmt.Printf("Returning /api/item/%s in %s\n", textureId, time.Since(timeNow))
	return c.Send(textureBytes)
}
