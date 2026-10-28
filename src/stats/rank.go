package stats

import (
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
)

func GetRank(player *skycrypttypes.Player) *models.RankOutput {
	output := models.RankOutput{
		RankText:  "",
		RankColor: "",
		PlusText:  "",
		PlusColor: "",
	}

	var rankName string
	if player.Prefix != "" {
		rankName = strings.ReplaceAll(utility.GetRawLore(player.Prefix), "[", "")
		rankName = strings.ReplaceAll(rankName, "]", "")
	} else if player.Rank != "" && player.Rank != "NORMAL" {
		rankName = player.Rank
	} else if player.MonthlyPackageRank != "" && player.MonthlyPackageRank != "NONE" {
		rankName = player.MonthlyPackageRank
	} else if player.NewPackageRank != "" && player.NewPackageRank != "NONE" {
		rankName = player.NewPackageRank
	} else if player.PackageRank != "" {
		rankName = player.PackageRank
	} else {
		rankName = "NONE"
	}

	if rank, exists := constants.RANKS[rankName]; exists {
		output.RankText = rank.Tag

		if rankName == "SUPERSTAR" {
			if color, ok := constants.RANK_PLUS_COLORS[player.MonthlyRankColor]; ok {
				output.RankColor = color
			} else {
				output.RankColor = rank.Color
			}
		} else {
			output.RankColor = rank.Color
		}

		if rank.Plus != "" {
			output.PlusText = rank.Plus

			if rankName == "SUPERSTAR" || rankName == "MVP_PLUS" {
				if color, ok := constants.RANK_PLUS_COLORS[player.RankPlusColor]; ok {
					output.PlusColor = color
				} else {
					output.PlusColor = rank.PlusColor
				}
			} else if rank.PlusColor != "" {
				output.PlusColor = rank.PlusColor
			}
		}
	}

	return &output
}
