package stats

import (
	"skycrypt/src/models"
	stats "skycrypt/src/stats/leveling"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
)

func GetSkyBlockLevel(userProfile *skycrypttypes.Member) models.Skill {
	return stats.GetLevelByXp(userProfile.Leveling.Experience, &stats.ExtraSkillData{Type: "skyblock_level"})
}
