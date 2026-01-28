package stats

import (
	"fmt"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func GetKuudraCompletions(userProfile *skycrypttypes.Member) int {
	if userProfile.CrimsonIsle.Kuudra == nil {
		return 0
	}

	kills := 0
	for kuudraId, kuudrakills := range userProfile.CrimsonIsle.Kuudra {
		if kuudraId == "total" || strings.HasPrefix(kuudraId, "highest") {
			continue
		}

		kills += kuudrakills * constants.KUUDRA_COMPLETIONS_MULTIPLIER[kuudraId]
	}

	return kills
}

func getKuudra(userProfile *skycrypttypes.Member) models.CrimsonIsleKuudra {
	tiers, totalKills := []models.CrimsonIsleKuudraTier{}, 0
	for kuudraId := range constants.KUUDRA_TIERS {
		if kuudraId == "total" || strings.HasPrefix(kuudraId, "highest") {
			continue
		}

		tier := models.CrimsonIsleKuudraTier{
			Name:    constants.KUUDRA_TIERS[kuudraId].Name,
			Id:      kuudraId,
			Texture: fmt.Sprintf("%s%s", utility.GetDomain(), constants.KUUDRA_TIERS[kuudraId].Texture),
			Kills:   userProfile.CrimsonIsle.Kuudra[kuudraId],
		}

		totalKills += tier.Kills
		tiers = append(tiers, tier)
	}

	return models.CrimsonIsleKuudra{
		TotalKills: totalKills,
		Tiers:      tiers,
	}
}

func getDojoRank(points int) string {
	if points >= 1000 {
		return "S"
	} else if points >= 800 {
		return "A"
	} else if points >= 600 {
		return "B"
	} else if points >= 400 {
		return "C"
	} else if points >= 200 {
		return "D"
	}

	return "F"
}

func getDojo(userProfile *skycrypttypes.Member) models.CrimsonIsleDojo {
	totalPoints, challenges := 0, []models.CrimsonIsleDojoChallenge{}
	for challengeId, challengeData := range constants.DOJO {
		points := userProfile.CrimsonIsle.Dojo[fmt.Sprintf("dojo_points_%s", challengeId)]
		time := userProfile.CrimsonIsle.Dojo[fmt.Sprintf("dojo_time_%s", challengeId)]
		challenge := models.CrimsonIsleDojoChallenge{
			Name:    challengeData.Name,
			Id:      challengeId,
			Texture: fmt.Sprintf("%s%s", utility.GetDomain(), challengeData.Texture),
			Points:  points,
			Time:    time,
			Rank:    getDojoRank(points),
		}

		totalPoints += points
		challenges = append(challenges, challenge)
	}

	return models.CrimsonIsleDojo{
		TotalPoints: totalPoints,
		Challenges:  challenges,
	}
}

func GetCrimsonIsle(userProfile *skycrypttypes.Member) *models.CrimsonIsleOutput {
	selectedFaction := userProfile.CrimsonIsle.SelectedFaction
	if selectedFaction == "" {
		selectedFaction = "None"
	}

	return &models.CrimsonIsleOutput{
		Factions: models.CrimsonIsleFactions{
			SelectedFaction:     selectedFaction,
			BarbarianReputation: int(userProfile.CrimsonIsle.BarbarianReputation),
			MagesReputation:     int(userProfile.CrimsonIsle.MagesReputation),
		},
		Kuudra: getKuudra(userProfile),
		Dojo:   getDojo(userProfile),
	}
}
