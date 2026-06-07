package models

type ProcessedPet struct {
	Type      string             `json:"type,omitempty"`
	Name      string             `json:"display_name,omitempty"`
	Rarity    string             `json:"rarity,omitempty"`
	Active    bool               `json:"active,omitempty"`
	Price     float64            `json:"price,omitempty"`
	Level     PetLevel           `json:"level"`
	Texture   string             `json:"texture_path,omitempty"`
	Lore      []string           `json:"lore,omitempty"`
	Stats     map[string]float64 `json:"stats,omitempty"`
	CandyUsed int                `json:"candyUsed,omitempty"`
	Skin      string             `json:"skin,omitempty"`
}

type PetLevel struct {
	Experience            int     `json:"xp,omitempty"`
	Level                 int     `json:"level,omitempty"`
	MaxLevel              int     `json:"maxLevel,omitempty"`
	CurrentExperience     int     `json:"currentXp,omitempty"`
	ExperienceForNext     int     `json:"xpForNext,omitempty"`
	Progress              float64 `json:"progress,omitempty"`
	ExperienceForMaxLevel int     `json:"xpMaxLevel,omitempty"`
}

type OutputPets struct {
	Pets               []StrippedPet `json:"pets"`
	MissingPets        []StrippedPet `json:"missing"`
	Amount             int           `json:"amount"`
	Total              int           `json:"total"`
	AmountSkins        int           `json:"amountSkins"`
	TotalPetExperience int           `json:"totalPetExp"`
	TotalCandyUsed     int           `json:"totalCandyUsed"`
	PetScore           PetScore      `json:"petScore,omitempty"`
}

type PetScore struct {
	Amount int                `json:"amount"`
	Stats  map[string]float64 `json:"stats"`
	Reward []PetScoreReward   `json:"reward"`
}

type PetScoreReward struct {
	Score    int  `json:"score"`
	Bonus    int  `json:"bonus"`
	Unlocked bool `json:"unlocked,omitempty"`
}

type StrippedPet struct {
	Active      bool               `json:"active,omitempty"`
	Type        string             `json:"type,omitempty"`
	Rarity      string             `json:"rarity,omitempty"`
	Level       int                `json:"level"`
	MaxLevel    int                `json:"maxLevel,omitempty"`
	DisplayName string             `json:"display_name"`
	Texture     string             `json:"texture_path,omitempty"`
	Lore        []string           `json:"lore,omitempty"`
	Stats       map[string]float64 `json:"stats,omitempty"`
}
