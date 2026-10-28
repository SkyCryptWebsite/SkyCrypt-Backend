package store

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"skycrypt/src/db"

	"github.com/cespare/xxhash/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	jsoniter "github.com/json-iterator/go"
	"github.com/redis/go-redis/v9"
)

// Children queue work in a Redis sorted set; the parent pops it. A member is the Redis key that holds the fetched
// body (profiles:<uuid>, player:<uuid>, museum:<profile>, garden:<profile>) or a small job that carries its own data.
// Re-queueing a key only moves its score forward, so a busy player is stored once per pop, not once per view.
const (
	queueKey = "store:queue"
	queueTTL = time.Hour

	jobNoGarden = "garden-none:"
	jobName     = "name:"
	jobTouch    = "touch:"

	ingestWorkers = 3
	ingestBatch   = 8
	// Past this many queued jobs, blobs are compressed faster at a slightly worse ratio.
	busyBacklog = 200

	// An active player's profile changes on every fetch; rewriting a multi-MB blob more than hourly buys nothing.
	blobRewriteInterval = time.Hour
	ingestTimeout       = time.Minute
)

// Enqueue asks the parent to store the body that was just cached in Redis under key.
func Enqueue(ctx context.Context, key string) {
	enqueue(ctx, key)
}

// EnqueueNoGarden records that Hypixel has no garden for a profile, so cache-only reads can answer that too.
func EnqueueNoGarden(ctx context.Context, profileID string) {
	enqueue(ctx, jobNoGarden+profileID)
}

// EnqueueIdentity stores a Mojang uuid and username pair.
func EnqueueIdentity(ctx context.Context, uuid, username string) {
	if uuid == "" || username == "" {
		return
	}
	enqueue(ctx, jobName+uuid+":"+username)
}

func enqueueTouch(ctx context.Context, kind int16, profileIDs ...string) {
	jobs := make([]string, len(profileIDs))
	for i, id := range profileIDs {
		jobs[i] = jobTouch + strconv.Itoa(int(kind)) + ":" + id
	}
	enqueue(ctx, jobs...)
}

func enqueue(ctx context.Context, jobs ...string) {
	if !enabled.Load() || len(jobs) == 0 {
		return
	}
	client, err := db.Redis()
	if err != nil {
		return
	}

	score := float64(time.Now().UnixMilli())
	members := make([]redis.Z, len(jobs))
	for i, job := range jobs {
		members[i] = redis.Z{Score: score, Member: job}
	}
	// The request may already be over by the time this runs.
	ctx = context.WithoutCancel(ctx)
	pipe := client.Pipeline()
	pipe.ZAddGT(ctx, queueKey, members...)
	pipe.Expire(ctx, queueKey, queueTTL)
	_, _ = pipe.Exec(ctx)
}

func runIngest() {
	for range ingestWorkers {
		go ingestWorker()
	}
}

func ingestWorker() {
	for {
		client, err := db.Redis()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		ctx := context.Background()
		jobs, err := client.ZPopMin(ctx, queueKey, ingestBatch).Result()
		if err != nil || len(jobs) == 0 {
			time.Sleep(time.Second)
			continue
		}
		backlog, _ := client.ZCard(ctx, queueKey).Result()
		busy := backlog > busyBacklog

		for _, job := range jobs {
			member, _ := job.Member.(string)
			fetchedAt := time.UnixMilli(int64(job.Score))
			jobCtx, cancel := context.WithTimeout(ctx, ingestTimeout)
			start := time.Now()
			err := withRetry(func() error { return ingest(jobCtx, member, fetchedAt, busy) })
			db.RecordPostgresDependency(jobCtx, "store", "ingest", time.Since(start), err)
			cancel()
			if err != nil {
				fmt.Printf("[STORE] could not store %s: %v\n", member, err)
			}
		}
	}
}

// withRetry runs f again when Postgres aborted it over a deadlock or serialization conflict. Coop members ingested
// at the same time touch the same rows in different orders, which Postgres resolves by aborting one of them.
func withRetry(f func() error) error {
	var err error
	for range 3 {
		err = f()
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "40P01" && pgErr.Code != "40001") {
			return err
		}
	}
	return err
}

