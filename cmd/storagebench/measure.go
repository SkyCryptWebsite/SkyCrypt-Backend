package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"time"

	"skycrypt/src/models"
	"skycrypt/src/store"

	"github.com/klauspost/compress/dict"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
)

type sample struct {
	players  map[string][]byte            // uuid -> raw /v2/player body
	profiles map[string][]byte            // uuid -> raw /v2/skyblock/profiles body
	again    map[string][]byte            // uuid -> second profiles fetch
	bundles  map[string][]byte            // profile id -> profile object without "selected"
	museums  map[string][]byte            // profile id -> raw museum body
	gardens  map[string][]byte            // profile id -> raw garden body
	seenBy   map[string]map[string][]byte // profile id -> requester uuid -> profile object without "selected"
}

func measure(dir string) error {
	s, err := loadSample(dir)
	if err != nil {
		return err
	}
	fmt.Printf("sample: %d players, %d unique profiles, %d museums, %d gardens\n\n",
		len(s.profiles), len(s.bundles), len(s.museums), len(s.gardens))

	checkData(s)
	measureSizes(s)
	measureRead(s)
	return nil
}

func loadSample(dir string) (*sample, error) {
	s := &sample{
		players: map[string][]byte{}, profiles: map[string][]byte{}, again: map[string][]byte{},
		bundles: map[string][]byte{}, museums: map[string][]byte{}, gardens: map[string][]byte{},
		seenBy: map[string]map[string][]byte{},
	}

	playerDirs, err := os.ReadDir(filepath.Join(dir, "players"))
	if err != nil {
		return nil, err
	}
	for _, entry := range playerDirs {
		uuid := entry.Name()
		base := filepath.Join(dir, "players", uuid)
		profiles, err := os.ReadFile(filepath.Join(base, "profiles.json"))
		if err != nil {
			continue
		}
		s.profiles[uuid] = profiles
		if player, err := os.ReadFile(filepath.Join(base, "player.json")); err == nil {
			s.players[uuid] = player
		}
		if again, err := os.ReadFile(filepath.Join(base, "profiles.again.json")); err == nil {
			s.again[uuid] = again
		}

		var parsed struct {
			Profiles []json.RawMessage `json:"profiles"`
		}
		if err := json.Unmarshal(profiles, &parsed); err != nil {
			return nil, fmt.Errorf("%s: %w", uuid, err)
		}
		for _, raw := range parsed.Profiles {
			id, _, object, err := store.SplitProfile(raw)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", uuid, err)
			}
			s.bundles[id] = object
			if s.seenBy[id] == nil {
				s.seenBy[id] = map[string][]byte{}
			}
			s.seenBy[id][uuid] = object
		}
	}

	profileDirs, _ := os.ReadDir(filepath.Join(dir, "profiles"))
	for _, entry := range profileDirs {
		base := filepath.Join(dir, "profiles", entry.Name())
		if museum, err := os.ReadFile(filepath.Join(base, "museum.json")); err == nil {
			s.museums[entry.Name()] = museum
		}
		if garden, err := os.ReadFile(filepath.Join(base, "garden.json")); err == nil {
			s.gardens[entry.Name()] = garden
		}
	}
	return s, nil
}

