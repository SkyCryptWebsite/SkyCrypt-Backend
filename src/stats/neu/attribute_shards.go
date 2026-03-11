package neustats

import (
	"log"
	"skycrypt/src/models"
	neu "skycrypt/src/models/NEU"
	"skycrypt/src/utility"
	"strings"
)

type ItemGetter func(name string) (models.NEUItem, error)

func FormatAttributeShards(shards []neu.AttributeShardRaw, getItem ItemGetter) []neu.FormattedShard {
	result := make([]neu.FormattedShard, 0, len(shards))

	for _, shard := range shards {
		itemData, err := getItem(shard.InternalName)
		if err != nil {
			log.Printf("Error getting item data for %s: %v", shard.InternalName, err)
		}

		formattedShard := neu.FormattedShard{
			Name:         shard.AbilityName,
			ShardName:    shard.DisplayName,
			Lore:         extractLore(itemData.Lore),
			Texture:      resolveTexture(itemData),
			ShardStackId: extractID(shard.InternalName, "ATTRIBUTE_SHARD_", ";"),
			ShardOwnedId: strings.ReplaceAll(shard.BazaarName, "SHARD_", ""),
			Rarity:       shard.Rarity,
		}

		result = append(result, formattedShard)
	}

	return result
}

func extractLore(lines []string) []string {
	var lore []string
	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		lore = append(lore, line)
	}
	return lore
}

func resolveTexture(item models.NEUItem) string {
	if item.NBT.SkullOwner != nil && len(item.NBT.SkullOwner.Properties.Textures) > 0 {
		return "/api/head/" + utility.GetSkinHash(item.NBT.SkullOwner.Properties.Textures[0].Value)
	}

	parts := strings.SplitN(item.NBT.ItemModel, ":", 2)
	if len(parts) == 2 {
		return "/api/item/" + parts[1]
	}

	return "/api/item/BARRIER" // Fallback texture
}

func extractID(name, prefix, sep string) string {
	id := strings.SplitN(name, sep, 2)[0]
	return strings.ToLower(strings.ReplaceAll(id, prefix, ""))
}
