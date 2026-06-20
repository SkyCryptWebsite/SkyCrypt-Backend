package routes

import (
	"fmt"
	"skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/utility"
	"time"

	"github.com/gofiber/fiber/v2"
)

// EmojisHandler godoc
//
//	@Summary		List emojis
//	@Description	Returns emoji documents stored by SkyCrypt along with a Unix timestamp indicating when the response was generated.
//	@ID				listEmojis
//	@Tags			Emojis
//	@Produce		json
//	@Success		200	{object}	models.EmojisResponse	"Emojis returned successfully."
//	@Failure		500	{object}	models.ProcessingError	"Emoji data could not be fetched or parsed."
//	@Router			/api/emojis [get]
func EmojisHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Emojis")()
	}

	timeNow := time.Now()

	emojis := db.GetMongoCollection("emojis")
	ctx := c.UserContext()
	cursor, err := emojis.Find(ctx, map[string]interface{}{})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch emojis",
		})
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			fmt.Println("Failed to close emoji cursor:", err)
		}
	}()

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to parse emojis",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"fetched_at": timeNow.Unix(),
		"emojis":     results,
	})
}