func ingest(ctx context.Context, job string, fetchedAt time.Time, busy bool) error {
	if postgres() == nil {
		return nil
	}

	kind, id, _ := strings.Cut(job, ":")
	switch kind + ":" {
	case jobNoGarden:
		return storeBlob(ctx, id, kindGarden, nil, fetchedAt, busy)
	case jobName:
		uuid, name, _ := strings.Cut(id, ":")
		return storeIdentity(ctx, uuid, name, fetchedAt)
	case jobTouch:
		kindText, profileID, _ := strings.Cut(id, ":")
		blobKind, err := strconv.Atoi(kindText)
		if err != nil {
			return err
		}
		return touchBlob(ctx, profileID, int16(blobKind))
	}

	body, err := db.GetBytesContext(ctx, job)
	if err != nil || len(body) == 0 {
		// The Redis copy expired before the queue got to it. The next fetch queues it again.
		return nil
	}

	switch kind {
	case "profiles":
		return storeProfiles(ctx, id, body, fetchedAt, busy)
	case "player":
		return storePlayer(ctx, id, body, fetchedAt)
	case "museum":
		if !successful(body) {
			return nil
		}
		return storeBlob(ctx, id, kindMuseum, body, fetchedAt, busy)
	case "garden":
		if !successful(body) {
			return nil
		}
		if err := storeBlob(ctx, id, kindGarden, body, fetchedAt, busy); err != nil {
			return err
		}
		profileID, err := parseUUID(id)
		if err != nil {
			return err
		}
		return updateGardenValues(ctx, profileID, body, fetchedAt)
	}
	return fmt.Errorf("unknown job %q", job)
}

func successful(body []byte) bool {
	fields, err := objectFields(body)
	if err != nil {
		return false
	}
	for _, f := range fields {
		if f.key == "success" {
			return string(f.value) == "true"
		}
	}
	return false
}

// parseUUID accepts Hypixel's undashed and Mojang's dashed form.
func parseUUID(value string) ([16]byte, error) {
	var out [16]byte
	clean := strings.ReplaceAll(value, "-", "")
	if len(clean) != 32 {
		return out, fmt.Errorf("invalid uuid %q", value)
	}
	if _, err := hex.Decode(out[:], []byte(clean)); err != nil {
		return out, fmt.Errorf("invalid uuid %q", value)
	}
	return out, nil
}

func formatUUID(id [16]byte) string {
	return hex.EncodeToString(id[:])
}

var gameModes = map[string]int16{"": 0, "ironman": 1, "island": 2, "stranded": 2, "bingo": 3}

// profileLight is what the light tables need from a profile. jsoniter skips everything else.
type profileLight struct {
	GameMode string                `json:"game_mode"`
	Members  map[string]memberJSON `json:"members"`
}

type profileWrite struct {
	id       [16]byte
	cuteName string
	gameMode int16
	members  []memberUpdate
	object   []byte

	packed []byte
	hash   int64
}

type blobState struct {
	hash      int64
	fetchedAt time.Time
}

