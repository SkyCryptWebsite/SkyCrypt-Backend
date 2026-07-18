package stats

import (
	"sort"
	"strings"

	"skycrypt/src/models"
	statsItems "skycrypt/src/stats/items"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func GetLoadouts(
	loadouts skycrypttypes.Loadouts,
	armorSets map[int][]models.ProcessedItem,
	equipmentSets map[int][]models.ProcessedItem,
	currentArmor []models.ProcessedItem,
	currentEquipment []models.ProcessedItem,
	userProfile *skycrypttypes.Member,
	tuningSlots map[int]map[string]int,
) models.LoadoutsOutput {
	output := make(models.LoadoutsOutput, 0, len(loadouts.Loadouts.Sets))
	petCtx := newPetProcessingContext()

	for setID, loadout := range loadouts.Loadouts.Sets {
		if !isConfiguredLoadout(loadout) {
			continue
		}

		id := loadout.ID
		if id == 0 {
			id = setID
		}

		armor := stripLoadoutItems(armorSets[loadout.ArmorSetID])
		if loadout.ArmorSetID == loadouts.Armor.EquippedSet {
			armor = statsItems.GetArmor(currentArmor).Armor
		}

		equipment := stripLoadoutItems(equipmentSets[loadout.EquipmentSetID])
		if loadout.EquipmentSetID == loadouts.Equipment.EquippedSet {
			equipment = statsItems.GetEquipment(currentEquipment).Equipment
		}

		resolved := models.ResolvedLoadout{
			ID:        id,
			Name:      loadout.Name,
			Armor:     armor,
			Equipment: equipment,
			Accessories: models.LoadoutAccessories{
				TuningPointsSlot: loadout.TuningPointsSlot,
				TuningPoints:     tuningSlots[loadout.TuningPointsSlot],
				PowerStone:       loadout.PowerStoneID,
			},
			MiningCoreSelectedSlot:   loadout.MiningCoreSelectedSlot,
			ForagingCoreSelectedSlot: loadout.ForagingCoreSelectedSlot,
			Pet:                      findLoadoutPet(userProfile, loadout.PetUUID, petCtx),
		}

		output = append(output, resolved)
	}

	sort.Slice(output, func(i, j int) bool {
		return output[i].ID < output[j].ID
	})

	return output
}

func findLoadoutPet(userProfile *skycrypttypes.Member, petUUID string, petCtx *petProcessingContext) *models.StrippedPet {
	if userProfile == nil || userProfile.Pets == nil || petUUID == "" {
		return nil
	}

	for _, pet := range userProfile.Pets.Pets {
		if pet.UniqueId != petUUID {
			continue
		}

		processedPets := getProfilePets(userProfile, &[]skycrypttypes.Pet{pet}, petCtx)
		if len(processedPets) == 0 {
			return nil
		}

		strippedPet := statsItems.StripPet(processedPets[0])
		return &strippedPet
	}

	return nil
}

func isConfiguredLoadout(loadout skycrypttypes.SavedLoadout) bool {
	return strings.TrimSpace(loadout.Name) != "" ||
		loadout.ArmorSetID != 0 ||
		loadout.EquipmentSetID != 0 ||
		loadout.PowerStoneID != "" ||
		loadout.PetUUID != "" ||
		loadout.TuningPointsSlot != 0 ||
		loadout.MiningCoreSelectedSlot != 0 ||
		loadout.ForagingCoreSelectedSlot != 0
}

func stripLoadoutItems(items []models.ProcessedItem) []models.StrippedItem {
	if items == nil {
		return []models.StrippedItem{}
	}

	return statsItems.StripItems(&items)
}
