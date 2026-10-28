package store

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
	jsoniter "github.com/json-iterator/go"

	"skycrypt/src/models"
)

// memberUpdate is what one profiles response says about one member.
type memberUpdate struct {
	uuid    [16]byte
	removed bool
	values  map[string]int64
	hidden  map[string]bool
}

// profileValueUpdate is one profile of a profiles response, as the leaderboards need it.
type profileValueUpdate struct {
	id       [16]byte
	gameMode int16
	members  []memberUpdate
	// networth is set when it was recomputed for this response; otherwise the stored networth is kept.
	networth map[[16]byte]map[string]int64
}

// updateProfileValues stores the values of every member of a profile and the coop sums, and moves them on the
// boards. Members missing from the response are handled by reconcileMembers.
func updateProfileValues(ctx context.Context, update profileValueUpdate, seenAt time.Time) error {
	pool := postgres()
	uuids := make([][16]byte, len(update.members))
	for i, m := range update.members {
		uuids[i] = m.uuid
	}

	rows, err := pool.Query(ctx, `SELECT m.id, m.player_uuid, h.uuid IS NOT NULL, v.seen_at, v.game_mode, v.ranked, v.data
		FROM profile_members m
		LEFT JOIN lb_hidden h ON h.uuid = m.player_uuid
		LEFT JOIN member_values v ON v.member_id = m.id
		WHERE m.profile_id = $1 AND m.player_uuid = ANY($2)`, update.id, uuids)
	if err != nil {
		return err
	}
	type stored struct {
		id     int64
		hidden bool
		old    *boardState
	}
	byUUID := map[[16]byte]stored{}
	for rows.Next() {
		var s stored
		var uuid [16]byte
		var oldSeen *time.Time
		var oldMode *int16
		var oldRanked *bool
		var data []byte
		if err := rows.Scan(&s.id, &uuid, &s.hidden, &oldSeen, &oldMode, &oldRanked, &data); err != nil {
			rows.Close()
			return err
		}
		if oldSeen != nil {
			values, err := decodeValues(ctx, data, *oldSeen)
			if err == nil {
				s.old = &boardState{values: values, mode: *oldMode, ranked: *oldRanked}
			}
		}
		byUUID[uuid] = s
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	coop := map[string]int64{}
	for _, m := range update.members {
		s, ok := byUUID[m.uuid]
		if !ok {
			continue
		}
		fresh := m.values
		keepNetworth := false
		if nw, ok := update.networth[m.uuid]; ok {
			for key, value := range nw {
				fresh[key] = value
			}
		} else {
			keepNetworth = true
		}

		var oldValues valueSet
		if s.old != nil {
			oldValues = s.old.values
		}
		merged := mergeValues(oldValues, fresh, m.hidden, seenAt, func(key string) bool {
			return !keepNetworth || hiddenCategory(key) != hiddenInventory
		})
		if keepNetworth && !m.hidden[hiddenInventory] {
			// Not recomputed because the profile didn't change, not because it was hidden: still current.
			for key, value := range merged {
				if hiddenCategory(key) == hiddenInventory {
					value.updatedAt = seenAt
					merged[key] = value
				}
			}
		}
		now := boardState{values: merged, mode: update.gameMode, ranked: !m.removed && !s.hidden}

		if err := saveMemberValues(ctx, s.id, seenAt, now); err != nil {
			return err
		}
		if err := syncBoards(ctx, memberEntity(s.id), rankedOnly(s.old), now); err != nil {
			fmt.Printf("[STORE] could not update leaderboards for member %d: %v\n", s.id, err)
		}

		if !m.removed {
			for key, value := range merged {
				if item, ok := strings.CutPrefix(key, "collection:"); ok {
					coop["coop_collection:"+item] += value.value
				}
			}
		}
	}

	return updateProfileRecord(ctx, update.id, seenAt, coop, nil, isCoopKey)
}

func rankedOnly(state *boardState) *boardState {
	if state == nil || !state.ranked {
		return nil
	}
	return state
}

func saveMemberValues(ctx context.Context, memberID int64, seenAt time.Time, state boardState) error {
	data, err := encodeValues(ctx, state.values, seenAt)
	if err != nil {
		return err
	}
	_, err = postgres().Exec(ctx, `INSERT INTO member_values (member_id, seen_at, version, game_mode, ranked, data)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (member_id) DO UPDATE SET seen_at = excluded.seen_at, version = excluded.version,
			game_mode = excluded.game_mode, ranked = excluded.ranked, data = excluded.data
		WHERE member_values.seen_at <= excluded.seen_at OR member_values.version < excluded.version`,
		memberID, seenAt, statsVersion, state.mode, state.ranked, data)
	return err
}

// updateProfileRecord replaces the keys owns says belong to this update (coop collections or the garden) and keeps
// the rest of the profile's record.
func updateProfileRecord(ctx context.Context, profileID [16]byte, seenAt time.Time, fresh map[string]int64, hidden map[string]bool, owns func(string) bool) error {
	pool := postgres()
	var id int64
	var mode int16
	var deleted bool
	var oldSeen *time.Time
	var oldMode *int16
	var oldRanked *bool
	var data []byte
	err := pool.QueryRow(ctx, `SELECT p.id, p.game_mode, p.deleted_at IS NOT NULL, v.seen_at, v.game_mode, v.ranked, v.data
		FROM profiles p LEFT JOIN profile_values v USING (profile_id) WHERE p.profile_id = $1`, profileID).
		Scan(&id, &mode, &deleted, &oldSeen, &oldMode, &oldRanked, &data)
	if err != nil {
		// A garden can arrive before the profile does; it is picked up by the backfill once the profile is stored.
		return nil
	}

	var old *boardState
	if oldSeen != nil {
		if values, err := decodeValues(ctx, data, *oldSeen); err == nil {
			old = &boardState{values: values, mode: *oldMode, ranked: *oldRanked}
		}
	}
	var oldValues valueSet
	if old != nil {
		oldValues = old.values
	}
	now := boardState{values: mergeValues(oldValues, fresh, hidden, seenAt, owns), mode: mode, ranked: !deleted}

	encoded, err := encodeValues(ctx, now.values, seenAt)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profile_values (profile_id, seen_at, version, game_mode, ranked, data)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (profile_id) DO UPDATE SET seen_at = excluded.seen_at, version = excluded.version,
			game_mode = excluded.game_mode, ranked = excluded.ranked, data = excluded.data`,
		profileID, seenAt, statsVersion, now.mode, now.ranked, encoded); err != nil {
		return err
	}
	return syncBoards(ctx, profileEntity(id), rankedOnly(old), now)
}

// updateGardenValues ranks a profile's garden level and crop milestones.
func updateGardenValues(ctx context.Context, profileID [16]byte, body []byte, seenAt time.Time) error {
	var garden gardenJSON
	if err := jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(body, &garden); err != nil {
		return err
	}
	return updateProfileRecord(ctx, profileID, seenAt, gardenStatValues(&garden), nil, isGardenKey)
}

// profileNetworth computes networth and non-cosmetic networth for every member of a raw profile object, with the
// museum stored for the profile if there is one. Members whose inventory API is off are left out, so their last
// known networth stays.
func profileNetworth(ctx context.Context, profileID [16]byte, object []byte) map[[16]byte]map[string]int64 {
	var profile struct {
		Members map[string]skycrypttypes.Member `json:"members"`
		Banking *skycrypttypes.Banking          `json:"banking"`
	}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(object, &profile); err != nil {
		return nil
	}
	var bank float64
	if profile.Banking != nil && profile.Banking.Balance != nil {
		bank = *profile.Banking.Balance
	}

	var museums map[string]*skycrypttypes.Museum
	if body, _, ok := readBlob(ctx, formatUUID(profileID), kindMuseum); ok && len(body) > 0 {
		var response models.HypixelMuseumResponse
		if json.Unmarshal(body, &response) == nil {
			museums = response.Members
		}
	}

	out := map[[16]byte]map[string]int64{}
	for uuidText, member := range profile.Members {
		if member.Inventory == nil {
			continue
		}
		uuid, err := parseUUID(uuidText)
		if err != nil {
			continue
		}
		member := member
		calculator, err := skyhelpernetworthgo.NewProfileNetworthCalculator(&member, museums[uuidText], bank)
		if err != nil {
			continue
		}
		normal, nonCosmetic := calculator.GetNetworthAndNonCosmetic(skyhelpernetworthgo.NetworthOptions{OnlyNetworth: true})
		values := map[string]int64{}
		if normal != nil && normal.Networth > 0 {
			values["networth"] = int64(math.Round(normal.Networth))
		}
		if nonCosmetic != nil && nonCosmetic.Networth > 0 {
			values["networth_noncosmetic"] = int64(math.Round(nonCosmetic.Networth))
		}
		out[uuid] = values
	}
	return out
}
