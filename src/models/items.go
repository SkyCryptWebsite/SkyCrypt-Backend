package models

import skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"

type DecodedInventory struct {
	Items []*skycrypttypes.Item `nbt:"i"`
}

type ProcessedItem struct {
	skycrypttypes.Item
	Texture        string          `json:"texture_path,omitempty"`
	TexturePack    string          `json:"texture_pack,omitempty"`
	DisplayName    string          `json:"display_name,omitempty"`
	Lore           []string        `json:"lore,omitempty"`
	Rarity         string          `json:"rarity,omitempty"`
	Recombobulated bool            `json:"recombobulated,omitempty"`
	Categories     []string        `json:"categories,omitempty"`
	ContainsItems  []ProcessedItem `json:"containsItems,omitempty"`
	Source         string          `json:"source,omitempty"`
	Id             string          `json:"id,omitempty"`
	IsInactive     *bool           `json:"isInactive,omitempty"`
	Shiny          bool            `json:"shiny,omitempty"`
	Wiki           *string         `json:"wiki,omitempty"`
	Price          float64         `json:"price,omitempty"`
	ItemIndex      int             `json:"itemIndex,omitempty"`
	Count          *int            `json:"Count,omitempty"`
}

type SkillToolsResult struct {
	Tools               []StrippedItem `json:"tools"`
	HighestPriorityTool *StrippedItem  `json:"highest_priority_tool"`
}

type ArmorResult struct {
	Armor     []StrippedItem     `json:"armor"`
	Stats     map[string]float64 `json:"stats"`
	SetName   *string            `json:"set_name,omitempty"`
	SetRarity *string            `json:"set_rarity,omitempty"`
}

type EquipmentResult struct {
	Equipment []StrippedItem     `json:"equipment"`
	Stats     map[string]float64 `json:"stats"`
}

type StrippedItem struct {
	DisplayName    string         `json:"display_name,omitempty"`
	Lore           []string       `json:"lore,omitempty"`
	Rarity         string         `json:"rarity,omitempty"`
	Recombobulated bool           `json:"recombobulated,omitempty"`
	ContainsItems  []StrippedItem `json:"containsItems,omitempty"`
	Source         string         `json:"source,omitempty"`
	Texture        string         `json:"texture_path,omitempty"`
	IsInactive     *bool          `json:"isInactive,omitempty"`
	Count          *int           `json:"Count,omitempty"`
	Shiny          bool           `json:"shiny,omitempty"`
	Wiki           *string        `json:"wiki,omitempty"`
	TexturePack    string         `json:"texture_pack,omitempty"`
	SourceTab      *SourceTab     `json:"sourceTab,omitempty"`
}

type SourceTab struct {
	Icon string `json:"icon"`
	Name string `json:"name"`
}
