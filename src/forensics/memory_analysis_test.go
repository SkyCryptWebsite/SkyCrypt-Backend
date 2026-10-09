package forensics

import (
	"errors"
	"strings"
	"testing"
)

func TestParseProfileSitesAggregatesStacks(t *testing.T) {
	profile := strings.NewReader(`2: 200 [4: 400]
	#	0x1	test.first
	#	0x2	test.second
1: 50 [1: 50]
	#	0x1	test.first
	#	0x2	test.second
`)

	sites, err := parseProfileSites(profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 {
		t.Fatalf("sites len = %d, want 1", len(sites))
	}
	if sites[0].InUseObjects != 3 || sites[0].InUseBytes != 250 ||
		sites[0].CumulativeObjects != 5 || sites[0].CumulativeBytes != 450 {
		t.Fatalf("aggregated site = %+v", sites[0])
	}
}

func TestParseProfileSitesEmpty(t *testing.T) {
	sites, err := parseProfileSites(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 {
		t.Fatalf("sites = %#v, want empty", sites)
	}
}

func TestParseProfileSitesPropagatesReaderError(t *testing.T) {
	wantErr := errors.New("profile read failed")
	reader := errorReader{err: wantErr}
	_, err := parseProfileSites(reader)
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
