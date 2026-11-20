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

func TestGetMissingListsUnownedAccessories(t *testing.T) {
	previous := constants.ItemsSnapshot()
	t.Cleanup(func() { constants.SetItems(previous) })
	constants.SetItems(map[string]models.ProcessedHypixelItem{
		"WOLF_TALISMAN":            {SkyblockID: "WOLF_TALISMAN", Category: "accessory", Rarity: "common", Name: "Wolf Talisman"},
		"WOLF_RING":                {SkyblockID: "WOLF_RING", Category: "accessory", Rarity: "rare", Name: "Wolf Ring"},
		"FEATHER_TALISMAN":         {SkyblockID: "FEATHER_TALISMAN", Category: "accessory", Rarity: "common", Name: "Feather Talisman"},
		"POTION_AFFINITY_TALISMAN": {SkyblockID: "POTION_AFFINITY_TALISMAN", Category: "accessory", Rarity: "common", Name: "Potion Affinity Talisman"},
		"NOT_AN_ACCESSORY":         {SkyblockID: "NOT_AN_ACCESSORY", Category: "sword", Rarity: "common"},
	})

	// The wolf ring covers the wolf talisman it upgrades, and the feather talisman is owned.
	owned := []models.InsertAccessory{
		{Id: "WOLF_RING", Rarity: "rare"},
		{Id: "FEATHER_TALISMAN", Rarity: "common"},
	}
	output := getMissing(&owned, nil)

	var ids []string
	for _, item := range append(output.Upgrades, output.Other...) {
		ids = append(ids, item.Id)
	}
	if len(ids) != 1 || ids[0] != "POTION_AFFINITY_TALISMAN" {
		t.Fatalf("missing = %v, want [POTION_AFFINITY_TALISMAN]", ids)
	}
	if item := append(output.Upgrades, output.Other...)[0]; item.Rarity != "common" || item.DisplayName != "Potion Affinity Talisman" {
		t.Fatalf("missing item = %#v", item)
	}
}
