package constants

import (
	"skycrypt/src/models"
	"slices"
	"sort"
	"strings"
	"time"
)

var MUSEUM_CATEGORIES = []string{"combat", "farming", "mining", "fishing", "foraging", "dungeoneering", "hunting"}

type MuseumConstants struct {
	ArmorSetToId map[string]string
	ArmorSets    map[string][]string
	Children     map[string]string
	Categories   map[string][]string
}

var MUSEUM = MuseumConstants{}

func (m *MuseumConstants) GetAllItems() []string {
	total := 0
	for _, items := range m.Categories {
		total += len(items)
	}

	result := make([]string, 0, total)
	for _, category := range MUSEUM_CATEGORIES {
		result = append(result, m.Categories[category]...)
	}
	return result
}

var priorityOrder = []string{"HAT", "HOOD", "HELMET", "CHESTPLATE", "TUNIC", "LEGGINGS", "TROUSERS", "SLIPPERS", "BOOTS", "NECKLACE", "CLOAK", "BELT", "GAUNTLET", "GLOVES"}

func sortMuseumItems(items []string) {
	sort.Slice(items, func(i, j int) bool {
		a := items[i]
		b := items[j]

		aItem, aOk := ITEMS[a]
		bItem, bOk := ITEMS[b]
		if !aOk || !bOk {
			return false
		}

		aId := aItem.SkyblockID
		bId := bItem.SkyblockID

		aIdx := len(priorityOrder)
		bIdx := len(priorityOrder)

		for idx, keyword := range priorityOrder {
			if strings.Contains(aId, keyword) && aIdx == len(priorityOrder) {
				aIdx = idx
			}

			if strings.Contains(bId, keyword) && bIdx == len(priorityOrder) {
				bIdx = idx
			}
		}

		return aIdx < bIdx
	})

}

func getMuseumItems() {
	output := MuseumConstants{
		Children:     make(map[string]string),
		ArmorSets:    make(map[string][]string),
		ArmorSetToId: make(map[string]string),
		Categories:   make(map[string][]string),
	}

	for _, cat := range MUSEUM_CATEGORIES {
		output.Categories[cat] = []string{}
	}

	for _, item := range ITEMS {
		if item.MuseumData == nil {
			continue
		}

		category := strings.ToLower(item.MuseumData.Category)

		if item.MuseumData.Parent != nil {
			for parentKey, parentValue := range item.MuseumData.Parent {
				output.Children[parentValue] = parentKey
			}
		}

		if item.MuseumData.ArmorSetExperience != nil {
			var armorSetId string
			for setId := range item.MuseumData.ArmorSetExperience {
				armorSetId = setId
			}

			output.ArmorSets[armorSetId] = append(output.ArmorSets[armorSetId], item.SkyblockID)

			sortMuseumItems(output.ArmorSets[armorSetId])

			output.ArmorSetToId[armorSetId] = output.ArmorSets[armorSetId][0]

			if !slices.Contains(output.Categories[category], armorSetId) {
				output.Categories[category] = append(output.Categories[category], armorSetId)
			}
		} else {
			output.Categories[category] = append(output.Categories[category], item.SkyblockID)
		}
	}

	MUSEUM = output
}

func init() {
	go func() {
		getMuseumItems()
		for len(MUSEUM.Categories["combat"]) == 0 {
			time.Sleep(1 * time.Second)
			getMuseumItems()
		}

		ticker := time.NewTicker(60 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			getMuseumItems()
		}
	}()
}

