package constants

import "skycrypt/src/models"

var POTION_COLORS = map[int]string{
	0:     "375cc4", // None
	1:     "cb5ba9", // Regeneration
	2:     "420a09", // Speed
	3:     "e19839", // Poison
	4:     "4d9130", // Fire Resistance
	5:     "f52423", // Instant Health
	6:     "1f1f9e", // Night Vision
	7:     "22fc4b", // Jump Boost
	8:     "474c47", // Weakness
	9:     "912423", // Strength
	10:    "5c6e83", // Slowness
	11:    "f500f5", // Uncraftable
	12:    "420a09", // Instant Damage
	13:    "2f549c", // Water Breathing
	14:    "818595", // Invisibility
	15:    "f500f5", // Uncraftable
	16397: "430a09", // Harming
	16394: "5a6c81", // Slowness
}

var TYPE_TO_CATEGORIES = map[string][]string{
	"helmet":         {"armor", "helmet"},
	"chestplate":     {"armor", "chestplate"},
	"leggings":       {"armor", "leggings"},
	"boots":          {"armor", "boots"},
	"sword":          {"weapon", "sword"},
	"bow":            {"weapon", "bow"},
	"longsword":      {"weapon", "longsword", "sword"},
	"wand":           {"weapon", "wand"},
	"hatccessory":    {"armor", "helmet", "accessory", "hatccessory"},
	"gauntlet":       {"weapon", "mining_tool", "tool", "gauntlet"},
	"pickaxe":        {"mining_tool", "tool", "pickaxe"},
	"drill":          {"mining_tool", "tool", "drill"},
	"axe":            {"foraging_tool", "tool", "axe"},
	"hoe":            {"farming_tool", "tool", "hoe"},
	"fishing rod":    {"fishing_tool", "tool"},
	"fishing weapon": {"fishing_tool", "tool", "weapon"},
	"shovel":         {"tool", "shovel"},
	"shears":         {"tool", "shears"},
	"bait":           {"bait"},
	"item":           {"item"},
	"accessory":      {"accessory"},
	"arrow":          {"arrow"},
	"reforge stone":  {"reforge_stone"},
	"cosmetic":       {"cosmetic"},
	"pet item":       {"pet_item"},
	"travel scroll":  {"travel_scroll"},
	"belt":           {"belt"},
	"cloak":          {"cloak"},
	"necklace":       {"necklace"},
	"gloves":         {"gloves"},
	"bracelet":       {"bracelet"},
	"deployable":     {"deployable"},
	"trophy fish":    {"trophy_fish"},
}

var ENCHANTMENTS_TO_CATEGORIES = map[string][]string{
	"farming_tool": {"cultivating", "dedication", "delicate", "harvesting", "replenish", "sunder", "turbo_cacti", "turbo_cane", "turbo_carrot", "turbo_coco", "turbo_mushrooms", "turbo_potato", "turbo_warts", "turbo_wheat"},
}

var RARITIES = []string{
	"common", "uncommon", "rare", "epic", "legendary", "mythic", "divine", "supreme", "special", "very_special", "admin",
}

type gemstoneStats map[string][]any

type gemstoneData struct {
	Name  string                   `json:"name"`
	Color string                   `json:"color"`
	Stats map[string]gemstoneStats `json:"stats"`
}

