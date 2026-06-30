package stats

import (
	"fmt"
	"regexp"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"slices"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

var rarityPattern = func() *regexp.Regexp {
	upperRarities := make([]string, len(constants.RARITIES))
	for i, rarity := range constants.RARITIES {
		if rarity == "very_special" {
			upperRarities[i] = "VERY\\s+SPECIAL"
		} else {
			upperRarities[i] = strings.ToUpper(strings.ReplaceAll(rarity, "_", " "))
		}
	}

	raritiesStr := strings.Join(upperRarities, "|")
	pattern := `^(?:a )?(?:SHINY )?(?:(` + raritiesStr + `) ?)?(?:DUNGEON )?([A-Z ]+)?(?:a)?$`
	return regexp.MustCompile(pattern)
}()

type itemDataType struct {
	Categories []string
	Rarity     string
}

func ParseItemTypeFromLore(lore []string, item skycrypttypes.Item) itemDataType {
	for i := len(lore) - 1; i >= 0; i-- {
		rawLine := lore[i]
		if strings.Contains(rawLine, "§") {
			rawLine = utility.GetRawLore(rawLine)
		}
		match := rarityPattern.FindStringSubmatch(rawLine)

		if len(match) > 0 {
			var rarity = "common"
			if len(match) > 1 && match[1] != "" {
				rarity = strings.ToLower(strings.ReplaceAll(match[1], " ", "_"))
			}

			categories := []string{}
			if len(match) > 2 && match[2] != "" {
				itemType := strings.TrimSpace(strings.ToLower(match[2]))
				categories = getCategories(itemType, item)
			}

			return itemDataType{
				Categories: categories,
				Rarity:     rarity,
			}
		}
	}

	return itemDataType{
		Categories: []string{},
		Rarity:     "common",
	}
}

func getCategories(itemType string, item skycrypttypes.Item) []string {
	categories := []string{}
	if item.Tag == nil || item.Tag.ExtraAttributes == nil {
		return append(categories, constants.TYPE_TO_CATEGORIES[itemType]...)
	}

	enchantments := item.Tag.ExtraAttributes.Enchantments
	for enchantment := range enchantments {
		for category, enchantmentList := range constants.ENCHANTMENTS_TO_CATEGORIES {
			if slices.Contains(enchantmentList, enchantment) {
				categories = append(categories, category)
			}
		}
	}

	return append(categories, constants.TYPE_TO_CATEGORIES[itemType]...)
}

type gemstone struct {
	SlotType   string `json:"slot_type"`
	SlotNumber int    `json:"slot_number"`
	GemType    string `json:"gem_type"`
	GemTier    string `json:"gem_tier"`
	Lore       string `json:"lore"`
}

type gemstoneSlot struct {
	normal  []string
	special []string
	ignore  []string
}

func ParseItemGems(gems map[string]any, rarity string) []gemstone {
	slots := gemstoneSlot{
		normal:  getGemstoneKeys(),
		special: []string{"UNIVERSAL", "COMBAT", "OFFENSIVE", "DEFENSIVE", "MINING", "CHISEL"},
		ignore:  []string{"unlocked_slots"},
	}

	var parsed []gemstone

	for key, value := range gems {
		slotTypeParts := strings.Split(key, "_")
		if len(slotTypeParts) == 0 {
			continue
		}
		slotType := slotTypeParts[0]

		if slices.Contains(slots.ignore, key) || (slices.Contains(slots.special, slotType) && strings.HasSuffix(key, "_gem")) {
			continue
		}

		keyParts := strings.Split(key, "_")
		if len(keyParts) < 2 {
			continue
		}

		slotNumber := 0
		if len(keyParts) >= 2 {
			if num, err := utility.ParseInt(keyParts[1]); err == nil {
				slotNumber = num
			}
		}

		if slices.Contains(slots.special, slotType) {
			gemTypeKey := fmt.Sprintf("%s_gem", key)
			gemType := ""
			if gemTypeValue, exists := gems[gemTypeKey]; exists {
				if gemTypeStr, ok := gemTypeValue.(string); ok {
					gemType = gemTypeStr
				}
			}

			gemTier := extractGemTier(value)

			parsed = append(parsed, gemstone{
				SlotType:   slotType,
				SlotNumber: slotNumber,
				GemType:    gemType,
				GemTier:    gemTier,
				Lore:       "",
			})
		} else if slices.Contains(slots.normal, slotType) {
			gemTier := extractGemTier(value)

			parsed = append(parsed, gemstone{
				SlotType:   slotType,
				SlotNumber: slotNumber,
				GemType:    keyParts[0],
				GemTier:    gemTier,
				Lore:       "",
			})
		}
	}

	for i := range parsed {
		parsed[i].Lore = generateGemLore(parsed[i].GemType, parsed[i].GemTier, rarity)
	}

	return parsed
}

func getGemstoneKeys() []string {
	keys := make([]string, 0, len(constants.GEMSTONES))
	for key := range constants.GEMSTONES {
		keys = append(keys, key)
	}
	return keys
}

func extractGemTier(value any) string {
	if valueMap, ok := value.(map[string]any); ok {
		if quality, exists := valueMap["quality"]; exists {
			if qualityStr, ok := quality.(string); ok {
				return qualityStr
			}
		}
	}

	if valueStr, ok := value.(string); ok {
		return valueStr
	}

	return ""
}

func generateGemLore(gemType, tier, rarity string) string {
	var lore []string
	var stats []string

	gemstoneData, exists := constants.GEMSTONES[strings.ToUpper(gemType)]
	if !exists {
		return "§c§oMISSING GEMSTONE DATA§r"
	}

	color := "§" + gemstoneData.Color
	if rarity != "" {
		gemstoneStats, statsExist := gemstoneData.Stats[strings.ToUpper(tier)]
		if statsExist {
			for stat, values := range gemstoneStats {
				rarityIndex := utility.RarityNameToInt(strings.ToUpper(rarity))
				var statValue any

				if rarityIndex < len(values) {
					statValue = values[rarityIndex]
				}

				// Fallback since skyblock devs didn't code all gemstone stats for divine rarity yet
				// ...they didn't expect people to own divine tier items other than divan's drill
				if strings.ToUpper(rarity) == "DIVINE" && statValue == nil {
					mythicIndex := utility.RarityNameToInt("MYTHIC")
					if mythicIndex < len(values) {
						statValue = values[mythicIndex]
					}
				}

				if statValue != nil {
					statsData, statExists := constants.STATS_DATA[stat]
					if statExists {
						colorChar := ""
						if len(statsData.Color) > 0 {
							colorChar = string(statsData.Color[len(statsData.Color)-1])
						}

						statStr := fmt.Sprintf("§%s+%v %s", colorChar, statValue, statsData.Symbol)
						stats = append(stats, statStr)
					} else {
						stats = append(stats, "§c§oMISSING VALUE§r")
					}
				} else {
					stats = append(stats, "§c§oMISSING VALUE§r")
				}
			}
		}
	}

	lore = append(lore, color, utility.TitleCase(tier), " ", utility.TitleCase(gemType))

	if len(stats) > 0 {
		lore = append(lore, "§7 (", strings.Join(stats, "§7, "), "§7)")
	}

	return strings.Join(lore, "")
}

func AddLevelableEnchantmentsToLore(amount int, constant constants.EnchantmentLadder, itemLore *[]string) {
	*itemLore = append(*itemLore, "", fmt.Sprintf("§7%s: §c%s", constant.Name, utility.FormatNumber(amount)))

	maxValue := 100
	if len(constant.Ladder) > 0 {
		maxValue = constant.Ladder[len(constant.Ladder)-1]
	}

	if amount >= maxValue || amount < 0 {
		*itemLore = append(*itemLore, "§8MAXED OUT!")
	} else {
		toNextLevel := 0
		for _, e := range constant.Ladder {
			if amount < e {
				toNextLevel = e - amount
				break
			}
		}
		*itemLore = append(*itemLore, fmt.Sprintf("§8%s to tier up!", utility.FormatNumber(toNextLevel)))
	}

}

func GetId(item models.ProcessedItem) string {
	if item.Tag == nil || item.Tag.ExtraAttributes == nil || item.Tag.ExtraAttributes.Id == "" {
		return ""
	}

	return item.Tag.ExtraAttributes.Id
}