// storeProfiles stores one player's /v2/skyblock/profiles response: every profile once, however many of its members
// get looked up, plus the light rows for profiles, members, stats and the player's own profile list.
func storeProfiles(ctx context.Context, uuidText string, body []byte, fetchedAt time.Time, busy bool) error {
	uuid, err := parseUUID(uuidText)
	if err != nil {
		return err
	}

	fields, err := objectFields(body)
	if err != nil {
		return err
	}
	var profilesRaw []byte
	success := false
	for _, f := range fields {
		switch f.key {
		case "success":
			success = string(f.value) == "true"
		case "profiles":
			profilesRaw = f.value
		}
	}
	if !success {
		return nil
	}

	var rawProfiles [][]byte
	if len(profilesRaw) > 0 && string(profilesRaw) != "null" {
		if rawProfiles, err = arrayValues(profilesRaw); err != nil {
			return err
		}
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	profiles := make([]profileWrite, 0, len(rawProfiles))
	order := make([][16]byte, 0, len(rawProfiles))
	var selected *[16]byte
	for _, raw := range rawProfiles {
		idText, cuteName, object, err := SplitProfile(raw)
		if err != nil {
			return err
		}
		id, err := parseUUID(idText)
		if err != nil {
			return err
		}
		if isSelected(raw) {
			selected = &id
		}

		var light profileLight
		if err := json.Unmarshal(object, &light); err != nil {
			return err
		}
		write := profileWrite{id: id, cuteName: cuteName, gameMode: gameModes[light.GameMode]}
		for memberText, member := range light.Members {
			memberUUID, err := parseUUID(memberText)
			if err != nil {
				return err
			}
			m := memberUpdate{uuid: memberUUID}
			m.values, m.hidden = memberStatValues(&member)
			if member.Profile != nil {
				m.removed = member.Profile.DeletionNotice != nil ||
					(member.Profile.CoopInvitation != nil && !member.Profile.CoopInvitation.Confirmed)
			}
			write.members = append(write.members, m)
		}
		slices.SortFunc(write.members, func(a, b memberUpdate) int { return compareUUID(a.uuid, b.uuid) })
		write.object = object

		if write.packed, err = Pack(object); err != nil {
			return err
		}
		write.hash = int64(xxhash.Sum64(write.packed))
		profiles = append(profiles, write)
		order = append(order, id)
	}
	// Rows are locked in id order, so two coop members stored at once can't deadlock on each other's profiles.
	slices.SortFunc(profiles, func(a, b profileWrite) int { return compareUUID(a.id, b.id) })

	blobs, err := compressProfiles(ctx, profiles, fetchedAt, busy)
	if err != nil {
		return err
	}

	applied := false
	err = pgx.BeginFunc(ctx, postgres(), func(tx pgx.Tx) error {
		// Lock the player first: an older response for the same player must not undo a newer one.
		if _, err := tx.Exec(ctx, `INSERT INTO players (uuid) VALUES ($1) ON CONFLICT DO NOTHING`, uuid); err != nil {
			return err
		}
		var storedAt *time.Time
		if err := tx.QueryRow(ctx, `SELECT profiles_fetched_at FROM players WHERE uuid = $1 FOR UPDATE`, uuid).Scan(&storedAt); err != nil {
			return err
		}
		if storedAt != nil && !storedAt.Before(fetchedAt) {
			return nil
		}

		batch := &pgx.Batch{}
		for _, p := range profiles {
			batch.Queue(`INSERT INTO profiles (profile_id, seen_at, game_mode) VALUES ($1, $2, $3)
				ON CONFLICT (profile_id) DO UPDATE SET
					seen_at = greatest(profiles.seen_at, excluded.seen_at), game_mode = excluded.game_mode, deleted_at = NULL`,
				p.id, fetchedAt, p.gameMode)
		}
		for _, b := range blobs {
			b.queue(batch)
		}
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return err
		}

		batch = &pgx.Batch{}
		for _, p := range profiles {
			uuids := make([][16]byte, len(p.members))
			for i, m := range p.members {
				uuids[i] = m.uuid
				var removedAt *time.Time
				if m.removed {
					removedAt = &fetchedAt
				}
				var cuteName *string
				if m.uuid == uuid {
					cuteName = &p.cuteName
				}
				batch.Queue(`INSERT INTO profile_members (profile_id, player_uuid, removed_at, cute_name) VALUES ($1, $2, $3, $4)
					ON CONFLICT (profile_id, player_uuid) DO UPDATE SET
						removed_at = CASE WHEN excluded.removed_at IS NULL THEN NULL
							ELSE coalesce(profile_members.removed_at, excluded.removed_at) END,
						cute_name = coalesce(excluded.cute_name, profile_members.cute_name)`,
					p.id, m.uuid, removedAt, cuteName)
			}
			// Someone who no longer shows up in the members at all has left too.
			batch.Queue(`UPDATE profile_members SET removed_at = $3
				WHERE profile_id = $1 AND removed_at IS NULL AND NOT (player_uuid = ANY($2))`, p.id, uuids, fetchedAt)
		}
		// Profiles that left the player's list: the player left or deleted them. A profile nobody is active on
		// anymore is deleted, and its rows go after a grace period in case it was a glitch.
		batch.Queue(`WITH left_profiles AS (
				UPDATE profile_members SET removed_at = $3
				WHERE player_uuid = $1 AND removed_at IS NULL AND NOT (profile_id = ANY($2))
				RETURNING profile_id
			)
			UPDATE profiles p SET deleted_at = $3
			WHERE p.profile_id IN (SELECT profile_id FROM left_profiles) AND p.deleted_at IS NULL
				AND NOT EXISTS (SELECT 1 FROM profile_members m WHERE m.profile_id = p.profile_id
					AND m.removed_at IS NULL AND m.player_uuid <> $1)`, uuid, order, fetchedAt)
		batch.Queue(`UPDATE players SET profile_ids = $2, selected_profile = $3, profiles_fetched_at = $4, touched_at = now()
			WHERE uuid = $1`, uuid, order, selected, fetchedAt)
		applied = true
		return tx.SendBatch(ctx, batch).Close()
	})
	if err != nil || !applied {
		return err
	}

	if err := updateLeaderboardValues(ctx, uuid, profiles, blobs, fetchedAt); err != nil {
		fmt.Printf("[STORE] could not update leaderboards for %s: %v\n", uuidText, err)
	}
	return nil
}

