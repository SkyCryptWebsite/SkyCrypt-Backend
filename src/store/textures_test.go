package store

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/gzip"
)

func nbtString(name, value string) []byte {
	out := []byte{8}
	out = binary.BigEndian.AppendUint16(out, uint16(len(name)))
	out = append(out, name...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(value)))
	return append(out, value...)
}

// skullNBT is a document with textures next to the other tag types, a list of compounds and a list of strings.
func skullNBT(texture, signature string) []byte {
	out := []byte{nbtCompound, 0, 0}
	out = append(out, 9, 0, 1, 'i', nbtCompound, 0, 0, 0, 2)
	for range 2 {
		out = append(out, 1, 0, 5, 'C', 'o', 'u', 'n', 't', 64)
		out = append(out, 11, 0, 1, 'x', 0, 0, 0, 2, 0, 0, 0, 1, 0, 0, 0, 2)
		out = append(out, nbtString("Value", texture)...)
		out = append(out, nbtString("Signature", signature)...)
		out = append(out, nbtString("Name", "§6Golden Head")...)
		out = append(out, 9, 0, 5, 'V', 'a', 'l', 'u', 'e', 8, 0, 0, 0, 1, 0, 1, 'a')
		out = append(out, 0)
	}
	return append(out, 0)
}

func TestInternPayloadRoundTrip(t *testing.T) {
	texture, signature := strings.Repeat("t", 300), strings.Repeat("s", 684)
	payload := skullNBT(texture, signature)

	found := map[string]struct{}{}
	if err := payloadTextures(payload, found); err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("found %d strings, want the 2 long ones", len(found))
	}
	ids := map[string]int32{texture: 1, signature: 200}
	interned, err := internPayload(payload, ids)
	if err != nil {
		t.Fatal(err)
	}
	if interned[0] != internedPayload || len(interned) >= len(payload)/4 {
		t.Fatalf("interned to %d of %d bytes", len(interned), len(payload))
	}

	wanted := map[int32]struct{}{}
	if err := internedIDs(interned, wanted); err != nil {
		t.Fatal(err)
	}
	if len(wanted) != 2 {
		t.Fatalf("restore asks for %v", wanted)
	}
	restored, err := restorePayload(interned, map[int32][]byte{1: []byte(texture), 200: []byte(signature)})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, payload) {
		t.Fatal("payload did not come back identical")
	}
}

func TestDropTagsRemovesOnlyThoseTags(t *testing.T) {
	payload := []byte{nbtCompound, 0, 0}
	payload = append(payload, nbtString("id", "HYPERION")...)
	payload = append(payload, nbtString("uuid", "d3bdf39e-78ab-40d3-ba9d-f6f969e5c59a")...)
	payload = append(payload, 1, 0, 1, 'c', 1, 0)
	want := []byte{nbtCompound, 0, 0}
	want = append(want, nbtString("id", "HYPERION")...)
	want = append(want, 1, 0, 1, 'c', 1, 0)
	got, err := dropTags(payload, droppedNBTTags)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x", got)
	}
}

func TestRewriteRejectsBadNBT(t *testing.T) {
	good := skullNBT(strings.Repeat("t", 100), strings.Repeat("s", 100))
	for i := range good {
		if err := payloadTextures(good[:i], map[string]struct{}{}); err == nil {
			t.Fatalf("a document cut at %d bytes was accepted", i)
		}
	}
	for _, bad := range [][]byte{nil, {0xfe}, []byte("not nbt at all"), {nbtCompound, 0, 0, 7, 0, 1, 'a', 0xff, 0xff, 0xff, 0xff}, {nbtCompound, 0, 0, 13, 0, 0}} {
		if err := payloadTextures(bad, map[string]struct{}{}); err == nil {
			t.Fatalf("%x was accepted", bad)
		}
	}
}

func gzipped(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, _ = writer.Write(data)
	_ = writer.Close()
	return buf.Bytes()
}

