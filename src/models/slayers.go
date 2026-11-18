package models

type SlayersOutput struct {
	Data                  map[string]SlayerData `json:"data"`
	TotalSlayerExperience int                   `json:"totalSlayerExp"`
	Stats                 map[string]float64    `json:"stats"`
}

type SlayerData struct {
	Name    string         `json:"name"`
	Texture string         `json:"texture"`
	Kills   map[string]int `json:"kills"`
	Level   SlayerLevel    `json:"level"`
	// MagicFind only applies against this slayer's bosses, so it is not part of the player's Magic Find stat.
	MagicFind float64 `json:"magicFind"`
}

type SlayerLevel struct {
	Experience        int  `json:"xp"`
	ExperienceForNext int  `json:"xpForNext"`
	Level             int  `json:"level"`
	MaxLevel          int  `json:"maxLevel"`
	Maxed             bool `json:"maxed"`
}
