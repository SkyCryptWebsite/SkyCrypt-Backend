package models

type AttributeShardsOutput struct {
	Unlocked    int              `json:"unlocked"`
	MaxUnlocked int              `json:"maxUnlocked"`
	Syphoned    int              `json:"syphoned"`
	MaxSyphoned int              `json:"maxSyphoned"`
	Shards      []AttributeShard `json:"shards"`
}

type AttributeShard struct {
	Texture string `json:"texture"`
	Name    string `json:"name"`

	ShardId string   `json:"shardId"`
	Family  []string `json:"family"`
	Rarity  string   `json:"rarity"`

	AbilityName     string `json:"abilityName"`
	AbilityLevel    int    `json:"abilityLevel"`
	AbilityMaxLevel int    `json:"abilityMaxLevel"`

	Lore []string `json:"lore"`

	Owned             int   `json:"owned"`
	Syphoned          int   `json:"syphoned"`
	MaxSyphon         int   `json:"maxSyphon"`
	CapturedTimestamp int64 `json:"capturedTimestamp"`
}
