package models

type WeaponsResult struct {
	Weapons               []StrippedItem `json:"weapons"`
	HighestPriorityWeapon *StrippedItem  `json:"highest_priority_weapon"`
}

type Gear struct {
	Armor             ArmorResult       `json:"armor"`
	Equipment         EquipmentResult   `json:"equipment"`
	Wardrobe          [][]*StrippedItem `json:"wardrobe"`
	EquipmentWardrobe [][]*StrippedItem `json:"equipmentWardrobe"`
	Weapons           WeaponsResult     `json:"weapons"`
}
