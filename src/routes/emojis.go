package routes

import (
	"fmt"
	"skycrypt/src/db"
	"time"

	"github.com/gofiber/fiber/v2"
)

// EmojisHandler godoc
//
//	@Summary		Get all emojis
//	@Description	Retrieves all emojis from the database
//	@Tags			emojis
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"Returns fetched_at timestamp and array of emojis"
//	@Failure		500	{object}	fiber.Map				"Failed to fetch or parse emojis"
//	@Router			/api/emojis [get]
func EmojisHandler(c *fiber.Ctx) error {
	timeNow := time.Now()

	emojis := db.GetMongoCollection("emojis")
	cursor, err := emojis.Find(c.Context(), map[string]interface{}{})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch emojis",
		})
	}
	defer func() {
		if err := cursor.Close(c.Context()); err != nil {
			fmt.Println("Failed to close emoji cursor:", err)
		}
	}()

	var results []map[string]interface{}
	if err := cursor.All(c.Context(), &results); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to parse emojis",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"fetched_at": timeNow.Unix(),
		"emojis":     results,
	})
}
