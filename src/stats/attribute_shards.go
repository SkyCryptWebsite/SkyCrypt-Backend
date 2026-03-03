package stats

import (
	"fmt"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/models"
	"skycrypt/src/utility"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func getMaxSyphon(shardRarity string) int {
	switch shardRarity {
	case "COMMON":
		return 96
	case "UNCOMMON":
		return 64
	case "RARE":
		return 48
	case "EPIC":
		return 32
	case "LEGENDARY":
		return 24
	}

	return 0
}

func GetAttributeShards(userProfile *skycrypttypes.Member) models.AttributeShardsOutput {
	output := models.AttributeShardsOutput{
		Shards:      []models.AttributeShard{},
		MaxUnlocked: len(notenoughupdates.NEUConstants.AttributeShards),
	}

	for _, shard := range notenoughupdates.NEUConstants.AttributeShards {

		owned := 0
		captureTimesamp := int64(0)
		syphoned := userProfile.Attributes.Stacks[shard.ShardStackId]
		for _, ownedShard := range userProfile.Shards.Owned {
			if ownedShard.Type == shard.ShardOwnedId {
				owned = ownedShard.AmountOwned
				captureTimesamp = ownedShard.Captured
				break
			}
		}

		output.Shards = append(output.Shards, models.AttributeShard{
			Name:      fmt.Sprintf("%s (%s Shard)", shard.Name, shard.ShardName),
			Lore:      shard.Lore,
			Texture:   fmt.Sprintf("%s%s", utility.GetDomain(), shard.Texture),
			Owned:     owned,
			Syphoned:  syphoned,
			MaxSyphon: getMaxSyphon(shard.Rarity),
			Captured:  captureTimesamp,
		})

		output.MaxSyphoned += getMaxSyphon(shard.Rarity)
		output.Syphoned += syphoned
		output.Unlocked += 1

	}

	return output
}
