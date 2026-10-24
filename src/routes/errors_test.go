package routes

import (
	"errors"
	"fmt"
	"skycrypt/src/api"
	"testing"

	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
	"github.com/gofiber/fiber/v3"
)

func TestLoadErrorStatus(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("failed to load profile: %w", fmt.Errorf("%w: profile with ID x not found", api.ErrProfileNotFound)), fiber.StatusNotFound, codeProfileNotFound},
		{fmt.Errorf("resolve: %w", api.ErrPlayerNotFound), fiber.StatusNotFound, codePlayerNotFound},
		{fmt.Errorf("%w: Hypixel returned status code 500", api.ErrUpstream), fiber.StatusBadGateway, codeUpstreamError},
		{fmt.Errorf("%w: Hypixel: timeout", api.ErrUpstreamTimeout), fiber.StatusGatewayTimeout, codeUpstreamTimeout},
		{api.ErrNotCached, fiber.StatusNotFound, codeNotCached},
		{fmt.Errorf("failed to fetch prices: %w", skyhelpernetworthgo.ErrPricesUnavailable), fiber.StatusServiceUnavailable, codePricesUnavailable},
		{errors.New("failed to format members"), fiber.StatusInternalServerError, codeInternalError},
	}
	for _, tc := range cases {
		status, code := loadErrorStatus(tc.err)
		if status != tc.status || code != tc.code {
			t.Errorf("%v: got %d %s, want %d %s", tc.err, status, code, tc.status, tc.code)
		}
	}
}
