package models

// Inventory    []StrippedItem   `json:"inventory,omitempty"`
type CombinedOutput struct {
	Gear         *Gear                       `json:"gear,omitempty"`
	Accesssories *GetMissingAccessoresOutput `json:"accessories,omitempty"`
	Pets         *OutputPets                 `json:"pets,omitempty"`
	Inventory    *Inventory                  `json:"inventory,omitempty"`
	Skills       *SkillsOutput               `json:"skills,omitempty"`
	Dungeons     *DungeonsOutput             `json:"dungeons,omitempty"`
	Slayer       *SlayersOutput              `json:"slayers,omitempty"`
	Minions      *MinionsOutput              `json:"minions,omitempty"`
	Bestiary     *BestiaryOutput             `json:"bestiary,omitempty"`
	Collections  *CollectionsOutput          `json:"collections,omitempty"`
	CrimsonIsle  *CrimsonIsleOutput          `json:"crimsonIsle,omitempty"`
	Rift         *RiftOutput                 `json:"rift,omitempty"`
	Misc         *MiscOutput                 `json:"misc,omitempty"`
}