var MUSEUM_INVENTORY = []models.MuseumInventoryItem{
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Museum",
			Rarity:      "rare",
			Texture:     "/api/head/438cf3f8e54afc3b3f91d20a49f324dca1486007fe545399055524c17941f4dc",
			Lore: []string{
				"§7The §9Museum §7is a compendium",
				"§7of all of your items in",
				"§7SkyBlock. Donate items to your",
				"§7Museum to unlock rewards.",
				"",
				"§7Other players can visit your",
				"§7Museum at any time! Display your",
				"§7best items proudly for all to", "§7see.",
				"",
			},
		},
		Position:     4,
		ProgressType: "total",
	},
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Combat",
			Rarity:      "special",
			Texture:     "/api/item/STONE_SWORD",
			Lore: []string{
				"§7View all of items related to the",
				"§cCombat Skill §7that you have donated to the",
				"§7to the §9Museum§7!",
				"",
			},
		},
		InventoryType: "combat",
		Position:      20,
		ProgressType:  "combat",
		ContainsItems: []models.MuseumInventoryItem{},
	},
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Farming",
			Rarity:      "uncommon",
			Texture:     "/api/item/GOLD_HOE",
			Lore: []string{
				"§7View all of items related to the",
				"§aFarming Skill §7that you have donated to the",
				"§7to the §9Museum§7!",
				"",
			},
		},
		InventoryType: "farming",
		Position:      21,
		ProgressType:  "farming",
		ContainsItems: []models.MuseumInventoryItem{},
	},
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Mining",
			Rarity:      "legendary",
			Texture:     "/api/item/STONE_PICKAXE",
			Lore: []string{
				"§7View all of items related to the",
				"§6Mining Skill §7that you have donated to the",
				"§7to the §9Museum§7!",
				"",
			},
		},
		InventoryType: "mining",
		Position:      22,
		ProgressType:  "mining",
		ContainsItems: []models.MuseumInventoryItem{},
	},
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Fishing",
			Rarity:      "divine",
			Texture:     "/api/item/FISHING_ROD",
			Lore: []string{
				"§7View all of items related to the",
				"§bFishing Skill §7that you have donated to the",
				"§7to the §9Museum§7!",
				"",
			},
		},
		InventoryType: "fishing",
		Position:      23,
		ProgressType:  "fishing",
		ContainsItems: []models.MuseumInventoryItem{},
	},
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Foraging",
			Rarity:      "uncommon",
			Texture:     "/api/item/JUNGLE_SAPLING",
			Lore: []string{
				"§7View all of items related to the",
				"§2Foraging Skill §7that you have donated to the",
				"§7to the §9Museum§7!",
				"",
			},
		},
		InventoryType: "foraging",
		Position:      24,
		ProgressType:  "foraging",
		ContainsItems: []models.MuseumInventoryItem{},
	},
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Dungeoneering",
			Rarity:      "very_special",
			Texture:     "/api/head/9b56895b9659896ad647f58599238af532d46db9c1b0389b8bbeb70999dab33d",
			Lore: []string{
				"§7View all of items related to the",
				"§4Dungeoneering Skill §7that you have donated to the",
				"§7to the §9Museum§7!",
				"",
			},
		},
		InventoryType: "dungeoneering",
		Position:      30,
		ProgressType:  "dungeoneering",
		ContainsItems: []models.MuseumInventoryItem{},
	},
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Hunting",
			Rarity:      "legendary",
			Texture:     "/api/item/LEAD",
			Lore: []string{
				"§7View all of items related to the",
				"§dHunting Skill §7that you have donated to the",
				"§7to the §9Museum§7!",
				"",
			},
		},
		InventoryType: "hunting",
		Position:      31,
		ProgressType:  "hunting",
		ContainsItems: []models.MuseumInventoryItem{},
	},
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Special Items",
			Rarity:      "mythic",
			Texture:     "/api/item/CAKE",
			Lore: []string{
				"§7View all of the §dSpecial Items §7that you",
				"§7have donated to the §9Museum§7",
				"",
				"§7These items don't count towards",
				"§7Museum progress and rewards, but",
				"§7are cool nonetheless. Items that",
				"§7are §9rare §7and §6prestigious",
				"§6§7fit into this category, and",
				"§7can be displayed in the Main",
				"§7room of the Museum.",
				"",
			},
		},
		InventoryType: "special",
		Position:      32,
		ProgressType:  "special",
		ContainsItems: []models.MuseumInventoryItem{},
	},
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Museum Appraisal",
			Rarity:      "legendary",
			Texture:     "/api/item/DIAMOND",
			Lore: []string{
				"§7§6Madame Goldsworth §7offers an",
				"§7appraisal service for Museums.",
				"§7When unlocked, she will appraise",
				"§7the value of your Museum each",
				"§7time you add or remove items.",
				"",
				"§7This service also allows you to",
				"§7appear on the §6Top Valued",
				"§6§7filter in the §9Museum",
				"§9Browser§7.",
				"",
			},
		},
		Position:     51,
		ProgressType: "appraisal",
	},
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Museum Rewards",
			Rarity:      "legendary",
			Texture:     "/api/item/GOLD_BLOCK",
			Lore: []string{
				"§7Each time you donate an item to",
				"§7your Museum, the §bCurator",
				"§b§7will reward you.",
				"",
				"§7§dSpecial Items §7do not count",
				"§7towards your Museum rewards",
				"§7progress.",
				"",
				"§7Currently, most rewards are",
				"§7§ccoming soon§7, but you can",
				"§7view them anyway.",
			},
		},
		Position: 48,
	},
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Close",
			Rarity:      "special",
			Texture:     "/api/item/BARRIER",
			Lore:        []string{},
		},
		Position: 49,
	},
	{
		ProcessedItem: models.ProcessedItem{
			DisplayName: "Museum Browser",
			Rarity:      "uncommon",
			Texture:     "/api/item/SIGN",
			Lore: []string{
				"§7View the Museums of your",
				"§7friends, top valued players, and",
				"§7more!",
			},
		},
		Position: 50,
	},
}

var MUSEUM_INVENTORY_ITEM_SLOTS = []int{10, 11, 12, 13, 14, 15, 16, 19, 20, 21, 22, 23, 24, 25, 28, 29, 30, 31, 32, 33, 34, 37, 38, 39, 40, 41, 42, 43}

var MUSEUM_INVENTORY_MISSING_ITEM_TEMPLATE = models.ProcessedItem{
	DisplayName: "Missing Item",
	Rarity:      "special",
	Texture:     "/api/item/INK_SACK:8",
	Lore: []string{
		"§7Click on this item in your",
		"§7inventory to add it to your",
		"§7§9Museum§7!",
	},
}

var MUSEUM_INVENTORY_MISSING_ARMOR_SET_TEMPLATE = models.ProcessedItem{
	DisplayName: "Missing Armor Set",
	Rarity:      "special",
	Texture:     "/api/item/INK_SACK:8",
	Lore: []string{
		"§7Click on an armor piece in your",
		"§7inventory that belongs to this",
		"§7armor set to donate the full set",
		"§7to your Museum.",
	},
}

var MUSEUM_INVENTORY_HIGHER_TIER_DONATED_TEMPLATE = models.ProcessedItem{
	DisplayName: "Higher Tier Donated",
	Texture:     "/api/item/INK_SACK:10",
	Rarity:      "special",
	Lore: []string{
		"§7Donated as higher tier",
	},
}
