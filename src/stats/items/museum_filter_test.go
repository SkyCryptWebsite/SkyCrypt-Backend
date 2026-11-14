package stats

import (
	"encoding/json"
	"testing"

	"skycrypt/src/constants"
	"skycrypt/src/models"
)

// Only museum entries that can hold skill gear are decoded for the skill sections. An entry is kept when its item,
// or a piece of its armor set, is in a skill's museum category, and whenever the item list can't tell.
func TestMuseumEntryMayHoldSkillGear(t *testing.T) {
	var items map[string]models.ProcessedHypixelItem
	if err := json.Unmarshal([]byte(`{
		"DRILL":       {"skyblock_id": "DRILL", "category": "DRILL", "museum_data": {"category": "MINING"}},
		"SWORD":       {"skyblock_id": "SWORD", "category": "SWORD", "museum_data": {"category": "COMBAT"}},
		"PLAIN":       {"skyblock_id": "PLAIN", "category": "MISC"},
		"FARM_HELMET": {"skyblock_id": "FARM_HELMET", "category": "HELMET", "museum_data": {"category": "farming", "armor_set_donation_xp": {"FARM_SET": 1}}},
		"FARM_BOOTS":  {"skyblock_id": "FARM_BOOTS", "category": "BOOTS", "museum_data": {"category": "farming", "armor_set_donation_xp": {"FARM_SET": 1}}},
		"WAR_HELMET":  {"skyblock_id": "WAR_HELMET", "category": "HELMET", "museum_data": {"category": "COMBAT", "armor_set_donation_xp": {"WAR_SET": 1}}}
	}`), &items); err != nil {
		t.Fatal(err)
	}
	previous := constants.ItemsSnapshot()
	constants.SetItems(items)
	t.Cleanup(func() { constants.SetItems(previous) })

	for id, want := range map[string]bool{
		"DRILL":    true,  // a mining item
		"SWORD":    false, // a combat item
		"PLAIN":    true,  // no museum data to judge by
		"FARM_SET": true,  // an armor set of farming pieces
		"WAR_SET":  false, // an armor set of combat pieces
		"UNKNOWN":  true,  // not in the item list
	} {
		if got := museumEntryMayHoldSkillGear(id); got != want {
			t.Errorf("museumEntryMayHoldSkillGear(%q) = %v, want %v", id, got, want)
		}
	}
}
