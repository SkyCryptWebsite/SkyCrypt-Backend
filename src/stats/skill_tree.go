package stats

import (
	"fmt"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/models"
	neu "skycrypt/src/models/NEU"
	"skycrypt/src/utility"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

var powderColors = map[string]string{
	"MITHRIL":         "§2",
	"GEMSTONE":        "§d",
	"GLACITE":         "§b",
	"FOREST_WHISPERS": "§b",
}

func lispValueToString(v interface{}) string {
	switch val := v.(type) {
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%g", val)
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func parseSkillTreeItemLore(LispParser *notenoughupdates.LispParser, perk neu.SkillTreePerk, perkLevel int, skillTreeLevel int) []string {
	output := []string{}

	replacer := strings.NewReplacer(
		"level", fmt.Sprintf("%d", perkLevel),
		"hotm", fmt.Sprintf("%d", skillTreeLevel),
	)

	stats := make(map[string]string)
	for key, expr := range perk.Keys {
		substituted := replacer.Replace(expr)
		substituted = strings.ReplaceAll(substituted, "\"", "")
		result, err := LispParser.Parse(substituted)
		if err != nil {
			stats[key] = expr
			continue
		}
		stats[key] = lispValueToString(result)
	}

	for _, line := range perk.Lore {
		if line.OnlyIf != "" {
			var matchedLine *neu.SkillTreeLoreEntry
			for i := range perk.Lore {
				l := &perk.Lore[i]
				if l.OnlyIf == "" {
					continue
				}
				condition := strings.NewReplacer("level", fmt.Sprintf("%d", perkLevel)).Replace(l.OnlyIf)
				result, err := LispParser.Parse(condition)
				if err != nil {
					continue
				}
				if b, ok := result.(bool); ok && b {
					matchedLine = l
				}
			}

			if matchedLine == nil {
				continue
			}

			text := matchedLine.Text
			for k, v := range stats {
				text = strings.ReplaceAll(text, "{"+k+"}", v)
			}
			output = append(output, text)
		}

		text := line.Text
		for k, v := range stats {
			text = strings.ReplaceAll(text, "{"+k+"}", v)
		}
		output = append(output, text)
	}

	// Add cost line
	if perkLevel > 0 && perkLevel != perk.MaxLevel && perk.Cost != "" {
		costExpr := replacer.Replace(perk.Cost)
		costExpr = strings.ReplaceAll(costExpr, "\"", "")
		costResult, err := LispParser.Parse(costExpr)
		if err == nil {
			costStr := lispValueToString(costResult)
			var n int
			if _, parseErr := fmt.Sscanf(costStr, "%d", &n); parseErr == nil {
				costStr = utility.AddCommas(n)
			}
			color := powderColors[perk.Powder]
			output = append(output, "")
			output = append(output, fmt.Sprintf("§7Cost: %s%s %s Powder", color, costStr, utility.TitleCase(perk.Powder)))
		}
	}

	// Add level header
	if perkLevel > 0 && perkLevel != perk.MaxLevel {
		output = append([]string{"", fmt.Sprintf("§7Level %d / §8%d", perkLevel, perk.MaxLevel)}, output...)
	} else if perkLevel > 0 {
		output = append([]string{"", fmt.Sprintf("§7Level %d ", perkLevel)}, output...)
	}

	// Add name
	if perkLevel > 0 {
		output = append([]string{fmt.Sprintf("§e%s", perk.Name)}, output...)
	} else {
		output = append([]string{fmt.Sprintf("§c%s", perk.Name)}, output...)
	}

	return output
}

func getSkillTree(userProfile *skycrypttypes.Member, SKILL_TREE_CONSTANTS map[string]neu.SkillTreePerk, prelude []string, skillName string, skillLevel models.Skill) []models.ProcessedItem {
	output := []models.ProcessedItem{}

	LispParser := notenoughupdates.NewLispParser(prelude)

	skillTreePerks := userProfile.SkillTree.Nodes[skillName].Levels
	for y := range skillLevel.MaxLevel {
		for x := range 9 {
			if x == 0 || x == 8 {
				output = append(output, models.ProcessedItem{})
				continue
			}

			perkId := ""
			for pId, perk := range SKILL_TREE_CONSTANTS {
				if perk.X == x-1 && perk.Y == y {
					perkId = pId
					break
				}
			}

			if perkId == "" {
				output = append(output, models.ProcessedItem{})
				continue
			}

			perkData := SKILL_TREE_CONSTANTS[perkId]
			itemModelString := strings.NewReplacer(
				"level0", fmt.Sprintf("%d", skillTreePerks[perkId]),
				"maxLevel", fmt.Sprintf("%d", SKILL_TREE_CONSTANTS[perkId].MaxLevel),
			).Replace(perkData.Item)

			itemModelTexture, err := LispParser.Parse(itemModelString)
			if err != nil {
				fmt.Printf("Error parsing item model for perk %s: %v\n", perkId, err)
				continue
			}

			output = append(output, models.ProcessedItem{
				DisplayName: perkData.Name,
				Texture:     fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), itemModelTexture),
				Lore:        parseSkillTreeItemLore(LispParser, perkData, skillTreePerks[perkId], skillLevel.Level),
			})
		}
	}

	return output
}
