package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"skycrypt/src/security"
)

// Errors that say whose fault a failed request was, so handlers can answer with the right status.
var (
	// ErrPlayerNotFound means Mojang has no player with that name or UUID.
	ErrPlayerNotFound = errors.New("player not found")
	// ErrProfileNotFound means the player has no SkyBlock profile matching the request.
	ErrProfileNotFound = errors.New("profile not found")
	// ErrUpstream means Hypixel or Mojang failed or sent something unusable.
	ErrUpstream = errors.New("upstream API failed")
	// ErrUpstreamTimeout means Hypixel or Mojang didn't answer in time.
	ErrUpstreamTimeout = errors.New("upstream API timed out")
	// ErrGardenNotFound means the profile has no garden, which Hypixel answers with a 404.
	ErrGardenNotFound = errors.New("garden not found")

	// errHypixelNotFound marks a Hypixel 404. It also counts as ErrUpstream, except where a caller expects one.
	errHypixelNotFound = errors.New("status code 404 from Hypixel")
)

// upstreamError wraps a failed call to service as ErrUpstreamTimeout or ErrUpstream, with secrets redacted.
func upstreamError(service string, err error) error {
	sentinel := ErrUpstream
	if isTimeout(err) {
		sentinel = ErrUpstreamTimeout
	}
	return fmt.Errorf("%w: %s: %v", sentinel, service, security.RedactError(err))
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
