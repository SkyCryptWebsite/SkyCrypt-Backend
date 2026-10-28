package stats

import (
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/models"
	neu "skycrypt/src/models/NEU"
	"slices"
	"strconv"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
)

func GetBestiaryFamily(userProfile *skycrypttypes.Member, mobName string) *models.BestiaryMobOutput {
	bestiaryConstants := notenoughupdates.Constants().Bestiary.Islands
	if userProfile.Bestiary == nil {
		return &models.BestiaryMobOutput{}
	}

	bestiary := userProfile.Bestiary.Kills

	for _, category := range bestiaryConstants {
		for _, mob := range category.Mobs {
			if mob.Name == mobName {
				mobOutput := models.BestiaryMobOutput{
					Kills: bestiary[mob.Name], // All we care about is kills (Dungeons)
				}
				return &mobOutput
			}
		}
	}

	return nil
}

func getCategoryMobs(userProfile *skycrypttypes.Member, mobs []neu.BestiaryMob) []models.BestiaryMobOutput {
	mobOutputs := make([]models.BestiaryMobOutput, 0)
	if userProfile.Bestiary == nil {
		userProfile.Bestiary = &skycrypttypes.Bestiary{Kills: make(map[string]int)}
	}

	bestiaryKills := userProfile.Bestiary.Kills
	brackets := notenoughupdates.Constants().Bestiary.Brackets
	for _, mobData := range mobs {
		kills := 0
		for _, mobName := range mobData.Mobs {
			if killCount, exists := bestiaryKills[mobName]; exists {
				kills += killCount
			}
		}

		mobBracket := brackets[strconv.Itoa(mobData.Bracket)]
		nextTierKills := 0
		tier := 0
		maxTier := slices.Index(mobBracket, mobData.Cap)
		if maxTier == -1 {
			maxTier = len(mobBracket)
		} else {
			maxTier++
		}

		for i, bracketKills := range mobBracket {
			if kills < bracketKills && bracketKills <= mobData.Cap {
				nextTierKills = bracketKills
				tier = i
				break
			}
		}

		if nextTierKills == 0 {
			tier = maxTier
		}

		mobOutput := models.BestiaryMobOutput{
			Name:          mobData.Name,
			Texture:       mobData.Texture,
			Kills:         kills,
			NextTierKills: nextTierKills,
			MaxKills:      mobData.Cap,
			Tier:          tier,
			MaxTier:       maxTier,
		}

		mobOutputs = append(mobOutputs, mobOutput)
	}

	return mobOutputs
}

func GetBestiary(userProfile *skycrypttypes.Member) *models.BestiaryOutput {
	output := &models.BestiaryOutput{
		Categories:        make(map[string]models.BestiaryCategoryOutput),
		FamiliesCompleted: 0,
		FamiliesUnlocked:  0,
		FamilyTiers:       0,
		MaxFamilyTiers:    0,
		TotalFamilies:     0,
	}

	for categoryId, categoryData := range notenoughupdates.Constants().Bestiary.Islands {
		categoryData := models.BestiaryCategoryOutput{
			Name:    categoryData.Name,
			Texture: categoryData.Texture,
			Mobs:    getCategoryMobs(userProfile, categoryData.Mobs),
		}

		categoryData.MobsUnlocked = 0
		categoryData.MobsMaxed = 0
		tiers, maxTiers := 0, 0
		for _, mob := range categoryData.Mobs {
			tiers += mob.Tier
			maxTiers += mob.MaxTier
			if mob.Kills > 0 {
				categoryData.MobsUnlocked++
			}

			if mob.Tier >= mob.MaxTier {
				categoryData.MobsMaxed++
			}

		}

		output.FamiliesUnlocked += categoryData.MobsUnlocked
		output.FamiliesCompleted += categoryData.MobsMaxed

		output.FamilyTiers += tiers
		output.MaxFamilyTiers += maxTiers
		output.TotalFamilies += len(categoryData.Mobs)

		output.Categories[categoryId] = categoryData
	}

	output.Level = float64(output.FamilyTiers) / 10.0
	output.MaxLevel = float64(output.MaxFamilyTiers) / 10.0

	return output
}
