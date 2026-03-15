package models

type EmbedData struct {
	DisplayName     string            `json:"displayName"`
	Username        string            `json:"username"`
	Rank            RankOutput        `json:"rank"`
	Uuid            string            `json:"uuid"`
	ProfileId       string            `json:"profile_id"`
	ProfileCuteName string            `json:"profile_cute_name"`
	Joined          int64             `json:"joined"`
	GameMode        string            `json:"game_mode"`
	SkyBlockLevel   float64           `json:"skyblock_level"`
	Skills          EmbedDataSkills   `json:"skills"`
	Networth        EmbedNetworth     `json:"networth"`
	Purse           float64           `json:"purse"`
	Bank            float64           `json:"bank"`
	Dungeons        EmbedDataDungeons `json:"dungeons"`
	Slayers         EmbedDataSlayers  `json:"slayers"`
}

type EmbedDataSkills struct {
	SkillAverage float64        `json:"skillAverage"`
	Skills       map[string]int `json:"skills"`
}

type EmbedDataDungeons struct {
	Dungeoneering float64        `json:"dungeoneering"`
	ClassAverage  float64        `json:"classAverage"`
	Classes       map[string]int `json:"classes"`
}

type EmbedDataSlayers struct {
	Experience float64        `json:"xp"`
	Slayers    map[string]int `json:"slayers"`
}

type EmbedNetworth struct {
	Normal      float64 `json:"normal"`
	NonCosmetic float64 `json:"nonCosmetic"`
}
