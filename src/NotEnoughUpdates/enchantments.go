package notenoughupdates

import (
	"fmt"
	"os"
	"regexp"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"sort"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

var (
	// enchantBookIDRegex matches enchanted book IDs, e.g. "SHARPNESS;7".
	enchantBookIDRegex = regexp.MustCompile(`^([A-Z0-9_]+);(\d+)$`)
	// enchantTierUpRegex matches tier-up lines of enchants levelled by use: "Kill 500 Sea Creatures for Tier V".
	enchantTierUpRegex = regexp.MustCompile(`for Tier ([IVXLC]+)\b`)
	// enchantSourceRegex matches source ranges: "Silex (VI-X)", "Bazaar (I-III)", "Vibrant Coral (V)".
	enchantSourceRegex = regexp.MustCompile(`\(([IVXLC]+)(?:-([IVXLC]+))?\)`)
	// enchantLevelSuffixRegex strips the level from a book's lore name. Some NEU books use digits ("Scuba 2").
	enchantLevelSuffixRegex = regexp.MustCompile(`\s+([IVXLC]+|\d+)$`)
)

type maxEnchant struct {
	name      string
	nameLevel int
	level     int
}

// parseMaxEnchants builds the lore strings of every enchantment at its maximum level ("Sharpness VII").
// The level is the highest of: the enchanted books NEU has, their parents.json chain, and the levels their lore
// names (tier-up lines and source ranges), since enchants levelled by use (Expertise, Champion) only ship book I.
// Ultimate enchants are left out: their lore colour already marks them.
func parseMaxEnchants(parents map[string][]string) ([]string, error) {
	entries, err := os.ReadDir("NotEnoughUpdates-REPO/items")
	if err != nil {
		return nil, fmt.Errorf("failed to list NEU items: %w", err)
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	enchants := map[string]*maxEnchant{}
	for _, entry := range entries {
		id, ok := strings.CutSuffix(entry.Name(), ".json")
		if !ok || entry.IsDir() {
			continue
		}
		match := enchantBookIDRegex.FindStringSubmatch(id)
		if match == nil || strings.HasPrefix(match[1], "ULTIMATE_") {
			continue
		}

		data, err := os.ReadFile("NotEnoughUpdates-REPO/items/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read item file %s: %w", entry.Name(), err)
		}
		var item models.RawNEUItem
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON from %s: %w", entry.Name(), err)
		}
		if item.MinecraftId != "minecraft:enchanted_book" || len(item.Lore) < 3 {
			continue
		}

		level, _ := strconv.Atoi(match[2])
		enchant := enchants[match[1]]
		if enchant == nil {
			enchant = &maxEnchant{}
			enchants[match[1]] = enchant
		}
		enchant.level = max(enchant.level, level)

		// Take the name from the lowest book: higher NEU books sometimes carry a malformed name.
		if name := enchantLevelSuffixRegex.ReplaceAllString(strings.TrimSpace(utility.GetRawLore(item.Lore[2])), ""); name != "" && (enchant.name == "" || level < enchant.nameLevel) {
			enchant.name = name
			enchant.nameLevel = level
		}

		for _, line := range item.Lore {
			line = utility.GetRawLore(line)
			for _, tier := range enchantTierUpRegex.FindAllStringSubmatch(line, -1) {
				enchant.level = max(enchant.level, romanToInt(tier[1]))
			}
			for _, source := range enchantSourceRegex.FindAllStringSubmatch(line, -1) {
				highest := source[2]
				if highest == "" {
					highest = source[1]
				}
				enchant.level = max(enchant.level, romanToInt(highest))
			}
		}
	}

	for parent, children := range parents {
		match := enchantBookIDRegex.FindStringSubmatch(parent)
		if match == nil {
			continue
		}
		enchant := enchants[match[1]]
		if enchant == nil {
			continue
		}
		for _, child := range children {
			childMatch := enchantBookIDRegex.FindStringSubmatch(child)
			if childMatch == nil || childMatch[1] != match[1] {
				continue
			}
			level, _ := strconv.Atoi(childMatch[2])
			enchant.level = max(enchant.level, level)
		}
	}

	output := make([]string, 0, len(enchants))
	for _, enchant := range enchants {
		if enchant.name == "" {
			continue
		}
		output = append(output, enchant.name+" "+intToRoman(enchant.level))
	}
	sort.Strings(output)

	return output, nil
}

var romanValues = map[byte]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100}

func romanToInt(roman string) int {
	total := 0
	for i := 0; i < len(roman); i++ {
		value := romanValues[roman[i]]
		if i+1 < len(roman) && romanValues[roman[i+1]] > value {
			total -= value
		} else {
			total += value
		}
	}
	return total
}

var romanNumerals = []struct {
	value  int
	symbol string
}{
	{100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
}

func intToRoman(number int) string {
	var output strings.Builder
	for _, numeral := range romanNumerals {
		for number >= numeral.value {
			output.WriteString(numeral.symbol)
			number -= numeral.value
		}
	}
	return output.String()
}
