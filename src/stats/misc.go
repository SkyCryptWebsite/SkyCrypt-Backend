package stats

import (
	"fmt"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func getEssence(userProfile *skycrypttypes.Member) []models.MiscEssence {
	essence := make([]models.MiscEssence, 0, len(constants.ESSENCE))
	for essenceId, essenceData := range constants.ESSENCE {
		essenceData := models.MiscEssence{
			Name:    essenceData.Name,
			Texture: fmt.Sprintf("%s%s", utility.GetDomain(), essenceData.Texture),
			Amount:  userProfile.Currencies.Essence[strings.ToUpper(essenceId)].Current,
		}

		essence = append(essence, essenceData)
	}

	return essence
}

func getKills(userProfile *skycrypttypes.Member) models.MiscKills {
	totalKills, totalDeaths := 0, 0
	kills, deaths := []models.MiscKill{}, []models.MiscKill{}
	for id, amount := range userProfile.PlayerStats.Kills {
		if id == "total" {
			continue
		}

		name := constants.MOB_NAMES[id]
		if name == "" {
			name = utility.TitleCase(id)
		}

		totalKills += int(amount)
		kills = append(kills, models.MiscKill{
			Name:   name,
			Amount: int(amount),
		})
	}

	for id, amount := range userProfile.PlayerStats.Deaths {
		if id == "total" {
			continue
		}

		totalDeaths += int(amount)
		name := constants.MOB_NAMES[id]
		if name == "" {
			name = utility.TitleCase(id)
		}

		deaths = append(deaths, models.MiscKill{
			Name:   name,
			Amount: int(amount),
		})
	}

	return models.MiscKills{
		TotalKills:  totalKills,
		TotalDeaths: totalDeaths,
		Kills:       kills,
		Deaths:      deaths,
	}
}

func getGifts(userProfile *skycrypttypes.Member) models.MiscGifts {
	return models.MiscGifts{
		Given:    int(userProfile.PlayerStats.Gifts.Given),
		Received: int(userProfile.PlayerStats.Gifts.Received),
	}
}

func getSeasonOfJerry(userProfile *skycrypttypes.Member) models.MiscSeasonOfJerry {
	return models.MiscSeasonOfJerry{
		MostSnowballsHit:     int(userProfile.PlayerStats.WinterIslandData.MostSnowballsHit),
		MostDamageDealt:      int(userProfile.PlayerStats.WinterIslandData.MostDamageDealt),
		MostMagmaDamageDealt: int(userProfile.PlayerStats.WinterIslandData.MostMagmaDamageDealt),
		MostCannonballsHit:   int(userProfile.PlayerStats.WinterIslandData.MostCannonballsHit),
	}
}

func getDragons(userProfile *skycrypttypes.Member) models.MiscDragons {
	dragonKills, dragonKillsTotal, dragonDeaths, dragonDeathsTotal := map[string]float64{}, 0.0, map[string]float64{}, 0.0
	for mobId, amount := range userProfile.PlayerStats.Kills {
		if strings.HasPrefix(mobId, "master_wither_king") {
			continue
		}

		if strings.HasSuffix(mobId, "_dragon") {
			dragonId := strings.ReplaceAll(mobId, "_dragon", "")
			dragonKills[dragonId] += float64(amount)
			dragonKillsTotal += float64(amount)
		}
	}

	dragonKills["total"] = dragonKillsTotal

	for mobId, amount := range userProfile.PlayerStats.Deaths {
		if strings.HasPrefix(mobId, "master_wither_king") {
			continue
		}

		if strings.HasSuffix(mobId, "_dragon") {
			dragonId := strings.ReplaceAll(mobId, "_dragon", "")
			dragonDeaths[dragonId] += float64(amount)
			dragonDeathsTotal += float64(amount)
		}
	}

	dragonDeaths["total"] = dragonDeathsTotal

	return models.MiscDragons{
		EnderCrystalsDestroyed: int(userProfile.PlayerStats.EndIsland.DragonFight.EnderCrystalsDestroyed),
		MostDamage:             userProfile.PlayerStats.EndIsland.DragonFight.MostDamage,
		FastestKill:            userProfile.PlayerStats.EndIsland.DragonFight.FastestKill,
		LastHits:               dragonKills,
		Deaths:                 dragonDeaths,
	}
}

func getEndstoneProtector(userProfile *skycrypttypes.Member) models.MiscEndstoneProtector {
	return models.MiscEndstoneProtector{
		Kills:  int(userProfile.PlayerStats.Kills["corrupted_protector"]),
		Deaths: int(userProfile.PlayerStats.Deaths["corrupted_protector"]),
	}
}

func getDamage(userProfile *skycrypttypes.Member) models.MiscDamage {
	return models.MiscDamage{
		HighestCriticalDamage: userProfile.PlayerStats.HighestCriticalDamage,
	}
}

func getPetMilestone(typeName string, amount float64) models.MiscPetMilestone {
	rarity := "common"
	milestones := constants.PET_MILESTONES[typeName]
	lastIndex := -1
	for i := len(milestones) - 1; i >= 0; i-- {
		if amount >= float64(milestones[i]) {
			lastIndex = i
			break
		}
	}
	if lastIndex >= 0 && lastIndex < len(constants.MILESTONE_RARITIES) {
		rarity = constants.MILESTONE_RARITIES[lastIndex]
	}
	total := int(amount)
	progress := "0"
	if amount > 0 && len(milestones) > 0 {
		maxMilestone := float64(milestones[len(milestones)-1])
		if maxMilestone > 0 {
			p := (amount / maxMilestone) * 100
			if p > 100 {
				p = 100
			}

			progress = fmt.Sprintf("%.2f%%", p)
		}
	}
	return models.MiscPetMilestone{
		Amount:   int(amount),
		Rarity:   rarity,
		Total:    total,
		Progress: progress,
	}
}

func getPetMilestones(userProfile *skycrypttypes.Member) map[string]models.MiscPetMilestone {
	return map[string]models.MiscPetMilestone{
		"sea_creatures_killed": getPetMilestone("sea_creatures_killed", userProfile.PlayerStats.Pets.Milestone.SeaCreaturesKilled),
		"ores_mined":           getPetMilestone("ores_mined", userProfile.PlayerStats.Pets.Milestone.OresMined),
	}
}

func getMythologicalEvent(userProfile *skycrypttypes.Member) models.MiscMythologicalEvent {
	return models.MiscMythologicalEvent{
		Kills:                 userProfile.PlayerStats.Mythos.Kills,
		BurrowsDugNext:        userProfile.PlayerStats.Mythos.BurrowsDugNext,
		BurrowsDugCombat:      userProfile.PlayerStats.Mythos.BurrowsDugCombat,
		BurrowsDugTreasure:    userProfile.PlayerStats.Mythos.BurrowsDugTreasure,
		BurrowsChainsComplete: userProfile.PlayerStats.Mythos.BurrowsChainsComplete,
	}
}

func getProfileUpgrades(profile *skycrypttypes.Profile) models.MiscProfileUpgrades {
	output := models.MiscProfileUpgrades{}
	for upgrade := range constants.PROFILE_UPGRADES {
		output[upgrade] = 0
	}

	if profile.CommunityUpgrades != nil && profile.CommunityUpgrades.UpgradeStates != nil {
		for _, u := range profile.CommunityUpgrades.UpgradeStates {
			if u.Tier > output[u.Upgrade] {
				output[u.Upgrade] = u.Tier
			}
		}
	}

	return output
}

func getAuctions(userProfile *skycrypttypes.Member) models.MiscAuctions {
	auctions := userProfile.PlayerStats.Auctions

	totalSold, totalSoldAmount, totalBought, totalBoughtAmount := map[string]float64{}, 0.0, map[string]float64{}, 0.0
	for item, amount := range auctions.TotalSold {
		if item == "total" {
			continue
		}

		totalSold[item] = amount
		totalSoldAmount += amount
	}

	totalSold["total"] = totalSoldAmount

	for item, amount := range auctions.TotalBought {
		if item == "total" {
			continue
		}

		totalBought[item] = amount
		totalBoughtAmount += amount
	}

	totalBought["total"] = totalBoughtAmount

	return models.MiscAuctions{
		Bids:        auctions.Bids,
		HighestBid:  auctions.HighestBid,
		Won:         auctions.Won,
		TotalBought: totalBought,
		GoldSpent:   auctions.GoldSpent,
		Created:     auctions.Created,
		Fees:        auctions.Fees,
		TotalSold:   totalSold,
		GoldEarned:  auctions.GoldEarned,
		NoBids:      auctions.NoBids,
	}
}

func getUncategorized(userProfile *skycrypttypes.Member) map[string]any {
	personalBank := constants.BANK_COOLDOWN[userProfile.Profile.PersonalBankUpgrade]
	if personalBank == "" {
		personalBank = "Unknown"
	}

	if userProfile.PlayerData == nil {
		userProfile.PlayerData = &skycrypttypes.PlayerData{}
	}

	return map[string]any{
		"soulflow":                 userProfile.ItemData.Soulflow,
		"teleporter_pill_consumed": userProfile.ItemData.TeleporterPillConsumed,
		"personal_bank":            personalBank,
		"metaphysical_serum":       userProfile.Experimentation.SerumsDrank,
		"reaper_peppers_eaten":     userProfile.PlayerData.ReaperPeppersEaten,
		"mcgrubber_burger":         userProfile.Rift.Castle.GrubberStacks,
		"wriggling_larva":          userProfile.Garden.LarvaConsumed,
		"refined_bottle_of_jyrre":  userProfile.WinterPlayerData.RefinedJyrreUses,
	}
}

func getClaimedItems(player *skycrypttypes.Player) map[string]int64 {
	return map[string]int64{
		"potato_talisman":         player.ClaimedPotatoTalisman,
		"potato_basket":           player.ClaimedPotatoBasket,
		"potato_war_silver_medal": player.ClaimPotatoWarSilverMedal,
		"potato_war_crown":        player.ClaimPotatoWarCrown,
		"skyblock_free_cookie":    player.SkyblockFreeCookie,
		"century_cake":            player.ClaimedCenturyCake,
		"century_cake_(year_200)": player.ClaimedCenturyCake200,
	}
}

func GetMisc(userProfile *skycrypttypes.Member, profile *skycrypttypes.Profile, player *skycrypttypes.Player) *models.MiscOutput {
	return &models.MiscOutput{
		Essence:           getEssence(userProfile),
		Kills:             getKills(userProfile),
		Gifts:             getGifts(userProfile),
		SeasonOfJerry:     getSeasonOfJerry(userProfile),
		Dragons:           getDragons(userProfile),
		EndstoneProtector: getEndstoneProtector(userProfile),
		Damage:            getDamage(userProfile),
		PetMilestones:     getPetMilestones(userProfile),
		MythologicalEvent: getMythologicalEvent(userProfile),
		ProfileUpgrades:   getProfileUpgrades(profile),
		Auctions:          getAuctions(userProfile),
		Uncategorized:     getUncategorized(userProfile),
		ClaimedItems:      getClaimedItems(player),
	}
}