// checkData answers the questions the storage design depends on.
func checkData(s *sample) {
	fmt.Println("== data checks")

	// Every NBT string must be extracted, or it stays gzipped and costs space.
	var candidates, extracted, escaped int
	for _, body := range s.profiles {
		c, e, x := countNBT(body)
		candidates += c
		extracted += e
		escaped += x
	}
	fmt.Printf("NBT strings in profiles: %d, extracted: %d, with JSON escapes: %d\n", candidates, extracted, escaped)

	// Pack must keep the JSON and every NBT payload exactly: packing the unpacked body again gives the same bytes.
	var roundTrips, roundTripFailures int
	check := func(body []byte) {
		packed, err := store.Pack(body)
		if err != nil {
			roundTripFailures++
			return
		}
		unpacked, err := store.Unpack(packed, gzip.BestSpeed)
		if err != nil {
			roundTripFailures++
			return
		}
		again, err := store.Pack(unpacked)
		if err != nil || !store.SameContent(packed, again) || !json.Valid(unpacked) {
			roundTripFailures++
			return
		}
		roundTrips++
	}
	for _, body := range s.profiles {
		check(body)
	}
	for _, body := range s.museums {
		check(body)
	}
	fmt.Printf("pack/unpack round trips: %d ok, %d failed\n", roundTrips, roundTripFailures)

	// The typed parse must not change either.
	var typedFailures int
	for uuid, body := range s.profiles {
		packed, _ := store.Pack(body)
		unpacked, _ := store.Unpack(packed, gzip.BestSpeed)
		var a, b models.HypixelProfilesResponse
		if json.Unmarshal(body, &a) != nil || json.Unmarshal(unpacked, &b) != nil {
			typedFailures++
			continue
		}
		if len(a.Profiles) != len(b.Profiles) {
			typedFailures++
			fmt.Printf("  typed mismatch %s\n", uuid)
		}
	}
	fmt.Printf("typed parse failures: %d\n", typedFailures)

	// Is Hypixel's gzip output stable? If so an unchanged body could be detected without unpacking it.
	var same, differ int
	for uuid, again := range s.again {
		if bytes.Equal(s.profiles[uuid], again) {
			same++
			continue
		}
		a, _ := store.Pack(s.profiles[uuid])
		b, _ := store.Pack(again)
		if bytes.Equal(a, b) {
			fmt.Printf("  %s: body differs, unpacked content identical\n", uuid)
		}
		differ++
	}
	fmt.Printf("second fetch: %d identical bodies, %d different\n", same, differ)

	// Does a profile look the same to every member who requests it?
	var shared, identical int
	fieldDiffs := map[string]int{}
	for _, byRequester := range s.seenBy {
		if len(byRequester) < 2 {
			continue
		}
		shared++
		var first []byte
		allSame := true
		for _, object := range byRequester {
			if first == nil {
				first = object
				continue
			}
			if !bytes.Equal(first, object) {
				allSame = false
				for _, key := range diffFields(first, object) {
					fieldDiffs[key]++
				}
			}
		}
		if allSame {
			identical++
		}
	}
	fmt.Printf("profiles fetched by 2+ members: %d, identical for every requester: %d\n", shared, identical)
	for key, count := range fieldDiffs {
		fmt.Printf("  differs: %s (%d)\n", key, count)
	}

	// Members who left: do they still list the profile?
	var removedListed int
	for uuid, body := range s.profiles {
		var parsed models.HypixelProfilesResponse
		if json.Unmarshal(body, &parsed) != nil {
			continue
		}
		for _, profile := range parsed.Profiles {
			member, ok := profile.Members[uuid]
			if ok && member.Profile != nil && member.Profile.DeletionNotice != nil {
				removedListed++
			}
		}
	}
	fmt.Printf("profiles listed by a member marked deleted on them: %d\n\n", removedListed)
}

func countNBT(body []byte) (candidates, extracted, escaped int) {
	for i := 0; i < len(body); i++ {
		if body[i] != '"' {
			continue
		}
		end := i + 1
		for end < len(body) && body[end] != '"' {
			if body[end] == '\\' {
				end++
			}
			end++
		}
		content := body[i+1 : min(end, len(body))]
		i = end
		if len(content) >= 32 && string(content[:4]) == "H4sI" {
			candidates++
			if bytes.IndexByte(content, '\\') >= 0 {
				escaped++
			}
		}
	}
	packed, _ := store.Pack(body)
	count, _ := binary.Uvarint(packed)
	return candidates, int(count), escaped
}

// diffFields names the top-level and member fields that differ between two copies of a profile object.
func diffFields(a, b []byte) []string {
	var x, y map[string]json.RawMessage
	_ = json.Unmarshal(a, &x)
	_ = json.Unmarshal(b, &y)
	var out []string
	for key, value := range x {
		if key == "members" {
			var mx, my map[string]map[string]json.RawMessage
			_ = json.Unmarshal(value, &mx)
			_ = json.Unmarshal(y[key], &my)
			for member, fields := range mx {
				for name, v := range fields {
					if !bytes.Equal(v, my[member][name]) {
						out = append(out, "members.*."+name)
					}
				}
			}
			continue
		}
		if !bytes.Equal(value, y[key]) {
			out = append(out, key)
		}
	}
	return out
}

type variant struct {
	name  string
	level zstd.EncoderLevel
	dict  []byte
}

