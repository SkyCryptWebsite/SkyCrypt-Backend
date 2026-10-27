package store

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/gzip"
)

// Hypixel sends inventories as base64(gzip(NBT)) strings inside the JSON. Gzipped data can't be compressed again, so
// Pack swaps each of those strings for a marker byte and keeps the raw NBT next to the JSON. The whole container is
// then compressed in one zstd frame, where the NBT compresses several times better. Unpack puts the strings back.
//
// Container layout: uvarint payload count, then each payload as uvarint length + bytes, then the JSON. In the JSON,
// a string whose only content is a marker byte stands for the next payload. Raw control bytes can't appear in valid
// JSON, so markers never clash with real data; a body that contains one anyway is stored without payloads.

const (
	// markerPlain stands for plain base64.
	markerPlain = 0x01
	// markerEscaped stands for base64 whose "=" padding is written as \u003d, which is how Hypixel sends most of it.
	markerEscaped = 0x02
)

const escapedPadding = `\u003d`

// gzipMagic is how base64 of a gzip stream starts: 1f 8b 08.
const gzipMagic = "H4sI"

// Pack returns the container for a raw JSON body. Every extracted string is checked to round-trip: canonical base64
// of a single, complete gzip member. Anything else stays in the JSON untouched.
func Pack(body []byte) ([]byte, error) {
	if bytes.IndexByte(body, markerPlain) >= 0 || bytes.IndexByte(body, markerEscaped) >= 0 {
		return appendRawContainer(nil, body), nil
	}

	var payloads [][]byte
	json := make([]byte, 0, len(body)/4)
	last := 0
	for i := 0; i < len(body); i++ {
		if body[i] != '"' {
			continue
		}
		end, err := skipString(body, i)
		if err != nil {
			return nil, err
		}
		content := body[i+1 : end-1]
		i = end - 1

		if len(content) < 32 || string(content[:4]) != gzipMagic {
			continue
		}
		payload, marker, ok := unwrapGzipBase64(content)
		if !ok {
			continue
		}

		json = append(json, body[last:i-len(content)]...)
		json = append(json, marker)
		last = end - 1
		payloads = append(payloads, payload)
	}
	json = append(json, body[last:]...)

	size := binary.MaxVarintLen64 + len(json)
	for _, payload := range payloads {
		size += binary.MaxVarintLen64 + len(payload)
	}
	out := make([]byte, 0, size)
	out = binary.AppendUvarint(out, uint64(len(payloads)))
	for _, payload := range payloads {
		out = binary.AppendUvarint(out, uint64(len(payload)))
		out = append(out, payload...)
	}
	return append(out, json...), nil
}

func appendRawContainer(out, body []byte) []byte {
	out = binary.AppendUvarint(out, 0)
	return append(out, body...)
}

var gzipReaderPool = sync.Pool{New: func() any { return new(gzip.Reader) }}

// unwrapGzipBase64 returns the decompressed content of a base64 gzip string and the marker that rebuilds its text. It
// only accepts strings that re-encode to the same text and hold one complete, checksummed gzip member.
func unwrapGzipBase64(content []byte) ([]byte, byte, bool) {
	marker := byte(markerPlain)
	text := content
	if bytes.IndexByte(content, '\\') >= 0 {
		text = bytes.ReplaceAll(content, []byte(escapedPadding), []byte("="))
		if bytes.IndexByte(text, '\\') >= 0 {
			return nil, 0, false
		}
		marker = markerEscaped
	}

	compressed := make([]byte, base64.StdEncoding.DecodedLen(len(text)))
	n, err := base64.StdEncoding.Decode(compressed, text)
	if err != nil || !bytes.Equal(appendBase64(nil, compressed[:n], marker), content) {
		return nil, 0, false
	}
	compressed = compressed[:n]

	source := bytes.NewReader(compressed)
	reader := gzipReaderPool.Get().(*gzip.Reader)
	defer gzipReaderPool.Put(reader)
	if err := reader.Reset(source); err != nil {
		return nil, 0, false
	}
	reader.Multistream(false)

	payload, err := io.ReadAll(reader)
	if err != nil || source.Len() != 0 {
		return nil, 0, false
	}
	return payload, marker, true
}

func appendBase64(out, data []byte, marker byte) []byte {
	start := len(out)
	out = base64.StdEncoding.AppendEncode(out, data)
	if marker != markerEscaped {
		return out
	}
	padding := 0
	for padding < 2 && len(out)-padding > start && out[len(out)-1-padding] == '=' {
		padding++
	}
	out = out[:len(out)-padding]
	for range padding {
		out = append(out, escapedPadding...)
	}
	return out
}

// Unpack rebuilds the JSON body from a container. Payloads are re-gzipped at gzipLevel; the bytes differ from what
// Hypixel sent, but every gzip reader gets back the same NBT.
func Unpack(container []byte, gzipLevel int) ([]byte, error) {
	count, n := binary.Uvarint(container)
	if n <= 0 {
		return nil, errors.New("store: bad container header")
	}
	rest := container[n:]

	payloads := make([][]byte, 0, count)
	encodedSize := 0
	for range count {
		length, n := binary.Uvarint(rest)
		if n <= 0 || uint64(len(rest)-n) < length {
			return nil, errors.New("store: truncated container payload")
		}
		payload := rest[n : n+int(length)]
		payloads = append(payloads, payload)
		encodedSize += base64.StdEncoding.EncodedLen(len(payload)/2 + 64)
		rest = rest[n+int(length):]
	}
	json := rest
	if count == 0 {
		return json, nil
	}

	out := make([]byte, 0, len(json)+encodedSize)
	var gz bytes.Buffer
	writer := gzipWriter(gzipLevel)
	defer putGzipWriter(gzipLevel, writer)

	next := 0
	last := 0
	for i := 0; i < len(json); i++ {
		marker := json[i]
		if marker != markerPlain && marker != markerEscaped {
			continue
		}
		if next >= len(payloads) {
			return nil, errors.New("store: more markers than payloads")
		}

		gz.Reset()
		writer.Reset(&gz)
		if _, err := writer.Write(payloads[next]); err != nil {
			return nil, err
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
		next++

		out = append(out, json[last:i]...)
		out = appendBase64(out, gz.Bytes(), marker)
		last = i + 1
	}
	if next != len(payloads) {
		return nil, fmt.Errorf("store: %d payloads but %d markers", len(payloads), next)
	}
	return append(out, json[last:]...), nil
}

var gzipWriterPools sync.Map // level -> *sync.Pool

func gzipWriter(level int) *gzip.Writer {
	pool, _ := gzipWriterPools.LoadOrStore(level, &sync.Pool{})
	if writer, ok := pool.(*sync.Pool).Get().(*gzip.Writer); ok {
		return writer
	}
	writer, err := gzip.NewWriterLevel(io.Discard, level)
	if err != nil {
		writer, _ = gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
	}
	return writer
}

func putGzipWriter(level int, writer *gzip.Writer) {
	pool, _ := gzipWriterPools.Load(level)
	pool.(*sync.Pool).Put(writer)
}

// SameContent reports whether two containers hold the same JSON and payloads. The marker bytes may differ: a payload
// re-gzipped by Unpack can end up without base64 padding, so the escaped-padding marker has nothing to record.
func SameContent(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x == markerEscaped {
			x = markerPlain
		}
		if y == markerEscaped {
			y = markerPlain
		}
		if x != y {
			return false
		}
	}
	return true
}
