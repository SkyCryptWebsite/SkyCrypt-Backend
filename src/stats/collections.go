package stats

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func getBossCollections(userProfile *skycrypttypes.Member) models.CollectionCategory {
	bossCollections := []models.CollectionCategoryItem{}

	dungeons := GetFloorCompletions(userProfile)
	var floorItems []models.BossCollectionsFloorData

	for floor, amount := range dungeons.Total {
		if floor == "total" {
			continue
		}

		index, _ := utility.ParseInt(floor)
		boss := constants.BOSS_COLLECTIONS[index-1]
		tier := 0
		for _, t := range boss.Collections {
			if t <= amount {
				tier++
			}
		}

		item := models.CollectionCategoryItem{
			Name:        boss.Name,
			Id:          floor,
			Texture:     fmt.Sprintf("%s%s", utility.GetDomain(), boss.Texture),
			Amount:      amount,
			TotalAmount: amount,
			Tier:        tier,
			MaxTier:     len(boss.Collections),
			Amounts: []models.CollectionCategoryItemAmount{
				{
					Username: "Normal",
					Amount:   dungeons.Normal[floor],
				},
				{
					Username: "Master Mode",
					Amount:   dungeons.Master[floor],
				},
			},
		}

		floorId, _ := utility.ParseInt(floor)
		floorItems = append(floorItems, models.BossCollectionsFloorData{
			FloorId: floorId,
			Item:    item,
		})

	}

	kuudraTier := 0
	KUUDRA_CONSTANTS := constants.BOSS_COLLECTIONS[7]
	kuudraCompletions := GetKuudraCompletions(userProfile)
	for _, t := range KUUDRA_CONSTANTS.Collections {
		if t <= kuudraCompletions {
			kuudraTier++
		}
	}

	kuudraAmounts := []models.CollectionCategoryItemAmount{}
	for tier := range constants.KUUDRA_COMPLETIONS_MULTIPLIER {
		amounts := userProfile.CrimsonIsle.Kuudra
		if tier == "none" {
			tier = "basic"
		}

		kuudraAmounts = append(kuudraAmounts, models.CollectionCategoryItemAmount{
			Username: utility.TitleCase(tier),
			Amount:   amounts[tier],
		})
	}

	kuudraBoss := models.CollectionCategoryItem{
		Name:        KUUDRA_CONSTANTS.Name,
		Id:          "kuudra",
		Texture:     fmt.Sprintf("%s%s", utility.GetDomain(), KUUDRA_CONSTANTS.Texture),
		Amount:      kuudraCompletions,
		TotalAmount: kuudraCompletions,
		Tier:        kuudraTier,
		MaxTier:     len(KUUDRA_CONSTANTS.Collections),
		Amounts:     kuudraAmounts,
	}

	floorItems = append(floorItems, models.BossCollectionsFloorData{
		FloorId: 8,
		Item:    kuudraBoss,
	})

	utility.SortSlice(floorItems, func(i, j int) bool {
		return floorItems[i].FloorId < floorItems[j].FloorId
	})

	maxedTiers := 0
	for _, fd := range floorItems {
		bossCollections = append(bossCollections, fd.Item)
		if fd.Item.MaxTier == fd.Item.Tier {
			maxedTiers++
		}
	}

	return models.CollectionCategory{
		Name:       "Boss",
		Texture:    fmt.Sprintf("%s/api/item/SKULL_ITEM:1", utility.GetDomain()),
		Items:      bossCollections,
		TotalTiers: len(bossCollections),
		MaxedTiers: maxedTiers,
	}
}

func GetCollections(userProfile *skycrypttypes.Member, profile *skycrypttypes.Profile) models.CollectionsOutput {
	usernames := map[string]string{}
	for memberId := range profile.Members {
		username, err := api.GetUsername(memberId)
		if err != nil {
			username = "Unknown"
		}

		usernames[memberId] = username
	}

	output := models.CollectionsOutput{
		Categories: map[string]models.CollectionCategory{},
	}

	userCollections := userProfile.Collections
	output.Categories["BOSSES"] = getBossCollections(userProfile)
	for categoryId, categoryData := range constants.COLLECTIONS {
		category := models.CollectionCategory{
			Name:    categoryData.Name,
			Texture: categoryData.Texture,
			Items:   []models.CollectionCategoryItem{},
		}

		for _, itemData := range categoryData.Collections {
			amount := userCollections[itemData.Id]

			totalAmount, amounts := 0, []models.CollectionCategoryItemAmount{}
			for memberId, memberData := range profile.Members {
				memberAmount := memberData.Collections[itemData.Id]
				totalAmount += memberAmount
				if memberAmount > 0 {
					amounts = append(amounts, models.CollectionCategoryItemAmount{
						Username: usernames[memberId],
						Amount:   memberAmount,
					})
				}
			}

			tier := 0
			for _, tierData := range itemData.Tiers {
				if totalAmount >= tierData.AmountRequired {
					tier = tierData.Tier
				}
			}

			item := models.CollectionCategoryItem{
				Name:        itemData.Name,
				Id:          itemData.Id,
				Texture:     itemData.Texture,
				Amount:      amount,
				TotalAmount: totalAmount,
				Tier:        tier,
				MaxTier:     itemData.MaxTier,
				Amounts:     amounts,
			}

			category.Items = append(category.Items, item)
			category.TotalTiers++
			if item.MaxTier == item.Tier {
				category.MaxedTiers++
			}
		}

		output.TotalCollections += len(category.Items)
		output.MaxedCollections += category.MaxedTiers
		output.Categories[categoryId] = category
	}

	return output
}
