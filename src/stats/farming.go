package stats

import (
	"fmt"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	statsItems "skycrypt/src/stats/items"
	"skycrypt/src/utility"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func getMedalType(contest *skycrypttypes.JacobContestData) string {
	position := contest.ClaimedPosition
	participants := contest.ClaimedParticipants
	if participants == nil || position == nil {
		return ""
	}

	pos := *position
	parts := *participants

	var medal string
	switch {
	case pos <= int(float64(parts)*0.02):
		medal = "diamond"
	case pos <= int(float64(parts)*0.05):
		medal = "platinum"
	case pos <= int(float64(parts)*0.1):
		medal = "gold"
	case pos <= int(float64(parts)*0.3):
		medal = "silver"
	case pos <= int(float64(parts)*0.6):
		medal = "bronze"
	default:
		return "bronze"
	}

	return medal
}

func GetFarming(userProfile *skycrypttypes.Member, items []models.ProcessedItem) models.FarmingOutput {
	output := models.FarmingOutput{
		UniqueGolds: len(userProfile.JacobsContest.UniqueBrackets["gold"]),
		Pelts:       userProfile.Quests.TrapperQuest.PeltCount,
		Copper:      userProfile.Garden.Copper,
		Medals:      map[string]*models.Medal{},
		Contests:    map[string]*models.Contest{},
		Tools:       statsItems.GetSkillTools("farming", items),
	}

	for _, medal := range constants.FARMING_MEDALS {
		output.Medals[medal] = &models.Medal{
			Amount: userProfile.JacobsContest.MedalsInv[medal],
			Total:  0,
		}
	}

	contestsAttended := 0
	for contestId, contestData := range userProfile.JacobsContest.Contests {
		isValid := contestData.Collected > 100
		if !isValid {
			continue
		}

		parts := strings.Split(contestId, ":")
		cropId := strings.Join(parts[2:], ":")
		contestsAttended++

		if output.Contests[cropId] == nil {
			output.Contests[cropId] = &models.Contest{
				Name:      constants.CROPS[cropId],
				Texture:   fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), cropId),
				Collected: contestData.Collected,
				Amount:    1,
				Medals: map[string]int{
					"bronze":   0,
					"silver":   0,
					"gold":     0,
					"platinum": 0,
					"diamond":  0,
				},
			}
		} else {
			if contestData.Collected > output.Contests[cropId].Collected {
				output.Contests[cropId].Collected = contestData.Collected
			}
			output.Contests[cropId].Amount += 1
		}

		medal := contestData.ClaimedMedal
		if medal == "" {
			medal = getMedalType(&contestData)
		}

		if medal != "" {
			output.Medals[medal].Total += 1
			output.Contests[cropId].Medals[medal] += 1

			if medal == "diamond" {
				output.Contests[cropId].Maxed = true
			}
		}
	}

	output.ContestsAttended = contestsAttended

	return output
}
