package stats

import (
	"fmt"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func decodeMuseumItems(museumData *skycrypttypes.Museum, enabledPacks ...[]string) models.DecodedMuseumItems {
	output := models.DecodedMuseumItems{
		Items:   make(map[string]models.ProcessedMuseumItem),
		Special: []models.ProcessedMuseumItem{},
		Value:   0,
	}

	for itemId, itemData := range *museumData.Items {
		decodedItem, err := utility.DecodeInventory(&itemData.Items.Data)
		if err != nil {
			continue
		}

		processedItems := ProcessItems(decodedItem.Items, "museum", enabledPacks...)
		data := models.ProcessedMuseumItem{
			Items:           processedItems,
			SkyblockID:      itemId,
			Missing:         false,
			DonatedAsAChild: false,
		}

		output.Items[itemId] = data
	}

	for _, itemData := range *museumData.Special {
		decodedItem, err := utility.DecodeInventory(&itemData.Items.Data)
		if err != nil {
			continue
		}

		processedItem := ProcessItems(decodedItem.Items, "museum", enabledPacks...)
		data := models.ProcessedMuseumItem{
			Items:           processedItem,
			Missing:         false,
			DonatedAsAChild: false,
		}

		output.Special = append(output.Special, data)
	}

	return output
}

func markChildrenAsDonated(children string, output *map[string]models.ProcessedMuseumItem, decodedMuseum models.DecodedMuseumItems) {
	if _, exists := decodedMuseum.Items[children]; exists {
		(*output)[children] = decodedMuseum.Items[children]
	} else {
		(*output)[children] = models.ProcessedMuseumItem{
			SkyblockID:      children,
			DonatedAsAChild: true,
		}
	}

	if childOfChild, exists := constants.MUSEUM.Children[children]; exists && childOfChild != "" {
		markChildrenAsDonated(childOfChild, output, decodedMuseum)
	}
}

func getCategoryItems(category string, output map[string]models.ProcessedMuseumItem) []string {
	categoryList := constants.MUSEUM.Categories[category]
	if categoryList == nil {
		return []string{}
	}

	var result []string
	for _, item := range categoryList {
		if _, exists := output[item]; exists {
			result = append(result, item)
		}
	}
	return result
}

func ProcessMuseumItems(museumData *skycrypttypes.Museum, enabledPacks ...[]string) models.MuseumResult {
	if museumData.Items == nil {
		museumData.Items = &map[string]skycrypttypes.MuseumItem{}
	}

	if museumData.Special == nil {
		museumData.Special = &[]skycrypttypes.MuseumItem{}
	}

	decodedMuseum := decodeMuseumItems(museumData, enabledPacks...)

	output := make(map[string]models.ProcessedMuseumItem)
	for _, itemId := range constants.MUSEUM.GetAllItems() {
		_, exists := (*museumData.Items)[itemId]
		_, alreadySet := output[itemId]
		if !exists && !alreadySet {
			output[itemId] = models.ProcessedMuseumItem{
				SkyblockID: itemId,
				Missing:    true,
			}
			continue
		}

		if _, hasChildren := constants.MUSEUM.Children[itemId]; hasChildren {
			markChildrenAsDonated(itemId, &output, decodedMuseum)
		}

		if itemData, exists := decodedMuseum.Items[itemId]; exists {
			output[itemId] = itemData
		}
	}

	categories := make(map[string]models.MuseumStats)
	for _, cat := range constants.MUSEUM_CATEGORIES {
		items := getCategoryItems(cat, output)
		nonMissing := 0
		for _, item := range items {
			if !output[item].Missing {
				nonMissing++
			}
		}
		categories[cat] = models.MuseumStats{
			Amount: nonMissing,
			Total:  len(items),
		}
	}

	totalNonMissing := 0
	for _, item := range output {
		if !item.Missing {
			totalNonMissing++
		}
	}

	return models.MuseumResult{
		Value:     museumData.Value,
		Appraisal: museumData.Appraisal,
		Total: models.MuseumStats{
			Amount: totalNonMissing,
			Total:  len(output),
		},
		Categories: categories,
		Special: models.MuseumSpecialStats{
			Amount: len(decodedMuseum.Special),
		},
		Items:        output,
		SpecialItems: decodedMuseum.Special,
	}
}

func formatProgressBar(amount, total int, completedColor, missingColor string) string {
	barLength := 25
	progress := float64(amount) / float64(total)
	if progress > 1 {
		progress = 1
	}

	progressBars := int(progress * float64(barLength))
	emptyBars := barLength - progressBars

	var output string
	for range progressBars {
		output += fmt.Sprintf("§%s§l§m-", completedColor)
	}
	for range emptyBars {
		output += fmt.Sprintf("§%s§l§m-", missingColor)
	}
	output += "§r"

	return output

}

func formatMuseumItemProgress(item *models.MuseumInventoryItem, museumData models.MuseumResult) *models.MuseumInventoryItem {
	switch item.ProgressType {
	case "appraisal":
		appraisalMsg := "§cNo"
		if museumData.Appraisal {
			appraisalMsg = "§aYes"
		}

		item.Lore = append(item.Lore,
			fmt.Sprintf("§7Museum Appraisal Unlocked: %s", appraisalMsg),
			"",
			fmt.Sprintf("§7Museum Value: §6%s Coins §7(§6%s§7)", utility.AddCommas(int(museumData.Value)), utility.FormatNumber(museumData.Value)),
		)
	case "special":
		item.Lore = append(item.Lore, fmt.Sprintf(`§7Items Donated: §b%d`, museumData.Special.Amount), "", "§eClick to view!")
	case "total":
		item.Lore = append(item.Lore,
			fmt.Sprintf("§7Items Donated: §e%d§6%%", (museumData.Total.Amount*100)/max(museumData.Total.Total, 1)),
			fmt.Sprintf("%s §b%d §9/ §b%d", formatProgressBar(museumData.Total.Amount, museumData.Total.Total, "9", "f"), museumData.Total.Amount, museumData.Total.Total),
			"",
			"§eClick to view!",
		)
	default:
		if stats, exists := museumData.Categories[item.ProgressType]; exists {
			item.Lore = append(item.Lore,
				fmt.Sprintf("§7Items Donated: §e%d§6%%", (stats.Amount*100)/max(stats.Total, 1)),
				fmt.Sprintf("%s §b%d §9/ §b%d", formatProgressBar(stats.Amount, stats.Total, "9", "f"), stats.Amount, stats.Total),
				"",
				"§eClick to view!",
			)
		}
	}

	return item
}

func getMuseumSectionAmount(museumData models.MuseumResult, section string) int {
	if section == "special" {
		return museumData.Special.Amount
	}

	if stats, exists := museumData.Categories[section]; exists {
		return stats.Total
	}

	return 0
}

func getMuseumCategoryItems(section string) []string {
	return constants.MUSEUM.Categories[section]
}

func GetMuseum(museum *skycrypttypes.Museum, enabledPacks ...[]string) []models.ProcessedItem {
	if museum == nil {
		return make([]models.ProcessedItem, 6*9)
	}

	museumItems := ProcessMuseumItems(museum, enabledPacks...)

	output := make([]models.ProcessedItem, 6*9)
	for _, item := range constants.MUSEUM_INVENTORY {
		// Setup the frame for the museum
		itemSlot := formatMuseumItemProgress(&item, museumItems)
		itemSlot.Texture = fmt.Sprintf("%s%s", utility.GetDomain(), itemSlot.Texture)

		if itemSlot.InventoryType == "" {
			output[itemSlot.Position] = itemSlot.ProcessedItem
			continue
		}

		museumItemsSection := getMuseumSectionAmount(museumItems, itemSlot.InventoryType)
		pages := museumItemsSection / len(constants.MUSEUM_INVENTORY_ITEM_SLOTS)
		if museumItemsSection%len(constants.MUSEUM_INVENTORY_ITEM_SLOTS) != 0 {
			pages++
		}

		// Reserve space for items in the slot
		itemSlot.ProcessedItem.ContainsItems = make([]models.ProcessedItem, pages*54)

		for page := range pages {
			// CATEGORIES
			for index, slot := range constants.MUSEUM_INVENTORY_ITEM_SLOTS {
				slotIndex := index + page*len(constants.MUSEUM_INVENTORY_ITEM_SLOTS)
				// SPECIAL ITEMS CATEGORY
				if itemSlot.InventoryType == "special" {
					if slotIndex >= len(museumItems.SpecialItems) {
						continue
					}

					itemSlot.ProcessedItem.ContainsItems[slot+page*54] = museumItems.SpecialItems[slotIndex].Items[0]
					continue
				}

				// OTHER CATEGORIES
				categoryItemsAmount := getMuseumSectionAmount(museumItems, itemSlot.InventoryType)
				if slotIndex >= categoryItemsAmount {
					continue
				}

				categoryItems := getMuseumCategoryItems(itemSlot.InventoryType)
				if categoryItems == nil || slotIndex >= len(categoryItems) {
					continue
				}

				itemId := categoryItems[slotIndex]
				if itemId == "" {
					continue
				}

				museumItem := museumItems.Items[itemId]

				// MISSING ITEM
				if museumItem.SkyblockID == "" || museumItem.Missing {
					var itemData models.ProcessedItem
					if _, isArmorSet := constants.MUSEUM.ArmorSets[itemId]; isArmorSet {
						itemData = constants.MUSEUM_INVENTORY_MISSING_ARMOR_SET_TEMPLATE
					} else {
						itemData = constants.MUSEUM_INVENTORY_MISSING_ITEM_TEMPLATE
					}
					itemData.Texture = fmt.Sprintf("%s%s", utility.GetDomain(), itemData.Texture)

					itemName := constants.MUSEUM.ArmorSetToId[itemId]
					if itemName == "" {
						itemName = itemId
					}

					itemData.DisplayName = utility.TitleCase(itemName)
					itemSlot.ProcessedItem.ContainsItems[slot+page*54] = itemData
					continue
				}

				// DONATED HIGHER TIER
				if museumItem.DonatedAsAChild {
					itemData := constants.MUSEUM_INVENTORY_HIGHER_TIER_DONATED_TEMPLATE
					itemData.Texture = fmt.Sprintf("%s%s", utility.GetDomain(), itemData.Texture)

					itemName := constants.MUSEUM.ArmorSetToId[itemId]
					if itemName == "" {
						itemName = itemId
					}

					itemData.DisplayName = utility.TitleCase(itemName)
					itemSlot.ProcessedItem.ContainsItems[slot+page*54] = itemData
					continue
				}

				// NORMAL ITEM
				itemData := museumItem.Items[0]
				if len(museumItem.Items) > 1 {
					itemData.ContainsItems = museumItem.Items
				}

				itemSlot.ProcessedItem.ContainsItems[slot+page*54] = itemData

			}
		}

		output[itemSlot.Position] = itemSlot.ProcessedItem
	}

	return output
}
