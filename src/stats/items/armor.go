package stats

import (
	"regexp"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"slices"
	"strings"
)

func isInvalidItem(a models.ProcessedItem) bool {
	return len(GetId(a)) == 0
}

func GetArmor(armor []models.ProcessedItem) models.ArmorResult {
	// If all armor pieces have no ID, return empty result
	if utility.Every(armor, isInvalidItem) {
		return models.ArmorResult{
			Armor: []models.StrippedItem{},
			Stats: map[string]float64{},
		}
	}

	// One armor piece
	if len(armor) == 1 {
		var armorPiece *models.ProcessedItem
		for i := range armor {
			if armor[i].Rarity != "" {
				armorPiece = &armor[i]
				break
			}
		}

		reversedArmor := make([]models.ProcessedItem, len(armor))
		copy(reversedArmor, armor)
		slices.Reverse(reversedArmor)

		result := models.ArmorResult{
			Armor: StripItems(&reversedArmor),
			Stats: GetStatsFromItems(armor),
		}

		if armorPiece != nil {
			result.SetName = &armorPiece.DisplayName
			result.SetRarity = &armorPiece.Rarity
		}

		return result
	}

	// Full armor set (4 pieces)
	allHaveID := !utility.Every(armor, isInvalidItem)
	if allHaveID && len(armor) == 4 {
		var outputName string
		var reforgeName string

		var armorNames = make([]string, len(armor))
		for i := range armor {
			name := ""
			if armor[i].DisplayName != "" {
				name = armor[i].DisplayName
			}

			name = utility.GetRawLore(name)
			// Removing stars, dye etc..
			name = utility.RemoveNonAscii(name)

			name = strings.TrimSpace(name)

			// Removing modifier (probably should use constants for this, but better than nothing)
			if armor[i].Tag != nil && armor[i].Tag.ExtraAttributes.Modifier != "" {
				parts := strings.Split(name, " ")
				if len(parts) > 1 {
					name = strings.Join(parts[1:], " ")
				}
			}

			armorPieceRegex := regexp.MustCompile(`^Armor .*? (Helmet|Chestplate|Leggings|Boots)$`)
			if armorPieceRegex.MatchString(name) {
				pieceTypeRegex := regexp.MustCompile(`(Helmet|Chestplate|Leggings|Boots)`)
				name = pieceTypeRegex.ReplaceAllString(name, "")
				name = strings.TrimSpace(name)
			} else {
				name = strings.ReplaceAll(name, "Armor", "")
				name = strings.ReplaceAll(name, "  ", " ")
				name = strings.TrimSpace(name)
				pieceTypeRegex := regexp.MustCompile(`(Helmet|Chestplate|Leggings|Boots)`)
				name = pieceTypeRegex.ReplaceAllString(name, "Armor")
				name = strings.TrimSpace(name)
			}

			armorNames[i] = name
		}

		sameModifierCount := 0
		if len(armor) > 0 && armor[0].Tag != nil && armor[0].Tag.ExtraAttributes.Modifier != "" {
			firstModifier := armor[0].Tag.ExtraAttributes.Modifier
			for _, a := range armor {
				if a.Tag != nil && a.Tag.ExtraAttributes.Modifier == firstModifier {
					sameModifierCount++
				}
			}

			if sameModifierCount == 4 {
				name := armor[0].DisplayName
				name = utility.GetRawLore(name)
				name = utility.RemoveNonAscii(name)
				name = strings.TrimSpace(name)
				parts := strings.Split(name, " ")
				if len(parts) > 0 {
					reforgeName = parts[0]
				}
			}
		}

		sameNameCount := 0
		if len(armor) > 0 {
			firstName := armorNames[0]
			for _, a := range armorNames {
				if a == firstName {
					sameNameCount++
				}
			}

			if sameNameCount == 4 {
				outputName = firstName
			}
		}

		for _, set := range constants.SPECIAL_SETS {
			matchCount := 0
			for _, a := range armor {
				id := GetId(a)
				if slices.Contains(set.Pieces, id) {
					matchCount++
				}
			}

			if matchCount == 4 {
				outputName = set.Name
			}
		}

		if reforgeName != "" && outputName != "" {
			outputName = reforgeName + " " + outputName
		}

		maxRarityInt := 0
		for _, a := range armor {
			rarityInt := utility.RarityNameToInt(a.Rarity)
			if rarityInt > maxRarityInt {
				maxRarityInt = rarityInt
			}
		}

		reversedArmor := make([]models.ProcessedItem, len(armor))
		copy(reversedArmor, armor)
		slices.Reverse(reversedArmor)

		result := models.ArmorResult{
			Armor: StripItems(&reversedArmor),
			Stats: GetStatsFromItems(armor),
		}

		if outputName != "" {
			result.SetName = &outputName
		}

		maxRarityName := constants.RARITIES[maxRarityInt]
		result.SetRarity = &maxRarityName

		return result
	}

	reversedArmor := make([]models.ProcessedItem, len(armor))
	copy(reversedArmor, armor)
	slices.Reverse(reversedArmor)

	return models.ArmorResult{
		Armor: StripItems(&reversedArmor),
		Stats: GetStatsFromItems(armor),
	}
}
