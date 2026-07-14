package stats

import (
	"fmt"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"slices"

	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
)

func ProcessItems(items []*skycrypttypes.Item, source string, enabledPacks ...[]string) []models.ProcessedItem {
	return ProcessItemsWithNEUCacheAndStats(items, source, map[string]models.NEUItem{}, nil, enabledPacks...)
}

func ProcessItemsWithNEUCache(items []*skycrypttypes.Item, source string, neuItemCache map[string]models.NEUItem, enabledPacks ...[]string) []models.ProcessedItem {
	return ProcessItemsWithNEUCacheAndStats(items, source, neuItemCache, nil, enabledPacks...)
}

func ProcessItemsWithNEUCacheAndStats(items []*skycrypttypes.Item, source string, neuItemCache map[string]models.NEUItem, itemStats *ItemProcessingStats, enabledPacks ...[]string) []models.ProcessedItem {
	if neuItemCache == nil {
		neuItemCache = map[string]models.NEUItem{}
	}
	if itemStats != nil {
		itemStats.ensure()
	}
	return processItemsWithStats(items, source, neuItemCache, itemStats, 0)
}

func processItemsWithStats(items []*skycrypttypes.Item, source string, neuItemCache map[string]models.NEUItem, itemStats *ItemProcessingStats, depth int) []models.ProcessedItem {
	processedItems := make([]models.ProcessedItem, 0, len(items))
	for _, item := range items {
		processedItem := processItemWithStats(item, source, neuItemCache, itemStats, depth)
		processedItem.ItemIndex = len(processedItems)

		processedItems = append(processedItems, processedItem)
	}

	return processedItems
}

func ProcessItem(item *skycrypttypes.Item, source string, enabledPacks ...[]string) models.ProcessedItem {
	return processItemWithStats(item, source, map[string]models.NEUItem{}, nil, 0)
}

