package routes

import (
	"context"
	"crypto/subtle"
	"errors"
	"os"
	"skycrypt/src/api"
	"skycrypt/src/models"
	"skycrypt/src/store"
	"skycrypt/src/utility"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
)

const (
	codeUnknownLeaderboard     = "UNKNOWN_LEADERBOARD"
	codeLeaderboardUnavailable = "LEADERBOARD_UNAVAILABLE"

	// Boards change with every profile view, so pages are only cached long enough to absorb bursts.
	leaderboardCacheSeconds = 60
	// New boards appear when a player has a stat nobody had before (a new mob), so the list is cached briefly too.
	leaderboardListCacheSeconds = 10 * 60
	leaderboardNameTimeout      = 2 * time.Second
)

func registerLeaderboards(humaAPI huma.API) {
	responses := errorResponses(humaAPI, map[int]string{
		fiber.StatusServiceUnavailable:  "LEADERBOARD_UNAVAILABLE: leaderboards need the Postgres store, which is unavailable.",
		fiber.StatusInternalServerError: "INTERNAL_ERROR: Unexpected failure.",
	})
	responses["200"] = jsonResponse[models.LeaderboardList](humaAPI, "Every leaderboard, by category.")
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "listLeaderboards",
		Method:      fiber.MethodGet,
		Path:        "/api/leaderboards",
		Summary:     "List leaderboards",
		Description: "Lists every leaderboard by category, with the game modes each can be filtered by and how many entries each keeps.",
		Tags:        []string{"Leaderboards"},
		Responses:   responses,
	}), func(ctx context.Context, _ *struct{}) (*jsonOutput, error) {
		cacheKey := responseCacheKey("leaderboards")
		if cached := cachedJSON(ctx, cacheKey); cached != nil {
			return cached, nil
		}
		keys, err := store.StatKeys(ctx)
		if err != nil {
			return nil, leaderboardError(err)
		}

		boards := make([]boardInfo, 0, len(keys))
		for _, key := range keys {
			if info, ok := describeBoard(key); ok {
				boards = append(boards, info)
			}
		}
		sortBoards(boards)

		list := models.LeaderboardList{Size: store.BoardSize(), Modes: store.Modes, Categories: []models.LeaderboardCategory{}}
		for _, category := range leaderboardCategories {
			out := models.LeaderboardCategory{ID: category.ID, Name: category.Name}
			for _, board := range boards {
				if board.Category == category.ID {
					out.Leaderboards = append(out.Leaderboards, leaderboardInfo(board))
				}
			}
			if len(out.Leaderboards) > 0 {
				list.Categories = append(list.Categories, out)
			}
		}
		return cacheJSON(ctx, cacheKey, list, leaderboardListCacheSeconds)
	})

	boardErrors := func() map[string]*huma.Response {
		return errorResponses(humaAPI, map[int]string{
			fiber.StatusNotFound:            "UNKNOWN_LEADERBOARD or PLAYER_NOT_FOUND: no leaderboard or player by that name.",
			fiber.StatusServiceUnavailable:  "LEADERBOARD_UNAVAILABLE: leaderboards need the Postgres store, which is unavailable.",
			fiber.StatusUnprocessableEntity: "INVALID_REQUEST: a parameter is malformed.",
			fiber.StatusInternalServerError: "INTERNAL_ERROR: Unexpected failure.",
		})
	}

	responses = boardErrors()
	responses["200"] = jsonResponse[models.LeaderboardPage](humaAPI, "A page of the leaderboard, best first.")
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "getLeaderboard",
		Method:      fiber.MethodGet,
		Path:        "/api/leaderboard/{leaderboard}",
		Summary:     "Get a leaderboard page",
		Description: "Ranks the best profile members (or profiles, for coop collections and the garden) SkyCrypt has " +
			"seen. A player appears once per profile. Values hidden from the API since are kept and marked outdated.",
		Tags:      []string{"Leaderboards"},
		Responses: responses,
	}), func(ctx context.Context, in *struct {
		Leaderboard string `path:"leaderboard" maxLength:"128" doc:"Leaderboard id from /api/leaderboards" example:"skill:farming"`
		Mode        string `query:"mode" enum:"all,normal,ironman,stranded,bingo" default:"all" doc:"Game mode filter; all mixes them."`
		Page        int64  `query:"page" minimum:"1" maximum:"1000" default:"1"`
		Limit       int64  `query:"limit" minimum:"1" maximum:"100" default:"50"`
	}) (*jsonOutput, error) {
		info, ok := describeBoard(in.Leaderboard)
		if !ok {
			return nil, NewAPIError(fiber.StatusNotFound, codeUnknownLeaderboard, "Unknown leaderboard.")
		}
		cacheKey := responseCacheKey("leaderboard", in.Leaderboard, in.Mode, strconv.FormatInt(in.Page, 10), strconv.FormatInt(in.Limit, 10))
		if cached := cachedJSON(ctx, cacheKey); cached != nil {
			return cached, nil
		}

		page, err := store.LeaderboardPage(ctx, in.Leaderboard, in.Mode, (in.Page-1)*in.Limit, in.Limit)
		if err != nil {
			return nil, leaderboardError(err)
		}

		out := models.LeaderboardPage{
			Leaderboard: leaderboardInfo(info), Mode: in.Mode, Page: in.Page, Limit: in.Limit,
			Total: page.Total, Ready: page.Ready, Entries: leaderboardEntries(ctx, info, page.Entries),
		}
		return cacheJSON(ctx, cacheKey, out, leaderboardCacheSeconds)
	})

	responses = boardErrors()
	responses["200"] = jsonResponse[models.LeaderboardPlayerRanks](humaAPI, "The player's ranks on every leaderboard.")
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "getPlayerLeaderboardRanks",
		Method:      fiber.MethodGet,
		Path:        "/api/leaderboards/player/{player}",
		Summary:     "Get a player's ranks on every leaderboard",
		Description: "Returns each active profile of the player with every leaderboard it is on, as the player and as " +
			"the whole profile. Only the top entries of each board are kept, so a player outside them isn't listed.",
		Tags:      []string{"Leaderboards"},
		Responses: responses,
	}), func(ctx context.Context, in *struct {
		Player string `path:"player" minLength:"3" maxLength:"36" doc:"Username or UUID" example:"tiltedhoney"`
	}) (*jsonOutput, error) {
		cacheKey := responseCacheKey("leaderboard-ranks", in.Player)
		if cached := cachedJSON(ctx, cacheKey); cached != nil {
			return cached, nil
		}
		uuid, err := leaderboardPlayerUUID(ctx, in.Player)
		if err != nil {
			return nil, err
		}

		profiles, err := store.PlayerRanks(ctx, uuid)
		if err != nil {
			return nil, leaderboardError(err)
		}
		out := models.LeaderboardPlayerRanks{UUID: uuid, Size: store.BoardSize(), Profiles: []models.LeaderboardProfileRanks{}}
		for _, profile := range profiles {
			ranks := []models.LeaderboardRank{}
			for _, r := range profile.Ranks {
				info, ok := describeBoard(r.Key)
				if !ok {
					continue
				}
				ranks = append(ranks, models.LeaderboardRank{
					Leaderboard: leaderboardInfo(info), Category: info.Category, Rank: r.Rank, ModeRank: r.ModeRank,
					Value: boardValue(info, r.Raw), Raw: r.Raw, UpdatedAt: r.UpdatedAt,
					Outdated: r.UpdatedAt.Before(r.SeenAt),
				})
			}
			slices.SortFunc(ranks, func(a, b models.LeaderboardRank) int {
				if a.Rank != b.Rank {
					return int(a.Rank - b.Rank)
				}
				return strings.Compare(a.Leaderboard.Name, b.Leaderboard.Name)
			})
			out.Profiles = append(out.Profiles, models.LeaderboardProfileRanks{
				ProfileID: profile.ProfileID, CuteName: profile.CuteName, GameMode: profile.GameMode, LastSeen: profile.SeenAt, Ranks: ranks,
			})
		}
		return cacheJSON(ctx, cacheKey, out, leaderboardCacheSeconds)
	})

	registerLeaderboardAdmin(humaAPI)
}

