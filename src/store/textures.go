package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// Every skull in an inventory carries its texture as a base64 Value and an RSA Signature. They are 39% of the NBT and
// nearly random, so zstd can't compress them, yet a few popular heads repeat across every profile. Blobs therefore
// keep only an id, and each string is stored once in texture_strings. Measured on real profiles, that makes bundles
// 18-25% smaller at the same zstd settings.
//
// An interned payload starts with internedPayload instead of NBT's compound tag, and every Value or Signature string
// in it is uvarint id+1, or 0 followed by the string as NBT wrote it. Payloads without the prefix are plain NBT.
const (
	internedPayload byte = 0xfe
	nbtCompound     byte = 0x0a

	// Shorter strings cost less inline than an id and a row. Real textures are hundreds of bytes.
	internMinLength = 64
	nbtMaxDepth     = 512
)

// droppedNBTTags are left out of stored items. Every item's uuid is random, so it can't compress (13% of a big
// bundle), and nothing reads it. Reads return the items without it.
var droppedNBTTags = map[string]bool{"uuid": true}

var errBadNBT = errors.New("store: bad NBT")

// nbtRewriter copies NBT to out, calling onTexture at each Value and Signature string. onTexture consumes the string
// at pos and writes its replacement to out.
type nbtRewriter struct {
	data      []byte
	pos       int
	out       []byte
	onTexture func(*nbtRewriter) error
	drop      map[string]bool
}

func (w *nbtRewriter) copyBytes(n int) error {
	if n < 0 || n > len(w.data)-w.pos {
		return errBadNBT
	}
	w.out = append(w.out, w.data[w.pos:w.pos+n]...)
	w.pos += n
	return nil
}

func (w *nbtRewriter) peekLength(size int) (int, error) {
	if size > len(w.data)-w.pos {
		return 0, errBadNBT
	}
	if size == 2 {
		return int(binary.BigEndian.Uint16(w.data[w.pos:])), nil
	}
	return int(int32(binary.BigEndian.Uint32(w.data[w.pos:]))), nil
}

func (w *nbtRewriter) compound(depth int) error {
	if depth > nbtMaxDepth {
		return errBadNBT
	}
	for {
		if w.pos >= len(w.data) {
			return errBadNBT
		}
		tagStart := len(w.out)
		tag := w.data[w.pos]
		if err := w.copyBytes(1); err != nil {
			return err
		}
		if tag == 0 {
			return nil
		}
		length, err := w.peekLength(2)
		if err != nil {
			return err
		}
		start := w.pos + 2
		if err := w.copyBytes(2 + length); err != nil {
			return err
		}
		name := w.data[start : start+length]
		isTexture := string(name) == "Value" || string(name) == "Signature"
		if err := w.value(tag, isTexture, depth); err != nil {
			return err
		}
		if w.drop[string(name)] {
			w.out = w.out[:tagStart]
		}
	}
}

func (w *nbtRewriter) value(tag byte, isTexture bool, depth int) error {
	switch tag {
	case 1:
		return w.copyBytes(1)
	case 2:
		return w.copyBytes(2)
	case 3, 5:
		return w.copyBytes(4)
	case 4, 6:
		return w.copyBytes(8)
	case 7, 11, 12:
		count, err := w.peekLength(4)
		if err != nil || count < 0 {
			return errBadNBT
		}
		size := map[byte]int{7: 1, 11: 4, 12: 8}[tag]
		if count > (len(w.data)-w.pos)/size {
			return errBadNBT
		}
		return w.copyBytes(4 + count*size)
	case 8:
		if isTexture {
			return w.onTexture(w)
		}
		length, err := w.peekLength(2)
		if err != nil {
			return err
		}
		return w.copyBytes(2 + length)
	case 9:
		if err := w.copyBytes(1); err != nil {
			return err
		}
		element := w.data[w.pos-1]
		count, err := w.peekLength(4)
		if err != nil || count < 0 {
			return errBadNBT
		}
		if err := w.copyBytes(4); err != nil {
			return err
		}
		for range count {
			if err := w.value(element, isTexture, depth+1); err != nil {
				return err
			}
		}
		return nil
	case 10:
		return w.compound(depth + 1)
	}
	return errBadNBT
}

