package stats

import (
	"fmt"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	stats "skycrypt/src/stats/leveling"
	"skycrypt/src/utility"
	"slices"
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
		texture := fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), cropId)

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
		texture := fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), cropId)

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

func GetGarden(gardenData *skycrypttypes.Garden) *models.Garden {
	return &models.Garden{
		Level:          stats.GetLevelByXp(int(gardenData.Experience), &stats.ExtraSkillData{Type: "garden"}),
		Visitors:       getVisitors(gardenData),
		CropMilestones: getCropMilestones(gardenData),
		CropUpgrades:   getCropUpgrades(gardenData),
		Composter:      getComposter(gardenData),
		Plot:           getPlotLayout(gardenData),
	}
}
