package stats

import (
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"testing"
)

func TestOwnedAccessoriesMatchesHasAccessory(t *testing.T) {
	accessories := []models.InsertAccessory{
		{Id: "WOLF_TALISMAN", Rarity: "common"},
		{Id: "WOLF_RING", Rarity: "rare"},
		{Id: "WOLF_RING", Rarity: "uncommon"},
		{Id: "HEGEMONY_ARTIFACT", Rarity: "mythic"},
		{Id: "POWER_RELIC", Rarity: "legendary"},
	}
	accessories[1].Recombobulated = true
	accessories[3].Recombobulated = true
	owned := ownedAccessories(accessories)

	ids := []string{"WOLF_TALISMAN", "WOLF_RING", "HEGEMONY_ARTIFACT", "POWER_RELIC", "MISSING_ONE"}
	for _, id := range ids {
		for _, rarity := range append([]string{""}, constants.RARITIES...) {
			for _, ignore := range []bool{true, false} {
				want := hasAccessory(&accessories, id, rarity, ignore)
				if got := owned.has(id, rarity, ignore); got != want {
					t.Errorf("%s %q ignore=%v: got %v, want %v", id, rarity, ignore, got, want)
				}
			}
		}
	}
}