// rewriteNBT runs onTexture over one NBT document. first is what the output starts with, in place of the root tag.
func rewriteNBT(data []byte, first byte, onTexture func(*nbtRewriter) error) ([]byte, error) {
	if len(data) < 3 || data[0] != nbtCompound {
		return nil, errBadNBT
	}
	w := &nbtRewriter{data: data, pos: 1, out: make([]byte, 0, len(data)), onTexture: onTexture}
	w.out = append(w.out, first)
	length, err := w.peekLength(2)
	if err != nil {
		return nil, err
	}
	if err := w.copyBytes(2 + length); err != nil {
		return nil, err
	}
	if err := w.compound(0); err != nil {
		return nil, err
	}
	// Anything after the root is kept as it is.
	w.out = append(w.out, w.data[w.pos:]...)
	return w.out, nil
}

// dropTags returns a plain payload without the given tags.
func dropTags(payload []byte, drop map[string]bool) ([]byte, error) {
	if len(payload) < 3 || payload[0] != nbtCompound {
		return nil, errBadNBT
	}
	w := &nbtRewriter{data: payload, pos: 1, out: make([]byte, 0, len(payload)), drop: drop, onTexture: func(w *nbtRewriter) error {
		length, err := w.peekLength(2)
		if err != nil {
			return err
		}
		return w.copyBytes(2 + length)
	}}
	w.out = append(w.out, nbtCompound)
	length, err := w.peekLength(2)
	if err != nil {
		return nil, err
	}
	if err := w.copyBytes(2 + length); err != nil {
		return nil, err
	}
	if err := w.compound(0); err != nil {
		return nil, err
	}
	w.out = append(w.out, w.data[w.pos:]...)
	return w.out, nil
}

// internPayload replaces a payload's texture strings using ids, which must hold every long one.
func internPayload(payload []byte, ids map[string]int32) ([]byte, error) {
	return rewriteNBT(payload, internedPayload, func(w *nbtRewriter) error {
		length, err := w.peekLength(2)
		if err != nil || length > len(w.data)-w.pos-2 {
			return errBadNBT
		}
		if length >= internMinLength {
			id, ok := ids[string(w.data[w.pos+2:w.pos+2+length])]
			if !ok {
				return errBadNBT
			}
			w.out = binary.AppendUvarint(w.out, uint64(id)+1)
			w.pos += 2 + length
			return nil
		}
		w.out = append(w.out, 0)
		return w.copyBytes(2 + length)
	})
}

// restorePayload is the reverse of internPayload.
func restorePayload(payload []byte, stringsByID map[int32][]byte) ([]byte, error) {
	if len(payload) == 0 || payload[0] != internedPayload {
		return payload, nil
	}
	// The rewriter expects a compound tag where the prefix is.
	data := bytes.Clone(payload)
	data[0] = nbtCompound
	return rewriteNBT(data, nbtCompound, func(w *nbtRewriter) error {
		value, n := binary.Uvarint(w.data[w.pos:])
		if n <= 0 {
			return errBadNBT
		}
		w.pos += n
		if value == 0 {
			length, err := w.peekLength(2)
			if err != nil {
				return err
			}
			return w.copyBytes(2 + length)
		}
		text, ok := stringsByID[int32(value-1)]
		if !ok {
			return fmt.Errorf("store: unknown texture string %d", value-1)
		}
		w.out = binary.BigEndian.AppendUint16(w.out, uint16(len(text)))
		w.out = append(w.out, text...)
		return nil
	})
}

// payloadTextures returns the long texture strings of a plain NBT payload.
func payloadTextures(payload []byte, found map[string]struct{}) error {
	_, err := rewriteNBT(payload, 0, func(w *nbtRewriter) error {
		length, err := w.peekLength(2)
		if err != nil || length > len(w.data)-w.pos-2 {
			return errBadNBT
		}
		if length >= internMinLength {
			found[string(w.data[w.pos+2:w.pos+2+length])] = struct{}{}
		}
		return w.copyBytes(2 + length)
	})
	return err
}

// internedIDs returns the ids a restore of payload will ask for.
func internedIDs(payload []byte, found map[int32]struct{}) error {
	if len(payload) == 0 || payload[0] != internedPayload {
		return nil
	}
	data := bytes.Clone(payload)
	data[0] = nbtCompound
	_, err := rewriteNBT(data, nbtCompound, func(w *nbtRewriter) error {
		value, n := binary.Uvarint(w.data[w.pos:])
		if n <= 0 {
			return errBadNBT
		}
		w.pos += n
		if value == 0 {
			length, err := w.peekLength(2)
			if err != nil {
				return err
			}
			return w.copyBytes(2 + length)
		}
		found[int32(value-1)] = struct{}{}
		return nil
	})
	return err
}