var GEMSTONES = map[string]gemstoneData{
	"JADE": {
		Name:  "Jade",
		Color: "a",
		Stats: map[string]gemstoneStats{
			"ROUGH": {
				"mining_fortune": []any{2, 4, 6, 8, 10, 12, 14},
			},
			"FLAWED": {
				"mining_fortune": []any{3, 5, 7, 10, 14, 18, 22},
			},
			"FINE": {
				"mining_fortune": []any{5, 7, 10, 15, 20, 25, 30},
			},
			"FLAWLESS": {
				"mining_fortune": []any{7, 10, 15, 20, 27, 35, 44},
			},
			"PERFECT": {
				"mining_fortune": []any{10, 14, 20, 30, 40, 50, 60},
			},
		},
	},
	"AMBER": {
		Name:  "Amber",
		Color: "6",
		Stats: map[string]gemstoneStats{
			"ROUGH": {
				"mining_speed": []any{4, 8, 12, 16, 20, 24, 28},
			},
			"FLAWED": {
				"mining_speed": []any{6, 10, 14, 18, 24, 30, 36},
			},
			"FINE": {
				"mining_speed": []any{10, 14, 20, 28, 36, 45, 54},
			},
			"FLAWLESS": {
				"mining_speed": []any{14, 20, 30, 44, 58, 75, 92},
			},
			"PERFECT": {
				"mining_speed": []any{20, 28, 40, 60, 80, 100, 120},
			},
		},
	},
	"TOPAZ": {
		Name:  "Topaz",
		Color: "e",
		Stats: map[string]gemstoneStats{
			"ROUGH": {
				"pristine": []any{0.4, 0.4, 0.4, 0.4, 0.4, 0.4, 0.5},
			},
			"FLAWED": {
				"pristine": []any{0.8, 0.8, 0.8, 0.8, 0.8, 0.8, 0.9},
			},
			"FINE": {
				"pristine": []any{1.2, 1.2, 1.2, 1.2, 1.2, 1.2, 1.3},
			},
			"FLAWLESS": {
				"pristine": []any{1.6, 1.6, 1.6, 1.6, 1.6, 1.6, 1.8},
			},
			"PERFECT": {
				"pristine": []any{2, 2, 2, 2, 2, 2, 2.2},
			},
		},
	},
	"SAPPHIRE": {
		Name:  "Sapphire",
		Color: "b",
		Stats: map[string]gemstoneStats{
			"ROUGH": {
				"intelligence": []any{2, 3, 4, 5, 6, 7, nil},
			},
			"FLAWED": {
				"intelligence": []any{5, 5, 6, 7, 8, 10, nil},
			},
			"FINE": {
				"intelligence": []any{7, 8, 9, 10, 11, 12, nil},
			},
			"FLAWLESS": {
				"intelligence": []any{10, 11, 12, 14, 17, 20, nil},
			},
			"PERFECT": {
				"intelligence": []any{12, 14, 17, 20, 24, 30, nil},
			},
		},
	},
	"AMETHYST": {
		Name:  "Amethyst",
		Color: "5",
		Stats: map[string]gemstoneStats{
			"ROUGH": {
				"defense": []any{1, 2, 3, 4, 5, 7, nil},
			},
			"FLAWED": {
				"defense": []any{3, 4, 5, 6, 8, 10, nil},
			},
			"FINE": {
				"defense": []any{4, 5, 6, 8, 10, 14, nil},
			},
			"FLAWLESS": {
				"defense": []any{5, 7, 10, 14, 18, 22, nil},
			},
			"PERFECT": {
				"defense": []any{6, 9, 13, 18, 24, 30, nil},
			},
		},
	},
	"JASPER": {
		Name:  "Jasper",
		Color: "d",
		Stats: map[string]gemstoneStats{
			"ROUGH": {
				"strength": []any{1, 1, 1, 2, 3, 4, nil},
			},
			"FLAWED": {
				"strength": []any{2, 2, 3, 4, 4, 5, nil},
			},
			"FINE": {
				"strength": []any{3, 3, 4, 5, 6, 7, nil},
			},
			"FLAWLESS": {
				"strength": []any{5, 6, 7, 8, 10, 12, nil},
			},
			"PERFECT": {
				"strength": []any{6, 7, 9, 11, 13, 16, nil},
			},
		},
	},
	"RUBY": {
		Name:  "Ruby",
		Color: "c",
		Stats: map[string]gemstoneStats{
			"ROUGH": {
				"health": []any{1, 2, 3, 4, 5, 7, nil},
			},
			"FLAWED": {
				"health": []any{3, 4, 5, 6, 8, 10, nil},
			},
			"FINE": {
				"health": []any{4, 5, 6, 8, 10, 14, nil},
			},
			"FLAWLESS": {
				"health": []any{5, 7, 10, 14, 18, 22, nil},
			},
			"PERFECT": {
				"health": []any{6, 9, 13, 18, 24, 30, nil},
			},
		},
	},
	"OPAL": {
		Name:  "Opal",
		Color: "f",
		Stats: map[string]gemstoneStats{
			"ROUGH": {
				"true_defense": []any{1, 1, 1, 2, 2, 3, nil},
			},
			"FLAWED": {
				"true_defense": []any{2, 2, 2, 3, 3, 4, nil},
			},
			"FINE": {
				"true_defense": []any{3, 3, 3, 4, 4, 5, nil},
			},
			"FLAWLESS": {
				"true_defense": []any{4, 4, 5, 6, 8, 9, nil},
			},
			"PERFECT": {
				"true_defense": []any{5, 6, 7, 9, 11, 13, nil},
			},
		},
	},
	"AQUAMARINE": {
		Name:  "Aquamarine",
		Color: "3",
		Stats: map[string]gemstoneStats{
			"ROUGH": {
				"fishing_speed": []any{0.5, 0.5, 1, 1, 1.5, 2, nil},
			},
			"FLAWED": {
				"fishing_speed": []any{1, 1, 1.5, 1.5, 2, 2.5, nil},
			},
			"FINE": {
				"fishing_speed": []any{1.5, 1.5, 2, 2, 2.5, 3, nil},
			},
			"FLAWLESS": {
				"fishing_speed": []any{2, 2, 2.5, 3, 3.5, 4, nil},
			},
			"PERFECT": {
				"fishing_speed": []any{2.5, 2.5, 3.5, 4, 4.5, 5, nil},
			},
		},
	},
	"CITRINE": {
		Name:  "Citrine",
		Color: "4",
		Stats: map[string]gemstoneStats{
			"ROUGH": {
				"foraging_fortune": []any{0.5, 1, 1.5, 2, 2.5, 3, nil},
			},
			"FLAWED": {
				"foraging_fortune": []any{1, 1.5, 2, 2.5, 3, 4, nil},
			},
			"FINE": {
				"foraging_fortune": []any{1.5, 2, 3, 4, 5, 6, nil},
			},
			"FLAWLESS": {
				"foraging_fortune": []any{2, 3, 4, 5, 6, 8, nil},
			},
			"PERFECT": {
				"foraging_fortune": []any{3, 4, 5, 6, 8, 10, nil},
			},
		},
	},
	"ONYX": {
		Name:  "Onyx",
		Color: "8",
		Stats: map[string]gemstoneStats{
			"ROUGH": {
				"critical_damage": []any{1, 1, 2, 2, 3, 4, nil},
			},
			"FLAWED": {
				"critical_damage": []any{2, 2, 3, 3, 4, 6, nil},
			},
			"FINE": {
				"critical_damage": []any{3, 3, 4, 5, 6, 8, nil},
			},
			"FLAWLESS": {
				"critical_damage": []any{4, 5, 6, 7, 8, 10, nil},
			},
			"PERFECT": {
				"critical_damage": []any{5, 6, 7, 8, 10, 12, nil},
			},
		},
	},
	"PERIDOT": {
		Name:  "Peridot",
		Color: "2",
		Stats: map[string]gemstoneStats{
			"ROUGH": {
				"farming_fortune": []any{0.5, 1, 1.5, 2, 2.5, 3, nil},
			},
			"FLAWED": {
				"farming_fortune": []any{1, 1.5, 2, 2.5, 3, 4, nil},
			},
			"FINE": {
				"farming_fortune": []any{1.5, 2, 3, 4, 5, 6, nil},
			},
			"FLAWLESS": {
				"farming_fortune": []any{2, 3, 4, 5, 6, 8, nil},
			},
			"PERFECT": {
				"farming_fortune": []any{3, 4, 5, 6, 8, 10, nil},
			},
		},
	},
}

