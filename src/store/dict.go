package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/klauspost/compress/dict"
)

// Dictionaries are trained from stored blobs, so they match what SkyCrypt actually stores. Measured on 150 real
// players, a 2 MB dictionary made profile bundles 45% smaller than plain zstd at the same level; 4 MB was worse.
const (
	dictSize = 2 << 20
	// A trimmed player is a few hundred bytes. 32 KB made them 67% smaller; a bigger one has nothing more to learn.
	playerDictSize = 32 << 10
	// So many are needed to fill even that dictionary.
	firstPlayerDictMinRows = 2000
	trainPlayerRows        = 20000

	// The first dictionary needs enough variety to be useful; later ones replace it as Hypixel adds items.
	firstDictMinBlobs = 300
	retrainMinBlobs   = 3000
	retrainAfter      = 30 * 24 * time.Hour

	trainSampleRows  = 400
	trainSampleBytes = 128 << 20
	trainChunkBytes  = 32 << 10
)

func runDictTraining() {
	for {
		for _, kind := range dictKinds {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			if err := maybeTrainDict(ctx, kind); err != nil {
				fmt.Printf("[STORE] could not train a dictionary for kind %d: %v\n", kind, err)
			}
			cancel()
		}
		time.Sleep(time.Hour)
	}
}

func maybeTrainDict(ctx context.Context, kind int16) error {
	pool := postgres()
	if pool == nil {
		return nil
	}

	var latestID int32
	var latestAt time.Time
	err := pool.QueryRow(ctx, `SELECT id, created_at FROM codec_dicts WHERE kind = $1 ORDER BY id DESC LIMIT 1`, kind).
		Scan(&latestID, &latestAt)
	hasDict := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if hasDict && time.Since(latestAt) < retrainAfter {
		return nil
	}

	var blobs int
	firstMin, size := firstDictMinBlobs, dictSize
	countRow := pool.QueryRow(ctx, `SELECT count(*) FROM blobs WHERE kind = $1 AND bytes > 0`, kind)
	if kind == kindPlayer {
		firstMin, size = firstPlayerDictMinRows, playerDictSize
		countRow = pool.QueryRow(ctx, `SELECT count(*) FROM players WHERE player_data IS NOT NULL`)
	}
	if err := countRow.Scan(&blobs); err != nil {
		return err
	}
	if (!hasDict && blobs < firstMin) || (hasDict && blobs < retrainMinBlobs) {
		return nil
	}

	samples, err := dictSamples(ctx, kind)
	if err != nil {
		return err
	}

	var id int32
	if err := pool.QueryRow(ctx, `SELECT coalesce(max(id), 0) + 1 FROM codec_dicts`).Scan(&id); err != nil {
		return err
	}
	start := time.Now()
	trained, err := trainDict(samples, id, size)
	if err != nil {
		return err
	}

	if _, err := pool.Exec(ctx, `INSERT INTO codec_dicts (id, kind, data) VALUES ($1, $2, $3)`, id, kind, trained); err != nil {
		return err
	}
	if err := useDict(kind, id, trained); err != nil {
		return err
	}
	fmt.Printf("[STORE] trained dictionary %d for kind %d from %d samples in %v\n", id, kind, len(samples), time.Since(start).Round(time.Second))
	return nil
}

// dictSamples returns random stored blobs of kind, unpacked to what gets compressed and cut into chunks. Both
// trainers only look at the start of each sample, so chunks let them see whole bundles.
func dictSamples(ctx context.Context, kind int16) ([][]byte, error) {
	query, args := `SELECT codec, data FROM blobs WHERE kind = $1 AND bytes > 0 ORDER BY random() LIMIT $2`, []any{kind, trainSampleRows}
	if kind == kindPlayer {
		query, args = `SELECT player_codec, player_data FROM players WHERE player_data IS NOT NULL ORDER BY random() LIMIT $1`, []any{trainPlayerRows}
	}
	rows, err := postgres().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples [][]byte
	total := 0
	for rows.Next() && total < trainSampleBytes {
		var codec int32
		var data []byte
		if err := rows.Scan(&codec, &data); err != nil {
			return nil, err
		}
		packed, err := decompress(ctx, codec, data)
		if err != nil {
			continue
		}
		for start := 0; start < len(packed); start += trainChunkBytes {
			samples = append(samples, packed[start:min(start+trainChunkBytes, len(packed))])
		}
		total += len(packed)
	}
	return samples, rows.Err()
}

// trainDict uses the zstd command when it is installed, which made dictionaries about 10% better than the Go
// builder in the same measurement, and the Go builder otherwise.
func trainDict(samples [][]byte, id int32, size int) ([]byte, error) {
	if _, err := exec.LookPath("zstd"); err == nil {
		if trained, err := trainWithZstdCommand(samples, id, size); err == nil {
			return trained, nil
		} else {
			fmt.Printf("[STORE] zstd --train failed, using the Go builder: %v\n", err)
		}
	}
	return dict.BuildZstdDict(samples, dict.Options{MaxDictSize: size, HashBytes: 6, ZstdDictID: uint32(id)})
}

func trainWithZstdCommand(samples [][]byte, id int32, size int) ([]byte, error) {
	dir, err := os.MkdirTemp("", "skycrypt-dict")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	sampleDir := filepath.Join(dir, "samples")
	if err := os.Mkdir(sampleDir, 0o700); err != nil {
		return nil, err
	}
	for i, sample := range samples {
		if err := os.WriteFile(filepath.Join(sampleDir, strconv.Itoa(i)), sample, 0o600); err != nil {
			return nil, err
		}
	}

	out := filepath.Join(dir, "dict")
	cmd := exec.Command("zstd", "--train", "-q", "-r", sampleDir, "-o", out,
		"--maxdict="+strconv.Itoa(size), "--dictID="+strconv.Itoa(int(id)))
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, output)
	}
	return os.ReadFile(out)
}