// internTextures drops droppedNBTTags from a container's items and moves their texture strings to texture_strings.
// A payload that can't be read stays as it is, and one that doesn't restore identically keeps its strings. Parent only.
func internTextures(ctx context.Context, container []byte) []byte {
	payloads, json, err := containerParts(container)
	if err != nil || len(payloads) == 0 {
		return container
	}
	for i, payload := range payloads {
		if dropped, err := dropTags(payload, droppedNBTTags); err == nil {
			payloads[i] = dropped
		}
	}

	found := map[string]struct{}{}
	plain := make([]bool, len(payloads))
	for i, payload := range payloads {
		plain[i] = payloadTextures(payload, found) == nil
	}
	ids, err := textureIDs(ctx, found)
	if err != nil {
		fmt.Printf("[STORE] could not intern textures: %v\n", err)
		return buildContainer(payloads, json)
	}
	byID := make(map[int32][]byte, len(ids))
	for text, id := range ids {
		byID[id] = []byte(text)
	}

	out := make([][]byte, len(payloads))
	for i, payload := range payloads {
		out[i] = payload
		if !plain[i] {
			continue
		}
		interned, err := internPayload(payload, ids)
		if err != nil {
			continue
		}
		if restored, err := restorePayload(interned, byID); err == nil && bytes.Equal(restored, payload) {
			out[i] = interned
		}
	}
	return buildContainer(out, json)
}

// textureIDs returns the ids of strings, adding the ones not stored yet.
func textureIDs(ctx context.Context, found map[string]struct{}) (map[string]int32, error) {
	ids := make(map[string]int32, len(found))
	if len(found) == 0 {
		return ids, nil
	}
	pool := postgres()
	if pool == nil {
		return nil, errors.New("store: Postgres is unavailable")
	}

	byHash := make(map[string]string, len(found))
	hashes := make([][]byte, 0, len(found))
	for text := range found {
		sum := sha256.Sum256([]byte(text))
		byHash[string(sum[:16])] = text
		hashes = append(hashes, sum[:16])
	}
	read := func(wanted [][]byte) error {
		rows, err := pool.Query(ctx, `SELECT id, hash FROM texture_strings WHERE hash = ANY($1)`, wanted)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id int32
			var hash []byte
			if err := rows.Scan(&id, &hash); err != nil {
				return err
			}
			ids[byHash[string(hash)]] = id
		}
		return rows.Err()
	}
	if err := read(hashes); err != nil {
		return nil, err
	}
	if len(ids) == len(found) {
		return ids, nil
	}

	var missingHashes, missingData [][]byte
	for _, hash := range hashes {
		text := byHash[string(hash)]
		if _, ok := ids[text]; !ok {
			_, data := compress(0, []byte(text))
			missingHashes = append(missingHashes, hash)
			missingData = append(missingData, data)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO texture_strings (hash, data)
		SELECT * FROM unnest($1::bytea[], $2::bytea[]) ON CONFLICT (hash) DO NOTHING`, missingHashes, missingData); err != nil {
		return nil, err
	}
	if err := read(missingHashes); err != nil {
		return nil, err
	}
	if len(ids) != len(found) {
		return nil, errors.New("store: a texture string has no id")
	}
	return ids, nil
}

// restoreTextures puts a container's texture strings back.
func restoreTextures(ctx context.Context, container []byte) ([]byte, error) {
	payloads, json, err := containerParts(container)
	if err != nil {
		return nil, err
	}
	wanted := map[int32]struct{}{}
	for _, payload := range payloads {
		if err := internedIDs(payload, wanted); err != nil {
			return nil, err
		}
	}
	if len(wanted) == 0 && !hasInterned(payloads) {
		return container, nil
	}

	stringsByID, err := loadTextures(ctx, wanted)
	if err != nil {
		return nil, err
	}
	out := make([][]byte, len(payloads))
	for i, payload := range payloads {
		if out[i], err = restorePayload(payload, stringsByID); err != nil {
			return nil, err
		}
	}
	return buildContainer(out, json), nil
}

func hasInterned(payloads [][]byte) bool {
	for _, payload := range payloads {
		if len(payload) > 0 && payload[0] == internedPayload {
			return true
		}
	}
	return false
}

func loadTextures(ctx context.Context, wanted map[int32]struct{}) (map[int32][]byte, error) {
	out := make(map[int32][]byte, len(wanted))
	if len(wanted) == 0 {
		return out, nil
	}
	pool := postgres()
	if pool == nil {
		return nil, errors.New("store: Postgres is unavailable")
	}
	ids := make([]int32, 0, len(wanted))
	for id := range wanted {
		ids = append(ids, id)
	}
	rows, err := pool.Query(ctx, `SELECT id, data FROM texture_strings WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int32
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		if out[id], err = decompress(ctx, 0, data); err != nil {
			return nil, err
		}
	}
	return out, rows.Err()
}
