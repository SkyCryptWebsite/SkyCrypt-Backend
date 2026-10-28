package store

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"skycrypt/src/db"
)

// These tests need a disposable Postgres: SKYCRYPT_TEST_DATABASE_URL is wiped before each run.
func testDatabase(t *testing.T) context.Context {
	t.Helper()
	url := os.Getenv("SKYCRYPT_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("SKYCRYPT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	if db.Postgres() == nil {
		if err := db.InitPostgres(url); err != nil {
			t.Fatal(err)
		}
	}
	pool := db.Postgres()
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	kindEncoders.Clear()
	if err := migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	enabled.Store(true)
	return ctx
}

func profileJSON(id, cuteName, gameMode string, selected bool, members map[string]string) string {
	var b strings.Builder
	b.WriteString(`{"profile_id":"` + id + `","cute_name":"` + cuteName + `","selected":`)
	if selected {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	if gameMode != "" {
		b.WriteString(`,"game_mode":"` + gameMode + `"`)
	}
	b.WriteString(`,"members":{`)
	for i, uuid := range slices.Sorted(maps.Keys(members)) {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"` + uuid + `":` + members[uuid])
	}
	b.WriteString(`}}`)
	return b.String()
}

const (
	alice   = "11111111111111111111111111111111"
	bob     = "22222222222222222222222222222222"
	coop    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	solo    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	ironman = "cccccccc-cccc-cccc-cccc-cccccccccccc"
)

func TestStoreProfilesRoundTripAndCoopDedupe(t *testing.T) {
	ctx := testDatabase(t)
	inventory := gzipBase64(t, strings.Repeat("sword ", 50))
	member := func(xp int) string {
		return `{"leveling":{"experience":` + itoa(xp) + `},"inventory":{"inv_contents":{"type":0,"data":"` + inventory + `"}}}`
	}

	now := time.Now().Truncate(time.Millisecond)
	aliceBody := `{"success":true,"profiles":[` +
		profileJSON(coop, "Apple", "", true, map[string]string{alice: member(5000), bob: member(100)}) + `,` +
		profileJSON(solo, "Banana", "ironman", false, map[string]string{alice: member(42)}) + `]}`
	bobBody := `{"success":true,"profiles":[` +
		profileJSON(coop, "Kiwi", "", true, map[string]string{alice: member(5000), bob: member(100)}) + `]}`

	if err := storeProfiles(ctx, alice, []byte(aliceBody), now); err != nil {
		t.Fatal(err)
	}
	if err := storeProfiles(ctx, bob, []byte(bobBody), now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	var blobs int
	if err := db.Postgres().QueryRow(ctx, `SELECT count(*) FROM blobs WHERE kind = 1`).Scan(&blobs); err != nil {
		t.Fatal(err)
	}
	if blobs != 2 {
		t.Fatalf("stored %d profile blobs, want 2 (the coop once)", blobs)
	}

	for uuid, original := range map[string]string{alice: aliceBody, bob: bobBody} {
		rebuilt, _, ok := LoadProfiles(ctx, uuid)
		if !ok {
			t.Fatalf("%s: nothing stored", uuid)
		}
		assertSameProfiles(t, []byte(original), rebuilt)
	}

	var mode int16
	if err := db.Postgres().QueryRow(ctx, `SELECT game_mode FROM profiles WHERE profile_id = $1`, mustUUID(t, solo)).Scan(&mode); err != nil || mode != 1 {
		t.Fatalf("solo profile mode = %d, err %v", mode, err)
	}
}

func TestStoreProfilesMarksLeftAndDeletedProfiles(t *testing.T) {
	ctx := testDatabase(t)
	now := time.Now()
	both := map[string]string{alice: `{}`, bob: `{}`}

	first := `{"success":true,"profiles":[` + profileJSON(coop, "Apple", "", true, both) + `,` +
		profileJSON(solo, "Banana", "", false, map[string]string{alice: `{}`}) + `]}`
	if err := storeProfiles(ctx, alice, []byte(first), now); err != nil {
		t.Fatal(err)
	}

	// Alice leaves the coop and deletes her solo profile.
	later := `{"success":true,"profiles":null}`
	if err := storeProfiles(ctx, alice, []byte(later), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	var coopDeleted, soloDeleted bool
	err := db.Postgres().QueryRow(ctx, `SELECT
		(SELECT deleted_at IS NOT NULL FROM profiles WHERE profile_id = $1),
		(SELECT deleted_at IS NOT NULL FROM profiles WHERE profile_id = $2)`, mustUUID(t, coop), mustUUID(t, solo)).
		Scan(&coopDeleted, &soloDeleted)
	if err != nil {
		t.Fatal(err)
	}
	if coopDeleted || !soloDeleted {
		t.Fatalf("coop deleted=%v (Bob is still on it), solo deleted=%v", coopDeleted, soloDeleted)
	}

	body, _, ok := LoadProfiles(ctx, alice)
	if !ok || string(body) != `{"success":true,"profiles":null}` {
		t.Fatalf("alice: %s %v", body, ok)
	}

	// An older response arriving late must not bring the profiles back.
	if err := storeProfiles(ctx, alice, []byte(first), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	body, _, _ = LoadProfiles(ctx, alice)
	if string(body) != `{"success":true,"profiles":null}` {
		t.Fatalf("an older response overwrote a newer one: %s", body)
	}
}

func TestEvictToBudgetDropsLeastRecentlyUsed(t *testing.T) {
	ctx := testDatabase(t)

	big := gzipBase64(t, randomText(400_000))
	for i, id := range []string{coop, solo, ironman} {
		body := []byte(`{"success":true,"members":{"x":{"items":{"data":"` + big + `"}}}}`)
		if err := storeBlob(ctx, id, kindMuseum, body, time.Now()); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Postgres().Exec(ctx, `UPDATE blobs SET accessed_on = current_date - $2::integer WHERE profile_id = $1`,
			mustUUID(t, id), 3-i); err != nil {
			t.Fatal(err)
		}
	}

	// Leave room for the light tables and a bit more than half of the blobs.
	var stored, light int64
	err := db.Postgres().QueryRow(ctx, `SELECT (SELECT sum(bytes) FROM blobs),
		pg_total_relation_size('players') + pg_total_relation_size('profiles') + pg_total_relation_size('profile_members')
			+ pg_total_relation_size('member_values') + pg_total_relation_size('profile_values') + pg_total_relation_size('stat_keys')
			+ pg_total_relation_size('resources') + pg_total_relation_size('codec_dicts')`).
		Scan(&stored, &light)
	if err != nil {
		t.Fatal(err)
	}
	limit := (float64(light) + float64(stored)*0.6) / blobShare
	t.Setenv("STORAGE_LIMIT_GB", strconv.FormatFloat(limit/(1<<30), 'g', -1, 64))

	if err := evictToBudget(ctx, db.Postgres()); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := LoadMuseum(ctx, coop); ok {
		t.Fatal("the least recently used blob survived")
	}
	if _, _, ok := LoadMuseum(ctx, ironman); !ok {
		t.Fatal("the most recently used blob was evicted")
	}
}

// TestStoreRealSample stores the storagebench sample and checks that every player's response comes back with the
// same content. Set SKYCRYPT_BENCHDATA to the benchdata directory.
func TestStoreRealSample(t *testing.T) {
	dir := os.Getenv("SKYCRYPT_BENCHDATA")
	if dir == "" {
		t.Skip("SKYCRYPT_BENCHDATA is not set")
	}
	ctx := testDatabase(t)

	entries, err := os.ReadDir(filepath.Join(dir, "players"))
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string][]byte{}
	var originalStored int
	start := time.Now()
	for i, entry := range entries {
		body, err := os.ReadFile(filepath.Join(dir, "players", entry.Name(), "profiles.json"))
		if err != nil {
			continue
		}
		bodies[entry.Name()] = body
		originalStored += len(plainEncoders.encoder.EncodeAll(body, nil))
		if err := storeProfiles(ctx, entry.Name(), body, start.Add(time.Duration(i)*time.Millisecond)); err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
	}
	t.Logf("stored %d players in %v", len(bodies), time.Since(start).Round(time.Millisecond))

	// Train the profile dictionary from what is stored, rewrite everything with it, and compare again.
	for _, train := range []bool{false, true} {
		if train {
			samples, err := dictSamples(ctx, kindProfile)
			if err != nil {
				t.Fatal(err)
			}
			trained, err := trainDict(samples, 1)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Postgres().Exec(ctx, `INSERT INTO codec_dicts (id, kind, data) VALUES (1, 1, $1)`, trained); err != nil {
				t.Fatal(err)
			}
			if err := useDict(kindProfile, 1, trained); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Postgres().Exec(ctx, `DELETE FROM blobs; UPDATE players SET profiles_fetched_at = NULL`); err != nil {
				t.Fatal(err)
			}
			for name, body := range bodies {
				if err := storeProfiles(ctx, name, body, time.Now()); err != nil {
					t.Fatal(err)
				}
			}
		}

		var stored int64
		if err := db.Postgres().QueryRow(ctx, `SELECT sum(bytes) FROM blobs`).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		t.Logf("dictionary=%v: %d players, old format %.1f MB, new blobs %.1f MB", train, len(bodies),
			float64(originalStored)/(1<<20), float64(stored)/(1<<20))

		var slowest time.Duration
		for name, body := range bodies {
			loadStart := time.Now()
			rebuilt, _, ok := LoadProfiles(ctx, name)
			slowest = max(slowest, time.Since(loadStart))
			if !ok {
				t.Fatalf("%s: nothing stored", name)
			}
			assertSameProfiles(t, body, rebuilt)
		}
		t.Logf("dictionary=%v: slowest load %v", train, slowest.Round(time.Millisecond))
	}
}

// assertSameProfiles checks that a rebuilt response holds the same profiles, the same requester fields and the same
// JSON and NBT content as the original.
func assertSameProfiles(t *testing.T, original, rebuilt []byte) {
	t.Helper()
	var a, b struct {
		Profiles []json.RawMessage `json:"profiles"`
	}
	if err := json.Unmarshal(original, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rebuilt, &b); err != nil {
		t.Fatalf("rebuilt body is not JSON: %v", err)
	}
	if len(a.Profiles) != len(b.Profiles) {
		t.Fatalf("%d profiles, want %d", len(b.Profiles), len(a.Profiles))
	}
	for i := range a.Profiles {
		idA, nameA, objectA, err := SplitProfile(a.Profiles[i])
		if err != nil {
			t.Fatal(err)
		}
		idB, nameB, objectB, err := SplitProfile(b.Profiles[i])
		if err != nil {
			t.Fatal(err)
		}
		if idA != idB || nameA != nameB || isSelected(a.Profiles[i]) != isSelected(b.Profiles[i]) {
			t.Fatalf("profile %d: got %s %q selected=%v, want %s %q selected=%v", i, idB, nameB,
				isSelected(b.Profiles[i]), idA, nameA, isSelected(a.Profiles[i]))
		}
		packedA, _ := Pack(objectA)
		packedB, _ := Pack(objectB)
		if !SameContent(packedA, packedB) {
			t.Fatalf("profile %s changed", idA)
		}
	}
}

func mustUUID(t *testing.T, value string) [16]byte {
	t.Helper()
	id, err := parseUUID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func randomText(n int) string {
	out := make([]byte, n)
	state := uint32(1)
	for i := range out {
		state = state*1664525 + 1013904223
		out[i] = 'a' + byte(state>>24)%26
	}
	return string(out)
}

// legacyTable creates fallback_cache the way the old store did and fills it with zstd bodies.
func legacyTable(t *testing.T, ctx context.Context, rows map[string][]byte) {
	t.Helper()
	pool := db.Postgres()
	if _, err := pool.Exec(ctx, `CREATE TABLE fallback_cache (
		key text PRIMARY KEY, body bytea NOT NULL, hash bigint NOT NULL, fetched_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	for key, body := range rows {
		if _, err := pool.Exec(ctx, `INSERT INTO fallback_cache (key, body, hash) VALUES ($1, $2, 0)`,
			key, plainEncoders.encoder.EncodeAll(body, nil)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestImportLegacyMovesEveryKindAndDropsTheTable(t *testing.T) {
	ctx := testDatabase(t)
	inventory := gzipBase64(t, strings.Repeat("bow ", 60))
	profiles := `{"success":true,"profiles":[` + profileJSON(coop, "Apple", "", true,
		map[string]string{alice: `{"inventory":{"inv_contents":{"type":0,"data":"` + inventory + `"}}}`}) + `]}`
	museum := `{"success":true,"members":{"` + alice + `":{"value":1,"items":{"X":{"items":{"type":0,"data":"` + inventory + `"}}}}}}`

	legacyTable(t, ctx, map[string][]byte{
		"profiles:" + alice:     []byte(profiles),
		"player:" + alice:       []byte(`{"success":true,"player":{"displayname":"Alice","uuid":"` + alice + `","rank":"ADMIN","extra":"dropped"}}`),
		"museum:" + coop:        []byte(museum),
		"garden:" + coop:        []byte(`{"success":true,"garden":{"uuid":"` + coop + `"}}`),
		"username:" + alice:     []byte("Alice"),
		"uuid:alice":            []byte(alice),
		"mowojangUUID:" + bob:   []byte(`{"id":"` + bob + `","name":"Bob"}`),
		"skyblock_collections":  []byte(`{"success":true,"collections":{}}`),
		"profiles:" + "notauid": []byte(profiles),
	})

	if err := importLegacy(ctx, db.Postgres()); err != nil {
		t.Fatal(err)
	}

	var exists bool
	if err := db.Postgres().QueryRow(ctx, `SELECT to_regclass('fallback_cache') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("fallback_cache still exists (err %v)", err)
	}
	rebuilt, _, ok := LoadProfiles(ctx, alice)
	if !ok {
		t.Fatal("profiles were not imported")
	}
	assertSameProfiles(t, []byte(profiles), rebuilt)
	if body, _, ok := LoadMuseum(ctx, coop); !ok || !strings.Contains(string(body), `"value":1`) {
		t.Fatalf("museum: %s %v", body, ok)
	}
	if _, _, ok := LoadGarden(ctx, coop); !ok {
		t.Fatal("garden was not imported")
	}
	if body, _, ok := LoadPlayer(ctx, alice); !ok || strings.Contains(string(body), "dropped") || !strings.Contains(string(body), "ADMIN") {
		t.Fatalf("player: %s %v", body, ok)
	}
	if uuid, name, ok := LookupUUID(ctx, "ALICE"); !ok || uuid != alice || name != "Alice" {
		t.Fatalf("alice identity: %s %s %v", uuid, name, ok)
	}
	if name, ok := LookupName(ctx, bob); !ok || name != "Bob" {
		t.Fatalf("bob identity: %s %v", name, ok)
	}
	if _, ok := LoadResource(ctx, "skyblock_collections"); !ok {
		t.Fatal("resource was not imported")
	}
}

// TestImportLegacyRealSample imports the storagebench sample from an old-format table.
func TestImportLegacyRealSample(t *testing.T) {
	dir := os.Getenv("SKYCRYPT_BENCHDATA")
	if dir == "" {
		t.Skip("SKYCRYPT_BENCHDATA is not set")
	}
	ctx := testDatabase(t)

	rows := map[string][]byte{}
	entries, _ := os.ReadDir(filepath.Join(dir, "players"))
	for _, entry := range entries {
		if body, err := os.ReadFile(filepath.Join(dir, "players", entry.Name(), "profiles.json")); err == nil {
			rows["profiles:"+entry.Name()] = body
		}
		if body, err := os.ReadFile(filepath.Join(dir, "players", entry.Name(), "player.json")); err == nil {
			rows["player:"+entry.Name()] = body
		}
	}
	profileDirs, _ := os.ReadDir(filepath.Join(dir, "profiles"))
	for _, entry := range profileDirs {
		if body, err := os.ReadFile(filepath.Join(dir, "profiles", entry.Name(), "museum.json")); err == nil {
			rows["museum:"+entry.Name()] = body
		}
	}
	legacyTable(t, ctx, rows)

	var before int64
	_ = db.Postgres().QueryRow(ctx, `SELECT pg_total_relation_size('fallback_cache')`).Scan(&before)
	start := time.Now()
	if err := importLegacy(ctx, db.Postgres()); err != nil {
		t.Fatal(err)
	}
	var after int64
	_ = db.Postgres().QueryRow(ctx, `SELECT sum(bytes) FROM blobs`).Scan(&after)
	t.Logf("imported %d rows in %v: old table %.1f MB, new blobs %.1f MB", len(rows),
		time.Since(start).Round(time.Second), float64(before)/(1<<20), float64(after)/(1<<20))

	for key, body := range rows {
		uuid, ok := strings.CutPrefix(key, "profiles:")
		if !ok {
			continue
		}
		rebuilt, _, ok := LoadProfiles(ctx, uuid)
		if !ok {
			t.Fatalf("%s: not imported", uuid)
		}
		assertSameProfiles(t, body, rebuilt)
	}
}
