package stats

import (
	"fmt"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	statsItems "skycrypt/src/stats/items"
	"skycrypt/src/utility"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func getTrophyFishProgress(userProfile *skycrypttypes.Member) []models.TrophyFishProgress {
	if len(userProfile.TrophyFish.Rewards) == 0 {
		return nil
	}
	output := []models.TrophyFishProgress{}
	for _, tier := range constants.TROPHY_FISH_TIERS {
		count := 0
		for key := range userProfile.TrophyFish.Extra {
			if len(key) > len(tier)+1 && key[len(key)-len(tier):] == tier {
				count++
			}
		}
		output = append(output, models.TrophyFishProgress{
			Tier:   tier,
			Caught: count,
			Total:  18,
		})
	}
	return output
}

func getTrophyFish(userProfile *skycrypttypes.Member) models.TrophyFishOutput {
	output := []models.TrophyFish{}
	for id, data := range constants.TROPHY_FISH {
		tf := models.TrophyFish{
			Id:          id,
			Name:        data.DisplayName,
			Description: data.Description,
		}

		for tierId := range data.Textures {
			key := strings.ToLower(id) + "_" + tierId
			count := 0
			if val, ok := userProfile.TrophyFish.Extra[key]; ok {
				count = val
			}

			switch tierId {
			case "bronze":
				tf.Bronze = count
			case "silver":
				tf.Silver = count
			case "gold":
				tf.Gold = count
			case "diamond":
				tf.Diamond = count
			}
		}

		highestTier := "bronze"
		for _, tier := range constants.TROPHY_FISH_TIERS {
			key := strings.ToLower(id) + "_" + tier
			if val, ok := userProfile.TrophyFish.Extra[key]; ok && val > 0 {
				highestTier = tier
			}
		}

		tf.Texture = fmt.Sprintf("%s%s", utility.GetDomain(), data.Textures[highestTier])

		tf.Maxed = highestTier == constants.TROPHY_FISH_TIERS[len(constants.TROPHY_FISH_TIERS)-1]
		output = append(output, tf)
	}

	totalCaught := userProfile.TrophyFish.TotalCaught
	stageName := "Bronze Hunter"
	stageIdx := min(len(userProfile.TrophyFish.Rewards), len(constants.TROPHY_FISH_STAGES))
	if stageIdx > 0 && stageIdx <= len(constants.TROPHY_FISH_STAGES) {
		stageName = constants.TROPHY_FISH_STAGES[stageIdx-1]
	}

	return models.TrophyFishOutput{
		TotalCaught: totalCaught,
		Stage: models.TrophyFishStage{
			Name:     stageName,
			Progress: getTrophyFishProgress(userProfile),
		},
		TrophyFish: output,
	}
}

func GetFishing(userProfile *skycrypttypes.Member, items []models.ProcessedItem) models.FishingOuput {
	output := models.FishingOuput{
		ItemsFished:        int(userProfile.PlayerStats.ItemsFished.Total),
		Treasure:           int(userProfile.PlayerStats.ItemsFished.Treasure),
		TreasureLarge:      int(userProfile.PlayerStats.ItemsFished.LargeTreasure),
		SeaCreaturesFished: int(userProfile.PlayerStats.Pets.Milestone.SeaCreaturesKilled),
		ShredderFished:     int(userProfile.PlayerStats.ShredderRod.Fished),
		ShredderBait:       int(userProfile.PlayerStats.ShredderRod.Bait),
		Tools:              statsItems.GetSkillTools("fishing", items),
		TrophyFish:         getTrophyFish(userProfile),
		WaterSeaCreatures:  []models.Kill{},
		LavaSeaCreatures:   []models.Kill{},
	}

	for _, id := range constants.WATER_SEA_CREATURES {
		if count, exists := userProfile.PlayerStats.Kills[id]; exists {
			name := constants.MOB_NAMES[id]
			if name == "" {
				name = utility.TitleCase(id)
			}

			output.WaterSeaCreatures = append(output.WaterSeaCreatures, models.Kill{
				Id:      id,
				Name:    name,
				Texture: fmt.Sprintf("%s/assets/static/sea_creatures/%s.webp", utility.GetDomain(), id),
				Amount:  int(count),
			})
		}
	}

	for _, id := range constants.LAVA_SEA_CREATURES {
		if count, exists := userProfile.PlayerStats.Kills[id]; exists {
			name := constants.MOB_NAMES[id]
			if name == "" {
				name = utility.TitleCase(id)
			}

			output.LavaSeaCreatures = append(output.LavaSeaCreatures, models.Kill{
				Id:      id,
				Name:    name,
				Texture: fmt.Sprintf("%s/assets/static/sea_creatures/%s.webp", utility.GetDomain(), id),
				Amount:  int(count),
			})
		}
	}

	return output
}
