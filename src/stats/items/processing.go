package stats

import (
	"fmt"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"slices"

	"skycrypt/src/constants"
	"skycrypt/src/lib"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
)

func ProcessItems(items []*skycrypttypes.Item, source string, disabledPacks ...[]string) []models.ProcessedItem {
	var processedItems []models.ProcessedItem
	for _, item := range items {
		processedItem := ProcessItem(item, source, disabledPacks...)
		processedItem.ItemIndex = len(processedItems)

		processedItems = append(processedItems, processedItem)
	}

	return processedItems
}

func ProcessItem(item *skycrypttypes.Item, source string, disabledPacks ...[]string) models.ProcessedItem {
	if item == nil || item.Tag == nil {
		return models.ProcessedItem{}
	}

	processedItem := models.ProcessedItem{
		Item:        *item,
		DisplayName: item.Tag.Display.Name,
		Lore:        item.Tag.Display.Lore,
		Source:      source,
	}

	rawLore := make([]string, len(processedItem.Lore))
	for i, lore := range processedItem.Lore {
		rawLore[i] = utility.GetRawLore(lore)
	}

	itemType := ParseItemTypeFromLore(rawLore, *item)
	processedItem.Rarity = itemType.Rarity
	processedItem.Categories = itemType.Categories
	if processedItem.Recombobulated {
		processedItem.Lore = append(processedItem.Lore, "§8(Recombobulated)")
	}

	if item.Tag.ExtraAttributes != nil {
		processedItem.Recombobulated = item.Tag.ExtraAttributes.Recombobulated == 1
		if item.Tag.SkullOwner == nil {
			// Do not apply shiny effecet to skulls
			processedItem.Shiny = len(item.Tag.ExtraAttributes.Enchantments) > 0
		}

		// Hex color
		if item.Tag.Display.Color != 0 {
			color := fmt.Sprintf("%06X", item.Tag.Display.Color)
			if item.Tag.ExtraAttributes.DyeItem == "" {
				if !utility.IsArmorHexColorsEnabled() {
					defaultHexColor := constants.ITEMS[item.Tag.ExtraAttributes.Id].Color
					if defaultHexColor != "" {
						color = defaultHexColor
					}
				}
			}

			if !slices.Contains(constants.BLACKLISTED_HEX_ARMOR_PIECES, item.Tag.ExtraAttributes.Id) {
				processedItem.Lore = append(processedItem.Lore, "", fmt.Sprintf("§7Color: #%s", color))
			}
		}

		// Timestamps
		if item.Tag.ExtraAttributes.Timestamp != nil {
			var timestampStr string
			switch timestamp := item.Tag.ExtraAttributes.Timestamp.(type) {
			case float64:
				timestampStr = fmt.Sprintf("%.0f", timestamp)
			case string:
				parsedTimestamp := utility.ParseTimestamp(timestamp)
				timestampStr = fmt.Sprintf("%d", parsedTimestamp)
			case int64:
				timestampStr = fmt.Sprintf("%d", timestamp)
			default:
				fmt.Printf("Unexpected type for timestamp: %T, %s\n", item.Tag.ExtraAttributes.Timestamp, item.Tag.ExtraAttributes.Timestamp)
			}

			if timestampStr != "" {
				if source == "museum" && len(timestampStr) == 10 {
					timestampStr += "000"
				}

				processedItem.Lore = append(processedItem.Lore, "", fmt.Sprintf("§7Obtained: §c{TIMESTAMP:%s}", timestampStr))
			}
		}

		// Gemstones
		if item.Tag.ExtraAttributes.Gems != nil {
			gems := ParseItemGems(item.Tag.ExtraAttributes.Gems, itemType.Rarity)
			if len(gems) > 0 {
				processedItem.Lore = append(processedItem.Lore, "", "§7Applied Gemstones:")
				for _, gem := range gems {
					processedItem.Lore = append(processedItem.Lore, fmt.Sprintf("§7 - %s", gem.Lore))
				}
			}
		}

		// Levelable enchantments
		if item.Tag.ExtraAttributes.HecatombSRuns != 0 {
			AddLevelableEnchantmentsToLore(item.Tag.ExtraAttributes.HecatombSRuns, constants.ENCHANTMENT_LADDERS["hecatomb_s_runs"], &processedItem.Lore)
		}

		if item.Tag.ExtraAttributes.ChampionCombatXP != 0 {
			AddLevelableEnchantmentsToLore(int(item.Tag.ExtraAttributes.ChampionCombatXP), constants.ENCHANTMENT_LADDERS["champion_combat_xp"], &processedItem.Lore)
		}

		if item.Tag.ExtraAttributes.FarmedCultivating != 0 {
			AddLevelableEnchantmentsToLore(item.Tag.ExtraAttributes.FarmedCultivating, constants.ENCHANTMENT_LADDERS["farmed_cultivating"], &processedItem.Lore)
		}

		if item.Tag.ExtraAttributes.ExpertiseKills != 0 {
			AddLevelableEnchantmentsToLore(item.Tag.ExtraAttributes.ExpertiseKills, constants.ENCHANTMENT_LADDERS["expertise_kills"], &processedItem.Lore)
		}

		if item.Tag.ExtraAttributes.CompactBlocks != 0 {
			AddLevelableEnchantmentsToLore(item.Tag.ExtraAttributes.CompactBlocks, constants.ENCHANTMENT_LADDERS["compact_blocks"], &processedItem.Lore)
		}

		// Wiki links
		NEUItem, err := notenoughupdates.GetItem(item.Tag.ExtraAttributes.Id)
		if err == nil && len(NEUItem.Wiki) > 0 {
			processedItem.Wiki = &models.WikipediaLinks{}
			if len(NEUItem.Wiki) == 1 {
				if strings.HasPrefix(NEUItem.Wiki[0], "https://wiki.hypixel.net/") {
					processedItem.Wiki.Official = NEUItem.Wiki[0]
				} else {
					processedItem.Wiki.Fandom = NEUItem.Wiki[0]
				}
			} else {
				if strings.HasPrefix(NEUItem.Wiki[0], "https://wiki.hypixel.net/") {
					processedItem.Wiki.Official = NEUItem.Wiki[0]
					processedItem.Wiki.Fandom = NEUItem.Wiki[1]
				} else {
					processedItem.Wiki.Fandom = NEUItem.Wiki[0]
					processedItem.Wiki.Official = NEUItem.Wiki[1]
				}
			}
		}
	}

	// POTIONS
	if *item.ID == 373 {
		color := constants.POTION_COLORS[*item.Damage]
		if color == "" {
			color = constants.POTION_COLORS[15] // Uncraftable potion
		}

		var potionType string
		if *item.Damage&16384 != 0 {
			potionType = "splash"
		} else {
			potionType = "normal"
		}

		processedItem.Texture = fmt.Sprintf("%s/api/potion/%s/%s", utility.GetDomain(), potionType, color)
	}

	if *item.Count > 1 {
		processedItem.Count = item.Count
	}

	if processedItem.Texture == "" {
		TextureItem := models.TextureItem{
			Count:  item.Count,
			Damage: item.Damage,
			ID:     item.ID,
			Tag:    item.Tag.ToMap(),
		}

		appliedTexture := lib.ApplyTexture(TextureItem, disabledPacks...)
		processedItem.Texture = appliedTexture.Texture
		if appliedTexture.TexturePack != "" {
			processedItem.TexturePack = appliedTexture.TexturePack
		}

		if processedItem.Texture == "" {
			fmt.Printf("[CUSTOM_RESOURCES] Found no textures for item: %s\n", item.Tag.ExtraAttributes.Id)
		}
	}

	if item.ContainsItems != nil {
		containerValue := 0.0
		for _, containedItem := range item.ContainsItems {
			if containedItem != nil && containedItem.Price > 0 {
				containerValue += containedItem.Price
			}
		}

		if containerValue > 0 {
			processedItem.Lore = append(processedItem.Lore, "", fmt.Sprintf("§7Container Value: §6%s Coins §7(§6%s§7)", utility.AddCommas(int(containerValue)), utility.FormatNumber(containerValue)))
		}

		processedItem.ContainsItems = ProcessItems(item.ContainsItems, source, disabledPacks...)
	}

	if item.Price > 0 {
		processedItem.Lore = append(processedItem.Lore, "", fmt.Sprintf("§7Item Value: §6%s Coins §7(§6%s§7)", utility.AddCommas(int(item.Price)), utility.FormatNumber(item.Price)))
	}

	// TODO: add cake bag & legacy backpack support

	return processedItem
}

