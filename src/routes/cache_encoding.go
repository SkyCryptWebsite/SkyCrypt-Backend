package routes

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/klauspost/compress/gzip"
)

// Cached responses are stored gzip-compressed with their ETag, so a cache hit sends the stored bytes as they are
// instead of compressing a multi-MB body again, and answers a matching If-None-Match with a 304 without reading the
// body. An entry is cachedResponseMagic, the uvarint length of the ETag, the ETag, then the gzip stream. Entries
// written before this format are plain JSON and still read.
const cachedResponseMagic = "\x00skr1"

var errBadCachedResponse = errors.New("bad cached response entry")

var gzipWriterPool = sync.Pool{
	New: func() any {
		writer, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression)
		return writer
	},
}

// encodeCachedResponse compresses body for the response cache and returns the entry with the body's ETag, the same
// weak tag the etag middleware gives the response when it is first built.
func encodeCachedResponse(body []byte) []byte {
	tag := etag.GenerateWeak(body)
	var out bytes.Buffer
	out.Grow(len(body)/6 + len(tag) + 16)
	out.WriteString(cachedResponseMagic)
	out.Write(binary.AppendUvarint(nil, uint64(len(tag))))
	out.Write(tag)

	writer := gzipWriterPool.Get().(*gzip.Writer)
	defer gzipWriterPool.Put(writer)
	writer.Reset(&out)
	_, _ = writer.Write(body)
	_ = writer.Close()
	return out.Bytes()
}

// cachedResponse is a response cache entry: gzipped is nil for an entry in the old plain format.
type cachedResponse struct {
	etag    string
	gzipped []byte
	plain   []byte
}

func decodeCachedResponse(entry []byte) (cachedResponse, error) {
	rest, ok := bytes.CutPrefix(entry, []byte(cachedResponseMagic))
	if !ok {
		return cachedResponse{plain: entry}, nil
	}
	length, n := binary.Uvarint(rest)
	if n <= 0 || uint64(len(rest)-n) < length {
		return cachedResponse{}, errBadCachedResponse
	}
	return cachedResponse{etag: string(rest[n : n+int(length)]), gzipped: rest[n+int(length):]}, nil
}

// body returns the uncompressed body.
func (r cachedResponse) body() ([]byte, error) {
	if r.gzipped == nil {
		return r.plain, nil
	}
	reader, err := gzip.NewReader(bytes.NewReader(r.gzipped))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(reader)
}

// requestEncoding is what the request accepts, read in the API key middleware for the handlers below it.
type requestEncoding struct {
	acceptsGzip bool
	ifNoneMatch string
}

type requestEncodingKey struct{}

func encodingMiddleware(ctx huma.Context, next func(huma.Context)) {
	encoding := requestEncoding{
		acceptsGzip: acceptsGzip(ctx.Header("Accept-Encoding")),
		ifNoneMatch: ctx.Header("If-None-Match"),
	}
	next(huma.WithValue(ctx, requestEncodingKey{}, encoding))
}

func requestEncodingFrom(ctx context.Context) requestEncoding {
	encoding, _ := ctx.Value(requestEncodingKey{}).(requestEncoding)
	return encoding
}

// acceptsGzip reads an Accept-Encoding header, honoring q=0 and the * wildcard.
func acceptsGzip(header string) bool {
	wildcard := false
	for part := range strings.SplitSeq(header, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		name = strings.ToLower(strings.TrimSpace(name))
		refused := strings.ReplaceAll(strings.TrimSpace(params), " ", "") == "q=0"
		switch name {
		case "gzip":
			return !refused
		case "*":
			wildcard = !refused
		}
	}
	return wildcard
}

// etagMatches reports whether an If-None-Match header lists tag, comparing weakly as RFC 9110 asks for GET.
func etagMatches(ifNoneMatch string, tag string) bool {
	if ifNoneMatch == "" || tag == "" {
		return false
	}
	want := strings.TrimPrefix(tag, "W/")
	for candidate := range strings.SplitSeq(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == want {
			return true
		}
	}
	return false
}
