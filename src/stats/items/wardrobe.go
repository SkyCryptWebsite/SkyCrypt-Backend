package stats

import (
	"skycrypt/src/models"
)

func GetWardrobe(wardrobeInventory []models.ProcessedItem) [][]*models.StrippedItem {
	wardrobeColumns := len(wardrobeInventory) / 4

	var wardrobe [][]*models.StrippedItem
	for i := range wardrobeColumns {

		var wardrobeSlot []*models.StrippedItem
		for j := range 4 {
			index := i*4 + j

			if index < len(wardrobeInventory) && len(GetId(wardrobeInventory[index])) > 0 {
				strippedItems := StripItems(&[]models.ProcessedItem{wardrobeInventory[index]})
				wardrobeSlot = append(wardrobeSlot, &strippedItems[0])
			} else {
				wardrobeSlot = append(wardrobeSlot, nil)
			}
		}

		hasItems := false
		for _, item := range wardrobeSlot {
			if item != nil {
				hasItems = true
				break
			}
		}

		if hasItems {
			wardrobe = append(wardrobe, wardrobeSlot)
		}
	}

	if len(wardrobe) == 0 {
		return [][]*models.StrippedItem{}
	}

	return wardrobe
}