// updateLeaderboardValues stores every member's values and moves them on the boards. Networth is recomputed only
// for profiles whose stored body changed, since it decodes every inventory.
func updateLeaderboardValues(ctx context.Context, requester [16]byte, profiles []profileWrite, blobs []blobWrite, seenAt time.Time) error {
	ids := make([][16]byte, len(profiles))
	for i, p := range profiles {
		update := profileValueUpdate{id: p.id, gameMode: p.gameMode, members: p.members}
		if blobs[i].replace {
			update.networth = profileNetworth(ctx, p.id, p.object)
		}
		if err := updateProfileValues(ctx, update, seenAt); err != nil {
			return err
		}
		ids[i] = p.id
	}

	// Members who left or were dropped from these profiles, and the requester's profiles that are gone.
	if err := reconcileMembers(ctx, `m.player_uuid = $1 OR m.profile_id = ANY($2)`, requester, ids); err != nil {
		return err
	}
	var gone [][16]byte
	rows, err := postgres().Query(ctx, `SELECT profile_id FROM profile_members WHERE player_uuid = $1`, requester)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id [16]byte
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		gone = append(gone, id)
	}
	rows.Close()
	return reconcileProfiles(ctx, gone)
}

func isSelected(raw []byte) bool {
	fields, err := objectFields(raw)
	if err != nil {
		return false
	}
	for _, f := range fields {
		if f.key == "selected" {
			return string(f.value) == "true"
		}
	}
	return false
}

func compareUUID(a, b [16]byte) int {
	return strings.Compare(string(a[:]), string(b[:]))
}

// blobWrite is one decision about a stored blob: replace it, only record that it is still current, or only keep it
// from being evicted.
type blobWrite struct {
	profileID [16]byte
	kind      int16
	fetchedAt time.Time
	hash      int64

	replace bool
	codec   int32
	data    []byte
	bytes   int
}

func (b blobWrite) queue(batch *pgx.Batch) {
	if b.replace {
		batch.Queue(`INSERT INTO blobs (profile_id, kind, codec, hash, fetched_at, bytes, data) VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (profile_id, kind) DO UPDATE SET
				codec = excluded.codec, hash = excluded.hash, fetched_at = excluded.fetched_at,
				bytes = excluded.bytes, data = excluded.data, accessed_on = current_date
			WHERE blobs.fetched_at < excluded.fetched_at`,
			b.profileID, b.kind, b.codec, b.hash, b.fetchedAt, b.bytes, b.data)
		return
	}
	// Same content: it is current as of this fetch. Different content within blobRewriteInterval: keep it.
	batch.Queue(`UPDATE blobs SET
			fetched_at = CASE WHEN hash = $3 AND fetched_at < $4 THEN $4 ELSE fetched_at END,
			accessed_on = current_date
		WHERE profile_id = $1 AND kind = $2`, b.profileID, b.kind, b.hash, b.fetchedAt)
}

// decideBlob compares new content with what is stored and compresses it only when it will be written.
func decideBlob(stored *blobState, profileID [16]byte, kind int16, packed []byte, hash int64, fetchedAt time.Time, busy bool) blobWrite {
	write := blobWrite{profileID: profileID, kind: kind, fetchedAt: fetchedAt, hash: hash}
	if stored != nil && (stored.hash == hash || fetchedAt.Sub(stored.fetchedAt) < blobRewriteInterval) {
		return write
	}
	write.replace = true
	write.bytes = len(packed)
	if len(packed) > 0 {
		write.codec, write.data = compress(kind, packed, busy)
		write.bytes = len(write.data)
	} else {
		write.data = []byte{}
	}
	return write
}

