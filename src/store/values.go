package store

import (
	"context"
	"encoding/binary"
	"errors"
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
func mergeValues(old valueSet, fresh map[string]int64, hidden map[string]bool, seenAt time.Time, replace func(string) bool) valueSet {
	out := make(valueSet, len(fresh))
	for key, value := range fresh {
		out[key] = statValue{value: value, updatedAt: seenAt}
	}
	for key, value := range old {
		if _, ok := out[key]; ok {
			continue
		}
		if (replace != nil && !replace(key)) || hidden[hiddenCategory(key)] {
			out[key] = value
		}
	}
	return out
}

// encodeValues packs a record: per value, uvarint stat id, zigzag varint value, uvarint seconds it is older than
// seenAt.
func encodeValues(ctx context.Context, values valueSet, seenAt time.Time) ([]byte, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	ids, err := statKeyIDs(ctx, keys)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(values)*6)
	for key, v := range values {
		out = binary.AppendUvarint(out, uint64(ids[key]))
		out = binary.AppendVarint(out, v.value)
		out = binary.AppendUvarint(out, uint64(max(0, seenAt.Sub(v.updatedAt)/time.Second)))
	}
	return out, nil
}

func decodeValues(ctx context.Context, data []byte, seenAt time.Time) (valueSet, error) {
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
