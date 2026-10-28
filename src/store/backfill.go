package store

import (
	"context"
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
)

// New stats are filled in from the stored profiles, which keep every field, so they don't wait for each player to
// be looked up again. Members whose profile was already evicted keep what they have until their next lookup.
const backfillBatch = 200

func runStatsBackfill() {
	ctx := context.Background()
	start := time.Now()
	profiles, err := backfillStats(ctx)
	if err != nil {
		fmt.Printf("[STORE] stats backfill stopped: %v\n", err)
	}
	if profiles > 0 {
		fmt.Printf("[STORE] recomputed leaderboard values of %d profiles in %v\n", profiles, time.Since(start).Round(time.Second))
	}
}

// backfillStats recomputes the values of every stored profile that has a member without values or with values from
// an older statsVersion, and returns how many profiles it did.
func backfillStats(ctx context.Context) (int, error) {
	pool := postgres()
	if pool == nil {
		return 0, nil
	}

	var last [16]byte
	done := 0
	for {
		rows, err := pool.Query(ctx, `SELECT p.profile_id, p.game_mode FROM profiles p
			WHERE p.profile_id > $1 AND p.deleted_at IS NULL
				AND EXISTS (SELECT 1 FROM blobs b WHERE b.profile_id = p.profile_id AND b.kind = $2)
				AND EXISTS (SELECT 1 FROM profile_members m LEFT JOIN member_values v ON v.member_id = m.id
					WHERE m.profile_id = p.profile_id AND (v.member_id IS NULL OR v.version < $3))
			ORDER BY p.profile_id LIMIT $4`, last, kindProfile, statsVersion, backfillBatch)
		if err != nil {
			return done, err
		}
		type todo struct {
			id   [16]byte
			mode int16
		}
		var batch []todo
		for rows.Next() {
			var t todo
			if err := rows.Scan(&t.id, &t.mode); err != nil {
				rows.Close()
				return done, err
			}
			batch = append(batch, t)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return done, err
		}
		if len(batch) == 0 {
			return done, nil
		}
		last = batch[len(batch)-1].id

		for _, t := range batch {
			if err := recomputeProfile(ctx, t.id, t.mode); err != nil {
				fmt.Printf("[STORE] could not recompute profile %s: %v\n", dashedUUID(t.id), err)
				continue
			}
			done++
		}
	}
}

// recomputeProfile rebuilds a profile's values from its stored body and garden, as ingest would have.
func recomputeProfile(ctx context.Context, profileID [16]byte, gameMode int16) error {
	var codec int32
	var data []byte
	var fetchedAt time.Time
	if err := postgres().QueryRow(ctx, `SELECT codec, data, fetched_at FROM blobs WHERE profile_id = $1 AND kind = $2`,
		profileID, kindProfile).Scan(&codec, &data, &fetchedAt); err != nil {
		return err
	}
	object, err := loadBody(ctx, codec, data)
	if err != nil {
		return err
	}

	var light profileLight
	if err := jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(object, &light); err != nil {
		return err
	}
	update := profileValueUpdate{id: profileID, gameMode: gameMode, networth: profileNetworth(ctx, profileID, object)}
	for memberText, member := range light.Members {
		uuid, err := parseUUID(memberText)
		if err != nil {
			continue
		}
		m := memberUpdate{uuid: uuid}
		m.values, m.hidden = memberStatValues(&member)
		if member.Profile != nil {
			m.removed = member.Profile.DeletionNotice != nil ||
				(member.Profile.CoopInvitation != nil && !member.Profile.CoopInvitation.Confirmed)
		}
		update.members = append(update.members, m)
	}
	if err := updateProfileValues(ctx, update, fetchedAt); err != nil {
		return err
	}

	if body, gardenAt, ok := readBlob(ctx, formatUUID(profileID), kindGarden); ok && len(body) > 0 {
		return updateGardenValues(ctx, profileID, body, gardenAt)
	}
	return nil
}
