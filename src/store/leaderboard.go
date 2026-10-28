package store

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"skycrypt/src/db"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Every stat key is a leaderboard. Postgres keeps every value (member_values, profile_values); Redis keeps the top
// LEADERBOARD_SIZE entries of each board as a sorted set lb:<key>:<mode>, scored so that higher is better (run
// times are negated). Entries are "m<profile_members.id>" for a player on a profile and "p<profiles.id>" for a
// profile (coop collections, garden). Each entry is on its game mode's board and on "all".
//
// Boards have no TTL, so Redis must evict with volatile-* (every cache key has a TTL). The parent rebuilds every
// board from Postgres when Redis lost them, when the size or the stats change, and once a day, which also refills
// boards that lost an entry to a player who dropped out of the top.

// Modes are the boards every stat has. "all" mixes every game mode.
var Modes = []string{"all", "normal", "ironman", "stranded", "bingo"}

// modeNames maps profiles.game_mode to its board.
var modeNames = map[int16]string{0: "normal", 1: "ironman", 2: "stranded", 3: "bingo"}

const (
	defaultBoardSize = 1000
	readyKey         = "lb:ready"
	boardsKey        = "lb:boards"
	rebuildEvery     = 24 * time.Hour
	checkEvery       = 10 * time.Minute
)

var (
	ErrUnknownMode   = errors.New("unknown leaderboard mode")
	ErrUnavailable   = errors.New("leaderboards are unavailable")
	errNoRebuildLock = errors.New("another process is rebuilding the leaderboards")
)

// BoardSize is LEADERBOARD_SIZE (default 1000): how many entries each board keeps.
func BoardSize() int64 {
	if size, err := strconv.ParseInt(os.Getenv("LEADERBOARD_SIZE"), 10, 64); err == nil && size > 0 {
		return size
	}
	return defaultBoardSize
}

func boardKey(stat, mode string) string { return "lb:" + stat + ":" + mode }

// readyValue changes when the boards must be rebuilt: another size or another set of stats.
func readyValue() string { return fmt.Sprintf("%d:%d", BoardSize(), statsVersion) }

func boardScore(key string, value int64) float64 {
	if Ascending(key) {
		return -float64(value)
	}
	return float64(value)
}

func memberEntity(id int64) string  { return "m" + strconv.FormatInt(id, 10) }
func profileEntity(id int64) string { return "p" + strconv.FormatInt(id, 10) }

func isMode(mode string) bool {
	for _, m := range Modes {
		if m == mode {
			return true
		}
	}
	return false
}

// putScript adds an entry and trims the board to its size. It returns the lowest score left once the board is full,
// which the caller remembers to skip values that can't get on.
var putScript = redis.NewScript(`
redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
redis.call('SADD', KEYS[2], KEYS[1])
local n = redis.call('ZCARD', KEYS[1])
local size = tonumber(ARGV[3])
if n > size then
	redis.call('ZREMRANGEBYRANK', KEYS[1], 0, n - size - 1)
	n = size
end
if n >= size then
	return redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')[2]
end
return false`)

// thresholds holds the lowest score of each full board (parent only). A stale threshold only means a value is
// written that the script trims again, or skipped until the next daily rebuild.
var thresholds sync.Map // board -> float64

// boardState is where a record sits: its values, its mode, and whether it belongs on the boards at all (not left,
// deleted or hidden).
type boardState struct {
	values valueSet
	mode   int16
	ranked bool
}

