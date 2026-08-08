package stats

import (
	"regexp"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strconv"
	"strings"
)

var regex = regexp.MustCompile(`^([A-Za-z ]+): ([+-]([0-9]+(?:,[0-9]{3})*(?:\.[0-9]{0,2})?))`)

func GetStatsFromItem(item models.ProcessedItem) ItemStats {
	stats := make(ItemStats)

	if item.Tag == nil || item.Tag.Display.Lore == nil {
		return stats
	}

	lore := make([]string, len(item.Tag.Display.Lore))
	for i, line := range item.Tag.Display.Lore {
		lore[i] = utility.GetRawLore(line)
	}

	for _, line := range lore {
		matches := regex.FindStringSubmatch(line)

		if matches == nil {
			continue
		}

		var statName string

		for _, statInfo := range constants.STATS_DATA {
			if statInfo.NameLore == matches[1] {
				statName = statInfo.ID
				break
			}
		}

		if statName == "" {
			continue
		}

		statValueStr := strings.ReplaceAll(matches[2], ",", "")
		statValue, err := strconv.ParseFloat(statValueStr, 64)
		if err != nil {
			continue
		}

		stats[statName] += statValue
	}

	return stats
}

type ItemStats map[string]float64

func GetStatsFromItems(items []models.ProcessedItem) ItemStats {
	stats := make(ItemStats)

	for _, item := range items {
		if item.Rarity == "" {
			continue
		}

		itemStats := GetStatsFromItem(item)
		for stat, value := range itemStats {
			if _, exists := stats[stat]; !exists {
				stats[stat] = 0
			}
			stats[stat] += value
		}
	}

	return stats
}
