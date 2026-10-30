package store

import (
	"context"
	"encoding/binary"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
)

// statValue is one stored value and when it was last visible in the API.
type statValue struct {
	value     int64
	updatedAt time.Time
}

type valueSet map[string]statValue

// mergeValues builds the new record: fresh values as of seenAt, plus the old values of every category the player has
// hidden since, which keep their old time so they show as outdated. replace limits which keys this update owns
// (profile records get coop collections and the garden from different responses); nil means all of them.
//
// The newest value wins, whatever order responses are stored in: a value written by a response fetched after
// seenAt is kept rather than replaced by an older one.
func mergeValues(old valueSet, fresh map[string]int64, hidden map[string]bool, seenAt time.Time, replace func(string) bool) valueSet {
	out := make(valueSet, len(fresh))
	for key, value := range fresh {
		if prev, ok := old[key]; ok && prev.updatedAt.After(seenAt) {
			out[key] = prev
			continue
		}
		out[key] = statValue{value: value, updatedAt: seenAt}
	}
	for key, value := range old {
		if _, ok := out[key]; ok {
			continue
		}
		if (replace != nil && !replace(key)) || hidden[hiddenCategory(key)] || value.updatedAt.After(seenAt) {
			out[key] = value
		}
	}
	return out
}

func latest(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// Value records start with valueRecordMarker, which the older format can't (its first byte was a stat id, never 0),
// then a byte saying whether the rest is zstd. Inside: uvarint count, per value the uvarint difference from the
// previous stat id (sorted) and the zigzag varint value, then uvarint count of values older than seenAt, per value
// the uvarint difference from the previous such index and uvarint seconds. Nearly every value is current, and ids
// are close together, so this is about 45% smaller than id, value, age per value.
const (
	valueRecordMarker byte = 0
	valuesPlain       byte = 0
	valuesCompressed  byte = 1
)

// encodeValues packs a record for storage. Parent only.
func encodeValues(ctx context.Context, values valueSet, seenAt time.Time) ([]byte, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	ids, err := statKeyIDs(ctx, keys)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(keys, func(a, b string) int { return int(ids[a] - ids[b]) })

	body := make([]byte, 0, len(values)*4)
	body = binary.AppendUvarint(body, uint64(len(keys)))
	var previous int32
	var ages []uint64
	var ageIndexes []int
	for i, key := range keys {
		body = binary.AppendUvarint(body, uint64(ids[key]-previous))
		body = binary.AppendVarint(body, values[key].value)
		previous = ids[key]
		if age := uint64(max(0, seenAt.Sub(values[key].updatedAt)/time.Second)); age > 0 {
			ages = append(ages, age)
			ageIndexes = append(ageIndexes, i)
		}
	}
	body = binary.AppendUvarint(body, uint64(len(ages)))
	previousIndex := 0
	for i, age := range ages {
		body = binary.AppendUvarint(body, uint64(ageIndexes[i]-previousIndex))
		body = binary.AppendUvarint(body, age)
		previousIndex = ageIndexes[i]
	}

	out := []byte{valueRecordMarker, valuesPlain}
	if _, compressed := compress(0, body); len(compressed) < len(body) {
		out[1] = valuesCompressed
		return append(out, compressed...), nil
	}
	return append(out, body...), nil
}

func decodeValues(ctx context.Context, data []byte, seenAt time.Time) (valueSet, error) {
	if len(data) >= 2 && data[0] == valueRecordMarker {
		return decodeValueRecord(ctx, data[1], data[2:], seenAt)
	}
	out := valueSet{}
	for len(data) > 0 {
		id, n := binary.Uvarint(data)
		if n <= 0 {
			return nil, errors.New("store: bad value record")
		}
		data = data[n:]
		value, n := binary.Varint(data)
		if n <= 0 {
			return nil, errors.New("store: bad value record")
		}
		data = data[n:]
		age, n := binary.Uvarint(data)
		if n <= 0 {
			return nil, errors.New("store: bad value record")
		}
		data = data[n:]

		key, err := statKeyName(ctx, int32(id))
		if err != nil {
			return nil, err
		}
		out[key] = statValue{value: value, updatedAt: seenAt.Add(-time.Duration(age) * time.Second)}
	}
	return out, nil
}

// Stat keys are numbered once in stat_keys and never renumbered, so every process can cache them.
var statKeyCache = struct {
	sync.RWMutex
	byKey map[string]int32
	byID  map[int32]string
}{byKey: map[string]int32{}, byID: map[int32]string{}}

func statKeyIDs(ctx context.Context, keys []string) (map[string]int32, error) {
	out := make(map[string]int32, len(keys))
	var missing []string
	statKeyCache.RLock()
	for _, key := range keys {
		if id, ok := statKeyCache.byKey[key]; ok {
			out[key] = id
		} else {
			missing = append(missing, key)
		}
	}
	statKeyCache.RUnlock()
	if len(missing) == 0 {
		return out, nil
	}

	pool := postgres()
	if pool == nil {
		return nil, errors.New("store: Postgres is unavailable")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO stat_keys (key) SELECT unnest($1::text[]) ON CONFLICT (key) DO NOTHING`, missing); err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT id, key FROM stat_keys WHERE key = ANY($1)`, missing)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statKeyCache.Lock()
	defer statKeyCache.Unlock()
	for rows.Next() {
		var id int32
		var key string
		if err := rows.Scan(&id, &key); err != nil {
			return nil, err
		}
		statKeyCache.byKey[key] = id
		statKeyCache.byID[id] = key
		out[key] = id
	}
	return out, rows.Err()
}

func statKeyName(ctx context.Context, id int32) (string, error) {
	statKeyCache.RLock()
	key, ok := statKeyCache.byID[id]
	statKeyCache.RUnlock()
	if ok {
		return key, nil
	}
	if err := loadStatKeys(ctx); err != nil {
		return "", err
	}
	statKeyCache.RLock()
	defer statKeyCache.RUnlock()
	if key, ok := statKeyCache.byID[id]; ok {
		return key, nil
	}
	return "", errors.New("store: unknown stat key id")
}

// loadStatKeys reads every key into the cache.
func loadStatKeys(ctx context.Context) error {
	pool := postgres()
	if pool == nil {
		return errors.New("store: Postgres is unavailable")
	}
	rows, err := pool.Query(ctx, `SELECT id, key FROM stat_keys`)
	if err != nil {
		return err
	}
	defer rows.Close()

	statKeyCache.Lock()
	defer statKeyCache.Unlock()
	for rows.Next() {
		var id int32
		var key string
		if err := rows.Scan(&id, &key); err != nil {
			return err
		}
		statKeyCache.byKey[key] = id
		statKeyCache.byID[id] = key
	}
	return rows.Err()
}

// StatKeys lists every leaderboard that has ever had a value.
func StatKeys(ctx context.Context) ([]string, error) {
	if err := loadStatKeys(ctx); err != nil {
		return nil, err
	}
	statKeyCache.RLock()
	defer statKeyCache.RUnlock()
	keys := make([]string, 0, len(statKeyCache.byKey))
	for key := range statKeyCache.byKey {
		keys = append(keys, key)
	}
	return keys, nil
}

// isCoopKey and isGardenKey split a profile record between the two responses that fill it.
func isCoopKey(key string) bool   { return strings.HasPrefix(key, "coop_collection:") }
func isGardenKey(key string) bool { return strings.HasPrefix(key, "garden:") }

func decodeValueRecord(ctx context.Context, format byte, body []byte, seenAt time.Time) (valueSet, error) {
	if format == valuesCompressed {
		var err error
		if body, err = decompress(ctx, 0, body); err != nil {
			return nil, err
		}
	}
	bad := errors.New("store: bad value record")
	next := func() (uint64, bool) {
		value, n := binary.Uvarint(body)
		if n <= 0 {
			return 0, false
		}
		body = body[n:]
		return value, true
	}

	count, ok := next()
	if !ok || count > uint64(len(body)) {
		return nil, bad
	}
	keys := make([]string, count)
	out := make(valueSet, count)
	var id uint64
	for i := range keys {
		delta, ok := next()
		if !ok {
			return nil, bad
		}
		id += delta
		value, n := binary.Varint(body)
		if n <= 0 {
			return nil, bad
		}
		body = body[n:]
		key, err := statKeyName(ctx, int32(id))
		if err != nil {
			return nil, err
		}
		keys[i] = key
		out[key] = statValue{value: value, updatedAt: seenAt}
	}

	aged, ok := next()
	if !ok {
		return nil, bad
	}
	index := uint64(0)
	for range aged {
		delta, ok := next()
		if !ok {
			return nil, bad
		}
		age, ok := next()
		index += delta
		if !ok || index >= count {
			return nil, bad
		}
		value := out[keys[index]]
		value.updatedAt = seenAt.Add(-time.Duration(age) * time.Second)
		out[keys[index]] = value
	}
	return out, nil
}