type EnchantmentLadder struct {
	Name   string `json:"name"`
	Ladder []int  `json:"ladder"`
}

var ENCHANTMENT_LADDERS = map[string]EnchantmentLadder{
	"hecatomb_s_runs": {
		Name:   "Hecatomb Runs",
		Ladder: []int{2, 5, 10, 20, 30, 40, 60, 80, 100},
	},
	"champion_combat_xp": {
		Name:   "Champion XP",
		Ladder: []int{50000, 100000, 250000, 500000, 1000000, 1500000, 2000000, 2500000, 3000000},
	},
	"farmed_cultivating": {
		Name:   "Cultivating Crops",
		Ladder: []int{1000, 5000, 25000, 100000, 300000, 1500000, 5000000, 20000000, 100000000},
	},
	"expertise_kills": {
		Name:   "Expertise Kills",
		Ladder: []int{50, 100, 250, 500, 1000, 2500, 5500, 10000, 15000},
	},
	"compact_blocks": {
		Name:   "Ores Mined",
		Ladder: []int{100, 500, 1500, 5000, 15000, 50000, 150000, 500000, 1000000},
	},
}

var ITEMS = map[string]models.ProcessedHypixelItem{}

var BLACKLISTED_HEX_ARMOR_PIECES = []string{
	"VELVET_TOP_HAT",
	"CASHMERE_JACKET",
	"SATIN_TROUSERS",
	"OXFORD_SHOES",
}

var ARMOR_TYPES = []string{"helmet", "chestplate", "leggings", "boots"}
