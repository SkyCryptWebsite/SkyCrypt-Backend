package routes

import (
	"errors"
	"fmt"
	"skycrypt/src/api"

	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
	"github.com/gofiber/fiber/v3"
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
	codeGardenNotFound    = "GARDEN_NOT_FOUND"
	codeInvalidRequest    = "INVALID_REQUEST"
	codeTextureNotFound   = "TEXTURE_NOT_FOUND"
	codeRenderFailed      = "RENDER_FAILED"
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
	case errors.Is(err, api.ErrGardenNotFound):
		return fiber.StatusNotFound, codeGardenNotFound
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

// loadError answers a failed load with its mapped status and code.
func loadError(message string, err error) error {
	status, code := loadErrorStatus(err)
	return NewAPIError(status, code, fmt.Sprintf("%s: %v", message, err))
}

// buildError is a failure inside a shared response build, with the message its handler answers with.
type buildError struct {
	message string
	err     error
}

func (e buildError) Error() string { return e.message + ": " + e.err.Error() }
func (e buildError) Unwrap() error { return e.err }

func failedTo(message string, err error) error {
	// A step that already chose its message (a player lookup inside the build) keeps it.
	var inner buildError
	if errors.As(err, &inner) {
		return inner
	}
	return buildError{message: message, err: err}
}

// buildFailure answers a failed shared build like loadError, with the message from failedTo.
func buildFailure(err error) error {
	var built buildError
	if errors.As(err, &built) {
		return loadError(built.message, built.err)
	}
	return loadError("Failed to build response", err)
}
