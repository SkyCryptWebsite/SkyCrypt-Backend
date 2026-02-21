package stats

import (
	"fmt"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	stats "skycrypt/src/stats/leveling"
	"skycrypt/src/utility"
	"slices"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func GetSkills(userProfile *skycrypttypes.Member, profile *skycrypttypes.Profile, player *skycrypttypes.Player) *models.Skills {
	output := &models.Skills{Skills: make(map[string]models.Skill)}
	if userProfile.PlayerData == nil {
		userProfile.PlayerData = &skycrypttypes.PlayerData{}
	}

	skillLevelCaps := stats.GetSkillLevelCaps(userProfile, player)

	if userProfile.PlayerData.Experience != nil {
		experienceMap := map[string]float64{
			"SKILL_FISHING":      userProfile.PlayerData.Experience.SkillFishing,
			"SKILL_ALCHEMY":      userProfile.PlayerData.Experience.SkillAlchemy,
			"SKILL_MINING":       userProfile.PlayerData.Experience.SkillMining,
			"SKILL_FARMING":      userProfile.PlayerData.Experience.SkillFarming,
			"SKILL_ENCHANTING":   userProfile.PlayerData.Experience.SkillEnchanting,
			"SKILL_TAMING":       userProfile.PlayerData.Experience.SkillTaming,
			"SKILL_FORAGING":     userProfile.PlayerData.Experience.SkillForaging,
			"SKILL_CARPENTRY":    userProfile.PlayerData.Experience.SkillCarpentry,
			"SKILL_COMBAT":       userProfile.PlayerData.Experience.SkillCombat,
			"SKILL_RUNECRAFTING": userProfile.PlayerData.Experience.SkillRunecrafting,
		}

		experienceMap["SKILL_SOCIAL"] = 0
		if profile != nil {
			for _, memberData := range profile.Members {
				if memberData.PlayerData == nil || memberData.PlayerData.Experience == nil {
					continue
				}

				experienceMap["SKILL_SOCIAL"] += memberData.PlayerData.Experience.SkillSocial
			}
		}

		for skillId, experience := range experienceMap {
			skill := strings.Split(strings.ToLower(skillId), "_")[1]

			capValue := skillLevelCaps[skill]
			extra := &stats.ExtraSkillData{
				Type:    skill,
				Texture: fmt.Sprintf("%s%s", utility.GetDomain(), constants.SKILL_ICONS[skill]),
				Cap:     &capValue,
			}

			output.Skills[skill] = stats.GetLevelByXp(int(experience), extra)
		}
	} else {
		experienceMap := map[string]int{
			"SKILL_FISHING":      player.Achievements.SkillFishing,
			"SKILL_ALCHEMY":      player.Achievements.SkillAlchemy,
			"SKILL_MINING":       player.Achievements.SkillMining,
			"SKILL_FARMING":      player.Achievements.SkillFarming,
			"SKILL_ENCHANTING":   player.Achievements.SkillEnchanting,
			"SKILL_TAMING":       player.Achievements.SkillTaming,
			"SKILL_FORAGING":     player.Achievements.SkillForaging,
			"SKILL_COMBAT":       player.Achievements.SkillCombat,
			"SKILL_SOCIAL":       0,
			"SKILL_CARPENTRY":    0,
			"SKILL_RUNECRAFTING": 0,
		}

		for skillId, level := range experienceMap {
			skill := strings.Split(strings.ToLower(skillId), "_")[1]

			output.Skills[skill] = stats.GetLevelByXp(stats.GetSkillExperience(skill, level), &stats.ExtraSkillData{Type: skill})
		}
	}

	var totalSkillXp int
	var totalSkillLevel int
	var totalSkillLevelWithProgress float64
	var skillCount int

	for skillName, skillData := range output.Skills {
		if slices.Contains(constants.COSMETIC_SKILLS, skillName) {
			continue
		}

		totalSkillLevelWithProgress += skillData.LevelWithProgress
		totalSkillLevel += skillData.Level
		totalSkillXp += skillData.XP
		skillCount++
	}

	output.TotalSkillXp = totalSkillXp
	output.AverageSkillLevel = float64(totalSkillLevel) / float64(skillCount)
	output.AverageSkillLevelWithProgress = totalSkillLevelWithProgress / float64(skillCount)

	return output
}
