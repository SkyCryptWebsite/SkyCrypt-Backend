package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"skycrypt/src/db"

	"github.com/jackc/pgx/v5"
	"github.com/klauspost/compress/gzip"
)

// Rebuilt bodies are parsed right away and never cached, so payloads are wrapped in stored gzip blocks: measured on
// the biggest player in the sample, that rebuilt the body in half the time of the fastest real compression.
const rebuildGzipLevel = gzip.NoCompression

// LoadProfiles rebuilds a player's /v2/skyblock/profiles response from the last stored copy. It is all or nothing:
// if any profile in the player's list was evicted, the player counts as not stored.
func LoadProfiles(ctx context.Context, uuidText string) ([]byte, time.Time, bool) {
	pool := postgres()
	uuid, err := parseUUID(uuidText)
	if pool == nil || err != nil {
		return nil, time.Time{}, false
	}
	start := time.Now()
	defer func() { db.RecordPostgresDependency(ctx, "blobs", "load_profiles", time.Since(start), nil) }()

	var ids [][16]byte
	var selected *[16]byte
	var fetchedAt *time.Time
	err = pool.QueryRow(ctx, `SELECT profile_ids, selected_profile, profiles_fetched_at FROM players WHERE uuid = $1`, uuid).
		Scan(&ids, &selected, &fetchedAt)
	if err != nil || ids == nil || fetchedAt == nil {
		return nil, time.Time{}, false
	}
	if len(ids) == 0 {
		return []byte(`{"success":true,"profiles":null}`), *fetchedAt, true
	}

	rows, err := pool.Query(ctx, `SELECT b.profile_id, b.codec, b.data, b.fetched_at, m.cute_name
		FROM blobs b LEFT JOIN profile_members m ON m.profile_id = b.profile_id AND m.player_uuid = $2
		WHERE b.kind = $3 AND b.profile_id = ANY($1)`, ids, uuid, kindProfile)
	if err != nil {
		return nil, time.Time{}, false
	}
	defer rows.Close()

	type part struct {
		codec    int32
		data     []byte
		cuteName *string
	}
	parts := make(map[[16]byte]part, len(ids))
	oldest := *fetchedAt
	for rows.Next() {
		var id [16]byte
		var p part
		var at time.Time
		if err := rows.Scan(&id, &p.codec, &p.data, &at, &p.cuteName); err != nil {
			return nil, time.Time{}, false
		}
		parts[id] = p
		if at.Before(oldest) {
			oldest = at
		}
	}
	if rows.Err() != nil || len(parts) != len(ids) {
		return nil, time.Time{}, false
	}

	out := make([]byte, 0, 1<<20)
	out = append(out, `{"success":true,"profiles":[`...)
	for i, id := range ids {
		p := parts[id]
		object, err := loadBody(ctx, p.codec, p.data)
		if err != nil || len(object) < 2 || object[0] != '{' {
			return nil, time.Time{}, false
		}
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, `{"selected":`...)
		if selected != nil && *selected == id {
			out = append(out, "true"...)
		} else {
			out = append(out, "false"...)
		}
		if p.cuteName != nil {
			name, _ := json.Marshal(*p.cuteName)
			out = append(out, `,"cute_name":`...)
			out = append(out, name...)
		}
		if object[1] != '}' {
			out = append(out, ',')
		}
		out = append(out, object[1:]...)
	}
	out = append(out, "]}"...)

	touched := make([]string, len(ids))
	for i, id := range ids {
		touched[i] = formatUUID(id)
	}
	enqueueTouch(ctx, kindProfile, touched...)
	return out, oldest, true
}

// LoadMuseum returns the last stored /v2/skyblock/museum response for a profile.
func LoadMuseum(ctx context.Context, profileID string) ([]byte, time.Time, bool) {
	return loadBlob(ctx, profileID, kindMuseum)
}

// LoadGarden returns the last stored /v2/skyblock/garden response for a profile. An empty body means Hypixel had no
// garden for it.
func LoadGarden(ctx context.Context, profileID string) ([]byte, time.Time, bool) {
	return loadBlob(ctx, profileID, kindGarden)
}

