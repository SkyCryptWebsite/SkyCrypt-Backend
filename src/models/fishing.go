package models

type TrophyFishProgress struct {
	Tier   string `json:"tier"`
	Caught int    `json:"caught"`
	Total  int    `json:"total"`
}

type TrophyFish struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Bronze      int    `json:"bronze"`
	Silver      int    `json:"silver"`
	Gold        int    `json:"gold"`
	Diamond     int    `json:"diamond"`
	Texture     string `json:"texture"`
	Maxed       bool   `json:"maxed"`
}

type TrophyFishOutput struct {
	TotalCaught int             `json:"totalCaught"`
	Stage       TrophyFishStage `json:"stage"`
	TrophyFish  []TrophyFish    `json:"trophyFish"`
}

type TrophyFishStage struct {
	Name     string               `json:"name"`
	Progress []TrophyFishProgress `json:"progress"`
}

type FishingOuput struct {
	ItemsFished        int              `json:"itemsFished"`
	Treasure           int              `json:"treasure"`
	TreasureLarge      int              `json:"treasureLarge"`
	SeaCreaturesFished int              `json:"seaCreaturesFished"`
	ShredderFished     int              `json:"shredderFished"`
	ShredderBait       int              `json:"shredderBait"`
	TrophyFish         TrophyFishOutput `json:"trophyFish"`
	Gear               SkillGear        `json:"gear"`
	LavaSeaCreatures   []Kill           `json:"lavaSeaCreatures"`
	WaterSeaCreatures  []Kill           `json:"waterSeaCreatures"`
}
