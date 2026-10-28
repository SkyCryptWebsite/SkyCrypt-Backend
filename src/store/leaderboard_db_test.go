package store

import (
	"context"
	"os"
	"testing"
	"time"

	"skycrypt/src/db"
)

// testRedis connects to SKYCRYPT_TEST_REDIS_ADDR, a disposable Redis that is flushed before each run.
func testRedis(t *testing.T, ctx context.Context) {
	t.Helper()
	addr := os.Getenv("SKYCRYPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("SKYCRYPT_TEST_REDIS_ADDR is not set")
	}
	if _, err := db.Redis(); err != nil {
		if err := db.InitRedis(addr, "", 0); err != nil {
			t.Fatal(err)
		}
	}
	client, _ := db.Redis()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	if err := putScript.Load(ctx, client).Err(); err != nil {
		t.Fatal(err)
	}
	thresholds.Clear()
	statKeyCache.Lock()
	statKeyCache.byKey, statKeyCache.byID = map[string]int32{}, map[int32]string{}
	statKeyCache.Unlock()
}

func board(t *testing.T, ctx context.Context, key, mode string) []Entry {
	t.Helper()
	page, err := LeaderboardPage(ctx, key, mode, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	return page.Entries
}

func rankedUUIDs(entries []Entry) []string {
	var out []string
	for _, entry := range entries {
		out = append(out, entry.UUID)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

const bigMember = `{"leveling":{"experience":30000},
	"player_data":{"experience":{"SKILL_FARMING":5000,"SKILL_DUNGEONEERING":10,"SKILL_FORAGING_extra_level_cap":3}},
	"slayer":{"slayer_bosses":{"zombie":{"xp":2000},"wolf":{"xp":1000},"vampire":{}}},
	"dungeons":{"dungeon_types":{"catacombs":{"experience":900,"tier_completions":{"7":4,"total":4},
		"fastest_time_s_plus":{"7":250000,"best":250000}}}},
	"nether_island_player_data":{"kuudra_completed_tiers":{"infernal":10,"hot":2,"highest_wave_hot":5}},
	"player_stats":{"kills":{"zombie":50,"pest_mite":7},"deaths":{"zombie":3}},
	"collection":{"WHEAT":100},
	"inventory":{}}`

const smallMember = `{"leveling":{"experience":10000},
	"player_data":{"experience":{"SKILL_FARMING":9000}},
	"dungeons":{"dungeon_types":{"catacombs":{"experience":100,"fastest_time_s_plus":{"7":200000}}}},
	"player_stats":{"kills":{"zombie":80}},
	"collection":{"WHEAT":50},
	"inventory":{}}`

func TestLeaderboardsRankEveryStat(t *testing.T) {
	ctx := testDatabase(t)
	testRedis(t, ctx)
	now := time.Now()

	body := `{"success":true,"profiles":[` +
		profileJSON(coop, "Apple", "", true, map[string]string{alice: bigMember, bob: smallMember}) + `,` +
		profileJSON(ironman, "Banana", "ironman", false, map[string]string{alice: `{"leveling":{"experience":20000},"inventory":{}}`}) + `]}`
	if err := storeProfiles(ctx, alice, []byte(body), now, false); err != nil {
		t.Fatal(err)
	}

	cases := map[string][]string{
		"skyblock_level":                         {alice, alice, bob},
		"skill:farming":                          {bob, alice},
		"slayer:total":                           {alice},
		"slayer:zombie":                          {alice},
		"dungeons:catacombs":                     {alice, bob},
		"dungeons:catacombs:7:completions":       {alice},
		"dungeons:catacombs:total:completions":   {alice},
		"dungeons:catacombs:7:fastest_s_plus":    {bob, alice}, // lower time ranks higher
		"kuudra:infernal":                        {alice},
		"kuudra:total":                           {alice},
		"kills:zombie":                           {bob, alice},
		"deaths:zombie":                          {alice},
		"pests:mite":                             {alice},
		"collection:WHEAT":                       {alice, bob},
		"skill:dungeoneering":                    nil,
		"kills:pest_mite":                        nil,
		"kuudra:highest_wave_hot":                nil,
		"slayer:vampire":                         nil,
		"dungeons:catacombs:best:fastest_s_plus": nil,
	}
	for key, want := range cases {
		if got := rankedUUIDs(board(t, ctx, key, "all")); !equal(got, want) {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}

	if entries := board(t, ctx, "dungeons:catacombs:7:fastest_s_plus", "all"); entries[0].Raw != 200000 {
		t.Errorf("fastest time raw = %d, want 200000", entries[0].Raw)
	}

	// The coop board sums every active member's collection.
	coopBoard := board(t, ctx, "coop_collection:WHEAT", "all")
	if len(coopBoard) != 1 || !coopBoard[0].Profile || coopBoard[0].Raw != 150 || len(coopBoard[0].Members) != 2 {
		t.Fatalf("coop board = %+v", coopBoard)
	}
	if got := rankedUUIDs(board(t, ctx, "skyblock_level", "ironman")); !equal(got, []string{alice}) {
		t.Errorf("ironman board = %v", got)
	}

	ranks, err := PlayerRanks(ctx, alice)
	if err != nil || len(ranks) != 2 {
		t.Fatalf("player ranks: %+v %v", ranks, err)
	}
	foundCoop := false
	for _, r := range ranks[0].Ranks {
		if r.Key == "coop_collection:WHEAT" && r.Profile && r.Rank == 1 {
			foundCoop = true
		}
	}
	if !foundCoop {
		t.Errorf("alice's ranks miss the coop collection: %+v", ranks[0].Ranks)
	}

	// Alice hides her collections: the old value stays, marked older than the fetch.
	hiddenBody := `{"success":true,"profiles":[` +
		profileJSON(coop, "Apple", "", true, map[string]string{alice: `{"leveling":{"experience":31000},"inventory":{}}`, bob: smallMember}) + `,` +
		profileJSON(ironman, "Banana", "ironman", false, map[string]string{alice: `{"leveling":{"experience":20000},"inventory":{}}`}) + `]}`
	if err := storeProfiles(ctx, alice, []byte(hiddenBody), now.Add(time.Minute), false); err != nil {
		t.Fatal(err)
	}
	wheat := board(t, ctx, "collection:WHEAT", "all")
	if len(wheat) != 2 || wheat[0].UUID != alice || !wheat[0].UpdatedAt.Before(wheat[0].SeenAt) {
		t.Fatalf("hidden collection: %+v", wheat)
	}
	if got := rankedUUIDs(board(t, ctx, "kills:zombie", "all")); !equal(got, []string{bob}) {
		t.Errorf("kills aren't hideable, so alice's are gone with them: %v", got)
	}

	// Hiding a player takes them off, unhiding brings them back.
	if err := HidePlayer(ctx, bob, "test"); err != nil {
		t.Fatal(err)
	}
	if got := rankedUUIDs(board(t, ctx, "skill:farming", "all")); !equal(got, []string{alice}) {
		t.Errorf("after hiding bob: %v", got)
	}
	if err := UnhidePlayer(ctx, bob); err != nil {
		t.Fatal(err)
	}
	if got := rankedUUIDs(board(t, ctx, "skill:farming", "all")); !equal(got, []string{bob, alice}) {
		t.Errorf("after unhiding bob: %v", got)
	}

	// Alice deletes her ironman profile.
	left := `{"success":true,"profiles":[` +
		profileJSON(coop, "Apple", "", true, map[string]string{alice: bigMember, bob: smallMember}) + `]}`
	if err := storeProfiles(ctx, alice, []byte(left), now.Add(2*time.Minute), false); err != nil {
		t.Fatal(err)
	}
	if got := board(t, ctx, "skyblock_level", "ironman"); len(got) != 0 {
		t.Errorf("deleted ironman profile still ranked: %v", rankedUUIDs(got))
	}

	// After a Redis restart the boards come back from Postgres.
	client, _ := db.Redis()
	_ = client.FlushDB(ctx).Err()
	if err := rebuildBoards(ctx); err != nil {
		t.Fatal(err)
	}
	page, _ := LeaderboardPage(ctx, "skill:farming", "all", 0, 10)
	if !page.Ready || !equal(rankedUUIDs(page.Entries), []string{bob, alice}) {
		t.Fatalf("rebuilt: ready=%v %v", page.Ready, rankedUUIDs(page.Entries))
	}
}

func TestLeaderboardsKeepOnlyTheTop(t *testing.T) {
	ctx := testDatabase(t)
	testRedis(t, ctx)
	t.Setenv("LEADERBOARD_SIZE", "2")

	carol := "33333333333333333333333333333333"
	members := map[string]string{
		alice: `{"leveling":{"experience":100}}`,
		bob:   `{"leveling":{"experience":300}}`,
		carol: `{"leveling":{"experience":200}}`,
	}
	body := `{"success":true,"profiles":[` + profileJSON(coop, "Apple", "", true, members) + `]}`
	if err := storeProfiles(ctx, alice, []byte(body), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	if got := rankedUUIDs(board(t, ctx, "skyblock_level", "all")); !equal(got, []string{bob, carol}) {
		t.Fatalf("capped board = %v, want bob and carol", got)
	}

	// A rebuild at another size refills from Postgres.
	t.Setenv("LEADERBOARD_SIZE", "3")
	if err := rebuildBoards(ctx); err != nil {
		t.Fatal(err)
	}
	if got := rankedUUIDs(board(t, ctx, "skyblock_level", "all")); !equal(got, []string{bob, carol, alice}) {
		t.Fatalf("rebuilt board = %v", got)
	}
}

func TestGardenAndBackfill(t *testing.T) {
	ctx := testDatabase(t)
	testRedis(t, ctx)
	now := time.Now()

	body := `{"success":true,"profiles":[` + profileJSON(coop, "Apple", "", true, map[string]string{alice: bigMember}) + `]}`
	if err := storeProfiles(ctx, alice, []byte(body), now, false); err != nil {
		t.Fatal(err)
	}
	garden := []byte(`{"success":true,"garden":{"garden_experience":5000,"resources_collected":{"WHEAT":123,"MELON":0}}}`)
	if err := storeBlob(ctx, coop, kindGarden, garden, now, false); err != nil {
		t.Fatal(err)
	}
	if err := updateGardenValues(ctx, mustUUID(t, coop), garden, now); err != nil {
		t.Fatal(err)
	}
	if got := board(t, ctx, "garden:milestone:WHEAT", "all"); len(got) != 1 || got[0].Raw != 123 || !got[0].Profile {
		t.Fatalf("garden milestone board = %+v", got)
	}
	if got := board(t, ctx, "garden:milestone:MELON", "all"); len(got) != 0 {
		t.Fatalf("zero milestone ranked: %+v", got)
	}
	// The garden must not wipe the coop collections it shares the record with.
	if got := board(t, ctx, "coop_collection:WHEAT", "all"); len(got) != 1 {
		t.Fatalf("coop collection lost after garden update: %+v", got)
	}

	// Records written by an older set of stats get recomputed from the stored profile.
	if _, err := db.Postgres().Exec(ctx, `UPDATE member_values SET version = 1, data = ''`); err != nil {
		t.Fatal(err)
	}
	if done, err := backfillStats(ctx); err != nil || done != 1 {
		t.Fatalf("backfilled %d profiles, err %v", done, err)
	}
	if got := board(t, ctx, "kills:zombie", "all"); len(got) != 1 || got[0].Raw != 50 {
		t.Fatalf("after backfill: %+v", got)
	}
	if done, _ := backfillStats(ctx); done != 0 {
		t.Fatalf("a second backfill did %d profiles", done)
	}
}
