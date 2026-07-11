package models

type ResolvedLoadout struct {
	ID                       int                `json:"id"`
	Name                     string             `json:"name"`
	Armor                    []StrippedItem     `json:"armor"`
	Equipment                []StrippedItem     `json:"equipment"`
	Accessories              LoadoutAccessories `json:"accessories"`
	MiningCoreSelectedSlot   int                `json:"miningCoreSelectedSlot"`
	ForagingCoreSelectedSlot int                `json:"foragingCoreSelectedSlot"`
	Pet                      *StrippedPet       `json:"pet,omitempty"`
}

type LoadoutAccessories struct {
	PowerStone       string         `json:"powerStone,omitempty"`
	TuningPointsSlot int            `json:"tuningPointsSlot"`
	TuningPoints     map[string]int `json:"tuningPoints"`
}

type LoadoutsOutput []ResolvedLoadout
