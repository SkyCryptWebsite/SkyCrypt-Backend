package stats

import (
	"fmt"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	statsItems "skycrypt/src/stats/items"
	stats "skycrypt/src/stats/leveling"
	"skycrypt/src/utility"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func getCenterOfTheForest(userProfile *skycrypttypes.Member) models.CenterOfTheForest {
	return models.CenterOfTheForest{
		Level:    userProfile.SkillTree.Nodes["foraging"].Levels["center_of_the_forest"],
		MaxLevel: constants.MAX_CENTER_OF_THE_FOREST_LEVEL,
	}
}

func getSelectedAxeAbility(userProfile *skycrypttypes.Member) string {
	if userProfile.SkillTree.SelectedAbility["foraging"] == "" {
		return "None"
	}

	return utility.TitleCase(userProfile.SkillTree.SelectedAbility["foraging"])
}

func calcHotfTokens(hotfTier int, cotfTier int) int {
	tokens := 0

	for tier := 1; tier <= hotfTier; tier++ {
		if reward, ok := constants.HOTF_REWARDS[tier]; ok {
			tokens += reward.TokenOfTheMountain
		}
	}

	for tier := 1; tier <= cotfTier; tier++ {
		if reward, ok := constants.COTF_REWARDS[tier]; ok {
			tokens += reward.TokenOfTheMountain
		}
	}

	return tokens
}

func getHotfTokens(hotfLevel models.Skill, userProfile *skycrypttypes.Member) models.HotfTokens {
	cotfTier := userProfile.SkillTree.Nodes["foraging"].Levels["center_of_the_forest"]
	hotfTokensAmount := calcHotfTokens(hotfLevel.Level, cotfTier)
	return models.HotfTokens{
		Total:     hotfTokensAmount,
		Spent:     userProfile.SkillTree.TokensSpent["forest"],
		Available: hotfTokensAmount - userProfile.SkillTree.TokensSpent["forest"],
	}
}

func getWhisper(userProfile *skycrypttypes.Member) models.Whispers {
	return models.Whispers{
		Total:     userProfile.ForagingCore.ForestsWhispers + userProfile.ForagingCore.ForestsWhispersSpent,
		Spent:     userProfile.ForagingCore.ForestsWhispersSpent,
		Available: userProfile.ForagingCore.ForestsWhispers,
	}
}

func getFishFamilyAmount(userProfile *skycrypttypes.Member) models.FishFamily {
	return models.FishFamily{
		Amount: len(userProfile.Foraging.FishFamily),
		Total:  constants.MAX_FISH_FAMILY_AMOUNT,
	}
}

func getHinaChapter(userProfile *skycrypttypes.Member) models.HinaChapter {
	if userProfile.Foraging.Hina == nil {
		return models.HinaChapter{
			Tier:    0,
			MaxTier: constants.MAX_HINA_CHAPTER,
		}
	}

	return models.HinaChapter{
		Tier:    userProfile.Foraging.Hina.Tasks.TierClaimed,
		MaxTier: constants.MAX_HINA_CHAPTER,
	}
}

func getTreeGifts(userProfile *skycrypttypes.Member) map[string]models.TreeGift {
	if userProfile.Foraging.TreeGifts == nil {
		return map[string]models.TreeGift{
			"FIG": {
				Milestone:    0,
				Texture:      fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), "FIG_LOG"),
				MaxMilestone: constants.MAX_TREE_GIFT_MILESTONE,
			},
			"MANGROVE": {
				Milestone:    0,
				Texture:      fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), "MANGROVE_LOG"),
				MaxMilestone: constants.MAX_TREE_GIFT_MILESTONE,
			},
		}
	}

	return map[string]models.TreeGift{
		"FIG": {
			Milestone:    userProfile.Foraging.TreeGifts.MilestoneTierClaimed["FIG"],
			Texture:      fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), "FIG_LOG"),
			MaxMilestone: constants.MAX_TREE_GIFT_MILESTONE,
		},
		"MANGROVE": {
			Milestone:    userProfile.Foraging.TreeGifts.MilestoneTierClaimed["MANGROVE"],
			Texture:      fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), "MANGROVE_LOG"),
			MaxMilestone: constants.MAX_TREE_GIFT_MILESTONE,
		},
	}
}

func GetForaging(userProfile *skycrypttypes.Member, player *skycrypttypes.Player, items []models.ProcessedItem) models.ForagingOutput {
	HOTFLevel := stats.GetLevelByXp(int(userProfile.SkillTree.Experience["foraging"]), &stats.ExtraSkillData{Type: "hotf"})
	skills := GetSkills(userProfile, nil, player)

	return models.ForagingOutput{
		Level:              HOTFLevel,
		ForagingLevel:      skills.Skills["foraging"],
		SelectedAxeAbility: getSelectedAxeAbility(userProfile),
		CenterOfTheForest:  getCenterOfTheForest(userProfile),
		Tokens:             getHotfTokens(HOTFLevel, userProfile),
		Whispers:           getWhisper(userProfile),
		FishFamily:         getFishFamilyAmount(userProfile),
		HinaChapter:        getHinaChapter(userProfile),
		TreeGift:           getTreeGifts(userProfile),
		Tools:              statsItems.GetSkillTools("foraging", items),
		Hotf: getSkillTree(userProfile,
			notenoughupdates.NEUConstants.HeartOfTheForest.Hotf.Perks,
			notenoughupdates.NEUConstants.HeartOfTheForest.Prelude,
			"foraging",
			HOTFLevel,
		),
	}
}