func TestArchiveBlobKeepsContent(t *testing.T) {
	ctx := testDatabase(t)
	profileID := "11111111-1111-1111-1111-111111111111"
	texture, signature := strings.Repeat("t", 300), strings.Repeat("s", 684)
	nbt := skullNBT(texture, signature)
	body := []byte(`{"success":true,"garden":{"inv":"` + base64Text(gzipped(t, nbt)) + `"}}`)
	fetchedAt := time.Now().Add(-30 * 24 * time.Hour).UTC().Truncate(time.Millisecond)
	if err := storeBlob(ctx, profileID, kindGarden, body, fetchedAt); err != nil {
		t.Fatal(err)
	}

	before, _, ok := readBlob(ctx, profileID, kindGarden)
	if !ok {
		t.Fatal("the stored blob could not be read")
	}
	var stored int
	if err := postgres().QueryRow(ctx, `SELECT count(*) FROM texture_strings`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 2 {
		t.Fatalf("texture_strings has %d rows", stored)
	}

	rows, err := nextArchiveRows(ctx, archiveRow{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("archive found %d rows", len(rows))
	}
	if err := archiveBlob(ctx, rows[0], map[int16]archiveDict{}); err != nil {
		t.Fatal(err)
	}
	after, _, ok := readBlob(ctx, profileID, kindGarden)
	if !ok || !bytes.Equal(before, after) {
		t.Fatalf("content changed: %s", after)
	}
	if rows, err = nextArchiveRows(ctx, archiveRow{}); err != nil || len(rows) != 0 {
		t.Fatalf("archive would redo the row: %v %v", rows, err)
	}

	// New content starts over.
	changed := []byte(`{"success":true,"garden":{"level":2}}`)
	if err := storeBlob(ctx, profileID, kindGarden, changed, time.Now()); err != nil {
		t.Fatal(err)
	}
	var level int16
	if err := postgres().QueryRow(ctx, `SELECT level FROM blobs`).Scan(&level); err != nil {
		t.Fatal(err)
	}
	if level != 0 {
		t.Fatalf("a replaced blob stayed at level %d", level)
	}
}

func TestArchiveBlobUsesTheNewestDictionary(t *testing.T) {
	ctx := testDatabase(t)
	resetCaches()
	profileID := "22222222-2222-2222-2222-222222222222"
	body := []byte(`{"success":true,"members":{"a":{"value":1234}}}`)
	if err := storeBlob(ctx, profileID, kindMuseum, body, time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	before, _, _ := readBlob(ctx, profileID, kindMuseum)
	rows, err := nextArchiveRows(ctx, archiveRow{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %v, %v", rows, err)
	}

	// Without its dictionary the row waits.
	dir := t.TempDir()
	noDict, err := writeArchiveDict(ctx, dir, kindMuseum)
	if err != nil || noDict.path != "" {
		t.Fatalf("dictionary = %+v, %v", noDict, err)
	}
	if err := archiveBlob(ctx, rows[0], map[int16]archiveDict{kindMuseum: noDict}); err != nil {
		t.Fatal(err)
	}
	if again, err := nextArchiveRows(ctx, archiveRow{}); err != nil || len(again) != 1 {
		t.Fatalf("a row without its dictionary was finished: %v, %v", again, err)
	}

	var samples [][]byte
	for i := range 500 {
		samples = append(samples, []byte(fmt.Sprintf(`{"success":true,"members":{"a":{"value":%d,"items":{"WITHER_SKULL_%d":{"donated_time":%d}}}}}`, i, i%40, i*7919)))
	}
	dict, err := trainWithZstdCommand(samples, 77, dictSize)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := postgres().Exec(ctx, `INSERT INTO codec_dicts (id, kind, data) VALUES (77, $1, $2)`, kindMuseum, dict); err != nil {
		t.Fatal(err)
	}
	withDict, err := writeArchiveDict(ctx, dir, kindMuseum)
	if err != nil || withDict.id != 77 {
		t.Fatalf("dictionary = %+v, %v", withDict, err)
	}
	if err := archiveBlob(ctx, rows[0], map[int16]archiveDict{kindMuseum: withDict}); err != nil {
		t.Fatal(err)
	}
	var codec int32
	var level int16
	if err := postgres().QueryRow(ctx, `SELECT codec, level FROM blobs`).Scan(&codec, &level); err != nil {
		t.Fatal(err)
	}
	if codec != 77 || level != archiveLevel {
		t.Fatalf("codec %d, level %d", codec, level)
	}
	after, _, ok := readBlob(ctx, profileID, kindMuseum)
	if !ok || !bytes.Equal(before, after) {
		t.Fatalf("content changed: %s", after)
	}
}

// Two parents walk the blobs in the same order, so each must leave the other's rows alone.
func TestArchiveBlobSkipsClaimedAndFinishedRows(t *testing.T) {
	ctx := testDatabase(t)
	testRedis(t, ctx)
	profileID := "33333333-3333-3333-3333-333333333333"
	body := []byte(`{"success":true,"garden":{"level":1}}`)
	if err := storeBlob(ctx, profileID, kindGarden, body, time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	rows, err := nextArchiveRows(ctx, archiveRow{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %v, %v", rows, err)
	}
	level := func() int16 {
		t.Helper()
		var level int16
		if err := postgres().QueryRow(ctx, `SELECT level FROM blobs`).Scan(&level); err != nil {
			t.Fatal(err)
		}
		return level
	}

	release, ok := claimArchiveRow(ctx, rows[0])
	if !ok {
		t.Fatal("a free row could not be claimed")
	}
	if err := archiveBlob(ctx, rows[0], map[int16]archiveDict{}); err != nil {
		t.Fatal(err)
	}
	if got := level(); got != 0 {
		t.Fatalf("a claimed row was compressed, level %d", got)
	}
	release()

	if err := archiveBlob(ctx, rows[0], map[int16]archiveDict{}); err != nil {
		t.Fatal(err)
	}
	if got := level(); got != archiveLevel {
		t.Fatalf("a released row stayed at level %d", got)
	}

	// Data that can't be decompressed shows whether a stale batch reads a finished row again.
	if _, err := postgres().Exec(ctx, `UPDATE blobs SET data = 'not zstd'`); err != nil {
		t.Fatal(err)
	}
	if err := archiveBlob(ctx, rows[0], map[int16]archiveDict{}); err != nil {
		t.Fatalf("a finished row was compressed again: %v", err)
	}
}

// Two parents share a database and only the one that trains a dictionary is told about it.
func TestLoadDictsPicksUpAnotherParentsDictionary(t *testing.T) {
	ctx := testDatabase(t)
	resetCaches()
	body := []byte(`{"success":true,"members":{"a":{"value":1234}}}`)
	if codec, _ := compress(kindMuseum, body); codec != 0 {
		t.Fatalf("codec %d before any dictionary", codec)
	}

	var samples [][]byte
	for i := range 500 {
		samples = append(samples, []byte(fmt.Sprintf(`{"success":true,"members":{"a":{"value":%d,"items":{"WITHER_SKULL_%d":{"donated_time":%d}}}}}`, i, i%40, i*7919)))
	}
	dict, err := trainWithZstdCommand(samples, 77, dictSize)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := postgres().Exec(ctx, `INSERT INTO codec_dicts (id, kind, data) VALUES (77, $1, $2)`, kindMuseum, dict); err != nil {
		t.Fatal(err)
	}

	if err := loadDicts(ctx, postgres()); err != nil {
		t.Fatal(err)
	}
	if codec, _ := compress(kindMuseum, body); codec != 77 {
		t.Fatalf("codec %d after loading", codec)
	}
	first, _ := kindEncoders.Load(kindMuseum)
	if err := loadDicts(ctx, postgres()); err != nil {
		t.Fatal(err)
	}
	if again, _ := kindEncoders.Load(kindMuseum); again != first {
		t.Fatal("loading the same dictionary again rebuilt its encoders")
	}
}

func TestPlayerDictionary(t *testing.T) {
	ctx := testDatabase(t)
	resetCaches()
	body := func(i int) []byte {
		return []byte(fmt.Sprintf(`{"success":true,"player":{"uuid":"%032d","displayname":"Player%d","achievements":{"skyblock_treasure_hunter":%d}}}`, i, i, i*3))
	}
	store := func(i int) {
		t.Helper()
		id := fmt.Sprintf("%08d-0000-0000-0000-000000000000", i)
		if err := storePlayer(ctx, id, body(i), time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	for i := range firstPlayerDictMinRows {
		store(i)
	}
	plainID := fmt.Sprintf("%08d-0000-0000-0000-000000000000", 1)
	before, _, ok := LoadPlayer(ctx, plainID)
	if !ok {
		t.Fatal("the player could not be read")
	}

	if err := maybeTrainDict(ctx, kindPlayer); err != nil {
		t.Fatal(err)
	}
	var dicts int
	if err := postgres().QueryRow(ctx, `SELECT count(*) FROM codec_dicts WHERE kind = $1`, kindPlayer).Scan(&dicts); err != nil {
		t.Fatal(err)
	}
	if dicts != 1 {
		t.Fatalf("%d player dictionaries", dicts)
	}

	store(firstPlayerDictMinRows)
	newID := fmt.Sprintf("%08d-0000-0000-0000-000000000000", firstPlayerDictMinRows)
	var codec int32
	if err := postgres().QueryRow(ctx, `SELECT player_codec FROM players WHERE uuid = $1`, newID).Scan(&codec); err != nil {
		t.Fatal(err)
	}
	if codec == 0 {
		t.Fatal("a new player was written without the dictionary")
	}
	if _, _, ok := LoadPlayer(ctx, newID); !ok {
		t.Fatal("a player written with the dictionary could not be read")
	}
	if after, _, ok := LoadPlayer(ctx, plainID); !ok || !bytes.Equal(before, after) {
		t.Fatal("an older player changed")
	}
}

func TestValueRecordRoundTrip(t *testing.T) {
	ctx := testDatabase(t)
	resetCaches()
	seenAt := time.Unix(1_800_000_000, 0)
	values := valueSet{
		"skill:farming":    {value: 123_456_789, updatedAt: seenAt},
		"skill:mining":     {value: 7, updatedAt: seenAt.Add(-time.Hour)},
		"collection:WHEAT": {value: 1 << 40, updatedAt: seenAt},
		"kills:zombie":     {value: 900, updatedAt: seenAt.Add(-3 * time.Second)},
	}
	for i := range 300 {
		values[fmt.Sprintf("kills:mob_%d", i)] = statValue{value: int64(i * 1000), updatedAt: seenAt}
	}

	encoded, err := encodeValues(ctx, values, seenAt)
	if err != nil {
		t.Fatal(err)
	}
	if encoded[0] != valueRecordMarker || encoded[1] != valuesCompressed {
		t.Fatalf("record starts %x", encoded[:2])
	}
	decoded, err := decodeValues(ctx, encoded, seenAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(values) {
		t.Fatalf("got %d values", len(decoded))
	}
	for key, want := range values {
		if got := decoded[key]; got.value != want.value || !got.updatedAt.Equal(want.updatedAt) {
			t.Errorf("%s = %+v, want %+v", key, got, want)
		}
	}

	// Records written before this format still read.
	ids, err := statKeyIDs(ctx, []string{"skill:mining"})
	if err != nil {
		t.Fatal(err)
	}
	var old []byte
	old = binary.AppendUvarint(old, uint64(ids["skill:mining"]))
	old = binary.AppendVarint(old, 42)
	old = binary.AppendUvarint(old, 60)
	decoded, err = decodeValues(ctx, old, seenAt)
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded["skill:mining"]; got.value != 42 || !got.updatedAt.Equal(seenAt.Add(-time.Minute)) {
		t.Fatalf("old record = %+v", got)
	}

	empty, err := encodeValues(ctx, valueSet{}, seenAt)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err = decodeValues(ctx, empty, seenAt); err != nil || len(decoded) != 0 {
		t.Fatalf("empty record = %v %v", decoded, err)
	}
}

// With SKYCRYPT_BENCHDATA set: every real profile and museum goes through interning and level 19 with a dictionary,
// and must come back as the same response. Prints the sizes.
func TestArchiveRealSample(t *testing.T) {
	root := os.Getenv("SKYCRYPT_BENCHDATA")
	if root == "" {
		t.Skip("SKYCRYPT_BENCHDATA is not set")
	}
	ctx := testDatabase(t)
	resetCaches()

	var profiles, museums [][]byte
	seen := map[string]bool{}
	paths, _ := filepath.Glob(filepath.Join(root, "players/*/profiles.json"))
	for _, path := range paths {
		body, _ := os.ReadFile(path)
		var response struct {
			Profiles []json.RawMessage `json:"profiles"`
		}
		if json.Unmarshal(body, &response) != nil {
			continue
		}
		for _, raw := range response.Profiles {
			id, _, object, err := SplitProfile(raw)
			if err != nil || seen[id] {
				continue
			}
			seen[id] = true
			profiles = append(profiles, object)
		}
	}
	paths, _ = filepath.Glob(filepath.Join(root, "profiles/*/museum.json"))
	for _, path := range paths {
		if body, err := os.ReadFile(path); err == nil && len(body) > 60 {
			museums = append(museums, body)
		}
	}
	if len(profiles) == 0 {
		t.Fatal("no samples")
	}

	for i, set := range [][][]byte{profiles, museums} {
		kind := []int16{kindProfile, kindMuseum}[i]
		var interned [][]byte
		for _, body := range set {
			packed, err := Pack(body)
			if err != nil {
				t.Fatal(err)
			}
			interned = append(interned, internTextures(ctx, packed))
		}
		var samples [][]byte
		for j, container := range interned {
			if j%3 == 2 {
				continue
			}
			for start := 0; start < len(container); start += trainChunkBytes {
				samples = append(samples, container[start:min(start+trainChunkBytes, len(container))])
			}
		}
		dictID := int32(kind) + 100
		dict, err := trainWithZstdCommand(samples, dictID, dictSize)
		if err != nil {
			t.Fatal(err)
		}
		if len(dict) < dictSize*9/10 {
			t.Fatalf("kind %d: dictionary is %d KB of the %d KB asked for", kind, len(dict)>>10, dictSize>>10)
		}
		if _, err := postgres().Exec(ctx, `INSERT INTO codec_dicts (id, kind, data) VALUES ($1, $2, $3)`, dictID, kind, dict); err != nil {
			t.Fatal(err)
		}
		dictFiles, err := writeArchiveDict(ctx, t.TempDir(), kind)
		if err != nil {
			t.Fatal(err)
		}

		var before, after, count int
		for j, container := range interned {
			if j%3 != 2 {
				continue
			}
			packed, _ := Pack(set[j])
			// The stored response is the original without item uuids.
			want, err := Unpack(withoutDroppedTags(t, packed), gzip.BestSpeed)
			if err != nil {
				t.Fatal(err)
			}
			compressed, err := zstdCommand(ctx, container, dictFiles.path)
			if err != nil {
				t.Fatal(err)
			}
			back, err := decompress(ctx, dictFiles.id, compressed)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := restoreTextures(ctx, back)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Unpack(restored, gzip.BestSpeed)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("kind %d sample %d changed", kind, j)
			}
			_, old := compress(0, packed)
			before += len(old)
			after += len(compressed)
			count++
		}
		var tableRows, tableBytes int64
		if err := postgres().QueryRow(ctx, `SELECT count(*), coalesce(sum(length(data)), 0) FROM texture_strings`).Scan(&tableRows, &tableBytes); err != nil {
			t.Fatal(err)
		}
		fmt.Printf("kind %d: %d samples, today %.1f KB avg, now %.1f KB avg (%.0f%% smaller); texture table %d rows, %d KB\n",
			kind, count, float64(before)/float64(count)/1024, float64(after)/float64(count)/1024,
			100-100*float64(after)/float64(before), tableRows, tableBytes>>10)
	}
}

func base64Text(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