func leaderboardInfo(info boardInfo) models.LeaderboardInfo {
	return models.LeaderboardInfo{
		ID: info.ID, Name: info.Name, Group: info.Group, Unit: info.Unit, Ascending: info.Ascending, Profile: info.Profile,
	}
}

// leaderboardPlayerUUID accepts a username or a UUID with or without dashes.
func leaderboardPlayerUUID(ctx context.Context, player string) (string, error) {
	uuid := strings.ToLower(strings.ReplaceAll(player, "-", ""))
	if utility.IsUUID(uuid) {
		return uuid, nil
	}
	resolved, err := api.GetUUIDContext(ctx, player)
	if err != nil {
		return "", loadError("Failed to resolve player", err)
	}
	return resolved, nil
}

// leaderboardEntries converts store entries, filling in usernames the store doesn't know yet (coop members who were
// never looked up themselves).
func leaderboardEntries(ctx context.Context, info boardInfo, entries []store.Entry) []models.LeaderboardEntry {
	var missing []string
	for _, entry := range entries {
		if !entry.Profile && entry.Username == "" {
			missing = append(missing, entry.UUID)
		}
		for _, member := range entry.Members {
			if member.Username == "" {
				missing = append(missing, member.UUID)
			}
		}
	}
	var names map[string]*models.MowojangResponse
	if len(missing) > 0 {
		// Resolved in parallel and cached (and stored, so the next page has them); names Mojang doesn't return
		// in time stay empty for the client to fall back on.
		lookupCtx, cancel := context.WithTimeout(ctx, leaderboardNameTimeout)
		names = api.ResolvePlayersContext(lookupCtx, missing)
		cancel()
	}
	name := func(uuid, known string) string {
		if known != "" {
			return known
		}
		if resolved := names[uuid]; resolved != nil {
			return resolved.Name
		}
		return ""
	}

	out := make([]models.LeaderboardEntry, len(entries))
	for i, entry := range entries {
		out[i] = models.LeaderboardEntry{
			Rank: entry.Rank, ProfileID: entry.ProfileID, CuteName: entry.CuteName, GameMode: entry.GameMode,
			Value: boardValue(info, entry.Raw), Raw: entry.Raw, UpdatedAt: entry.UpdatedAt, LastSeen: entry.SeenAt,
			Outdated: entry.UpdatedAt.Before(entry.SeenAt),
		}
		if entry.Profile {
			for _, member := range entry.Members {
				out[i].Members = append(out[i].Members, models.LeaderboardPlayer{UUID: member.UUID, Username: name(member.UUID, member.Username)})
			}
		} else {
			out[i].UUID, out[i].Username = entry.UUID, name(entry.UUID, entry.Username)
		}
	}
	return out
}

