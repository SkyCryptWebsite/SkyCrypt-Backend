package stats

import (
	"fmt"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	stats "skycrypt/src/stats/items"
	"skycrypt/src/utility"
	"slices"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
)

func hasAccessory(accessories *[]models.InsertAccessory, id string, rarity string, ignoreRarity bool) bool {
	for _, accessory := range *accessories {
		if accessory.Id == id {
			if ignoreRarity {
				return true
			}

			if slices.Index(constants.RARITIES, accessory.Rarity) >= slices.Index(constants.RARITIES, rarity) {
				return true
			}
		}
	}

	return false
}

func getAccessory(accessories *[]models.InsertAccessory, id string) (*models.InsertAccessory, bool) {
	for i := range *accessories {
		if (*accessories)[i].Id == id {
			return &(*accessories)[i], true
		}
	}
	return &models.InsertAccessory{}, false
}

func getEnrichments(accessories []models.InsertAccessory) map[string]int {
	output := make(map[string]int)
	for _, item := range accessories {
		specialAccessory, exists := constants.SPECIAL_ACCESSORIES[item.Id]
		if slices.Index(constants.RARITIES, item.Rarity) < slices.Index(constants.RARITIES, "legendary") || (exists && !specialAccessory.AllowsEnrichment) || item.IsInactive {
			continue
		}

		enrichmentKey := item.Tag.ExtraAttributes.TalismanEnrichment
		if enrichmentKey == "" {
			enrichmentKey = "missing"
		}

		enrichment := constants.ENRICHMENT_TO_STAT[enrichmentKey]
		if enrichment == "" {
			enrichment = enrichmentKey
		}

		output[enrichment]++
	}

	return output
}

func GetRecombobulatedCount(accessories []models.InsertAccessory) int {
	count := 0
	for _, accessory := range accessories {
		if accessory.Tag.ExtraAttributes.Recombobulated > 0 {
			count++
		}
	}

	return count
}

func GetMagicalPower(rarity string, id string) int {
	if id == "HEGEMONY_ARTIFACT" {
		return (2 * constants.MAGICAL_POWER[rarity])
	}

	if id == "RIFT_PRISM" {
		return 11
	}

	return constants.MAGICAL_POWER[rarity]
}

func getMagicalPowerData(accessories *[]models.InsertAccessory, userProfile *skycrypttypes.Member) models.GetMagicalPowerOutput {
	output := models.GetMagicalPowerOutput{
		Rarities: models.GetMagicalPowerRarities{},
	}

	for _, rarity := range constants.RARITIES {
		output.Rarities[rarity] = struct {
			Amount       int `json:"amount"`
			MagicalPower int `json:"magicalPower"`
		}{
			Amount:       0,
			MagicalPower: 0,
		}
	}

	for _, accessory := range *accessories {
		if accessory.IsInactive {
			continue
		}

		magicalPower := GetMagicalPower(accessory.Rarity, accessory.Id)

		rarity := output.Rarities[accessory.Rarity]
		rarity.MagicalPower += magicalPower
		rarity.Amount++
		output.Rarities[accessory.Rarity] = rarity

		output.Accessories += magicalPower
		output.Total += magicalPower

		switch accessory.Id {
		case "ABICASE":
			abiphoneContacts := len(userProfile.CrimsonIsle.Abiphone.ActiveContacts)
			output.Abiphone += int(abiphoneContacts / 2)
			output.Total += int(abiphoneContacts / 2)

		case "HEGEMONY_ARTIFACT":
			output.Hegemony.Rarity = accessory.Rarity
			output.Hegemony.Amount += magicalPower
		}
	}

	if userProfile.Rift.Access.ConsumedPrism {
		output.RiftPrism += 11
		output.Total += 11
	}

	return output
}

