package routes

import (
	"context"
	"skycrypt/src/api"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

// playerLookupErrors documents the failures of a Mojang lookup; malformed describes its 422.
func playerLookupErrors(humaAPI huma.API, malformed string) map[string]*huma.Response {
	return errorResponses(humaAPI, map[int]string{
		fiber.StatusNotFound:            "PLAYER_NOT_FOUND: no player has that name or UUID.",
		fiber.StatusInternalServerError: "INTERNAL_ERROR: Unexpected failure.",
		fiber.StatusBadGateway:          "UPSTREAM_ERROR: Hypixel or Mojang failed or sent an unusable response.",
		fiber.StatusGatewayTimeout:      "UPSTREAM_TIMEOUT: Hypixel or Mojang didn't answer in time.",
		fiber.StatusUnprocessableEntity: "INVALID_REQUEST: " + malformed,
	})
}

func registerPlayers(humaAPI huma.API) {
	responses := playerLookupErrors(humaAPI, "The username is malformed.")
	responses["200"] = jsonResponse[models.PlayerResolve](humaAPI, "UUID resolved successfully.")
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "resolveUuidByUsername",
		Method:      fiber.MethodGet,
		Path:        "/api/uuid/{username}",
		Summary:     "Resolve UUID by username",
		Description: "Resolves a Minecraft username to its UUID and returns the normalized player identity payload.",
		Tags:        []string{"Mojang"},
		Responses:   responses,
	}), func(ctx context.Context, in *struct {
		Username string `path:"username" minLength:"3" maxLength:"16" doc:"Minecraft username" example:"duckysolucky"`
	}) (*jsonOutput, error) {
		timeNow := time.Now()
		username := in.Username

		cacheKey := responseCacheKey("uuid", username)
		if cached := cachedJSON(ctx, cacheKey); cached != nil {
			return cached, nil
		}

		uuid, err := api.GetUUIDContext(ctx, username)
		if err != nil {
			return nil, loadError("Failed to resolve player", err)
		}

		utility.LogVerbose("Returning /api/uuid/%s in %s\n", username, time.Since(timeNow))
		return cacheJSON(ctx, cacheKey, models.PlayerResolve{Username: username, UUID: uuid}, 24*60*60)
	})

	responses = playerLookupErrors(humaAPI, "The UUID is malformed.")
	responses["200"] = jsonResponse[models.PlayerResolve](humaAPI, "Username resolved successfully.")
	huma.Register(humaAPI, huma.Operation{
		OperationID: "resolveUsernameByUuid",
		Method:      fiber.MethodGet,
		Path:        "/api/username/{uuid}",
		Summary:     "Resolve username by UUID",
		Description: "Resolves a Minecraft UUID to the current username and display name known to SkyCrypt.",
		Tags:        []string{"Mojang"},
		Responses:   responses,
	}, func(ctx context.Context, in *struct {
		UUID string `path:"uuid" pattern:"^[0-9a-f]{32}$" patternDescription:"lowercase UUID without dashes" doc:"Minecraft UUID" example:"4855c53ee4fb4100997600a92fc50984"`
	}) (*jsonOutput, error) {
		timeNow := time.Now()
		uuid := in.UUID

		cacheKey := responseCacheKey("username", uuid)
		if cached := cachedJSON(ctx, cacheKey); cached != nil {
			return cached, nil
		}

		username, err := api.GetUsernameContext(ctx, uuid)
		if err != nil {
			return nil, loadError("Failed to resolve player", err)
		}

		utility.LogVerbose("Returning /api/username/%s in %s", username, time.Since(timeNow))
		return cacheJSON(ctx, cacheKey, models.PlayerResolve{Username: username, UUID: uuid}, 24*60*60)
	})
}