func compressProfiles(ctx context.Context, profiles []profileWrite, fetchedAt time.Time, busy bool) ([]blobWrite, error) {
	ids := make([][16]byte, len(profiles))
	for i, p := range profiles {
		ids[i] = p.id
	}
	states, err := blobStates(ctx, kindProfile, ids)
	if err != nil {
		return nil, err
	}

	writes := make([]blobWrite, len(profiles))
	for i, p := range profiles {
		writes[i] = decideBlob(states[p.id], p.id, kindProfile, p.packed, p.hash, fetchedAt, busy)
	}
	return writes, nil
}

func blobStates(ctx context.Context, kind int16, ids [][16]byte) (map[[16]byte]*blobState, error) {
	rows, err := postgres().Query(ctx, `SELECT profile_id, hash, fetched_at FROM blobs WHERE kind = $1 AND profile_id = ANY($2)`, kind, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := map[[16]byte]*blobState{}
	for rows.Next() {
		var id [16]byte
		var state blobState
		if err := rows.Scan(&id, &state.hash, &state.fetchedAt); err != nil {
			return nil, err
		}
		states[id] = &state
	}
	return states, rows.Err()
}

// storeBlob stores a museum or garden body. A nil body records that Hypixel has none.
func storeBlob(ctx context.Context, profileText string, kind int16, body []byte, fetchedAt time.Time, busy bool) error {
	profileID, err := parseUUID(profileText)
	if err != nil {
		return err
	}

	var packed []byte
	if body != nil {
		if packed, err = Pack(body); err != nil {
			return err
		}
	}
	hash := int64(xxhash.Sum64(packed))

	states, err := blobStates(ctx, kind, [][16]byte{profileID})
	if err != nil {
		return err
	}
	write := decideBlob(states[profileID], profileID, kind, packed, hash, fetchedAt, busy)

	batch := &pgx.Batch{}
	write.queue(batch)
	return postgres().SendBatch(ctx, batch).Close()
}

func storePlayer(ctx context.Context, uuidText string, body []byte, fetchedAt time.Time) error {
	uuid, err := parseUUID(uuidText)
	if err != nil {
		return err
	}
	trimmed, err := TrimPlayer(body)
	if err != nil {
		return nil
	}
	_, data := compress(0, trimmed, false)

	_, err = postgres().Exec(ctx, `INSERT INTO players (uuid, player_data, player_fetched_at) VALUES ($1, $2, $3)
		ON CONFLICT (uuid) DO UPDATE SET player_data = excluded.player_data, player_fetched_at = excluded.player_fetched_at,
			touched_at = now()
		WHERE players.player_fetched_at IS NULL OR players.player_fetched_at < excluded.player_fetched_at`,
		uuid, data, fetchedAt)
	return err
}

func storeIdentity(ctx context.Context, uuidText, name string, seenAt time.Time) error {
	uuid, err := parseUUID(uuidText)
	if err != nil {
		return err
	}
	// A name is only rewritten when it changed or once a day, so lookups don't churn rows.
	_, err = postgres().Exec(ctx, `INSERT INTO players (uuid, username, name_seen_at) VALUES ($1, $2, $3)
		ON CONFLICT (uuid) DO UPDATE SET username = excluded.username, name_seen_at = excluded.name_seen_at, touched_at = now()
		WHERE players.username IS DISTINCT FROM excluded.username
			OR players.name_seen_at IS NULL OR players.name_seen_at < excluded.name_seen_at - interval '1 day'`,
		uuid, name, seenAt)
	return err
}

func touchBlob(ctx context.Context, profileText string, kind int16) error {
	profileID, err := parseUUID(profileText)
	if err != nil {
		return err
	}
	_, err = postgres().Exec(ctx, `UPDATE blobs SET accessed_on = current_date
		WHERE profile_id = $1 AND kind = $2 AND accessed_on < current_date`, profileID, kind)
	return err
}
