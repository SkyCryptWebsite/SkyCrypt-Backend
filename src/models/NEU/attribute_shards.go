package neu

type FormattedShard struct {
	Name         string   `json:"name"`
	ShardName    string   `json:"shardName"`
	Lore         []string `json:"lore"`
	Texture      string   `json:"texture"`
	ShardStackId string   `json:"shardStackId"`
	ShardOwnedId string   `json:"shardOwnedId"`
	Rarity       string   `json:"rarity"`
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
	Attributes []AttributeShardRaw `json:"attributes"`
}