func processItemWithStats(item *skycrypttypes.Item, source string, neuItemCache map[string]models.NEUItem, itemStats *ItemProcessingStats, depth int) models.ProcessedItem {
	if item == nil || item.Tag == nil {
		return models.ProcessedItem{}
	}
	recordStats := itemStats != nil
	if recordStats {
		itemStats.recordItemStart(source, item, depth)
		itemStart := time.Now()
		defer func() {
			itemStats.recordItemDuration(source, item, depth, time.Since(itemStart))
		}()
	}

	processedItem := models.ProcessedItem{
		Item:        *item,
		DisplayName: item.Tag.Display.Name,
		Lore:        item.Tag.Display.Lore,
		Source:      source,
	}

	var stageStart time.Time

	if recordStats {
		stageStart = time.Now()
	}
	itemType := ParseItemTypeFromLore(processedItem.Lore, *item)
	if recordStats {
		itemStats.recordStageDuration(itemProcessingStageTypeParse, time.Since(stageStart))
	}
	processedItem.Rarity = itemType.Rarity
	processedItem.Categories = itemType.Categories
	if processedItem.Recombobulated {
		processedItem.Lore = append(processedItem.Lore, "§8(Recombobulated)")
	}

	if item.Tag.ExtraAttributes != nil {
		if recordStats {
			stageStart = time.Now()
		}
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
					if itemData, ok := constants.GetItem(item.Tag.ExtraAttributes.Id); ok && itemData.Color != "" {
						defaultHexColor := itemData.Color
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
				if len(timestampStr) == 10 {
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
		if recordStats {
			itemStats.recordStageDuration(itemProcessingStageExtra, time.Since(stageStart))
		}

		if recordStats {
			stageStart = time.Now()
		}
		wiki, ok, wikiCacheHit := getNEUItemWiki(item.Tag.ExtraAttributes.Id, neuItemCache)
		if recordStats {
			itemStats.recordNEUWikiLookup(wikiCacheHit, time.Since(stageStart))
		}
		if ok {
			processedItem.Wiki = selectWikiLink(wiki)
		}
	}

	// POTIONS
	if item.ID != nil && *item.ID == 373 {
		damage := 0
		if item.Damage != nil {
			damage = *item.Damage
		}
		color := constants.POTION_COLORS[damage]
		if color == "" {
			color = constants.POTION_COLORS[15] // Uncraftable potion
		}

		var potionType string
		if damage&16384 != 0 {
			potionType = "splash"
		} else {
			potionType = "normal"
		}

		processedItem.Texture = fmt.Sprintf("%s/api/potion/%s/%s", utility.GetDomain(), potionType, color)
	}

	if item.Count != nil && *item.Count > 1 {
		processedItem.Count = item.Count
	}

	if processedItem.Texture == "" {
		if recordStats {
			stageStart = time.Now()
		}
		numericId := 0
		if item.ID != nil {
			numericId = *item.ID
		}

		damage := 0
		if item.Damage != nil {
			damage = *item.Damage
		}

		itemId := constants.GetVanillaItemId(constants.ItemModel{
			// ItemId:     item.Tag.ItemModel,
			NumericId:  numericId,
			ItemDamage: damage,
		})
		itemModel := ""
		if item.Tag != nil {
			itemModel = strings.TrimSpace(item.Tag.ItemModel)
		}
		if itemModel != "" {
			itemId = strings.TrimPrefix(strings.ToLower(itemModel), "minecraft:")
		}

		skyblockId := ""
		if item.Tag != nil && item.Tag.ExtraAttributes != nil {
			skyblockId = item.Tag.ExtraAttributes.Id
		}

		textureIdentifier := strings.TrimSpace(skyblockId)
		if textureIdentifier != "" {
			processedItem.Texture = fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), textureIdentifier)
		} else if item.Tag.SkullOwner != nil && len(item.Tag.SkullOwner.Properties.Textures) > 0 {
			if skinHash := utility.GetSkinHash(item.Tag.SkullOwner.Properties.Textures[0].Value); skinHash != "" {
				processedItem.Texture = fmt.Sprintf("%s/api/head/%s", utility.GetDomain(), skinHash)
			}
		}
		if processedItem.Texture == "" {
			textureIdentifier = strings.TrimPrefix(strings.TrimSpace(itemModel), "minecraft:")
		}
		if processedItem.Texture == "" && textureIdentifier == "" {
			textureIdentifier = strings.TrimSpace(itemId)
		}
		if recordStats {
			itemStats.recordStageDuration(itemProcessingStageTextureInput, time.Since(stageStart))
		}
		if processedItem.Texture == "" && textureIdentifier != "" {
			processedItem.Texture = fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), textureIdentifier)
		}
	}

	if item.ContainsItems != nil {
		if recordStats {
			stageStart = time.Now()
		}
		containerValue := 0.0
		for _, containedItem := range item.ContainsItems {
			if containedItem != nil && containedItem.Price > 0 {
				containerValue += containedItem.Price
			}
		}

		if containerValue > 0 {
			processedItem.Lore = append(processedItem.Lore, "", fmt.Sprintf("§7Container Value: §6%s Coins §7(§6%s§7)", utility.AddCommas(int(containerValue)), utility.FormatNumber(containerValue)))
		}
		if recordStats {
			itemStats.recordStageDuration(itemProcessingStageValueLore, time.Since(stageStart))
		}

		if recordStats {
			stageStart = time.Now()
		}
		processedItem.ContainsItems = processItemsWithStats(item.ContainsItems, source, neuItemCache, itemStats, depth+1)
		if recordStats {
			itemStats.recordStageDuration(itemProcessingStageNested, time.Since(stageStart))
		}
	}

	if item.Price > 0 {
		if recordStats {
			stageStart = time.Now()
		}
		processedItem.Lore = append(processedItem.Lore, "", fmt.Sprintf("§7Item Value: §6%s Coins §7(§6%s§7)", utility.AddCommas(int(item.Price)), utility.FormatNumber(item.Price)))
		if recordStats {
			itemStats.recordStageDuration(itemProcessingStageValueLore, time.Since(stageStart))
		}
	}

	// TODO: add cake bag & legacy backpack support

	return processedItem
}

func getNEUItemWiki(itemID string, neuItemCache map[string]models.NEUItem) ([]string, bool, bool) {
	if itemID == "" {
		return nil, false, false
	}
	if NEUItem, ok := neuItemCache[itemID]; ok {
		return NEUItem.Wiki, len(NEUItem.Wiki) > 0, true
	}
	wiki, ok := notenoughupdates.GetItemWiki(itemID)
	neuItemCache[itemID] = models.NEUItem{Wiki: wiki}
	return wiki, ok && len(wiki) > 0, false
}

func selectWikiLink(wiki []string) *string {
	if len(wiki) == 0 {
		return nil
	}
	if len(wiki) == 1 {
		if !strings.HasPrefix(wiki[0], "https://wiki.hypixel.net/") {
			return &wiki[0]
		}
		return nil
	}
	if strings.HasPrefix(wiki[0], "https://wiki.hypixel.net/") {
		return &wiki[1]
	}
	return &wiki[0]
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
				itemTexture = fmt.Sprintf("%s/api/item/%s", utility.GetDomain(), rawItemId)
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

			rarity := ""
			if itemData, ok := constants.GetItem(rawItemId); ok {
				rarity = itemData.Rarity
			}

			items[i].ContainsItems = append(items[i].ContainsItems, models.ProcessedItem{
				DisplayName: NEUItem.Name,
				Rarity:      rarity,
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