func loadBlob(ctx context.Context, profileText string, kind int16) ([]byte, time.Time, bool) {
	pool := postgres()
	profileID, err := parseUUID(profileText)
	if pool == nil || err != nil {
		return nil, time.Time{}, false
	}
	start := time.Now()

	var codec int32
	var data []byte
	var fetchedAt time.Time
	err = pool.QueryRow(ctx, `SELECT codec, data, fetched_at FROM blobs WHERE profile_id = $1 AND kind = $2`, profileID, kind).
		Scan(&codec, &data, &fetchedAt)
	db.RecordPostgresDependency(ctx, "blobs", "load", time.Since(start), ignoreNoRows(err))
	if err != nil {
		return nil, time.Time{}, false
	}

	enqueueTouch(ctx, kind, profileText)
	if len(data) == 0 {
		return []byte{}, fetchedAt, true
	}
	body, err := loadBody(ctx, codec, data)
	if err != nil {
		return nil, time.Time{}, false
	}
	return body, fetchedAt, true
}

func loadBody(ctx context.Context, codec int32, data []byte) ([]byte, error) {
	packed, err := decompress(ctx, codec, data)
	if err != nil {
		return nil, err
	}
	return Unpack(packed, rebuildGzipLevel)
}

// LoadPlayer returns the last stored /v2/player response, trimmed to what SkyCrypt reads.
func LoadPlayer(ctx context.Context, uuidText string) ([]byte, time.Time, bool) {
	pool := postgres()
	uuid, err := parseUUID(uuidText)
	if pool == nil || err != nil {
		return nil, time.Time{}, false
	}

	var data []byte
	var fetchedAt *time.Time
	err = pool.QueryRow(ctx, `SELECT player_data, player_fetched_at FROM players WHERE uuid = $1`, uuid).Scan(&data, &fetchedAt)
	if err != nil || data == nil || fetchedAt == nil {
		return nil, time.Time{}, false
	}
	body, err := decompress(ctx, 0, data)
	if err != nil {
		return nil, time.Time{}, false
	}
	return body, *fetchedAt, true
}

// LookupUUID returns the uuid and username last seen for a username, ignoring case.
func LookupUUID(ctx context.Context, username string) (string, string, bool) {
	pool := postgres()
	if pool == nil {
		return "", "", false
	}

	var uuid [16]byte
	var name string
	err := pool.QueryRow(ctx, `SELECT uuid, username FROM players WHERE lower(username) = lower($1)
		ORDER BY name_seen_at DESC NULLS LAST LIMIT 1`, username).Scan(&uuid, &name)
	if err != nil {
		return "", "", false
	}
	return formatUUID(uuid), name, true
}

// LookupName returns the username last seen for a uuid.
func LookupName(ctx context.Context, uuidText string) (string, bool) {
	pool := postgres()
	uuid, err := parseUUID(uuidText)
	if pool == nil || err != nil {
		return "", false
	}

	var name *string
	if err := pool.QueryRow(ctx, `SELECT username FROM players WHERE uuid = $1`, uuid).Scan(&name); err != nil || name == nil {
		return "", false
	}
	return *name, true
}

// SaveResource stores a Hypixel resource body (skyblock items, collections). Only the parent fetches resources.
func SaveResource(ctx context.Context, name string, body []byte) {
	pool := postgres()
	if pool == nil || len(body) == 0 {
		return
	}
	_, data := compress(0, body, false)
	start := time.Now()
	_, err := pool.Exec(ctx, `INSERT INTO resources (name, fetched_at, data) VALUES ($1, now(), $2)
		ON CONFLICT (name) DO UPDATE SET fetched_at = excluded.fetched_at, data = excluded.data`, name, data)
	db.RecordPostgresDependency(ctx, "resources", "upsert", time.Since(start), err)
	if err != nil {
		fmt.Printf("[STORE] could not store %s: %v\n", name, err)
	}
}

// LoadResource returns the last stored resource body.
func LoadResource(ctx context.Context, name string) ([]byte, bool) {
	pool := postgres()
	if pool == nil {
		return nil, false
	}
	var data []byte
	if err := pool.QueryRow(ctx, `SELECT data FROM resources WHERE name = $1`, name).Scan(&data); err != nil {
		return nil, false
	}
	body, err := decompress(ctx, 0, data)
	return body, err == nil
}

func ignoreNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}
