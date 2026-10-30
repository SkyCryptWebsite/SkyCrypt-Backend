package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"skycrypt/src/db"

	"github.com/jackc/pgx/v5"
)

// The store is an archive, so reads can afford more than ingest can. Ingest compresses with the Go encoder to keep
// up with fetches; this worker then recompresses each blob with the zstd command at level 19, which is 12-20%
// smaller. It works through blobs one by one, so existing rows get the same treatment, and a row whose content
// changes goes back to level 0 and comes round again. The result is a normal zstd frame the Go decoder reads.
//
// Measured: about 3 MB/s of unpacked data per worker and 100 MB of memory per zstd process.
const (
	archiveWorkers  = 2
	archiveBatch    = 64
	archiveLevel    = 1
	archiveIdle     = 10 * time.Minute
	archiveMinAge   = time.Hour
	archiveZstdFlag = "-19"
	// Longer than the time archiveBlob gets, so a claim outlives the work and a crashed parent's claims still expire.
	archiveClaimTTL = 6 * time.Minute
)

type archiveRow struct {
	profileID [16]byte
	kind      int16
}

func runArchive() {
	if _, err := exec.LookPath("zstd"); err != nil {
		fmt.Println("[STORE] the zstd command is not installed, blobs stay at ingest compression")
		return
	}
	dir, err := os.MkdirTemp("", "skycrypt-archive")
	if err != nil {
		fmt.Printf("[STORE] archive compression is off: %v\n", err)
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	var last archiveRow
	for {
		rows, err := nextArchiveRows(context.Background(), last)
		if err != nil {
			fmt.Printf("[STORE] archive compression failed: %v\n", err)
			time.Sleep(archiveIdle)
			continue
		}
		if len(rows) == 0 {
			last = archiveRow{}
			time.Sleep(archiveIdle)
			continue
		}
		last = rows[len(rows)-1]

		dicts := map[int16]archiveDict{}
		for _, kind := range []int16{kindProfile, kindMuseum} {
			dict, err := writeArchiveDict(context.Background(), dir, kind)
			if err != nil {
				fmt.Printf("[STORE] could not load a dictionary: %v\n", err)
			}
			dicts[kind] = dict
		}

		work := make(chan archiveRow)
		var wg sync.WaitGroup
		for range archiveWorkers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for row := range work {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					if err := archiveBlob(ctx, row, dicts); err != nil {
						fmt.Printf("[STORE] could not recompress a blob: %v\n", err)
					}
					cancel()
				}
			}()
		}
		for _, row := range rows {
			work <- row
		}
		close(work)
		wg.Wait()
	}
}

func nextArchiveRows(ctx context.Context, after archiveRow) ([]archiveRow, error) {
	pool := postgres()
	if pool == nil {
		return nil, errors.New("store: Postgres is unavailable")
	}
	rows, err := pool.Query(ctx, `SELECT profile_id, kind FROM blobs
		WHERE (profile_id, kind) > ($1, $2) AND level < $3 AND bytes > 0 AND fetched_at < now() - make_interval(secs => $4)
		ORDER BY profile_id, kind LIMIT $5`, after.profileID, after.kind, archiveLevel, archiveMinAge.Seconds(), archiveBatch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []archiveRow
	for rows.Next() {
		var row archiveRow
		if err := rows.Scan(&row.profileID, &row.kind); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type archiveDict struct {
	id   int32
	path string
}

// writeArchiveDict puts the newest dictionary of kind where the zstd command can read it. It returns a zero
// archiveDict when kind has none yet.
func writeArchiveDict(ctx context.Context, dir string, kind int16) (archiveDict, error) {
	pool := postgres()
	if pool == nil {
		return archiveDict{}, errors.New("store: Postgres is unavailable")
	}
	var dict archiveDict
	var data []byte
	err := pool.QueryRow(ctx, `SELECT id, data FROM codec_dicts WHERE kind = $1 ORDER BY id DESC LIMIT 1`, kind).Scan(&dict.id, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		return archiveDict{}, nil
	}
	if err != nil {
		return archiveDict{}, err
	}
	dict.path = filepath.Join(dir, strconv.Itoa(int(dict.id)))
	if err := os.WriteFile(dict.path, data, 0o600); err != nil {
		return archiveDict{}, err
	}
	return dict, nil
}

// claimArchiveRow stops parents sharing a database from compressing the same row at once. It reports false when
// another worker has the row. Without Redis the row is compressed anyway, since doing it twice only wastes work.
func claimArchiveRow(ctx context.Context, row archiveRow) (release func(), ok bool) {
	client, err := db.Redis()
	if err != nil {
		return func() {}, true
	}
	key := fmt.Sprintf("store:archive:%x:%d", row.profileID, row.kind)
	claimed, err := client.SetNX(ctx, key, 1, archiveClaimTTL).Result()
	if err != nil {
		return func() {}, true
	}
	if !claimed {
		return nil, false
	}
	return func() { client.Del(context.WithoutCancel(ctx), key) }, true
}

func archiveBlob(ctx context.Context, row archiveRow, dicts map[int16]archiveDict) error {
	dict := dicts[row.kind]
	// A kind that gets a dictionary waits for its first one, or its rows would finish without it.
	if isDictKind(row.kind) && dict.path == "" {
		return nil
	}

	release, ok := claimArchiveRow(ctx, row)
	if !ok {
		return nil
	}
	defer release()

	var codec int32
	var hash int64
	var data []byte
	pool := postgres()
	if pool == nil {
		return errors.New("store: Postgres is unavailable")
	}
	// The level check skips a row another worker finished after this batch was listed.
	err := pool.QueryRow(ctx, `SELECT codec, hash, data FROM blobs WHERE profile_id = $1 AND kind = $2 AND level < $3`,
		row.profileID, row.kind, archiveLevel).Scan(&codec, &hash, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	packed, err := decompress(ctx, codec, data)
	if err != nil {
		return err
	}
	packed = internTextures(ctx, packed)

	compressed, err := zstdCommand(ctx, packed, dict.path)
	if err != nil {
		return err
	}
	check, err := decompress(ctx, dict.id, compressed)
	if err != nil {
		return err
	}
	if !bytes.Equal(check, packed) {
		return errors.New("store: recompressed blob does not match")
	}

	// The hash check leaves a blob alone if ingest replaced it meanwhile.
	_, err = pool.Exec(ctx, `UPDATE blobs SET codec = $3, data = $4, bytes = $5, level = $6
		WHERE profile_id = $1 AND kind = $2 AND hash = $7`,
		row.profileID, row.kind, dict.id, compressed, len(compressed), archiveLevel, hash)
	return err
}

func isDictKind(kind int16) bool {
	for _, k := range dictKinds {
		if k == kind {
			return true
		}
	}
	return false
}

func zstdCommand(ctx context.Context, data []byte, dictPath string) ([]byte, error) {
	args := []string{archiveZstdFlag, "--no-check", "-q", "-c", "--stream-size=" + strconv.Itoa(len(data))}
	if dictPath != "" {
		args = append(args, "-D", dictPath)
	}
	cmd := exec.CommandContext(ctx, "zstd", args...)
	cmd.Stdin = bytes.NewReader(data)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}
