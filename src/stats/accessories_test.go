package stats

import (
	"fmt"
	"reflect"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"slices"
	"strings"
	"testing"
)

// markInactiveAccessoriesQuadratic is the scan-everything version markInactiveAccessories replaced, kept as the
// reference its output must match.
func markInactiveAccessoriesQuadratic(accessories []models.InsertAccessory) {
	var activeAccessories []models.InsertAccessory
	for _, a := range accessories {
		if !a.IsInactive {
			activeAccessories = append(activeAccessories, a)
		}
	}

	// Process duplicates
	for _, accessory := range activeAccessories {
		id := accessory.Id
		rarity := accessory.Rarity

		var duplicates []models.InsertAccessory
		for _, a := range accessories {
			if constants.GetBaseIdFromAlias(a.Id) == constants.GetBaseIdFromAlias(id) {
				duplicates = append(duplicates, a)
			}
		}

		if len(duplicates) > 1 {
			for i, duplicate := range duplicates {
				duplicateRarity := duplicate.Rarity

				if utility.RarityNameToInt(duplicateRarity) < utility.RarityNameToInt(rarity) {
					duplicates[i].IsInactive = true

					appendInactiveLore(&duplicates[i], fmt.Sprintf("§7Inactive: §cLower rarity duplicate of %s", accessory.DisplayName))
				} else if duplicate.Rarity == rarity {
					duplicates[i].IsInactive = true

					appendInactiveLore(&duplicates[i], fmt.Sprintf("§7Inactive: §cDuplicate of %s", accessory.DisplayName))

				}
			}

			// Update accessories slice with modified duplicates
			for j, acc := range accessories {
				for _, dup := range duplicates {
					if acc.Id == dup.Id && acc.Rarity == dup.Rarity {
						accessories[j].IsInactive = dup.IsInactive

						// NOTE: This could be done better but it works for now (cba to refactor)
						for _, lore := range dup.Lore {
							if strings.HasPrefix(lore, "§7Obtained:") && !loreContains(accessories[j].Lore, "§7Obtained:") {
								accessories[j].Lore = append(accessories[j].Lore, "")
								accessories[j].Lore = append(accessories[j].Lore, lore)
							} else if strings.HasPrefix(lore, "§7Item Value:") && !loreContains(accessories[j].Lore, "§7Item Value:") {
								accessories[j].Lore = append(accessories[j].Lore, "")
								accessories[j].Lore = append(accessories[j].Lore, lore)
							} else if strings.HasPrefix(lore, "§7Inactive:") && !loreContains(accessories[j].Lore, "§7Inactive:") {
								accessories[j].Lore = append(accessories[j].Lore, "")
								accessories[j].Lore = append(accessories[j].Lore, lore)
							}
						}

						break
					}
				}
			}

			// Check if all duplicates are inactive
			allInactive := true
			for _, dup := range duplicates {
				if !dup.IsInactive {
					allInactive = false
					break
				}
			}

			if allInactive {
				// Find and reactivate the current accessory
				for i, acc := range accessories {
					if acc.Id == id && acc.Rarity == rarity {
						accessories[i].IsInactive = false

						// Remove the inactive lore line
						for j := len(accessories[i].Lore) - 1; j >= 0; j-- {
							if strings.HasPrefix(accessories[i].Lore[j], "§7Inactive:") {
								accessories[i].Lore = append(accessories[i].Lore[:j], accessories[i].Lore[j+1:]...)
								if j > 0 {
									accessories[i].Lore = append(accessories[i].Lore[:j-1], accessories[i].Lore[j-1+1:]...)
								}
								break
							}
						}
						break
					}
				}
			}
		}
	}

	// Process upgrade accessories
	for _, accessory := range accessories {
		id := accessory.Id
		upgradeList := constants.GetUpgradeList(id)
		for _, upgrade := range upgradeList {
			if slices.Index(upgradeList, upgrade) < slices.Index(upgradeList, id) {
				for j, acc := range accessories {
					if acc.Id == upgrade {
						accessories[j].IsInactive = true
						appendInactiveLore(&accessories[j], fmt.Sprintf("§7Inactive: §cUpgraded to %s", accessory.DisplayName))
					}
				}
			}
		}
	}
}

func accessoryFixture() []models.InsertAccessory {
	accessory := func(id, rarity, name string, inactive bool, lore ...string) models.InsertAccessory {
		a := models.InsertAccessory{Id: id, Rarity: rarity, IsInactive: inactive}
		a.DisplayName = name
		// Spare capacity, as real lore often has, so appends share backing arrays the way they do in production.
		a.Lore = append(make([]string, 0, len(lore)+4), lore...)
		return a
	}
	return []models.InsertAccessory{
		accessory("WOLF_TALISMAN", "common", "Wolf Talisman", false, "§7Obtained: x"),
		accessory("WOLF_RING", "rare", "Wolf Ring", false, "§7Item Value: 5"),
		accessory("WOLF_RING", "uncommon", "Wolf Ring", false),
		accessory("WOLF_RING", "rare", "Wolf Ring", true, "", "§7Inactive: §cFound in Inventory"),
		accessory("CAMPFIRE_TALISMAN_1", "common", "Campfire 1", false),
		accessory("CAMPFIRE_TALISMAN_3", "uncommon", "Campfire 3", false, "§7Obtained: y"),
		accessory("CAMPFIRE_TALISMAN_2", "uncommon", "Campfire 2", true, "", "§7Inactive: §cFound in Backpack"),
		accessory("CANDY_TALISMAN", "common", "Candy Talisman", false),
		accessory("CANDY_ARTIFACT", "rare", "Candy Artifact", false),
		accessory("CANDY_RING", "uncommon", "Candy Ring", true, "", "§7Inactive: §cFound in Enderchest"),
		accessory("CANDY_RELIC", "epic", "Candy Relic", true, "", "§7Inactive: §cFound in Inventory"),
		accessory("HEGEMONY_ARTIFACT", "legendary", "Hegemony", false, "§7Item Value: 9", "§7Obtained: z"),
		accessory("HEGEMONY_ARTIFACT", "legendary", "Hegemony", false),
	}
}

func TestMarkInactiveAccessoriesMatchesQuadratic(t *testing.T) {
	got, want := accessoryFixture(), accessoryFixture()
	markInactiveAccessories(got)
	markInactiveAccessoriesQuadratic(want)
	if !reflect.DeepEqual(got, want) {
		for i := range got {
			if !reflect.DeepEqual(got[i], want[i]) {
				t.Errorf("%d %s: got %v %q, want %v %q", i, got[i].Id, got[i].IsInactive, got[i].Lore, want[i].IsInactive, want[i].Lore)
			}
		}
	}
}
