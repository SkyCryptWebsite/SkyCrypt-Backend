package constants

import "github.com/gofiber/fiber/v2"

var InvalidUserError = fiber.Map{
	"error":   "User not found",
	"status":  "error",
	"message": "Please provide a valid username",
}

var FoundNoProfilesError = fiber.Map{
	"error":   "No profiles found",
	"status":  "error",
	"message": "This user has no profiles",
}

var FoundNoPlayerError = fiber.Map{
	"error":   "No player found",
	"status":  "error",
	"message": "This user has no player data",
}

var InvalidProfileIdError = fiber.Map{
	"error":   "Invalid profile ID",
	"status":  "error",
	"message": "Please provide a valid profile ID",
}

var InvalidItemProvidedError = fiber.Map{
	"error":   "Invalid item provided",
	"status":  "error",
	"message": "Please provide a valid item ID",
}

var InternalServerError = fiber.Map{
	"error":   "Internal server error",
	"status":  "error",
	"message": "An unexpected error occurred",
}

var FAIRY_SOULS = map[string]int{
	"normal":   289,
	"stranded": 4,
}