func ProcessSacks(items []models.ProcessedItem, sackContents map[string]int) []models.ProcessedItem {
	prices, _ := skyhelpernetworthgo.GetPrices(true, 0, 0)
	sacksConstants := notenoughupdates.NEUConstants.Sacks

	for i := range items {
		sackData := sacksConstants[GetId(items[i])]
		if sackData == nil {
			continue
		}

		totalValue := 0.0
		for _, rawItemId := range sackData {
			NEUItem, err := notenoughupdates.GetItem(rawItemId)
			if err != nil {
				continue
			}

			itemId := rawItemId
			if NEUItem.NBT.ItemModel != "" {
				itemId = strings.Replace(NEUItem.NBT.ItemModel, "minecraft:", "", 1)
			}

			itemTexture := fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), itemId)
			if NEUItem.NBT.SkullOwner != nil {
				itemTexture = fmt.Sprintf("%s/api/head/%s", utility.GetDomain(), utility.GetSkinHash(NEUItem.NBT.SkullOwner.Properties.Textures[0].Value))
			}

			itemAmount := sackContents[rawItemId]
			lore := NEUItem.Lore
			if itemAmount > 1 {
				lore = append(lore, "", fmt.Sprintf("§7Amount: §c%s", utility.AddCommas(itemAmount)))

				itemPrice := prices[rawItemId]
				if itemPrice > 0 {
					totalPrice := float64(itemAmount) * itemPrice
					totalValue += totalPrice

					lore = append(lore, "", fmt.Sprintf("§7Unit Price: §6%s Coins §7(§6%s§7)", utility.AddCommas(int(itemPrice)), utility.FormatNumber(itemPrice)))
					lore = append(lore, "", fmt.Sprintf("§7Total Value: §6%s Coins §7(§6%s§7)", utility.AddCommas(int(totalPrice)), utility.FormatNumber(totalPrice)))
				}
			}

			items[i].ContainsItems = append(items[i].ContainsItems, models.ProcessedItem{
				DisplayName: NEUItem.Name,
				Rarity:      constants.ITEMS[rawItemId].Rarity,
				Texture:     itemTexture,
				Lore:        lore,
				Count:       &itemAmount,
			})
		}

		if totalValue > 0 {
			items[i].Lore = append(items[i].Lore, "", fmt.Sprintf("§7Sack Contents Value: §6%s Coins §7(§6%s§7)", utility.AddCommas(int(totalValue)), utility.FormatNumber(totalValue)))
		}
	}

	return items
}
