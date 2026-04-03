package stats

import (
	"fmt"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"slices"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func getMinionSlots(profile *skycrypttypes.Profile, tiers int) *models.MinionSlotsOutput {
	keys := make([]int, 0, len(constants.MINION_SLOTS))
	for k := range constants.MINION_SLOTS {
		keys = append(keys, k)
	}
	utility.SortInts(keys)

	highestKey := keys[0]
	nextTier := 0
	for _, key := range keys {
		if tiers >= key {
			highestKey = key
			nextTier = keys[slices.Index(keys, key)+1]
		} else {
			break
		}
	}

	if profile.CommunityUpgrades == nil {
		profile.CommunityUpgrades = &skycrypttypes.CommunityUpgrades{}
	}

	bonusSlots := 0
	for _, upgrade := range profile.CommunityUpgrades.UpgradeStates {
		if upgrade.Upgrade == "minion_slots" {
			bonusSlots++
		}
	}

	return &models.MinionSlotsOutput{
		BonusSlots: bonusSlots,
		Current:    constants.MINION_SLOTS[highestKey],
		Next:       nextTier - tiers,
	}
}

func getCraftedMinions(profile *skycrypttypes.Profile) map[string][]int {
	craftedMinions := make(map[string][]int)
	for _, member := range profile.Members {
		if member.PlayerData == nil {
			return craftedMinions
		}

		for _, minion := range member.PlayerData.Minions {
			parts := strings.Split(minion, "_")
			tierStr := parts[len(parts)-1]
			minionType := strings.Join(parts[:len(parts)-1], "_")

			tier, err := utility.ParseInt(tierStr)
			if err != nil {
				continue
			}

			if slices.Contains(craftedMinions[minionType], tier) {
				continue
			}

			craftedMinions[minionType] = append(craftedMinions[minionType], tier)
		}
	}

	for minionType := range craftedMinions {
		utility.SortInts(craftedMinions[minionType])
	}

	return craftedMinions
}

func GetMinions(profile *skycrypttypes.Profile) *models.MinionsOutput {
	craftedMinions := getCraftedMinions(profile)

	output := &models.MinionsOutput{
		Minions: make(map[string]models.MinionCategory),
	}

	for categoryId, categoryData := range constants.MINIONS {
		category := models.MinionCategory{
			Minions:      []models.Minion{},
			Texture:      fmt.Sprintf("%s%s", utility.GetDomain(), constants.MINION_CATEGORY_ICONS[categoryId]),
			TotalMinions: 0,
			MaxedMinions: 0,
			TotalTiers:   0,
			MaxedTiers:   0,
		}

		totalTiers := 0
		for minionId, minionData := range categoryData {
			name := minionData.Name
			if name == "" {
				name = utility.TitleCase(minionId)
			}

			maxTier := minionData.MaxTier
			if maxTier == 0 {
				maxTier = 11 // Default max tier if not specified
			}

			craftedTiers := craftedMinions[minionId]
			if craftedTiers == nil {
				craftedTiers = []int{}
			}

			minionData := models.Minion{
				Name:    name,
				Texture: fmt.Sprintf("%s%s", utility.GetDomain(), minionData.Texture),
				MaxTier: maxTier,
				Tiers:   craftedTiers,
			}

			category.Minions = append(category.Minions, minionData)

			totalTiers += maxTier
			category.MaxedTiers += len(craftedMinions[minionId])
			if craftedMinions[minionId] != nil && craftedMinions[minionId][len(craftedMinions[minionId])-1] == maxTier {
				category.MaxedMinions++
			}
		}

		category.TotalMinions = len(category.Minions)
		category.TotalTiers = totalTiers

		output.Minions[categoryId] = category

		output.TotalMinions += category.TotalMinions
		output.MaxedMinions += category.MaxedMinions
		output.TotalTiers += category.TotalTiers
		output.MaxedTiers += category.MaxedTiers
	}

	output.MinionSlots = getMinionSlots(profile, output.MaxedTiers)

	return output
}
