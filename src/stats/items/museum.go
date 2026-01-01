package stats

import (
	"fmt"
	"os"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func decodeMuseumItems(museumData *skycrypttypes.Museum, disabledPacks ...[]string) models.DecodedMuseumItems {
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

		processedItems := ProcessItems(decodedItem.Items, "museum", disabledPacks...)
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

		processedItem := ProcessItems(decodedItem.Items, "museum", disabledPacks...)
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
	var categoryList []string
	switch category {
	case "weapons":
		categoryList = constants.MUSEUM.Weapons
	case "armor":
		categoryList = constants.MUSEUM.Armor
	case "rarities":
		categoryList = constants.MUSEUM.Rarities
	default:
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

/*

? NOTE: For Debug purposes only, not used in production code

func getMissingItems(category string, output map[string]ProcessedMuseumItem) []string {
	categoryItems := getCategoryItems(category, output)
	var missing []string
	for _, item := range categoryItems {
		if output[item].Missing {
			missing = append(missing, item)
		}
	}
	return missing
}

func getMaxMissingItems(category string, output map[string]ProcessedMuseumItem) []string {
	missingItems := getMissingItems(category, output)
	var maxMissing []string

	childrenValues := make(map[string]bool)
	for _, child := range constants.MUSEUM.Children {
		childrenValues[child] = true
	}

	for _, item := range missingItems {
		if !childrenValues[item] {
			maxMissing = append(maxMissing, item)
		}
	}
	return maxMissing
}
*/

func ProcessMuseumItems(museumData *skycrypttypes.Museum, disabledPacks ...[]string) models.MuseumResult {
	if museumData.Items == nil || museumData.Special == nil {
		return models.MuseumResult{}
	}

	decodedMuseum := decodeMuseumItems(museumData, disabledPacks...)

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

	weapons := getCategoryItems("weapons", output)
	armor := getCategoryItems("armor", output)
	rarities := getCategoryItems("rarities", output)

	totalNonMissing := 0
	weaponsNonMissing := 0
	armorNonMissing := 0
	raritiesNonMissing := 0
	for _, item := range weapons {
		if !output[item].Missing {
			weaponsNonMissing++
		}
	}
	for _, item := range armor {
		if !output[item].Missing {
			armorNonMissing++
		}
	}
	for _, item := range rarities {
		if !output[item].Missing {
			raritiesNonMissing++
		}
	}
	for _, item := range output {
		if !item.Missing {
			totalNonMissing++
		}
	}

	/*
		? NOTE: For Debug purposes only, not used in production code

		var allMissing []string
		var allMaxMissing []string
		categories := []string{"weapons", "armor", "rarities"}
		for _, category := range categories {
			allMissing = append(allMissing, getMissingItems(category, output)...)
			allMaxMissing = append(allMaxMissing, getMaxMissingItems(category, output)...)
		}
	*/

	return models.MuseumResult{
		Value:     museumData.Value,
		Appraisal: museumData.Appraisal,
		Total: models.MuseumStats{
			Amount: totalNonMissing,
			Total:  len(output),
		},
		Weapons: models.MuseumStats{
			Amount: weaponsNonMissing,
			Total:  len(weapons),
		},
		Armor: models.MuseumStats{
			Amount: armorNonMissing,
			Total:  len(armor),
		},
		Rarities: models.MuseumStats{
			Amount: raritiesNonMissing,
			Total:  len(rarities),
		},
		Special: models.MuseumSpecialStats{
			Amount: len(decodedMuseum.Special),
		},
		Items:        output,
		SpecialItems: decodedMuseum.Special,
		/*
			? NOTE: For Debug purposes only, not used in production code

			Missing: MuseumMissing{
				Main: allMissing,
				Max:  allMaxMissing,
			},
		*/
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
	case "weapons":
		item.Lore = append(item.Lore,
			fmt.Sprintf("§7Items Donated: §e%d§6%%", (museumData.Weapons.Amount*100)/max(museumData.Weapons.Total, 1)),
			fmt.Sprintf("%s §b%d §9/ §b%d", formatProgressBar(museumData.Weapons.Amount, museumData.Weapons.Total, "9", "f"), museumData.Weapons.Amount, museumData.Weapons.Total),
			"",
			"§eClick to view!",
		)
	case "armor":
		item.Lore = append(item.Lore,
			fmt.Sprintf("§7Items Donated: §e%d§6%%", (museumData.Armor.Amount*100)/max(museumData.Armor.Total, 1)),
			fmt.Sprintf("%s §b%d §9/ §b%d", formatProgressBar(museumData.Armor.Amount, museumData.Armor.Total, "9", "f"), museumData.Armor.Amount, museumData.Armor.Total),
			"",
			"§eClick to view!",
		)
	case "rarities":
		item.Lore = append(item.Lore,
			fmt.Sprintf("§7Items Donated: §e%d§6%%", (museumData.Rarities.Amount*100)/max(museumData.Rarities.Total, 1)),
			fmt.Sprintf("%s §b%d §9/ §b%d", formatProgressBar(museumData.Rarities.Amount, museumData.Rarities.Total, "9", "f"), museumData.Rarities.Amount, museumData.Rarities.Total),
			"",
			"§eClick to view!",
		)
	case "total":
		item.Lore = append(item.Lore,
			fmt.Sprintf("§7Items Donated: §e%d§6%%", (museumData.Total.Amount*100)/max(museumData.Total.Total, 1)),
			fmt.Sprintf("%s §b%d §9/ §b%d", formatProgressBar(museumData.Total.Amount, museumData.Total.Total, "9", "f"), museumData.Total.Amount, museumData.Total.Total),
			"",
			"§eClick to view!",
		)
	}

	return item
}

func getMuseumSectionAmount(museumData models.MuseumResult, section string) int {
	switch section {
	case "weapons":
		return museumData.Weapons.Total
	case "armor":
		return museumData.Armor.Total
	case "rarities":
		return museumData.Rarities.Total
	case "special":
		return museumData.Special.Amount
	default:
		return 0
	}
}

func getMuseumItems(section string) []string {
	switch section {
	case "weapons":
		return constants.MUSEUM.Weapons
	case "armor":
		return constants.MUSEUM.Armor
	case "rarities":
		return constants.MUSEUM.Rarities
	default:
		return []string{}
	}
}

func GetMuseum(museum *skycrypttypes.Museum, disabledPacks ...[]string) []models.ProcessedItem {
	museumItems := ProcessMuseumItems(museum, disabledPacks...)

	output := make([]models.ProcessedItem, 6*9)
	for _, item := range constants.MUSEUM_INVENTORY {
		// Setup the frame for the museum
		itemSlot := formatMuseumItemProgress(&item, museumItems)
		if os.Getenv("DEV") == "true" {
			itemSlot.Texture = strings.Replace(itemSlot.Texture, "/api/", "http://localhost:8080/api/", 1)
		}

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

				categoryItems := getMuseumItems(itemSlot.InventoryType)
				itemId := categoryItems[slotIndex]
				if itemId == "" {
					continue
				}

				museumItem := museumItems.Items[itemId]

				// MISSING ITEM
				if museumItem.SkyblockID == "" || museumItem.Missing {
					itemData := constants.MUSEUM_INVENTORY_MISSING_ITEM_TEMPLATE[itemSlot.InventoryType]
					if os.Getenv("DEV") == "true" {
						itemData.Texture = strings.Replace(itemData.Texture, "/api/", "http://localhost:8080/api/", 1)
					}

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
					if os.Getenv("DEV") == "true" {
						itemData.Texture = strings.Replace(itemData.Texture, "/api/", "http://localhost:8080/api/", 1)
					}

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
