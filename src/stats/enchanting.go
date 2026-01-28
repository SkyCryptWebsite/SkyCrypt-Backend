package stats

import (
	"fmt"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func getGame(gameData *skycrypttypes.ExperimentationGame, gameId string) []models.EnchantingGame {
	if gameData == nil {
		gameData = &skycrypttypes.ExperimentationGame{}
	}

	var output []models.EnchantingGame
	for index, tier := range constants.EXPERIMENTS.Tiers {
		attempts := gameData.Attempts[index]
		claims := gameData.Claims[index]
		bestScore := gameData.BestScores[index]
		if attempts == 0 && claims == 0 && bestScore == 0 {
			continue
		}

		switch gameId {
		case "numbers":
			index += 2
		case "simon":
			index = min(index+1, 5)
		}

		tier = constants.EXPERIMENTS.Tiers[index]
		experimentData := models.EnchantingGame{
			Name:      tier.Name,
			Texture:   fmt.Sprintf("%s%s", utility.GetDomain(), tier.Texture),
			Attempts:  attempts,
			Claims:    claims,
			BestScore: bestScore,
		}

		output = append(output, experimentData)
	}

	return output
}

func GetEnchanting(userProfie *skycrypttypes.Member) models.EnchantingOutput {
	if userProfie.Experimentation.Simon == nil {
		return models.EnchantingOutput{
			Unlocked: false,
		}
	}

	output := map[string]models.EnchantingGameData{}
	games := []struct {
		key      string
		gameData *skycrypttypes.ExperimentationGame
	}{
		{"simon", userProfie.Experimentation.Simon},
		{"numbers", userProfie.Experimentation.Numbers},
		{"pairings", userProfie.Experimentation.Pairings},
	}

	for _, g := range games {
		if g.gameData == nil {
			continue
		}

		output[g.key] = models.EnchantingGameData{
			Name: constants.EXPERIMENTS.Games[g.key].Name,
			Stats: models.EnchantingGameStats{
				LastAttempt: g.gameData.LastAttempt,
				LastClaimed: g.gameData.LastClaimed,
				BonusClicks: g.gameData.BonusClicks,
				Games:       getGame(g.gameData, g.key),
			},
		}
	}

	return models.EnchantingOutput{
		Unlocked: true,
		Data:     output,
	}
}
