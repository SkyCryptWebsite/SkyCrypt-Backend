package forensics

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// readerOnce opens the same text on every pass, as topProfileSites reads a profile twice.
func readerOnce(reader *strings.Reader) func() (io.ReadCloser, error) {
	text, _ := io.ReadAll(reader)
	return func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(string(text))), nil }
}

func TestTopProfileSitesAggregatesStacks(t *testing.T) {
	profile := strings.NewReader(`2: 200 [4: 400]
	#	0x1	test.first
	#	0x2	test.second
1: 50 [1: 50]
	#	0x1	test.first
	#	0x2	test.second
`)

	sites, err := topProfileSites(readerOnce(profile))
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 {
		t.Fatalf("sites len = %d, want 1", len(sites))
	}
	if sites[0].Stack != "test.first\ntest.second" {
		t.Fatalf("stack = %q", sites[0].Stack)
	}
	if sites[0].InUseObjects != 3 || sites[0].InUseBytes != 250 ||
		sites[0].CumulativeObjects != 5 || sites[0].CumulativeBytes != 450 {
		t.Fatalf("aggregated site = %+v", sites[0])
	}
}

func TestTopProfileSitesEmpty(t *testing.T) {
	sites, err := topProfileSites(readerOnce(strings.NewReader("")))
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 {
		t.Fatalf("sites = %#v, want empty", sites)
	}
}

func TestTopProfileSitesPropagatesReaderError(t *testing.T) {
	wantErr := errors.New("profile read failed")
	reader := errorReader{err: wantErr}
	_, err := topProfileSites(func() (io.ReadCloser, error) { return io.NopCloser(reader), nil })
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) {
	return 0, reader.err
}
