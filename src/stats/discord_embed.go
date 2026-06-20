package stats

import (
	"math"
	"skycrypt/src/models"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func RoundToOneDecimal(value float64) float64 {
	rounded := math.Round(value*10) / 10
	if rounded == math.Floor(rounded) {
		return math.Floor(rounded)
	}

	return rounded
}

func getSkillsForEmbed(skills *models.Skills) models.EmbedDataSkills {
	output := models.EmbedDataSkills{
		SkillAverage: math.Floor(skills.AverageSkillLevelWithProgress),
		Skills:       make(map[string]int, len(skills.Skills)),
	}

	for skill, level := range skills.Skills {
		output.Skills[skill] = int(level.Level)
	}

	return output
}

func getDungeonsForEmbed(dungeons *models.DungeonsOutput) models.EmbedDataDungeons {
	output := models.EmbedDataDungeons{
		Dungeoneering: math.Floor(dungeons.Level.LevelWithProgress),
		ClassAverage:  math.Floor(dungeons.Classes.ClassAverageWithProgress),
		Classes:       make(map[string]int, len(dungeons.Classes.Classes)),
	}

	for class, level := range dungeons.Classes.Classes {
		output.Classes[class] = level.Level
	}

	return output
}

func getSlayersForEmbed(slayers *models.SlayersOutput) models.EmbedDataSlayers {
	output := models.EmbedDataSlayers{
		Experience: slayers.TotalSlayerExperience,
		Slayers:    make(map[string]int, len(slayers.Data)),
	}

	for slayer, level := range slayers.Data {
		output.Slayers[slayer] = level.Level.Level
	}

	return output
}

func GetEmbedData(mowojang *models.MowojangResponse, player *skycrypttypes.Player, userProfile *skycrypttypes.Member, profile *skycrypttypes.Profile, networth map[string]float64) *models.EmbedData {
	skills := GetSkills(userProfile, profile, player)
	dungeons := GetDungeons(userProfile)
	slayers := GetSlayers(userProfile)

	bank := 0.0
	if profile.Banking != nil {
		bank = *profile.Banking.Balance
	}

	if userProfile.Profile != nil {
		bank += userProfile.Profile.BankAccount
	}

	formattedNetworth := models.EmbedNetworth{
		Normal:      RoundToOneDecimal(networth["normal"]),
		NonCosmetic: RoundToOneDecimal(networth["nonCosmetic"]),
	}

	return &models.EmbedData{
		DisplayName:     mowojang.Name,
		Username:        mowojang.Name,
		Rank:            *GetRank(player),
		Uuid:            mowojang.UUID,
		ProfileId:       profile.ProfileID,
		ProfileCuteName: profile.CuteName,
		Joined:          userProfile.Profile.FirstJoin,
		GameMode:        profile.GameMode,
		SkyBlockLevel:   math.Floor(GetSkyBlockLevel(userProfile).LevelWithProgress),
		Skills:          getSkillsForEmbed(skills),
		Networth:        formattedNetworth,
		Purse:           RoundToOneDecimal(userProfile.Currencies.CoinPurse),
		Bank:            RoundToOneDecimal(bank),
		Dungeons:        getDungeonsForEmbed(dungeons),
		Slayers:         getSlayersForEmbed(slayers),
	}
}
