package neu

type FormattedShard struct {
	Texture      string   `json:"texture"`
	Name         string   `json:"name"`
	AbilityName  string   `json:"abilityName"`
	Lore         []string `json:"lore"`
	Rarity       string   `json:"rarity"`
	Family       []string `json:"family"`
	ShardID      string   `json:"shardId"`
	ShardStackId string   `json:"shardStackId"`
	ShardOwnedId string   `json:"shardOwnedId"`
	BazaarName   string   `json:"bazaarName"`
}

type AttributeShardRaw struct {
	BazaarName   string   `json:"bazaarName"`
	DisplayName  string   `json:"displayName"`
	Rarity       string   `json:"rarity"`
	InternalName string   `json:"internalName"`
	AbilityName  string   `json:"abilityName"`
	Alignment    string   `json:"alignment"`
	Family       []string `json:"family"`
	ShardId      string   `json:"shardId"`
}

type AttributeShardsRaw struct {
	Attributes         []AttributeShardRaw `json:"attributes"`
	Level              map[string][]int    `json:"attribute_levelling"`
	UnconsumableShards []string            `json:"unconsumable_attributes"`
}

type FormattedShards struct {
	Shards             []FormattedShard `json:"shards"`
	Level              map[string][]int `json:"attribute_levelling"`
	UnconsumableShards []string         `json:"unconsumable_attributes"`
}
