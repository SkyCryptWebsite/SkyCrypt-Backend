package stats

import (
	"fmt"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	stats "skycrypt/src/stats/leveling"
	"skycrypt/src/utility"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func getSecrets(data *skycrypttypes.Dungeons) models.SecretsOutput {
	secretsFound := max(data.Secrets, 1)
	totalRuns := 0.0
	for _, dungeonType := range data.DungeonTypes {
		for tier, tierCompletion := range dungeonType.TierCompletions {
			if tier == "total" {
				continue
			}

			if tierCompletion > 0 {
				totalRuns += tierCompletion
			}
		}
	}

	if totalRuns == 0 {
		return models.SecretsOutput{
			Found:         int(secretsFound),
			SecretsPerRun: 0,
		}
	}

	return models.SecretsOutput{
		Found:         int(secretsFound),
		SecretsPerRun: utility.RoundFloat(secretsFound/totalRuns, 2),
	}
}

func getDungeonStats(userProfile *skycrypttypes.Member) models.DungeonStatsOutput {
	return models.DungeonStatsOutput{
		Secrets:                  getSecrets(&userProfile.Dungeons),
		HighestFloorBeatenNormal: userProfile.Dungeons.DungeonTypes["catacombs"].HighestTierCompleted,
		HighestFloorBeatenMaster: userProfile.Dungeons.DungeonTypes["master_catacombs"].HighestTierCompleted,
		BloodMobKills:            GetBestiaryFamily(userProfile, "Undead").Kills,
	}
}

func getScoreGrade(data skycrypttypes.BestRun) string {
	totalScore := data.ScoreExploration + data.ScoreSpeed + data.ScoreSkill + data.ScoreBonus
	switch {
	case totalScore <= 99:
		return "D"
	case totalScore <= 159:
		return "C"
	case totalScore <= 229:
		return "B"
	case totalScore <= 269:
		return "A"
	case totalScore <= 299:
		return "S"
	default:
		return "S+"
	}
}

func getBestRun(bestRunData *[]skycrypttypes.BestRun) *models.BestRunOutput {
	if bestRunData == nil || len(*bestRunData) == 0 {
		return nil
	}

	var bestScore int
	var bestRun *skycrypttypes.BestRun
	for _, run := range *bestRunData {
		score := run.ScoreExploration + run.ScoreSpeed + run.ScoreSkill + run.ScoreBonus
		if bestRun == nil || score > bestScore {
			bestRun = &run
			bestScore = score
		}
	}

	if bestRun == nil {
		return nil
	}

	result := models.BestRunOutput{
		Grade:            getScoreGrade(*bestRun),
		Timestamp:        bestRun.Timestamp,
		ScoreExploration: bestRun.ScoreExploration,
		ScoreSpeed:       bestRun.ScoreSpeed,
		ScoreSkill:       bestRun.ScoreSkill,
		ScoreBonus:       bestRun.ScoreBonus,
		DungeonClass:     bestRun.DungeonClass,
		ElapsedTime:      bestRun.ElapsedTime,
		DamageDealt:      bestRun.DamageDealt,
		Deaths:           bestRun.Deaths,
		MobsKilled:       bestRun.MobsKilled,
		SecretsFound:     bestRun.SecretsFound,
		DamageMitigated:  bestRun.DamageMitigated,
	}
	return &result
}

func getMostDamage(data *skycrypttypes.DungeonData, floorID string) models.MostDamageOutput {
	damageFields := map[string]map[string]float64{
		"berserk": data.MostDamageBerserk,
		"mage":    data.MostDamageMage,
		"healer":  data.MostDamageHealer,
		"archer":  data.MostDamageArcher,
		"tank":    data.MostDamageTank,
	}

	maxDamage, mostDamageClass := 0.0, ""
	for className, field := range damageFields {
		if dmg, ok := field[floorID]; ok && dmg > maxDamage {
			maxDamage = dmg
			mostDamageClass = className
		}
	}

	return models.MostDamageOutput{
		Damage: maxDamage,
		Type:   mostDamageClass,
	}
}

func formatCatacombsFloor(data *skycrypttypes.DungeonData, dungeonType string) []models.FormattedDungeonFloor {
	output := make([]models.FormattedDungeonFloor, 0, len(constants.DUNGEONS.Floors[dungeonType]))

	floorData := constants.DUNGEONS.Floors[dungeonType]
	for _, f := range floorData {
		stats := models.DungeonFloorStats{
			TimesPlayed:          data.TimesPlayed[f.ID],
			TierCompletions:      data.TierCompletions[f.ID],
			MilestoneCompletions: data.MilestoneCompletions[f.ID],
			MobsKilled:           data.MobsKilled[f.ID],
			BestScore:            data.BestScore[f.ID],
			WatcherKills:         data.WatcherKills[f.ID],
			MostMobsKilled:       data.MostMobsKilled[f.ID],
			FastestTime:          data.FastestTime[f.ID],
			FastestTimeS:         data.FastestTimeS[f.ID],
			FastestTimeSPlus:     data.FastestTimeSPlus[f.ID],
			MostHealing:          data.MostHealing[f.ID],
			MostDamage:           getMostDamage(data, f.ID),
		}

		floorData := models.FormattedDungeonFloor{
			Name:    f.Name,
			Texture: fmt.Sprintf("%s%s", utility.GetDomain(), f.Texture),
			Stats:   stats,
			BestRun: getBestRun(data.BestRuns[f.ID]),
		}

		output = append(output, floorData)
	}

	return output
}

func getClassData(userProfile *skycrypttypes.Member) models.ClassData {
	selectedClass := userProfile.Dungeons.SelectedDungeonClass
	if selectedClass == "" {
		selectedClass = "none"
	}

	output := models.ClassData{
		SelectedClass:   utility.TitleCase(selectedClass),
		Classes:         make(map[string]models.Skill),
		TotalExperience: 0,
	}

	classLevelSum, classLevelWithProgressSum := 0, 0.0
	for class, skill := range userProfile.Dungeons.Classes {
		output.TotalExperience += skill.Experience
		output.Classes[class] = stats.GetLevelByXp(int(skill.Experience), &stats.ExtraSkillData{Type: "dungeoneering", Texture: class})
		classLevelSum += output.Classes[class].Level
		classLevelWithProgressSum += output.Classes[class].LevelWithProgress
	}

	if len(output.Classes) > 0 {
		output.ClassAverage = float64(classLevelSum) / float64(len(output.Classes))
		output.ClassAverageWithProgress = classLevelWithProgressSum / float64(len(output.Classes))
	}

	return output
}

func GetFloorCompletions(userProfile *skycrypttypes.Member) *models.FloorCompletionsOutput {
	normalCompletions := userProfile.Dungeons.DungeonTypes["catacombs"].TierCompletions
	masterCompletions := userProfile.Dungeons.DungeonTypes["master_catacombs"].TierCompletions
	if normalCompletions == nil && masterCompletions == nil {
		return &models.FloorCompletionsOutput{
			Normal: map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0, "6": 0, "7": 0},
			Master: map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0, "6": 0, "7": 0},
			Total:  map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0, "6": 0, "7": 0},
		}
	}

	normal := make(map[string]int)
	for key, value := range normalCompletions {
		if key != "0" && key != "total" {
			normal[key] = int(value)
		}
	}

	master := make(map[string]int)
	for key, value := range masterCompletions {
		if key != "0" && key != "total" {
			master[key] = int(value) * 2 // Master completions count as double
		}
	}

	total := make(map[string]int)
	for key := range normal {
		total[key] = normal[key] + master[key]
	}

	return &models.FloorCompletionsOutput{
		Normal: normal,
		Master: master,
		Total:  total,
	}
}

func GetDungeons(userProfile *skycrypttypes.Member) models.DungeonsOutput {
	catacombs := userProfile.Dungeons.DungeonTypes["catacombs"]
	masterCatacombs := userProfile.Dungeons.DungeonTypes["master_catacombs"]

	output := models.DungeonsOutput{
		Level:           stats.GetLevelByXp(int(catacombs.Experience), &stats.ExtraSkillData{Type: "dungeoneering"}),
		Classes:         getClassData(userProfile),
		Catacombs:       formatCatacombsFloor(&catacombs, "catacombs"),
		MasterCatacombs: formatCatacombsFloor(&masterCatacombs, "master_catacombs"),
		Stats:           getDungeonStats(userProfile),
	}

	return output
}
