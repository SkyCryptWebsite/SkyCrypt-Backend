package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
)

type failingBody struct {
	readCount atomic.Int32
}

func (b *failingBody) Read(p []byte) (int, error) {
	if b.readCount.Add(1) == 1 {
		return 0, errors.New("stream error: stream ID 5153; INTERNAL_ERROR; received from peer")
	}
	return 0, io.EOF
}

func (b *failingBody) Close() error {
	return nil
}

func TestGetHypixelBodyRetriesResponseReadError(t *testing.T) {
	t.Setenv("HYPIXEL_API_KEY", "test-key")

	originalClient := HTTPClient
	t.Cleanup(func() {
		HTTPClient = originalClient
	})

	var requests atomic.Int32
	HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if requests.Add(1) == 1 {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       &failingBody{},
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"success":true}`))),
			}, nil
		}),
	}

	body, err := getHypixelBody(context.Background(), "https://api.hypixel.net/v2/skyblock/profiles", nil)
	if err != nil {
		t.Fatalf("getHypixelBody() error = %v", err)
	}
	if got := string(body); got != `{"success":true}` {
		t.Fatalf("getHypixelBody() body = %q, want %q", got, `{"success":true}`)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("request count = %d, want 2", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
