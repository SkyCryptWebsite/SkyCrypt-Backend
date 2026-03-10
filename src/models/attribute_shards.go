package models

type AttributeShardsOutput struct {
	Unlocked    int              `json:"unlocked"`
	MaxUnlocked int              `json:"maxUnlocked"`
	Syphoned    int              `json:"syphoned"`
	MaxSyphoned int              `json:"maxSyphoned"`
	Shards      []AttributeShard `json:"shards"`
}

type AttributeShard struct {
	Name      string   `json:"name"`
	Lore      []string `json:"lore"`
	Texture   string   `json:"texture"`
	Owned     int      `json:"owned"`
	Syphoned  int      `json:"syphoned"`
	MaxSyphon int      `json:"maxSyphon"`
	Captured  int64    `json:"captured"`
}