func getMissing(accessories *[]models.InsertAccessory, accessoryIds []models.AccessoryIds) models.MissingOutput {
	ACCESSORIES := constants.GetAllAccessories()
	unique := make([]models.InsertAccessory, 0)
	for _, acc := range ACCESSORIES {
		unique = append(unique, models.InsertAccessory{
			Id:     acc.SkyBlockID,
			Rarity: acc.Rarity,
		})
	}

	for _, item := range unique {
		var aliases, exists = constants.ACCESSORY_ALIASES[item.Id]
		if !exists {
			continue
		}

		for _, duplicate := range aliases {
			if hasAccessory(accessories, duplicate, "", true) {
				accessory, found := getAccessory(accessories, duplicate)
				if found {
					accessory.Id = item.Id
				}
			}
		}
	}

	missing := make([]models.InsertAccessory, 0)
	for _, accessory := range unique {
		if !hasAccessory(accessories, accessory.Id, accessory.Rarity, true) {
			missing = append(missing, accessory)
		}
	}

	filteredMissing := make([]models.InsertAccessory, 0)
	for _, missingAccessory := range missing {
		upgrades := constants.GetUpgradeList(missingAccessory.Id)
		if len(upgrades) == 0 {
			filteredMissing = append(filteredMissing, missingAccessory)
			continue
		}

		shouldKeep := true
		for _, upgrade := range upgrades {
			if hasAccessory(accessories, upgrade, missingAccessory.Rarity, false) {
				shouldKeep = false
				break
			}
		}

		if shouldKeep {
			filteredMissing = append(filteredMissing, missingAccessory)
		}
	}

	upgrades := make([]models.ProcessedItem, 0)
	other := make([]models.ProcessedItem, 0)
	for _, missingAccessory := range filteredMissing {
		accessory := constants.ITEMS[missingAccessory.Id]
		object := models.ProcessedItem{
			Texture:     accessory.Texture,
			DisplayName: accessory.Name,
			Rarity:      missingAccessory.Rarity,
			Id:          missingAccessory.Id,
		}

		// Wiki links
		NEUItem, err := notenoughupdates.GetItem(missingAccessory.Id)
		if err == nil && len(NEUItem.Wiki) > 0 {
			object.Wiki = &models.WikipediaLinks{}
			if len(NEUItem.Wiki) == 1 {
				if strings.HasPrefix(NEUItem.Wiki[0], "https://wiki.hypixel.net/") {
					object.Wiki.Official = NEUItem.Wiki[0]
				} else {
					object.Wiki.Fandom = NEUItem.Wiki[0]
				}
			} else {
				if strings.HasPrefix(NEUItem.Wiki[0], "https://wiki.hypixel.net/") {
					object.Wiki.Official = NEUItem.Wiki[0]
					object.Wiki.Fandom = NEUItem.Wiki[1]
				} else {
					object.Wiki.Fandom = NEUItem.Wiki[0]
					object.Wiki.Official = NEUItem.Wiki[1]
				}
			}
		}

		upgradeList := constants.GetUpgradeList(missingAccessory.Id)
		specialAccessory, isSpecial := constants.SPECIAL_ACCESSORIES[missingAccessory.Id]

		if (len(upgradeList) > 0 && upgradeList[0] != missingAccessory.Id) || (isSpecial && len(specialAccessory.Rarities) > 0) {
			upgrades = append(upgrades, object)
		} else {
			other = append(other, object)
		}
	}

	return models.MissingOutput{
		Upgrades:     upgrades,
		Other:        other,
		AccessoryIds: accessoryIds,
	}
}

