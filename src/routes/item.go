package routes

import (
	"encoding/json"
	"os"
	"skycrypt/src/constants"
	"skycrypt/src/lib"
	"strings"

	"github.com/gofiber/fiber/v2"
)

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

	disabledPacks := []string{""}
	disabledPacksCookies := c.Cookies("disabledPacks", "FAILED")
	if disabledPacksCookies != "FAILED" {
		var parsedPacks []string
		err := json.Unmarshal([]byte(disabledPacksCookies), &parsedPacks)
		if err == nil {
			disabledPacks = append(disabledPacks, parsedPacks...)
		}
	} else if os.Getenv("DEV") == "true" {
		disabledResourcePacks := c.Query("disabledPacks", "")
		if disabledResourcePacks != "" {
			disabledPacks = strings.Split(disabledResourcePacks, ",")
		}
	}

	textureBytes, err := lib.RenderItem(textureId, disabledPacks, false)
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
