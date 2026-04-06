package stats

import (
	"fmt"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func getSlayerKills(slayerData skycrypttypes.SlayerBoss) map[string]int {
	tiers := []int{
		slayerData.BossKillsTier0,
		slayerData.BossKillsTier1,
		slayerData.BossKillsTier2,
		slayerData.BossKillsTier3,
		slayerData.BossKillsTier4,
	}

	total := 0
	kills := make(map[string]int)
	for i, count := range tiers {
		tier := i + 1
		kills[fmt.Sprintf("%d", tier)] = count
		total += count
	}

	kills["total"] = total

	return kills
}

func getSlayerLevel(experience int, slayerId string) models.SlayerLevel {
	if constants.SLAYER_INFO[slayerId].Levelling == nil {
		return models.SlayerLevel{
			Experience:        0,
			ExperienceForNext: 0,
			Level:             0,
			MaxLevel:          0,
			Maxed:             false,
		}
	}

	level := 0
	maxLevel := len(constants.SLAYER_INFO[slayerId].Levelling)
	for i := 1; i <= maxLevel; i++ {
		if experience < constants.SLAYER_INFO[slayerId].Levelling[i] {
			break
		}

		level = i
	}

	experienceForNext := 0
	if level < maxLevel {
		experienceForNext = constants.SLAYER_INFO[slayerId].Levelling[level+1]
	} else {
		experienceForNext = 0
	}

	return models.SlayerLevel{
		Experience:        experience,
		ExperienceForNext: experienceForNext,
		Level:             level,
		MaxLevel:          maxLevel,
		Maxed:             level == maxLevel,
	}
}

func GetSlayers(userProfile *skycrypttypes.Member) *models.SlayersOutput {
	output := &models.SlayersOutput{
		Data:  make(map[string]models.SlayerData),
		Stats: make(map[string]float64),
	}

	totalExperience := 0
	for slayerId, slayerData := range userProfile.Slayer.SlayerBosses {
		texture := fmt.Sprintf("%s%s", utility.GetDomain(), constants.SLAYER_INFO[slayerId].Head)

		output.Data[slayerId] = models.SlayerData{
			Name:    constants.SLAYER_INFO[slayerId].Name,
			Texture: texture,
			Kills:   getSlayerKills(slayerData),
			Level:   getSlayerLevel(int(slayerData.Experience), slayerId),
		}

		totalExperience += int(slayerData.Experience)

		statsBonus := constants.STATS_BONUS[fmt.Sprintf("slayer_%s", slayerId)]
		if statsBonus == nil {
			continue
		}

		stats := constants.GetBonusStats(output.Data[slayerId].Level.Level, statsBonus)
		for stat, value := range stats {
			if _, exists := output.Stats[stat]; !exists {
				output.Stats[stat] = 0
			}

			output.Stats[stat] += value
		}
	}

	output.TotalSlayerExperience = totalExperience

	return output
}
