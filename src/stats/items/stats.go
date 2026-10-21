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

// statIDByLoreName maps a stat's lore name to its ID, keeping the first match as the old linear scan did.
var statIDByLoreName = func() map[string]string {
	output := make(map[string]string, len(constants.STATS_DATA))
	for _, statInfo := range constants.STATS_DATA {
		if _, exists := output[statInfo.NameLore]; !exists {
			output[statInfo.NameLore] = statInfo.ID
		}
	}
	return output
}()

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
		// Every stat line has ": "; most lore lines don't, so skip the regex for them.
		if !strings.Contains(line, ": ") {
			continue
		}
		matches := regex.FindStringSubmatch(line)

		if matches == nil {
			continue
		}

		statName := statIDByLoreName[matches[1]]
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
