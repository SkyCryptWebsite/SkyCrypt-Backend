package stats

import (
	"skycrypt/src/models"
	stats "skycrypt/src/stats/items"
)

func GetGear(processedItems map[string][]models.ProcessedItem, allItems []models.ProcessedItem) *models.Gear {
	return &models.Gear{
		Armor:             stats.GetArmor(processedItems["armor"]),
		Equipment:         stats.GetEquipment(processedItems["equipment"]),
		Wardrobe:          stats.GetWardrobe(processedItems["wardrobe"]),
		EquipmentWardrobe: stats.GetWardrobe(processedItems["equipment_wardrobe"]),
		Weapons:           stats.GetWeapons(allItems),
	}
}