func leaderboardError(err error) error {
	switch {
	case errors.Is(err, store.ErrUnknownMode):
		return NewAPIError(fiber.StatusNotFound, codeUnknownLeaderboard, err.Error())
	default:
		store.LogUnavailable("a leaderboard request")
		return NewAPIError(fiber.StatusServiceUnavailable, codeLeaderboardUnavailable, "Leaderboards are unavailable right now.")
	}
}

// registerLeaderboardAdmin adds the moderation routes. They take only the frontend's server token, and are left out
// of the public API reference.
func registerLeaderboardAdmin(humaAPI huma.API) {
	admin := func(op huma.Operation) huma.Operation {
		op.Hidden = true
		op.Tags = []string{"Leaderboards"}
		op.Middlewares = append(op.Middlewares, serverTokenMiddleware)
		return op
	}

	huma.Register(humaAPI, admin(huma.Operation{
		OperationID: "listHiddenLeaderboardPlayers",
		Method:      fiber.MethodGet,
		Path:        "/api/internal/leaderboards/hidden",
		Summary:     "List players hidden from leaderboards",
	}), func(ctx context.Context, _ *struct{}) (*struct{ Body models.LeaderboardHiddenList }, error) {
		hidden, err := store.HiddenPlayers(ctx)
		if err != nil {
			return nil, leaderboardError(err)
		}
		out := &struct{ Body models.LeaderboardHiddenList }{Body: models.LeaderboardHiddenList{Hidden: []models.LeaderboardHiddenPlayer{}}}
		for _, h := range hidden {
			out.Body.Hidden = append(out.Body.Hidden, models.LeaderboardHiddenPlayer{UUID: h.UUID, Username: h.Username, Reason: h.Reason, HiddenAt: h.HiddenAt})
		}
		return out, nil
	})

	type hideInput struct {
		UUID string `path:"uuid" pattern:"^[0-9a-f]{32}$" doc:"Minecraft UUID"`
		Body struct {
			Reason string `json:"reason,omitempty" maxLength:"500"`
		}
	}
	huma.Register(humaAPI, admin(huma.Operation{
		OperationID:   "hideLeaderboardPlayer",
		Method:        fiber.MethodPut,
		Path:          "/api/internal/leaderboards/hidden/{uuid}",
		Summary:       "Hide a player from every leaderboard",
		DefaultStatus: fiber.StatusNoContent,
	}), func(ctx context.Context, in *hideInput) (*struct{}, error) {
		if err := store.HidePlayer(ctx, in.UUID, in.Body.Reason); err != nil {
			return nil, leaderboardError(err)
		}
		return nil, nil
	})

	huma.Register(humaAPI, admin(huma.Operation{
		OperationID:   "unhideLeaderboardPlayer",
		Method:        fiber.MethodDelete,
		Path:          "/api/internal/leaderboards/hidden/{uuid}",
		Summary:       "Show a hidden player on the leaderboards again",
		DefaultStatus: fiber.StatusNoContent,
	}), func(ctx context.Context, in *struct {
		UUID string `path:"uuid" pattern:"^[0-9a-f]{32}$" doc:"Minecraft UUID"`
	}) (*struct{}, error) {
		if err := store.UnhidePlayer(ctx, in.UUID); err != nil {
			return nil, leaderboardError(err)
		}
		return nil, nil
	})
}

// serverTokenMiddleware only lets the frontend in, with its shared X-API-Token. SkyCrypt API keys don't work here.
func serverTokenMiddleware(ctx huma.Context, next func(huma.Context)) {
	c := humafiber.Unwrap(ctx)
	if os.Getenv("DISABLE_SERVER_API_AUTH") != "true" {
		token := strings.TrimSpace(c.Get("X-API-Token"))
		expected := strings.TrimSpace(os.Getenv("SERVER_API_TOKEN"))
		if token == "" || expected == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			apiKeyError(c, fiber.StatusUnauthorized, "INVALID_API_TOKEN", "Invalid X-API-Token.")
			return
		}
	}
	next(huma.WithContext(ctx, api.WithRequestState(ctx.Context(), &api.RequestState{})))
}
