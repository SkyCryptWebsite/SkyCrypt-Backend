package stats

import (
	"testing"

	"skycrypt/src/models"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
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

// A member whose name the caller already has is listed under it without a lookup.
func TestFormatMembersUsesKnownPlayer(t *testing.T) {
	uuid := "0123456789abcdef0123456789abcdef"
	profile := &skycrypttypes.Profile{ProfileID: "p", CuteName: "Apple", Members: map[string]skycrypttypes.Member{uuid: {}}}

	members, err := FormatMembersContext(t.Context(), profile, &models.MowojangResponse{UUID: uuid, Name: "Known"})
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].Name != "Known" || members[0].UUID != uuid {
		t.Fatalf("members = %+v", members)
	}
}
