package stats

import (
	"fmt"
	"math"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"slices"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

var skillTables = map[string]map[int]int{
	"default":        constants.DEFAULT_LEVELLING_XP,
	"runecrafting":   constants.RUNECRAFTING_XP,
	"social":         constants.SOCIAL_XP,
	"dungeoneering":  constants.DUNGEONEERING_XP,
	"hotm":           constants.HOTM_XP,
	"hotf":           constants.HOTF_XP,
	"skyblock_level": constants.SKYBLOCK_XP,
}

type ExtraSkillData struct {
	Type    string `json:"type,omitempty"`
	Texture string `json:"texture,omitempty"`
	Cap     *int   `json:"cap,omitempty"`
}

func getXpTable(skillType string) map[int]int {
	if table, exists := skillTables[skillType]; exists {
		return table
	}

	if skillType == "garden" {
		return notenoughupdates.NEUConstants.Garden.GardenExperience
	} else if skillType == "crop_upgrade" {
		return notenoughupdates.NEUConstants.Garden.CropUpgrades
	} else if cropId, ok := strings.CutPrefix(skillType, "crop_milestone_"); ok {
		return notenoughupdates.NEUConstants.Garden.CropMilestones[cropId]
	}

	return constants.DEFAULT_LEVELLING_XP
}

func GetLevelByXp(xp int, extra *ExtraSkillData) models.Skill {
	if extra == nil {
		extra = &ExtraSkillData{Type: "default"}
	}
	if extra.Type == "" {
		extra.Type = "default"
	}

	xpTable := getXpTable(extra.Type)
	if xp < 0 {
		xp = 0
	}

	// the level that this player is capped at
	levelCap := constants.DEFAULT_SKILL_CAPS[extra.Type]
	if extra.Cap != nil && *extra.Cap != 0 {
		levelCap = *extra.Cap
	}

	if levelCap == 0 {
		maxKey := 0
		for key := range xpTable {
			if key > maxKey {
				maxKey = key
			}
		}
		levelCap = maxKey
	}

	// the level ignoring the cap and using only the table
	uncappedLevel := 0

	// the amount of xp over the amount required for the level
	xpCurrent := xp

	// like xpCurrent but ignores cap
	xpRemaining := xp

	for {
		nextXp, exists := xpTable[uncappedLevel+1]
		if !exists || nextXp > xpRemaining {
			break
		}

		uncappedLevel++
		xpRemaining -= nextXp
		if uncappedLevel <= levelCap {
			xpCurrent = xpRemaining
		}
	}

	// Whether the skill has infinite leveling
	isInfiniteLevelable := slices.Contains(constants.INFINITE, extra.Type)

	// adds support for infinite leveling
	if isInfiniteLevelable {
		maxExperience := utility.GetLastValue(xpTable)
		uncappedLevel += xpRemaining / maxExperience
		xpRemaining %= maxExperience
		xpCurrent = xpRemaining
	}

	// the maximum level that any player can achieve
	maxLevel := levelCap
	if isInfiniteLevelable {
		maxLevel = max(uncappedLevel, levelCap)
	} else if maxedCap, exists := constants.MaxedSkillCaps[extra.Type]; exists {
		maxLevel = maxedCap
	}

	// the level as displayed by in game UI
	level := uncappedLevel
	if !isInfiniteLevelable {
		level = min(levelCap, uncappedLevel)
	}

	// the amount of xp needed to reach the next level
	xpForNext := math.MaxInt32
	if level < maxLevel {
		if nextXp, exists := xpTable[level+1]; exists {
			xpForNext = int(nextXp)
		} else {
			xpForNext = utility.GetLastValue(xpTable)
		}
	} else if isInfiniteLevelable {
		xpForNext = utility.GetLastValue(xpTable)
	}

	// the fraction of the way toward the next level
	progress := 0.0
	if level < maxLevel || isInfiniteLevelable {
		if xpForNext > 0 {
			progress = math.Max(0, math.Min(float64(xpCurrent)/float64(xpForNext), 1))
		}
	}

	// a floating point value representing the current level
	levelWithProgress := float64(uncappedLevel) + progress
	if !isInfiniteLevelable {
		levelWithProgress = math.Min(levelWithProgress, float64(levelCap))
	}

	// a floating point value representing the current level ignoring caps
	unlockableLevelWithProgress := levelWithProgress
	if extra.Cap != nil {
		unlockableLevelWithProgress = math.Min(float64(uncappedLevel)+progress, float64(maxLevel))
	}

	// whether the skill is maxed or not
	maxed := level >= maxLevel
	if extra.Type == "skyblock_level" {
		maxed = false
	}

	texture := constants.SKILL_ICONS[extra.Type]
	if extra.Texture != "" {
		if textureIcon, exists := constants.SKILL_ICONS[extra.Texture]; exists {
			texture = textureIcon
		}
	}

	texture = fmt.Sprintf("%s%s", utility.GetDomain(), texture)

	return models.Skill{
		XP:                          xp,
		Level:                       level,
		MaxLevel:                    maxLevel,
		XPCurrent:                   xpCurrent,
		XPForNext:                   xpForNext,
		Progress:                    progress,
		LevelCap:                    levelCap,
		UncappedLevel:               uncappedLevel,
		LevelWithProgress:           levelWithProgress,
		UnlockableLevelWithProgress: unlockableLevelWithProgress,
		Maxed:                       maxed,
		Texture:                     texture,
	}
}

func GetSkillLevelCaps(userProfile *skycrypttypes.Member, player *skycrypttypes.Player) map[string]int {
	caps := map[string]int{
		"farming":      50,
		"taming":       50,
		"runecrafting": 3,
	}

	if userProfile.JacobsContest.Perks != nil {
		caps["farming"] += userProfile.JacobsContest.Perks.FarmingLevelCap
	}

	if userProfile.Pets != nil && userProfile.Pets.PetCare != nil {
		caps["taming"] += len(userProfile.Pets.PetCare.PetTypesSacrificed)
	}

	if player.NewPackageRank != "NONE" && player.NewPackageRank != "" {
		caps["runecrafting"] = 25
	}

	return caps
}

// GetSocialSkillExperience calculates the total social skill experience for a given profile
func GetSocialSkillExperience(profile *skycrypttypes.Profile) float64 {
	total := 0.00
	for _, member := range profile.Members {
		total += member.PlayerData.Experience.SkillSocial
	}
	return total
}

// GetXpByLevel does same as GetLevelByXp but uses level instead of xp
func GetXpByLevel(level int, extra *ExtraSkillData) models.Skill {
	if extra == nil {
		extra = &ExtraSkillData{Type: "default"}
	}
	if extra.Type == "" {
		extra.Type = "default"
	}

	xpTable := getXpTable(extra.Type)
	if level < 0 {
		level = 0
	}

	// the level that this player is capped at
	levelCap := constants.DEFAULT_SKILL_CAPS[extra.Type]
	if extra.Cap != nil {
		levelCap = *extra.Cap
	}
	if levelCap == 0 {
		maxKey := 0
		for key := range xpTable {
			if key > maxKey {
				maxKey = key
			}
		}
		levelCap = maxKey
	}

	// the level ignoring the cap and using only the table
	uncappedLevel := 0

	// the amount of xp over the amount required for the level
	xpCurrent := 0

	// like xpCurrent but ignores cap
	xpRemaining := 0

	for i := 0; i < level; i++ {
		uncappedLevel++
		xpRemaining += xpTable[uncappedLevel]
		if uncappedLevel <= levelCap {
			xpCurrent = xpRemaining
		}
	}

	// Whether the skill has infinite leveling
	isInfiniteLevelable := slices.Contains(constants.INFINITE, extra.Type)

	// adds support for infinite leveling
	if isInfiniteLevelable {
		maxExperience := utility.GetLastValue(xpTable)
		uncappedLevel += xpRemaining / maxExperience
		xpRemaining %= maxExperience
		xpCurrent = xpRemaining
	}

	// the maximum level that any player can achieve
	maxLevel := levelCap
	if isInfiniteLevelable {
		maxLevel = max(uncappedLevel, levelCap)
	} else if maxedCap, exists := constants.MaxedSkillCaps[extra.Type]; exists {
		maxLevel = maxedCap
	}

	// the amount of xp needed to reach the next level
	xpForNext := math.MaxInt32
	if level < maxLevel {
		if nextXp, exists := xpTable[level+1]; exists {
			xpForNext = int(nextXp)
		} else {
			xpForNext = utility.GetLastValue(xpTable)
		}
	} else if isInfiniteLevelable {
		xpForNext = utility.GetLastValue(xpTable)
	}

	// the fraction of the way toward the next level
	progress := 0.0
	if level < maxLevel || isInfiniteLevelable {
		if xpForNext > 0 {
			progress = math.Max(0, math.Min(float64(xpCurrent)/float64(xpForNext), 1))
		}
	}

	// a floating point value representing the current level
	levelWithProgress := float64(uncappedLevel) + progress
	if !isInfiniteLevelable {
		levelWithProgress = math.Min(levelWithProgress, float64(levelCap))
	}

	// a floating point value representing the current level ignoring caps
	unlockableLevelWithProgress := levelWithProgress
	if extra.Cap != nil {
		unlockableLevelWithProgress = math.Min(float64(uncappedLevel)+progress, float64(maxLevel))
	}

	// whether the skill is maxed or not
	maxed := level >= maxLevel

	texture := constants.SKILL_ICONS[extra.Type]
	if extra.Texture != "" {
		if textureIcon, exists := constants.SKILL_ICONS[extra.Texture]; exists {
			texture = textureIcon
		}
	}

	texture = fmt.Sprintf("%s%s", utility.GetDomain(), texture)

	return models.Skill{
		XP:                          xpCurrent,
		Level:                       level,
		MaxLevel:                    maxLevel,
		XPCurrent:                   xpCurrent,
		XPForNext:                   xpForNext,
		Progress:                    progress,
		LevelCap:                    levelCap,
		UncappedLevel:               uncappedLevel,
		LevelWithProgress:           levelWithProgress,
		UnlockableLevelWithProgress: unlockableLevelWithProgress,
		Maxed:                       maxed,
		Texture:                     texture,
	}
}

func GetSkillExperience(skill string, level int) int {
	skillTable := getXpTable(skill)
	total := 0

	for key, value := range skillTable {
		if key <= level {
			total += value
		}
	}

	return total
}
