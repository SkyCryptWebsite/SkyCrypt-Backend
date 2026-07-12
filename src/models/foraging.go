package models

type ForagingOutput struct {
	Level              Skill               `json:"level"`
	ForagingLevel      Skill               `json:"foragingLevel"`
	CenterOfTheForest  CenterOfTheForest   `json:"cotf"`
	SelectedAxeAbility string              `json:"selectedAxeAbility"`
	Tokens             HotfTokens          `json:"tokens"`
	Gear               SkillGear           `json:"gear"`
	Hotf               []ProcessedItem     `json:"hotf"`
	Whispers           Whispers            `json:"whispers"`
	TreeGift           map[string]TreeGift `json:"treeGift"`
	FishFamily         FishFamily          `json:"fishFamily"`
	HinaChapter        HinaChapter         `json:"hinaChapter"`
}

type CenterOfTheForest struct {
	Level    int `json:"level"`
	MaxLevel int `json:"maxLevel"`
}

type HotfTokens struct {
	Total     int `json:"total"`
	Spent     int `json:"spent"`
	Available int `json:"available"`
}

type Whispers struct {
	Total     int `json:"total"`
	Spent     int `json:"spent"`
	Available int `json:"available"`
}

type FishFamily struct {
	Amount int `json:"collected"`
	Total  int `json:"total"`
}

type HinaChapter struct {
	Tier    int `json:"tier"`
	MaxTier int `json:"maxTier"`
}

type TreeGift struct {
	Milestone    int    `json:"milestone"`
	Texture      string `json:"texture"`
	MaxMilestone int    `json:"maxMilestone"`
}
