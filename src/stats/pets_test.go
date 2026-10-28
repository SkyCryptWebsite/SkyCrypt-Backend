package stats

import (
	"os"
	"reflect"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"testing"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
)

// Needs a NEU checkout (and fetches prices), so it only runs with NEU_REPO_ROOT set to the directory holding
// NotEnoughUpdates-REPO.
func TestActivePetStatsMatchesGetPets(t *testing.T) {
	root := os.Getenv("NEU_REPO_ROOT")
	if root == "" {
		t.Skip("NEU_REPO_ROOT not set")
	}
	t.Chdir(root)
	if err := notenoughupdates.ParseNEURepository(); err != nil {
		t.Fatal(err)
	}

	newMember := func(active ...bool) *skycrypttypes.Member {
		member := &skycrypttypes.Member{Pets: &skycrypttypes.Pets{Pets: []skycrypttypes.Pet{
			{Type: "BEE", Rarity: "RARE", Experience: 50_000, Active: active[0]},
			{Type: "ENDER_DRAGON", Rarity: "EPIC", Experience: 25_000_000, HeldItem: "PET_ITEM_TIER_BOOST", Active: active[1]},
			{Type: "WOLF", Rarity: "LEGENDARY", Experience: 1_000, Active: active[2]},
			{Type: "SHEEP", Rarity: ""},
		}}}
		member.Rift.DeadCats.Montezuma = skycrypttypes.Pet{Type: "FRACTURED_MONTEZUMA_SOUL", Rarity: "EPIC", Active: true}
		return member
	}

	for _, active := range [][]bool{{false, false, false}, {true, false, false}, {false, true, true}} {
		stats, score, ok := getActivePetStats(newMember(active...))
		pets := GetPets(newMember(active...), &skycrypttypes.Profile{})

		var wantStats map[string]float64
		for _, pet := range pets.Pets {
			if pet.Active {
				wantStats = pet.Stats
				break
			}
		}
		if !ok || len(stats) != len(wantStats) || (len(stats) > 0 && !reflect.DeepEqual(stats, wantStats)) {
			t.Errorf("active %v: stats %v, want %v", active, stats, wantStats)
		}
		if !reflect.DeepEqual(score, pets.PetScore) {
			t.Errorf("active %v: score %+v, want %+v", active, score, pets.PetScore)
		}
	}

	if _, _, ok := getActivePetStats(&skycrypttypes.Member{}); ok {
		t.Error("expected no pets")
	}
}