func addMissingDataToTheAccessory(accessories *[]models.ProcessedItem, prices map[string]float64) {
	for i := range *accessories {
		specialAccessory := constants.SPECIAL_ACCESSORIES[(*accessories)[i].Id]
		if specialAccessory.CustomPrice {
			// Custom Price (POWER_RELIC for example)
			if (*accessories)[i].Id == "POWER_RELIC" && (*accessories)[i].Rarity == "legendary" {
				price := 0.0
				for _, slot := range constants.ITEMS["POWER_RELIC"].GemstoneSlots {
					price += prices[fmt.Sprintf("PERFECT_%s_GEM", slot.SlotType)]
				}

				(*accessories)[i].Lore = append((*accessories)[i].Lore, "", fmt.Sprintf("§7Price: §6%s Coins §7(§6%s§7 per MP)", utility.AddCommas(int(price)), utility.FormatNumber(price/float64(GetMagicalPower((*accessories)[i].Rarity, (*accessories)[i].Id)))))
				(*accessories)[i].Price = price
				continue
			}

			// Item Upgrade (POWER_RELIC with all perfect gemstones for example)
			if specialAccessory.Upgrade != nil {
				upgradeItem := specialAccessory.Upgrade.Item
				upgradeCost := specialAccessory.Upgrade.Cost
				if upgradeItem != "" && upgradeCost != nil {
					amount := upgradeCost[(*accessories)[i].Rarity]
					if amount > 0 {
						price := prices[upgradeItem] * float64(amount)
						(*accessories)[i].Lore = append((*accessories)[i].Lore, "", fmt.Sprintf("§7Price: §6%s Coins §7(§6%s§7 per MP)", utility.AddCommas(int(price)), utility.FormatNumber(price/float64(GetMagicalPower((*accessories)[i].Rarity, (*accessories)[i].Id)))))
						(*accessories)[i].Price = price
						continue
					}
				}
			}

		}

		price := prices[(*accessories)[i].Id]
		if price > 0 {
			(*accessories)[i].Lore = append((*accessories)[i].Lore, "", fmt.Sprintf("§7Price: §6%s Coins §7(§6%s§7 per MP)", utility.AddCommas(int(price)), utility.FormatNumber(price/float64(GetMagicalPower((*accessories)[i].Rarity, (*accessories)[i].Id)))))
			(*accessories)[i].Price = price
		}
	}

	slices.SortFunc(*accessories, func(a, b models.ProcessedItem) int {
		// if price is equal to 0 move it to the end
		if a.Price == 0 && b.Price > 0 {
			return 1
		} else if a.Price > b.Price {
			return 1
		} else if a.Price < b.Price {
			return -1
		}

		return 0
	})

}

func GetMissingAccessories(accessories models.AccessoriesOutput, userProfile *skycrypttypes.Member) models.GetMissingAccessoresOutput {
	if len(accessories.AccessoryIds) == 0 && accessories.Accessories == nil {
		return models.GetMissingAccessoresOutput{}
	}

	missingAccessories := getMissing(&accessories.Accessories, accessories.AccessoryIds)

	prices, err := skyhelpernetworthgo.GetPrices(true, 0, 0)
	if err == nil {
		addMissingDataToTheAccessory(&missingAccessories.Other, prices)
		addMissingDataToTheAccessory(&missingAccessories.Upgrades, prices)
	}

	var activeAccessories []models.InsertAccessory
	for _, accessory := range accessories.Accessories {
		if !accessory.IsInactive {
			activeAccessories = append(activeAccessories, accessory)
		}
	}

	processedItems := make([]models.ProcessedItem, len(accessories.Accessories))
	for i, accessory := range accessories.Accessories {
		processedItems[i] = accessory.ProcessedItem
		processedItems[i].IsInactive = &accessory.IsInactive
	}

	output := models.GetMissingAccessoresOutput{
		Stats:               stats.GetStatsFromItems(processedItems),
		Enrichments:         getEnrichments(accessories.Accessories),
		Unique:              len(activeAccessories),
		Total:               constants.GetUniqueAccessoriesCount(),
		Recombobulated:      GetRecombobulatedCount(activeAccessories),
		TotalRecombobulated: constants.GetRecombableAccessoriesCount(),
		SelectedPower:       userProfile.AccessoryBagStorage.SelectedPower,
		MagicalPower:        getMagicalPowerData(&activeAccessories, userProfile),
		Accessories:         stats.StripItems(&processedItems),
		Upgrades:            stats.StripItems(&missingAccessories.Upgrades),
		Missing:             stats.StripItems(&missingAccessories.Other),
	}

	return output

}
