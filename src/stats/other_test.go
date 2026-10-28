package stats

import (
	"testing"

	"skycrypt/src/models"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

// The parsed profiles are shared between requests, and callers fill in fields (Banking, CommunityUpgrades), so
// every branch must hand out a copy.
func TestGetProfileNeverReturnsTheCachedProfile(t *testing.T) {
	profiles := &models.HypixelProfilesResponse{Profiles: []skycrypttypes.Profile{{ProfileID: "a", CuteName: "Apple"}}}

	for _, id := range []string{"", "a", "apple"} {
		profile, err := GetProfile(profiles, id)
		if err != nil {
			t.Fatal(err)
		}
		profile.Banking = &skycrypttypes.Banking{}
		if profiles.Profiles[0].Banking != nil {
			t.Fatalf("GetProfile(%q) returned the cached profile itself", id)
		}
	}
}