// syncBoards moves an entity from the boards it was on (old, nil when it was on none) to the ones it belongs on
// now. Unchanged values aren't touched, and values below a full board's lowest entry are skipped.
func syncBoards(ctx context.Context, entity string, old *boardState, now boardState) error {
	client, err := db.Redis()
	if err != nil {
		return nil
	}

	size := BoardSize()
	run := func() error {
		pipe := client.Pipeline()
		type put struct {
			board string
			cmd   *redis.Cmd
		}
		var puts []put

		if old != nil && old.ranked {
			oldMode := modeNames[old.mode]
			for key := range old.values {
				if _, still := now.values[key]; still && now.ranked && old.mode == now.mode {
					continue
				}
				pipe.ZRem(ctx, boardKey(key, "all"), entity)
				pipe.ZRem(ctx, boardKey(key, oldMode), entity)
			}
		}

		if now.ranked {
			mode := modeNames[now.mode]
			sameBoards := old != nil && old.ranked && old.mode == now.mode
			for key, value := range now.values {
				var prev *statValue
				if sameBoards {
					if p, ok := old.values[key]; ok {
						if p.value == value.value {
							continue
						}
						prev = &p
					}
				}
				score := boardScore(key, value.value)
				for _, board := range []string{boardKey(key, "all"), boardKey(key, mode)} {
					if t, ok := thresholds.Load(board); ok && score < t.(float64) {
						// It can't get on, and wasn't on unless its old value was above the line.
						if prev == nil || boardScore(key, prev.value) < t.(float64) {
							continue
						}
					}
					puts = append(puts, put{board, putScript.EvalSha(ctx, pipe, []string{board, boardsKey}, score, entity, size)})
				}
			}
		}

		if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
		for _, p := range puts {
			result, err := p.cmd.Result()
			switch {
			case errors.Is(err, redis.Nil):
				thresholds.Delete(p.board)
			case err == nil:
				if text, ok := result.(string); ok {
					if lowest, err := strconv.ParseFloat(text, 64); err == nil {
						thresholds.Store(p.board, lowest)
					}
				}
			}
		}
		return nil
	}

	err = run()
	if err != nil && strings.Contains(err.Error(), "NOSCRIPT") {
		if err := putScript.Load(ctx, client).Err(); err != nil {
			return err
		}
		err = run()
	}
	return err
}

func runLeaderboards() {
	if client, err := db.Redis(); err == nil {
		_ = putScript.Load(context.Background(), client).Err()
	}
	lastRebuild := time.Time{}
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
		if client, err := db.Redis(); err == nil {
			ready, _ := client.Get(ctx, readyKey).Result()
			if ready != readyValue() || time.Since(lastRebuild) >= rebuildEvery {
				if err := rebuildBoards(ctx); err == nil {
					lastRebuild = time.Now()
				} else if !errors.Is(err, errNoRebuildLock) {
					fmt.Printf("[STORE] could not rebuild the leaderboards: %v\n", err)
				}
			}
		}
		cancel()
		time.Sleep(checkEvery)
	}
}

// topEntries is a min-heap that keeps the best entries of one board while scanning.
type topEntries []redis.Z

func (t topEntries) Len() int           { return len(t) }
func (t topEntries) Less(i, j int) bool { return t[i].Score < t[j].Score }
func (t topEntries) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t *topEntries) Push(x any)        { *t = append(*t, x.(redis.Z)) }
func (t *topEntries) Pop() any {
	old := *t
	last := old[len(old)-1]
	*t = old[:len(old)-1]
	return last
}

type boardSet struct {
	size   int
	boards map[string]*topEntries
}

func (b *boardSet) add(board string, score float64, entity string) {
	top, ok := b.boards[board]
	if !ok {
		top = &topEntries{}
		b.boards[board] = top
	}
	if top.Len() < b.size {
		heap.Push(top, redis.Z{Score: score, Member: entity})
		return
	}
	if score > (*top)[0].Score {
		(*top)[0] = redis.Z{Score: score, Member: entity}
		heap.Fix(top, 0)
	}
}

func (b *boardSet) addRecord(entity string, mode int16, values valueSet) {
	modeName := modeNames[mode]
	for key, value := range values {
		score := boardScore(key, value.value)
		b.add(boardKey(key, "all"), score, entity)
		b.add(boardKey(key, modeName), score, entity)
	}
}