func measureSizes(s *sample) {
	fmt.Println("== sizes")

	// Today: one zstd body per looked-up player, coop data repeated for every member.
	var baseline int
	for _, body := range s.profiles {
		baseline += len(encode(body, zstd.SpeedDefault, nil))
	}
	var baselineRaw int
	for _, body := range s.profiles {
		baselineRaw += len(body)
	}
	fmt.Printf("current profiles storage: %s raw, %s stored (%s per player)\n",
		mb(baselineRaw), mb(baseline), kb(baseline/len(s.profiles)))

	// Split unique profiles into a training half and a test half for dictionaries.
	ids := make([]string, 0, len(s.bundles))
	for id := range s.bundles {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var train, test []string
	for _, id := range ids {
		h := fnv.New32a()
		_, _ = h.Write([]byte(id))
		if h.Sum32()%2 == 0 {
			train = append(train, id)
		} else {
			test = append(test, id)
		}
	}

	packed := map[string][]byte{}
	var packedRaw, unpackedRaw int
	for id, object := range s.bundles {
		p, _ := store.Pack(object)
		packed[id] = p
		packedRaw += len(p)
		unpackedRaw += len(object)
	}
	fmt.Printf("unique profile bundles: %d, %s raw JSON, %s packed (NBT unzipped)\n", len(ids), mb(unpackedRaw), mb(packedRaw))

	trainSamples := chunks(packed, train, 32<<10)
	variants := []variant{
		{"zstd default, gzip kept (no pack)", zstd.SpeedDefault, nil},
		{"pack + zstd default", zstd.SpeedDefault, nil},
		{"pack + zstd better", zstd.SpeedBetterCompression, nil},
		{"pack + zstd best", zstd.SpeedBestCompression, nil},
	}
	if d, err := dict.BuildZstdDict(trainSamples, dict.Options{MaxDictSize: 1 << 20, HashBytes: 6, ZstdDictID: 1, ZstdLevel: zstd.SpeedBestCompression}); err == nil {
		variants = append(variants, variant{"pack + best + klauspost dict 1024K", zstd.SpeedBestCompression, d})
	}
	for _, size := range []int{1 << 20, 2 << 20, 4 << 20} {
		d, err := zstdTrain(trainSamples, size)
		if err != nil {
			fmt.Printf("zstd --train %dK: %v\n", size>>10, err)
			continue
		}
		for _, level := range []zstd.EncoderLevel{zstd.SpeedDefault, zstd.SpeedBetterCompression, zstd.SpeedBestCompression} {
			variants = append(variants, variant{fmt.Sprintf("pack + %s + zstd --train dict %dK", level, size>>10), level, d})
		}
	}

	fmt.Printf("\nheld-out profiles (%d), stored size per variant:\n", len(test))
	for i, v := range variants {
		var total int
		start := time.Now()
		for _, id := range test {
			body := packed[id]
			if i == 0 {
				body = s.bundles[id]
			}
			total += len(encode(body, v.level, v.dict))
		}
		fmt.Printf("  %-44s %10s  %6s/profile  %v\n", v.name, mb(total), kb(total/len(test)), time.Since(start).Round(time.Millisecond))
	}

	// Separate frames for the JSON part and the NBT part, to see whether they compress better apart.
	var together, apart int
	for _, id := range test {
		p := packed[id]
		together += len(encode(p, zstd.SpeedBestCompression, nil))
		jsonPart, nbtPart := splitContainer(p)
		apart += len(encode(jsonPart, zstd.SpeedBestCompression, nil)) + len(encode(nbtPart, zstd.SpeedBestCompression, nil))
	}
	fmt.Printf("  one frame vs JSON/NBT frames apart: %s vs %s\n", mb(together), mb(apart))

	// Museum, garden and player bodies.
	var museumRaw, museumNow, museumPacked int
	for _, body := range s.museums {
		museumRaw += len(body)
		museumNow += len(encode(body, zstd.SpeedDefault, nil))
		p, _ := store.Pack(body)
		museumPacked += len(encode(p, zstd.SpeedBestCompression, nil))
	}
	var gardenNow, gardenBest int
	for _, body := range s.gardens {
		gardenNow += len(encode(body, zstd.SpeedDefault, nil))
		gardenBest += len(encode(body, zstd.SpeedBestCompression, nil))
	}
	var playerNow, playerTrimmed int
	for _, body := range s.players {
		playerNow += len(encode(body, zstd.SpeedDefault, nil))
		trimmed, err := store.TrimPlayer(body)
		if err == nil {
			playerTrimmed += len(encode(trimmed, zstd.SpeedBestCompression, nil))
		}
	}
	fmt.Printf("\nmuseums (%d): %s raw, %s today, %s packed+best\n", len(s.museums), mb(museumRaw), mb(museumNow), mb(museumPacked))
	fmt.Printf("gardens (%d): %s today, %s best\n", len(s.gardens), mb(gardenNow), mb(gardenBest))
	fmt.Printf("players (%d): %s today, %s trimmed+best (%s each)\n\n", len(s.players), mb(playerNow), mb(playerTrimmed), bytesString(playerTrimmed/max(1, len(s.players))))
}

func measureRead(s *sample) {
	fmt.Println("== read cost (GOMAXPROCS=1, like a prefork child)")
	runtime.GOMAXPROCS(1)

	// The biggest player in the sample stands in for the worst case.
	var worst string
	for uuid, body := range s.profiles {
		if worst == "" || len(body) > len(s.profiles[worst]) {
			worst = uuid
		}
	}
	var parsed struct {
		Profiles []json.RawMessage `json:"profiles"`
	}
	_ = json.Unmarshal(s.profiles[worst], &parsed)
	frames := make([][]byte, 0, len(parsed.Profiles))
	for _, raw := range parsed.Profiles {
		_, _, object, _ := store.SplitProfile(raw)
		p, _ := store.Pack(object)
		frames = append(frames, encode(p, zstd.SpeedBestCompression, nil))
	}
	fmt.Printf("player %s: %s body, %d profiles\n", worst, kb(len(s.profiles[worst])), len(frames))

	decoder, _ := zstd.NewReader(nil)
	for _, level := range []int{gzip.NoCompression, gzip.BestSpeed} {
		const runs = 10
		var rebuild, parse time.Duration
		var size int
		for range runs {
			start := time.Now()
			out := []byte(`{"success":true,"profiles":[`)
			for i, frame := range frames {
				p, err := decoder.DecodeAll(frame, nil)
				if err != nil {
					panic(err)
				}
				body, err := store.Unpack(p, level)
				if err != nil {
					panic(err)
				}
				if i > 0 {
					out = append(out, ',')
				}
				out = append(out, `{"selected":false,`...)
				out = append(out, body[1:]...)
			}
			out = append(out, "]}"...)
			rebuild += time.Since(start)
			size = len(out)

			start = time.Now()
			var response models.HypixelProfilesResponse
			if err := json.Unmarshal(out, &response); err != nil {
				panic(err)
			}
			parse += time.Since(start)
		}
		fmt.Printf("  gzip level %d: rebuild %v, rebuilt body %s, parse %v\n", level, rebuild/runs, kb(size), parse/runs)
	}
}

func encode(body []byte, level zstd.EncoderLevel, d []byte) []byte {
	options := []zstd.EOption{zstd.WithEncoderLevel(level)}
	if d != nil {
		options = append(options, zstd.WithEncoderDict(d))
	}
	encoder, err := zstd.NewWriter(nil, options...)
	if err != nil {
		panic(err)
	}
	defer func() { _ = encoder.Close() }()
	return encoder.EncodeAll(body, nil)
}

func chunks(packed map[string][]byte, ids []string, size int) [][]byte {
	var out [][]byte
	for _, id := range ids {
		p := packed[id]
		for start := 0; start < len(p); start += size {
			out = append(out, p[start:min(start+size, len(p))])
		}
	}
	return out
}

func zstdTrain(samples [][]byte, size int) ([]byte, error) {
	dir, err := os.MkdirTemp("", "storagebench-train")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	for i, sample := range samples {
		if err := os.WriteFile(filepath.Join(dir, strconv.Itoa(i)), sample, 0o644); err != nil {
			return nil, err
		}
	}
	out := filepath.Join(dir, "dict")
	cmd := exec.Command("zstd", "--train", "-q", "-r", dir, "-o", out, "--maxdict="+strconv.Itoa(size))
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, output)
	}
	return os.ReadFile(out)
}

func splitContainer(p []byte) (jsonPart, nbtPart []byte) {
	count, n := binary.Uvarint(p)
	rest := p[n:]
	start := rest
	for range count {
		length, n := binary.Uvarint(rest)
		rest = rest[n+int(length):]
	}
	return rest, start[:len(start)-len(rest)]
}

func mb(n int) string { return fmt.Sprintf("%.2f MB", float64(n)/(1<<20)) }
func kb(n int) string { return fmt.Sprintf("%.1f KB", float64(n)/(1<<10)) }
func bytesString(n int) string {
	return strconv.Itoa(n) + " B"
}
