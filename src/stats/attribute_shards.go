package stats

import (
	"fmt"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"slices"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func getMaxSyphon(shardRarity string) int {
	sum := 0
	for _, i := range notenoughupdates.NEUConstants.AttributeShards.Level[shardRarity] {
		sum += i
	}

	return sum
}

func getAbilityLevel(rarity string, amount int) int {
	table := notenoughupdates.NEUConstants.AttributeShards.Level[rarity]
	if len(table) == 0 {
		return 0
	}

	for i, levelReq := range table {
		if amount < levelReq {
			return i
		}
	}

	return len(table)
}

func GetAttributeShards(userProfile *skycrypttypes.Member) models.AttributeShardsOutput {
	output := models.AttributeShardsOutput{
		Shards:      []models.AttributeShard{},
		MaxUnlocked: len(notenoughupdates.NEUConstants.AttributeShards.Shards) - len(notenoughupdates.NEUConstants.AttributeShards.UnconsumableShards),
	}

	for _, shard := range notenoughupdates.NEUConstants.AttributeShards.Shards {
		if slices.Contains(notenoughupdates.NEUConstants.AttributeShards.UnconsumableShards, shard.BazaarName) {
			continue
		}

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
			Name:              shard.Name,
			Texture:           fmt.Sprintf("%s%s", utility.GetDomain(), shard.Texture),
			ShardId:           shard.ShardID,
			Family:            shard.Family,
			Rarity:            shard.Rarity,
			AbilityName:       shard.AbilityName,
			AbilityLevel:      getAbilityLevel(shard.Rarity, syphoned),
			AbilityMaxLevel:   getAbilityLevel(shard.Rarity, getMaxSyphon(shard.Rarity)),
			Lore:              shard.Lore,
			Owned:             owned,
			Syphoned:          syphoned,
			MaxSyphon:         getMaxSyphon(shard.Rarity),
			CapturedTimestamp: captureTimesamp,
		})

		output.MaxSyphoned += getMaxSyphon(shard.Rarity)
		output.Syphoned += syphoned
		if syphoned > 0 {
			output.Unlocked += 1
		}
	}

	return output
}
