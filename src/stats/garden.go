package stats

import (
	"fmt"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	stats "skycrypt/src/stats/leveling"
	"skycrypt/src/utility"
	"slices"
	"strconv"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func getVisitors(gardenData *skycrypttypes.Garden) models.Visitors {
	VISITOR_RARITIES := notenoughupdates.NEUConstants.Garden.Visitors
	MAX_VISITORS := notenoughupdates.NEUConstants.Garden.MaxVisitors

	visited, completed, unique := 0, 0, map[string]bool{}
	visitors := make(map[string]models.VisitorRarityData, len(gardenData.CommissionData.Visits))
	for visitorId, amount := range gardenData.CommissionData.Visits {
		completed += gardenData.CommissionData.Completed[visitorId]
		unique[visitorId] = true
		visited += amount

		visitorData := visitors[VISITOR_RARITIES[visitorId]]
		if visitorData.MaxUnique == 0 {
			visitorData = models.VisitorRarityData{
				MaxUnique: MAX_VISITORS[VISITOR_RARITIES[visitorId]],
			}
		}

		visitorData.Unique += 1
		visitorData.Visited += amount
		visitorData.Completed += gardenData.CommissionData.Completed[visitorId]

		visitors[VISITOR_RARITIES[visitorId]] = visitorData

	}

	return models.Visitors{
		Visited:        visited,
		Completed:      completed,
		UniqueVisitors: len(unique),
		Visitors:       visitors,
	}
}

func getCropMilestones(gardenData *skycrypttypes.Garden) []models.CropMilestone {
	milestones := make([]models.CropMilestone, 0, len(gardenData.ResourcesCollected))
	for cropId, cropName := range constants.CROPS {
		texture := fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), constants.CROP_TEXTURES[cropId])

		milestones = append(milestones, models.CropMilestone{
			Name:    cropName,
			Texture: texture,
			Level: stats.GetLevelByXp(int(gardenData.ResourcesCollected[cropId]), &stats.ExtraSkillData{
				Type: fmt.Sprintf("crop_milestone_%s", constants.CROP_TO_ID[cropId]),
			}),
		})
	}

	return milestones
}

func getCropUpgrades(gardenData *skycrypttypes.Garden) []models.CropUpgrade {
	upgrades := make([]models.CropUpgrade, 0, len(gardenData.CropUpgradeLevels))
	for cropId, cropName := range constants.CROPS {
		experience := stats.GetSkillExperience("crop_upgrade", int(gardenData.CropUpgradeLevels[cropId]))
		texture := fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), constants.CROP_TEXTURES[cropId])

		upgrades = append(upgrades, models.CropUpgrade{
			Name:    cropName,
			Texture: texture,
			Level: stats.GetLevelByXp(experience, &stats.ExtraSkillData{
				Type: "crop_upgrade",
			}),
		})
	}

	return upgrades
}

func getComposter(gardenData *skycrypttypes.Garden) map[string]int {
	output := make(map[string]int, len(gardenData.ComposterData.Upgrades))
	for _, upgrade := range notenoughupdates.NEUConstants.Garden.ComposterUpgrades {
		output[upgrade] = int(gardenData.ComposterData.Upgrades[upgrade])
	}

	return output
}

func getPlotLayout(gardenData *skycrypttypes.Garden) models.PlotLayout {
	PLOT_LAYOUT := notenoughupdates.NEUConstants.Garden.SortedPlots
	PLOT_NAMES := notenoughupdates.NEUConstants.Garden.Plots

	output := models.PlotLayout{
		Unlocked: len(gardenData.UnlockedPlotsIds),
		Total:    len(PLOT_LAYOUT),
		BarnSkin: "",
		Layout:   make([]models.ProcessedItem, 0, len(PLOT_LAYOUT)),
	}

	for index, plot := range PLOT_LAYOUT {
		checkPlots := []string{}

		if index-5 >= 0 && index-5 < len(PLOT_LAYOUT) { // ABOVE
			checkPlots = append(checkPlots, PLOT_LAYOUT[index-5])
		} else if index+1 >= 0 && index+1 < len(PLOT_LAYOUT) { // RIGHT
			checkPlots = append(checkPlots, PLOT_LAYOUT[index+1])
		} else if index+5 >= 0 && index+5 < len(PLOT_LAYOUT) { // BELOW
			checkPlots = append(checkPlots, PLOT_LAYOUT[index+5])
		} else if index-1 >= 0 && index-1 < len(PLOT_LAYOUT) { // LEFT
			checkPlots = append(checkPlots, PLOT_LAYOUT[index-1])
		}

		hasAdjacentUnlocked := false
		for _, plotId := range checkPlots {
			if slices.Contains(gardenData.UnlockedPlotsIds, plotId) {
				hasAdjacentUnlocked = true
				break
			}
		}

		// BARN SKIN
		if index == 12 {
			item := notenoughupdates.NEUConstants.Garden.BarnSkins[gardenData.SelectedBarnSkin]
			if item == nil {
				item = notenoughupdates.NEUConstants.Garden.BarnSkins["default_1"]
				output.BarnSkin = utility.TitleCase(gardenData.SelectedBarnSkin)
			} else {
				output.BarnSkin = utility.GetRawLore(item.Name)
			}

			texture := fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), strings.ReplaceAll(item.ItemId, "-", ":"))

			output.Layout = append(output.Layout, models.ProcessedItem{
				DisplayName: item.Name,
				Texture:     texture,
			})
		}

		textureId := "STAINED_GLASS_PANE:14"
		if slices.Contains(gardenData.UnlockedPlotsIds, plot) {
			textureId = "GRASS"
		} else if hasAdjacentUnlocked {
			textureId = "WOOD_BUTTON"
		}

		texture := fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), textureId)

		output.Layout = append(output.Layout, models.ProcessedItem{
			DisplayName: PLOT_NAMES[plot],
			Texture:     texture,
		})

	}

	return output
}