// rebuildBoards recomputes every board from Postgres in one pass, writes each to a temporary key and renames it into
// place, so readers never see a half-built board.
func rebuildBoards(ctx context.Context) error {
	pool := postgres()
	client, err := db.Redis()
	if pool == nil || err != nil {
		return nil
	}
	if ok, err := client.SetNX(ctx, "lb:rebuilding", 1, time.Hour).Result(); err != nil || !ok {
		return errNoRebuildLock
	}
	defer client.Del(context.Background(), "lb:rebuilding")

	start := time.Now()
	set := &boardSet{size: int(BoardSize()), boards: map[string]*topEntries{}}
	records := 0
	if err := scanRecords(ctx, pool, `SELECT m.id, p.game_mode, v.seen_at, v.data FROM member_values v
			JOIN profile_members m ON m.id = v.member_id
			JOIN profiles p USING (profile_id)
			LEFT JOIN lb_hidden h ON h.uuid = m.player_uuid
			WHERE m.removed_at IS NULL AND p.deleted_at IS NULL AND h.uuid IS NULL`,
		func(id int64, mode int16, values valueSet) {
			set.addRecord(memberEntity(id), mode, values)
			records++
		}); err != nil {
		return err
	}
	if err := scanRecords(ctx, pool, `SELECT p.id, p.game_mode, v.seen_at, v.data FROM profile_values v
			JOIN profiles p USING (profile_id) WHERE p.deleted_at IS NULL`,
		func(id int64, mode int16, values valueSet) {
			set.addRecord(profileEntity(id), mode, values)
			records++
		}); err != nil {
		return err
	}

	written := make(map[string]bool, len(set.boards))
	pipe := client.Pipeline()
	for board, top := range set.boards {
		temp := board + ":rebuild"
		pipe.Del(ctx, temp)
		pipe.ZAdd(ctx, temp, (*top)...)
		pipe.Rename(ctx, temp, board)
		written[board] = true
		if pipe.Len() >= 300 {
			if _, err := pipe.Exec(ctx); err != nil {
				return err
			}
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	// Boards nobody is on anymore.
	existing, err := client.SMembers(ctx, boardsKey).Result()
	if err != nil {
		return err
	}
	for _, board := range existing {
		if !written[board] {
			pipe.Del(ctx, board)
		}
	}
	pipe.Del(ctx, boardsKey)
	members := make([]any, 0, len(written))
	for board := range written {
		members = append(members, board)
	}
	if len(members) > 0 {
		pipe.SAdd(ctx, boardsKey, members...)
	}
	pipe.Set(ctx, readyKey, readyValue(), 0)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	thresholds.Clear()
	fmt.Printf("[STORE] rebuilt %d leaderboards from %d records in %v\n", len(written), records, time.Since(start).Round(time.Millisecond))
	return nil
}

func scanRecords(ctx context.Context, pool *pgxpool.Pool, sql string, each func(int64, int16, valueSet)) error {
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var mode int16
		var seenAt time.Time
		var data []byte
		if err := rows.Scan(&id, &mode, &seenAt, &data); err != nil {
			return err
		}
		values, err := decodeValues(ctx, data, seenAt)
		if err != nil {
			continue
		}
		each(id, mode, values)
	}
	return rows.Err()
}

// warnAboutEviction says so at startup when Redis may evict leaderboards.
func warnAboutEviction(ctx context.Context) {
	client, err := db.Redis()
	if err != nil {
		return
	}
	config, err := client.ConfigGet(ctx, "maxmemory-policy").Result()
	if err != nil {
		return
	}
	if policy := config["maxmemory-policy"]; strings.HasPrefix(policy, "allkeys") {
		fmt.Printf("[STORE] Redis evicts with %s, which can drop leaderboards; use volatile-lfu (they get rebuilt, but go missing until then)\n", policy)
	}
}

// Player is a member of a profile entry.
type Player struct {
	UUID     string
	Username string
}

// Entry is one ranked player on a profile, or one profile.
type Entry struct {
	Rank      int64
	Profile   bool // a profile entry (coop collections, garden); Members lists who is on it
	UUID      string
	Username  string
	Members   []Player
	ProfileID string
	CuteName  string
	GameMode  string
	Raw       int64
	// UpdatedAt is when the value was last visible; SeenAt is when the profile was last fetched. UpdatedAt before
	// SeenAt means the player hid it from the API since, and the value is the last one known.
	UpdatedAt time.Time
	SeenAt    time.Time
}

type Page struct {
	Total   int64
	Ready   bool
	Entries []Entry
}

// LeaderboardPage returns entries offset to offset+limit-1 of a board, best first.
func LeaderboardPage(ctx context.Context, key, mode string, offset, limit int64) (Page, error) {
	if !isMode(mode) {
		return Page{}, ErrUnknownMode
	}
	client, err := db.Redis()
	if err != nil || postgres() == nil {
		return Page{}, ErrUnavailable
	}

	board := boardKey(key, mode)
	pipe := client.Pipeline()
	total := pipe.ZCard(ctx, board)
	ready := pipe.Get(ctx, readyKey)
	ranked := pipe.ZRevRangeWithScores(ctx, board, offset, offset+limit-1)
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return Page{}, err
	}

	page := Page{Total: total.Val(), Ready: ready.Val() == readyValue()}
	entities := make([]string, 0, len(ranked.Val()))
	scores := map[string]float64{}
	for _, z := range ranked.Val() {
		entity := fmt.Sprint(z.Member)
		entities = append(entities, entity)
		scores[entity] = z.Score
	}

	details, err := entryDetails(ctx, key, entities)
	if err != nil {
		return Page{}, err
	}
	for i, entity := range entities {
		entry, ok := details[entity]
		if !ok {
			// Purged since it was ranked; the next rebuild drops it.
			continue
		}
		entry.Rank = offset + int64(i) + 1
		entry.Raw = int64(math.Abs(scores[entity]))
		page.Entries = append(page.Entries, entry)
	}
	return page, nil
}

