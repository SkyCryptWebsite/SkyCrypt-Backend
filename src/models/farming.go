package models

type FarmingOutput struct {
	UniqueGolds      int                 `json:"uniqueGolds"`
	Pelts            int                 `json:"pelts"`
	Copper           int                 `json:"copper"`
	Medals           map[string]*Medal   `json:"medals"`
	Contests         map[string]*Contest `json:"contests"`
	ContestsAttended int                 `json:"contestsAttended"`
	Gear             SkillGear           `json:"gear"`
}

type Medal struct {
	Amount int `json:"amount"`
	Total  int `json:"total"`
}

type Contest struct {
	Name      string         `json:"name"`
	Texture   string         `json:"texture"`
	Collected int            `json:"collected"`
	Amount    int            `json:"amount"`
	Medals    map[string]int `json:"medals"`
	Maxed     bool           `json:"maxed"`
}
