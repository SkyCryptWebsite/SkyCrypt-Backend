package routes

import (
	"errors"
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/models"

	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
	"github.com/gofiber/fiber/v2"
)

// Codes sent with failed loads, so clients can tell "doesn't exist" from "try again later" without parsing text.
const (
	codePlayerNotFound    = "PLAYER_NOT_FOUND"
	codeProfileNotFound   = "PROFILE_NOT_FOUND"
	codeNotCached         = "NOT_CACHED"
	codeUpstreamError     = "UPSTREAM_ERROR"
	codeUpstreamTimeout   = "UPSTREAM_TIMEOUT"
	codePricesUnavailable = "PRICES_UNAVAILABLE"
	codeInternalError     = "INTERNAL_ERROR"
)

// loadErrorStatus maps a failed load to the status that says whose fault it was.
func loadErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, api.ErrNotCached):
		return fiber.StatusNotFound, codeNotCached
	case errors.Is(err, api.ErrPlayerNotFound):
		return fiber.StatusNotFound, codePlayerNotFound
	case errors.Is(err, api.ErrProfileNotFound):
		return fiber.StatusNotFound, codeProfileNotFound
	case errors.Is(err, api.ErrUpstreamTimeout):
		return fiber.StatusGatewayTimeout, codeUpstreamTimeout
	case errors.Is(err, api.ErrUpstream):
		return fiber.StatusBadGateway, codeUpstreamError
	case errors.Is(err, skyhelpernetworthgo.ErrPricesUnavailable):
		return fiber.StatusServiceUnavailable, codePricesUnavailable
	default:
		return fiber.StatusInternalServerError, codeInternalError
	}
}

// sendLoadError answers a failed load with its mapped status and code.
func sendLoadError(c *fiber.Ctx, message string, err error) error {
	status, code := loadErrorStatus(err)
	return c.Status(status).JSON(models.APIError{
		Error: fmt.Sprintf("%s: %v", message, err),
		Code:  code,
	})
}