func getGardenUpgradeLevels(gardenData *skycrypttypes.Garden) []models.GardenUpgrade {
	return []models.GardenUpgrade{
		{
			Name:     constants.GARDEN_UPGRADES["growth_speed"].Name,
			Texture:  fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), constants.GARDEN_UPGRADES["growth_speed"].Texture),
			Level:    gardenData.GardenUpgrades["GROWTH_SPEED"],
			MaxLevel: constants.GARDEN_UPGRADES["growth_speed"].MaxLevel,
		},
		{
			Name:     constants.GARDEN_UPGRADES["plot_limit"].Name,
			Texture:  fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), constants.GARDEN_UPGRADES["plot_limit"].Texture),
			Level:    gardenData.GardenUpgrades["PLOT_LIMIT"],
			MaxLevel: constants.GARDEN_UPGRADES["plot_limit"].MaxLevel,
		},
		{
			Name:     constants.GARDEN_UPGRADES["yield"].Name,
			Texture:  fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), constants.GARDEN_UPGRADES["yield"].Texture),
			Level:    gardenData.GardenUpgrades["YIELD"],
			MaxLevel: constants.GARDEN_UPGRADES["yield"].MaxLevel,
		},
	}
}

func getGardenChips(userProfile *skycrypttypes.Member) []models.GardenChip {
	output := []models.GardenChip{}
	if userProfile.PlayerData == nil {
		userProfile.PlayerData = &skycrypttypes.PlayerData{}
	}

	for _, chipId := range constants.GARDEN_CHIPS {
		output = append(output, models.GardenChip{
			Name:    utility.TitleCase(chipId),
			Texture: fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), fmt.Sprintf("%s_GARDEN_CHIP", chipId)),
			Amount:  userProfile.PlayerData.GardenChips[chipId],
			Max:     constants.MAX_GARDEN_CHIPS,
		})
	}

	return output
}

func getDNAAnalysisMilestone(userProfile *skycrypttypes.Member) models.DNAAnalysisMilestone {
	if userProfile.Objectives == nil {
		return models.DNAAnalysisMilestone{
			Level:    0,
			MaxLevel: constants.MAX_DNA_ANALYSIS_MILESTONE,
		}
	}

	// t goes up to level 6. And you get it from objectives.tutorial by looking for the highest number on strings like dna_analysis_rewardskyblock_xp_1 - dna_analysis_rewardskyblock_xp_6
	milestone := 0
	for _, objectiveId := range userProfile.Objectives.Tutorial {
		if strings.HasPrefix(objectiveId, "dna_analysis_rewardskyblock_xp_") {
			parts := strings.Split(objectiveId, "_")
			levelStr := parts[len(parts)-1]
			level, err := strconv.Atoi(levelStr)
			if err == nil && level > milestone {
				milestone = level
			}
		}
	}

	return models.DNAAnalysisMilestone{
		Level:    milestone,
		MaxLevel: constants.MAX_DNA_ANALYSIS_MILESTONE,
	}
}

func GetGarden(userProfile *skycrypttypes.Member, gardenData *skycrypttypes.Garden) *models.Garden {
	return &models.Garden{
		Level:                stats.GetLevelByXp(int(gardenData.Experience), &stats.ExtraSkillData{Type: "garden"}),
		Visitors:             getVisitors(gardenData),
		CropMilestones:       getCropMilestones(gardenData),
		CropUpgrades:         getCropUpgrades(gardenData),
		Composter:            getComposter(gardenData),
		Plot:                 getPlotLayout(gardenData),
		GardenUpgrades:       getGardenUpgradeLevels(gardenData),
		GardenChips:          getGardenChips(userProfile),
		DNAAnalysisMilestone: getDNAAnalysisMilestone(userProfile),
	}
}
