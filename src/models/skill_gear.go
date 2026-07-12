package models

type SkillGear struct {
	Armor     *SkillArmorSet `json:"armor"`
	Equipment SkillEquipment `json:"equipment"`
	Tools     []StrippedItem `json:"tools"`
	Misc      []StrippedItem `json:"misc"`
}

type SkillArmorSet struct {
	SetID     string           `json:"set_id"`
	GameStage string           `json:"game_stage,omitempty"`
	Pieces    SkillArmorPieces `json:"pieces"`
}

type SkillArmorPieces struct {
	Helmet     *StrippedItem `json:"helmet"`
	Chestplate *StrippedItem `json:"chestplate"`
	Leggings   *StrippedItem `json:"leggings"`
	Boots      *StrippedItem `json:"boots"`
}

type SkillEquipment struct {
	Necklace *StrippedItem `json:"necklace"`
	Cloak    *StrippedItem `json:"cloak"`
	Belt     *StrippedItem `json:"belt"`
	Gloves   *StrippedItem `json:"gloves"`
}