func splitEntities(entities []string) (members, profiles []int64) {
	for _, entity := range entities {
		if len(entity) < 2 {
			continue
		}
		id, err := strconv.ParseInt(entity[1:], 10, 64)
		if err != nil {
			continue
		}
		switch entity[0] {
		case 'm':
			members = append(members, id)
		case 'p':
			profiles = append(profiles, id)
		}
	}
	return members, profiles
}

// A coop member who was never looked up has no fruit name of their own; a partner's is close enough.
const cuteNameSQL = `coalesce(m.cute_name, (SELECT o.cute_name FROM profile_members o
	WHERE o.profile_id = m.profile_id AND o.cute_name IS NOT NULL LIMIT 1), '')`

// entryDetails loads who each entity is and when its value of key was last visible.
func entryDetails(ctx context.Context, key string, entities []string) (map[string]Entry, error) {
	out := make(map[string]Entry, len(entities))
	members, profiles := splitEntities(entities)
	pool := postgres()

	if len(members) > 0 {
		rows, err := pool.Query(ctx, `SELECT m.id, m.player_uuid, m.profile_id, `+cuteNameSQL+`, p.game_mode,
				coalesce(pl.username, ''), v.seen_at, v.data
			FROM profile_members m
			JOIN profiles p USING (profile_id)
			JOIN member_values v ON v.member_id = m.id
			LEFT JOIN players pl ON pl.uuid = m.player_uuid
			WHERE m.id = ANY($1)`, members)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			var player, profile [16]byte
			var mode int16
			var data []byte
			var entry Entry
			if err := rows.Scan(&id, &player, &profile, &entry.CuteName, &mode, &entry.Username, &entry.SeenAt, &data); err != nil {
				rows.Close()
				return nil, err
			}
			entry.UUID = formatUUID(player)
			entry.ProfileID = dashedUUID(profile)
			entry.GameMode = modeNames[mode]
			entry.UpdatedAt = valueUpdatedAt(ctx, data, entry.SeenAt, key)
			out[memberEntity(id)] = entry
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	if len(profiles) > 0 {
		rows, err := pool.Query(ctx, `SELECT p.id, p.profile_id, p.game_mode, v.seen_at, v.data,
				coalesce((SELECT o.cute_name FROM profile_members o WHERE o.profile_id = p.profile_id AND o.cute_name IS NOT NULL LIMIT 1), '')
			FROM profiles p JOIN profile_values v USING (profile_id) WHERE p.id = ANY($1)`, profiles)
		if err != nil {
			return nil, err
		}
		byProfile := map[[16]byte]string{}
		for rows.Next() {
			var id int64
			var profile [16]byte
			var mode int16
			var data []byte
			entry := Entry{Profile: true}
			if err := rows.Scan(&id, &profile, &mode, &entry.SeenAt, &data, &entry.CuteName); err != nil {
				rows.Close()
				return nil, err
			}
			entry.ProfileID = dashedUUID(profile)
			entry.GameMode = modeNames[mode]
			entry.UpdatedAt = valueUpdatedAt(ctx, data, entry.SeenAt, key)
			out[profileEntity(id)] = entry
			byProfile[profile] = profileEntity(id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}

		ids := make([][16]byte, 0, len(byProfile))
		for id := range byProfile {
			ids = append(ids, id)
		}
		rows, err = pool.Query(ctx, `SELECT m.profile_id, m.player_uuid, coalesce(pl.username, '')
			FROM profile_members m LEFT JOIN players pl ON pl.uuid = m.player_uuid
			WHERE m.profile_id = ANY($1) AND m.removed_at IS NULL ORDER BY m.id`, ids)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var profile, player [16]byte
			var username string
			if err := rows.Scan(&profile, &player, &username); err != nil {
				rows.Close()
				return nil, err
			}
			entity := byProfile[profile]
			entry := out[entity]
			entry.Members = append(entry.Members, Player{UUID: formatUUID(player), Username: username})
			out[entity] = entry
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func valueUpdatedAt(ctx context.Context, data []byte, seenAt time.Time, key string) time.Time {
	values, err := decodeValues(ctx, data, seenAt)
	if err != nil {
		return seenAt
	}
	if value, ok := values[key]; ok {
		return value.updatedAt
	}
	return seenAt
}

// dashedUUID formats a profile id the way Hypixel sends it.
func dashedUUID(id [16]byte) string {
	s := formatUUID(id)
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

// ProfileRanks is one profile of a player with every board it is on.
type ProfileRanks struct {
	ProfileID string
	CuteName  string
	GameMode  string
	SeenAt    time.Time
	Ranks     []StatRank
}

type StatRank struct {
	Key       string
	Profile   bool  // ranked as the whole profile (coop collections, garden)
	Rank      int64 // among all modes
	ModeRank  int64 // within the profile's game mode
	Raw       int64
	UpdatedAt time.Time
	// SeenAt is when the record this value is in was last written; UpdatedAt before it means the value is outdated.
	SeenAt time.Time
}

// PlayerRanks returns every board each active profile of a player is on, as the player and as the profile.
func PlayerRanks(ctx context.Context, uuidText string) ([]ProfileRanks, error) {
	uuid, err := parseUUID(uuidText)
	if err != nil {
		return nil, err
	}
	pool := postgres()
	client, err := db.Redis()
	if pool == nil || err != nil {
		return nil, ErrUnavailable
	}

	rows, err := pool.Query(ctx, `SELECT m.id, p.id, m.profile_id, `+cuteNameSQL+`, p.game_mode, v.seen_at, v.data,
			pv.seen_at, pv.data
		FROM profile_members m
		JOIN profiles p USING (profile_id)
		JOIN member_values v ON v.member_id = m.id
		LEFT JOIN profile_values pv ON pv.profile_id = m.profile_id
		WHERE m.player_uuid = $1 AND m.removed_at IS NULL AND p.deleted_at IS NULL AND v.ranked
		ORDER BY m.id`, uuid)
	if err != nil {
		return nil, err
	}
	type membership struct {
		profile        ProfileRanks
		memberID       int64
		profileID      int64
		mode           string
		member, shared valueSet
		sharedSeen     time.Time
	}
	var memberships []membership
	for rows.Next() {
		var m membership
		var profileUUID [16]byte
		var gameMode int16
		var data, profileData []byte
		var profileSeen *time.Time
		if err := rows.Scan(&m.memberID, &m.profileID, &profileUUID, &m.profile.CuteName, &gameMode, &m.profile.SeenAt,
			&data, &profileSeen, &profileData); err != nil {
			rows.Close()
			return nil, err
		}
		m.profile.ProfileID = dashedUUID(profileUUID)
		m.mode = modeNames[gameMode]
		m.profile.GameMode = m.mode
		if m.member, err = decodeValues(ctx, data, m.profile.SeenAt); err != nil {
			continue
		}
		if profileSeen != nil {
			m.shared, _ = decodeValues(ctx, profileData, *profileSeen)
			m.sharedSeen = *profileSeen
		}
		memberships = append(memberships, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	type queued struct {
		key       string
		profile   bool
		all, mode *redis.IntCmd
		value     statValue
		seenAt    time.Time
	}
	pipe := client.Pipeline()
	commands := make([][]queued, len(memberships))
	for i, m := range memberships {
		for _, side := range []struct {
			values  valueSet
			entity  string
			profile bool
			seenAt  time.Time
		}{{m.member, memberEntity(m.memberID), false, m.profile.SeenAt}, {m.shared, profileEntity(m.profileID), true, m.sharedSeen}} {
			for key, value := range side.values {
				commands[i] = append(commands[i], queued{
					key: key, profile: side.profile, value: value, seenAt: side.seenAt,
					all:  pipe.ZRevRank(ctx, boardKey(key, "all"), side.entity),
					mode: pipe.ZRevRank(ctx, boardKey(key, m.mode), side.entity),
				})
			}
		}
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	out := make([]ProfileRanks, 0, len(memberships))
	for i, m := range memberships {
		profile := m.profile
		for _, c := range commands[i] {
			if c.all.Err() != nil || c.mode.Err() != nil {
				continue
			}
			profile.Ranks = append(profile.Ranks, StatRank{
				Key: c.key, Profile: c.profile, Rank: c.all.Val() + 1, ModeRank: c.mode.Val() + 1,
				Raw: c.value.value, UpdatedAt: c.value.updatedAt, SeenAt: c.seenAt,
			})
		}
		out = append(out, profile)
	}
	return out, nil
}

// HiddenPlayer is a player kept off the leaderboards.
type HiddenPlayer struct {
	UUID     string
	Username string
	Reason   string
	HiddenAt time.Time
}

// HidePlayer takes a player off every leaderboard until UnhidePlayer. Their coops' profile entries stay.
func HidePlayer(ctx context.Context, uuidText, reason string) error {
	return setHidden(ctx, uuidText, func(uuid [16]byte) error {
		_, err := postgres().Exec(ctx, `INSERT INTO lb_hidden (uuid, reason) VALUES ($1, $2)
			ON CONFLICT (uuid) DO UPDATE SET reason = excluded.reason`, uuid, reason)
		return err
	})
}

func UnhidePlayer(ctx context.Context, uuidText string) error {
	return setHidden(ctx, uuidText, func(uuid [16]byte) error {
		_, err := postgres().Exec(ctx, `DELETE FROM lb_hidden WHERE uuid = $1`, uuid)
		return err
	})
}

func setHidden(ctx context.Context, uuidText string, change func([16]byte) error) error {
	uuid, err := parseUUID(uuidText)
	if err != nil {
		return err
	}
	if postgres() == nil {
		return ErrUnavailable
	}
	if err := change(uuid); err != nil {
		return err
	}
	return reconcileMembers(ctx, `m.player_uuid = $1`, uuid)
}

// reconcileMembers puts every member record matching where (over profile_members m) on exactly the boards it
// belongs on now: off when the member left, the profile was deleted or the player is hidden, back on otherwise.
func reconcileMembers(ctx context.Context, where string, args ...any) error {
	rows, err := postgres().Query(ctx, `SELECT v.member_id, v.seen_at, v.game_mode, v.ranked, v.data,
			p.game_mode, m.removed_at IS NULL AND p.deleted_at IS NULL AND h.uuid IS NULL
		FROM member_values v
		JOIN profile_members m ON m.id = v.member_id
		JOIN profiles p USING (profile_id)
		LEFT JOIN lb_hidden h ON h.uuid = m.player_uuid
		WHERE `+where, args...)
	if err != nil {
		return err
	}
	type change struct {
		id       int64
		old, now boardState
	}
	var changes []change
	for rows.Next() {
		var c change
		var seenAt time.Time
		var data []byte
		if err := rows.Scan(&c.id, &seenAt, &c.old.mode, &c.old.ranked, &data, &c.now.mode, &c.now.ranked); err != nil {
			rows.Close()
			return err
		}
		if c.old.ranked == c.now.ranked && c.old.mode == c.now.mode {
			continue
		}
		values, err := decodeValues(ctx, data, seenAt)
		if err != nil {
			continue
		}
		c.old.values, c.now.values = values, values
		changes = append(changes, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, c := range changes {
		var prev *boardState
		if c.old.ranked {
			prev = &c.old
		}
		if err := syncBoards(ctx, memberEntity(c.id), prev, c.now); err != nil {
			return err
		}
		if _, err := postgres().Exec(ctx, `UPDATE member_values SET ranked = $2, game_mode = $3 WHERE member_id = $1`,
			c.id, c.now.ranked, c.now.mode); err != nil {
			return err
		}
	}
	return nil
}

// reconcileProfiles does the same for profile records: off the boards once the profile is deleted.
func reconcileProfiles(ctx context.Context, profileIDs [][16]byte) error {
	rows, err := postgres().Query(ctx, `SELECT p.id, v.profile_id, v.seen_at, v.game_mode, v.ranked, v.data, p.game_mode, p.deleted_at IS NULL
		FROM profile_values v JOIN profiles p USING (profile_id) WHERE v.profile_id = ANY($1)`, profileIDs)
	if err != nil {
		return err
	}
	type change struct {
		id       int64
		profile  [16]byte
		old, now boardState
	}
	var changes []change
	for rows.Next() {
		var c change
		var seenAt time.Time
		var data []byte
		if err := rows.Scan(&c.id, &c.profile, &seenAt, &c.old.mode, &c.old.ranked, &data, &c.now.mode, &c.now.ranked); err != nil {
			rows.Close()
			return err
		}
		if c.old.ranked == c.now.ranked && c.old.mode == c.now.mode {
			continue
		}
		values, err := decodeValues(ctx, data, seenAt)
		if err != nil {
			continue
		}
		c.old.values, c.now.values = values, values
		changes = append(changes, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, c := range changes {
		var prev *boardState
		if c.old.ranked {
			prev = &c.old
		}
		if err := syncBoards(ctx, profileEntity(c.id), prev, c.now); err != nil {
			return err
		}
		if _, err := postgres().Exec(ctx, `UPDATE profile_values SET ranked = $2, game_mode = $3 WHERE profile_id = $1`,
			c.profile, c.now.ranked, c.now.mode); err != nil {
			return err
		}
	}
	return nil
}

func HiddenPlayers(ctx context.Context) ([]HiddenPlayer, error) {
	pool := postgres()
	if pool == nil {
		return nil, ErrUnavailable
	}
	rows, err := pool.Query(ctx, `SELECT h.uuid, coalesce(p.username, ''), h.reason, h.hidden_at
		FROM lb_hidden h LEFT JOIN players p USING (uuid) ORDER BY h.hidden_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HiddenPlayer
	for rows.Next() {
		var uuid [16]byte
		var hidden HiddenPlayer
		if err := rows.Scan(&uuid, &hidden.Username, &hidden.Reason, &hidden.HiddenAt); err != nil {
			return nil, err
		}
		hidden.UUID = formatUUID(uuid)
		out = append(out, hidden)
	}
	return out, rows.Err()
}
