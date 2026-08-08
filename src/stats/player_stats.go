package stats

import (
	"fmt"
	"maps"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	statsItems "skycrypt/src/stats/items"
	statsLeveling "skycrypt/src/stats/leveling"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func GetPlayerStats(
	userProfile *skycrypttypes.Member,
	profile *skycrypttypes.Profile,
	profileId string,
	memberUUID string,
) []models.PlayerStat {
	stats := make(map[string]models.StatsInfo, len(constants.PLAYER_STATS))

	for statName, statInfo := range constants.PLAYER_STATS {
		stats[statName] = models.StatsInfo{}
		maps.Copy(stats[statName], statInfo)
	}

	items := getItems(userProfile, profileId, memberUUID)
	processedItems := processItems(items)

	accessoriesStats := GetAccessories(userProfile, processedItems)
	for statName, statValue := range accessoriesStats.Stats {
		if _, exists := stats[statName]; !exists {
			continue
		}

		stats[statName]["accessories"] += int(statValue)
	}

	armorStats := statsItems.GetStatsFromItems(processedItems["armor"])
	for statName, statValue := range armorStats {
		if _, exists := stats[statName]; !exists {
			continue
		}

		stats[statName]["armor"] += int(statValue)
	}

	equipmentStats := statsItems.GetStatsFromItems(processedItems["equipment"])
	for statName, statValue := range equipmentStats {
		if _, exists := stats[statName]; !exists {
			continue
		}

		stats[statName]["equipment"] += int(statValue)
	}

	skyblockLevel := GetSkyBlockLevel(userProfile)
	if skyblockLevel.Level > 0 {
		stats["health"]["skyblock_level"] = int(skyblockLevel.Level * 5)
		stats["strength"]["skyblock_level"] = int(skyblockLevel.Level / 5)
	}

	slayerStats := GetSlayers(userProfile).Stats
	for statName, statValue := range slayerStats {
		if _, exists := stats[statName]; !exists {
			continue
		}

		stats[statName]["slayers"] += int(statValue)
	}

	pets := GetPets(userProfile, &skycrypttypes.Profile{})
	if len(pets.Pets) > 0 {
		activePet := pets.Pets[0]

		if !activePet.Active {
			for i := range pets.Pets {
				if pets.Pets[i].Active {
					activePet = pets.Pets[i]
					break
				}
			}
		}

		for statName, statValue := range activePet.Stats {
			if _, exists := stats[statName]; !exists {
				continue
			}

			stats[statName]["active_pet"] += int(statValue)
		}

		for statName, statValue := range pets.PetScore.Stats {
			if _, exists := stats[statName]; !exists {
				continue
			}

			stats[statName]["pet_score"] += int(statValue)
		}
	}

	skills := GetSkills(
		userProfile,
		&skycrypttypes.Profile{},
		&skycrypttypes.Player{},
	)

	for skillID, skillData := range skills.Skills {
		key := fmt.Sprintf("skill_%s", skillID)

		if constants.STATS_BONUS[key] == nil {
			continue
		}

		skillStats := constants.GetBonusStat(
			skillData.Level,
			key,
			skillData.MaxLevel,
		)

		for statName, value := range skillStats {
			if _, exists := stats[statName]; !exists {
				continue
			}

			stats[statName]["skills"] += int(value)
		}
	}

	catacombs := userProfile.Dungeons.DungeonTypes["catacombs"]

	if catacombs.Experience > 0 {
		dungeoneeringLevel := statsLeveling.GetLevelByXp(
			int(catacombs.Experience),
			&statsLeveling.ExtraSkillData{
				Type: "dungeoneering",
			},
		)

		skillStats := constants.GetBonusStat(
			dungeoneeringLevel.Level,
			"skill_dungeoneering",
			50,
		)

		for statName, value := range skillStats {
			if _, exists := stats[statName]; !exists {
				continue
			}

			stats[statName]["dungeons"] += int(value)
		}
	}

	bestiaryData := GetBestiary(userProfile)
	if bestiaryData.Level > 0 {
		stats["health"]["bestiary"] = int(bestiaryData.Level)
	}

	// Calculate totals while the data is still a map.
	for statName, statInfo := range stats {
		total := 0

		for _, value := range statInfo {
			total += value
		}

		stats[statName]["total"] = total
	}

	output := make([]models.PlayerStat, 0, len(stats))
	for _, statData := range constants.STATS_DATA {
		statInfo, exists := stats[statData.ID]
		if !exists {
			continue
		}

		output = append(output, models.PlayerStat{
			ID:                      statData.ID,
			Name:                    statData.Name,
			NameLore:                statData.NameLore,
			NameShort:               statData.NameShort,
			NameTiny:                statData.NameTiny,
			Symbol:                  statData.Symbol,
			Suffix:                  statData.Suffix,
			Color:                   statData.Color,
			Category:                statData.Category,
			Description:             statData.Description,
			Percent:                 statData.Percent,
			Cap:                     statData.Cap,
			DisabledOnPrivateIsland: statData.DisabledOnPrivateIsland,
			StatsInfo:               statInfo,
		})
	}

	return output
}

func getItems(
	userProfile *skycrypttypes.Member,
	profileId string,
	memberUUID string,
) map[string][]*skycrypttypes.Item {
	items, err := GetItems(userProfile, profileId, memberUUID)
	if err != nil {
		return map[string][]*skycrypttypes.Item{}
	}

	return items
}

func processItems(
	rawItems map[string][]*skycrypttypes.Item,
) map[string][]models.ProcessedItem {
	processedItems := make(map[string][]models.ProcessedItem)

	inventoryKeys := []string{
		"armor",
		"equipment",
	}

	for _, inventoryID := range inventoryKeys {
		inventoryData := rawItems[inventoryID]

		if len(inventoryData) == 0 {
			continue
		}

		processedItems[inventoryID] = statsItems.ProcessItems(
			inventoryData,
			inventoryID,
			[]string{},
		)
	}

	return processedItems
}
