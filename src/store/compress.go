package store

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/sync/singleflight"
)

const (
	kindProfile int16 = 1
	kindMuseum  int16 = 2
	kindGarden  int16 = 3
)

// Kinds that get a trained dictionary. Gardens and trimmed players are too small to need one.
var dictKinds = []int16{kindProfile, kindMuseum}

// encoderSet compresses one kind. best is used normally; better (about 7% bigger, 4x faster) takes over while the
// queue is backed up.
type encoderSet struct {
	dictID int32
	better *zstd.Encoder
	best   *zstd.Encoder
}

func newEncoderSet(dictID int32, dict []byte) (*encoderSet, error) {
	set := &encoderSet{dictID: dictID}
	for _, target := range []struct {
		encoder **zstd.Encoder
		level   zstd.EncoderLevel
	}{{&set.better, zstd.SpeedBetterCompression}, {&set.best, zstd.SpeedBestCompression}} {
		options := []zstd.EOption{zstd.WithEncoderLevel(target.level), zstd.WithEncoderConcurrency(1)}
		if dict != nil {
			options = append(options, zstd.WithEncoderDict(dict))
		}
		encoder, err := zstd.NewWriter(nil, options...)
		if err != nil {
			return nil, err
		}
		*target.encoder = encoder
	}
	return set, nil
}

var (
	plainEncoders, _ = newEncoderSet(0, nil)
	plainDecoder, _  = zstd.NewReader(nil)

	// Newest dictionary per kind, used for writing. Only the parent writes.
	kindEncoders sync.Map // int16 -> *encoderSet

	// Decoders by dictionary id, copied on write. Children load a dictionary the first time they read a blob that
	// uses it; dictionaries never change once stored.
	dictDecoders atomic.Pointer[map[int32]*zstd.Decoder]
	dictLoads    singleflight.Group
)

// compress returns the codec (dictionary id, 0 for none) and the compressed body.
func compress(kind int16, body []byte, busy bool) (int32, []byte) {
	set := plainEncoders
	if value, ok := kindEncoders.Load(kind); ok {
		set = value.(*encoderSet)
	}
	if busy {
		return set.dictID, set.better.EncodeAll(body, nil)
	}
	return set.dictID, set.best.EncodeAll(body, nil)
}

func decompress(ctx context.Context, codec int32, data []byte) ([]byte, error) {
	if codec == 0 {
		return plainDecoder.DecodeAll(data, nil)
	}
	decoder, err := decoderFor(ctx, codec)
	if err != nil {
		return nil, err
	}
	return decoder.DecodeAll(data, nil)
}

func decoderFor(ctx context.Context, id int32) (*zstd.Decoder, error) {
	if decoders := dictDecoders.Load(); decoders != nil {
		if decoder, ok := (*decoders)[id]; ok {
			return decoder, nil
		}
	}

	value, err, _ := dictLoads.Do(strconv.Itoa(int(id)), func() (any, error) {
		if decoders := dictDecoders.Load(); decoders != nil {
			if decoder, ok := (*decoders)[id]; ok {
				return decoder, nil
			}
		}
		pool := postgres()
		if pool == nil {
			return nil, errors.New("store: Postgres is unavailable")
		}
		var dict []byte
		if err := pool.QueryRow(ctx, `SELECT data FROM codec_dicts WHERE id = $1`, id).Scan(&dict); err != nil {
			return nil, fmt.Errorf("store: load dictionary %d: %w", id, err)
		}
		return addDecoder(id, dict)
	})
	if err != nil {
		return nil, err
	}
	return value.(*zstd.Decoder), nil
}

var decodersMutex sync.Mutex

func addDecoder(id int32, dict []byte) (*zstd.Decoder, error) {
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderDicts(dict))
	if err != nil {
		return nil, err
	}

	decodersMutex.Lock()
	defer decodersMutex.Unlock()
	next := map[int32]*zstd.Decoder{}
	if current := dictDecoders.Load(); current != nil {
		for key, value := range *current {
			next[key] = value
		}
	}
	next[id] = decoder
	dictDecoders.Store(&next)
	return decoder, nil
}

// useDict makes dict the one new blobs of kind are written with.
func useDict(kind int16, id int32, dict []byte) error {
	if got, ok := dictID(dict); !ok || got != uint32(id) {
		return fmt.Errorf("store: dictionary %d has id %d in its header", id, got)
	}
	set, err := newEncoderSet(id, dict)
	if err != nil {
		return err
	}
	if _, err := addDecoder(id, dict); err != nil {
		return err
	}
	kindEncoders.Store(kind, set)
	return nil
}

// dictID reads the id from a zstd dictionary header: magic 0xEC30A437, then the id, both little-endian.
func dictID(dict []byte) (uint32, bool) {
	if len(dict) < 8 || binary.LittleEndian.Uint32(dict) != 0xEC30A437 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(dict[4:]), true
}

// loadDicts sets up the newest dictionary of every kind for writing.
func loadDicts(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `SELECT DISTINCT ON (kind) kind, id, data FROM codec_dicts ORDER BY kind, id DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var kind int16
		var id int32
		var dict []byte
		if err := rows.Scan(&kind, &id, &dict); err != nil {
			return err
		}
		if err := useDict(kind, id, dict); err != nil {
			return err
		}
	}
	return rows.Err()
}
