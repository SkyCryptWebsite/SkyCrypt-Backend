package stats

import (
	"testing"

	"skycrypt/src/models"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
)

// A profile with the bank API off has no banking object; GetStats read through the nil pointer.
func TestGetStatsWithoutBanking(t *testing.T) {
	profile := &skycrypttypes.Profile{ProfileID: "profile"}
	out, err := GetStats(&models.MowojangResponse{UUID: "uuid", Name: "name"}, &models.HypixelProfilesResponse{}, profile,
		&skycrypttypes.Player{}, &skycrypttypes.Member{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Bank != nil || out.APISettings["banking"] {
		t.Fatalf("bank = %v, banking setting = %v, want no bank", out.Bank, out.APISettings["banking"])
	}

	balance := 12.5
	profile = &skycrypttypes.Profile{ProfileID: "profile", Banking: &skycrypttypes.Banking{Balance: &balance}}
	out, err = GetStats(&models.MowojangResponse{UUID: "uuid", Name: "name"}, &models.HypixelProfilesResponse{}, profile,
		&skycrypttypes.Player{}, &skycrypttypes.Member{}, nil, nil)
	if err != nil || out.Bank == nil || *out.Bank != balance || !out.APISettings["banking"] {
		t.Fatalf("bank = %v, err = %v, want %v", out.Bank, err, balance)
	}
}
