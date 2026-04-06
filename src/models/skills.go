package models

type Skills struct {
	Skills                        map[string]Skill `json:"skills,omitempty"`
	TotalSkillXp                  int              `json:"totalSkillXp,omitempty"`
	AverageSkillLevel             float64          `json:"averageSkillLevel,omitempty"`
	AverageSkillLevelWithProgress float64          `json:"averageSkillLevelWithProgress,omitempty"`
}

type Skill struct {
	XP                          int     `json:"xp"`
	Level                       int     `json:"level"`
	MaxLevel                    int     `json:"maxLevel"`
	XPCurrent                   int     `json:"xpCurrent"`
	XPForNext                   int     `json:"xpForNext"`
	Progress                    float64 `json:"progress"`
	LevelCap                    int     `json:"levelCap"`
	UncappedLevel               int     `json:"uncappedLevel"`
	LevelWithProgress           float64 `json:"levelWithProgress"`
	UnlockableLevelWithProgress float64 `json:"unlockableLevelWithProgress"`
	Maxed                       bool    `json:"maxed"`
	Texture                     string  `json:"texture"`
}

type SkillsOutput struct {
	Mining     MiningOutput          `json:"mining"`
	Foraging   ForagingOutput        `json:"foraging"`
	Farming    FarmingOutput         `json:"farming"`
	Fishing    FishingOuput          `json:"fishing"`
	Enchanting EnchantingOutput      `json:"enchanting"`
	Hunting    AttributeShardsOutput `json:"hunting"`
}
