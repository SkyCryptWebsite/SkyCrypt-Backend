package constants

import (
	"sort"

	"skycrypt/src/models"
)

var PLAYER_STATS = map[string]models.StatsInfo{
	// Combat
	"health":             {"base": 100},
	"defense":            {"base": 0},
	"true_defense":       {"base": 0},
	"strength":           {"base": 0},
	"critical_chance":    {"base": 30},
	"critical_damage":    {"base": 50},
	"bonus_attack_speed": {"base": 0},
	"ferocity":           {"base": 0},
	"swing_range":        {"base": 3},
	"intelligence":       {"base": 0},
	"ability_damage":     {"base": 0},
	"health_regen":       {"base": 100},
	"vitality":           {"base": 100},
	"mending":            {"base": 100},

	// Mining
	"breaking_power":        {"base": 0},
	"mining_speed":          {"base": 0},
	"mining_spread":         {"base": 10000},
	"gemstone_spread":       {"base": 0},
	"pristine":              {"base": 0},
	"mining_fortune":        {"base": 0},
	"ore_fortune":           {"base": 0},
	"block_fortune":         {"base": 0},
	"dwarven_metal_fortune": {"base": 0},
	"gemstone_fortune":      {"base": 0},

	// Farming
	"bonus_pest_chance":   {"base": 0},
	"overbloom":           {"base": 0},
	"farming_fortune":     {"base": 0},
	"wheat_fortune":       {"base": 0},
	"carrot_fortune":      {"base": 0},
	"potato_fortune":      {"base": 0},
	"pumpkin_fortune":     {"base": 0},
	"sugar_cane_fortune":  {"base": 0},
	"melon_slice_fortune": {"base": 0},
	"cactus_fortune":      {"base": 0},
	"cocoa_beans_fortune": {"base": 0},
	"mushroom_fortune":    {"base": 0},
	"nether_wart_fortune": {"base": 0},
	"sunflower_fortune":   {"base": 0},
	"moonflower_fortune":  {"base": 0},
	"wild_rose_fortune":   {"base": 0},

	// Foraging
	"sweep":            {"base": 0},
	"foraging_fortune": {"base": 0},
	"fig_fortune":      {"base": 0},
	"mangrove_fortune": {"base": 0},

	// Fishing
	"fishing_speed":       {"base": 0},
	"sea_creature_chance": {"base": 20},
	"double_hook_chance":  {"base": 0},
	"trophy_chance":       {"base": 150},
	"treasure_chance":     {"base": 100},

	// Hunting
	"pull":            {"base": 0},
	"hunting_fortune": {"base": 0},

	// Miscellaneous
	"speed":               {"base": 100},
	"magic_find":          {"base": 0},
	"pet_luck":            {"base": 0},
	"heat_resistance":     {"base": 0},
	"cold_resistance":     {"base": 0},
	"respiration":         {"base": 30},
	"pressure_resistance": {"base": 0},
	"tracking":            {"base": 0},
	"fear":                {"base": 0},

	// Wisdom
	"combat_wisdom":       {"base": 0},
	"farming_wisdom":      {"base": 0},
	"fishing_wisdom":      {"base": 0},
	"mining_wisdom":       {"base": 0},
	"foraging_wisdom":     {"base": 0},
	"enchanting_wisdom":   {"base": 0},
	"alchemy_wisdom":      {"base": 0},
	"carpentry_wisdom":    {"base": 0},
	"runecrafting_wisdom": {"base": 0},
	"taming_wisdom":       {"base": 0},
	"social_wisdom":       {"base": 0},
	"hunting_wisdom":      {"base": 0},

	// Rift
	"rift_time":         {"base": 480},
	"rift_damage":       {"base": 20},
	"rift_intelligence": {"base": 0},
	"mana_regen":        {"base": 0},
	"rift_speed":        {"base": 100},
	"hearts":            {"base": 10},
}

var STATS_DATA = []models.StatData{
	// ---------------------------------------------------------------------
	// Combat
	// ---------------------------------------------------------------------

	{
		ID:          "health",
		Name:        "Health",
		NameLore:    "Health",
		NameShort:   "Health",
		NameTiny:    "HP",
		Symbol:      "",
		Suffix:      "",
		Color:       "c",
		Category:    "combat",
		Description: "The Max Health of the player.",
	},
	{
		ID:          "defense",
		Name:        "Defense",
		NameLore:    "Defense",
		NameShort:   "Defense",
		NameTiny:    "Def",
		Symbol:      "",
		Suffix:      "",
		Color:       "a",
		Category:    "combat",
		Description: "Reduces damage taken from Damage.",
	},
	{
		ID:          "true_defense",
		Name:        "True Defense",
		NameLore:    "True Defense",
		NameShort:   "True Defense",
		NameTiny:    "TD",
		Symbol:      "",
		Suffix:      "",
		Color:       "f",
		Category:    "combat",
		Description: "Reduces damage taken from True Damage.",
	},
	{
		ID:          "strength",
		Name:        "Strength",
		NameLore:    "Strength",
		NameShort:   "Strength",
		NameTiny:    "Str",
		Symbol:      "",
		Suffix:      "",
		Color:       "c",
		Category:    "combat",
		Description: "Increases Damage dealt by the player.",
	},
	{
		ID:          "critical_chance",
		Name:        "Crit Chance",
		NameLore:    "Crit Chance",
		NameShort:   "Crit Chance",
		NameTiny:    "CC",
		Symbol:      "",
		Suffix:      "%",
		Color:       "9",
		Category:    "combat",
		Description: "Increases the chance of the player landing a critical hit.",
		Percent:     true,
	},
	{
		ID:          "critical_damage",
		Name:        "Crit Damage",
		NameLore:    "Crit Damage",
		NameShort:   "Crit Damage",
		NameTiny:    "CD",
		Symbol:      "",
		Suffix:      "%",
		Color:       "9",
		Category:    "combat",
		Description: "Increases Damage dealt by the player on critical hits.",
		Percent:     true,
	},
	{
		ID:          "bonus_attack_speed",
		Name:        "Attack Speed",
		NameLore:    "Attack Speed",
		NameShort:   "Attack Speed",
		NameTiny:    "Atk",
		Symbol:      "",
		Suffix:      "",
		Color:       "e",
		Category:    "combat",
		Description: "Determines the cooldown between attacks. The higher the value the faster the attack.",
		Cap:         intPtr(100),
	},
	{
		ID:          "ability_damage",
		Name:        "Ability Damage",
		NameLore:    "Ability Damage",
		NameShort:   "Ability Damage",
		NameTiny:    "AD",
		Symbol:      "",
		Suffix:      "%",
		Color:       "c",
		Category:    "combat",
		Description: "Multiplies the player's magic damage.",
		Percent:     true,
	},
	{
		ID:          "ferocity",
		Name:        "Ferocity",
		NameLore:    "Ferocity",
		NameShort:   "Ferocity",
		NameTiny:    "Frc",
		Symbol:      "",
		Suffix:      "",
		Color:       "c",
		Category:    "combat",
		Description: "Gives a chance, as a percentage, for player hits to count as double or more. Each 100 Ferocity guarantees an additional hit.",
		Cap:         intPtr(500),
	},
	{
		ID:          "swing_range",
		Name:        "Swing Range",
		NameLore:    "Swing Range",
		NameShort:   "Swing Range",
		NameTiny:    "SR",
		Symbol:      "",
		Suffix:      "",
		Color:       "e",
		Category:    "combat",
		Description: "Determines the player's melee hit range.",
		Cap:         intPtr(15),
	},
	{
		ID:          "intelligence",
		Name:        "Intelligence",
		NameLore:    "Intelligence",
		NameShort:   "Intelligence",
		NameTiny:    "Int",
		Symbol:      "",
		Suffix:      "",
		Color:       "b",
		Category:    "combat",
		Description: "Increases the player's Mana and magic damage.",
	},
	{
		ID:          "health_regen",
		Name:        "Health Regen",
		NameLore:    "Health Regen",
		NameShort:   "Health Regen",
		NameTiny:    "HPR",
		Symbol:      "",
		Suffix:      "",
		Color:       "c",
		Category:    "combat",
		Description: "Increases the natural regeneration of the player's Health.",
	},
	{
		ID:          "vitality",
		Name:        "Vitality",
		NameLore:    "Vitality",
		NameShort:   "Vitality",
		NameTiny:    "Vit",
		Symbol:      "",
		Suffix:      "",
		Color:       "5",
		Category:    "combat",
		Description: "Used as resource for healing abilities.",
	},
	{
		ID:          "mending",
		Name:        "Mending",
		NameLore:    "Mending",
		NameShort:   "Mending",
		NameTiny:    "Mend",
		Symbol:      "",
		Suffix:      "",
		Color:       "a",
		Category:    "combat",
		Description: "Increases the amount of healing applied by the player to others.",
	},

	// ---------------------------------------------------------------------
	// Mining
	// ---------------------------------------------------------------------

	{
		ID:          "breaking_power",
		Name:        "Breaking Power",
		NameLore:    "Breaking Power",
		NameShort:   "Breaking Power",
		NameTiny:    "BP",
		Symbol:      "",
		Suffix:      "",
		Color:       "2",
		Category:    "mining",
		Description: "Allows mining stronger blocks.",
	},
	{
		ID:          "mining_speed",
		Name:        "Mining Speed",
		NameLore:    "Mining Speed",
		NameShort:   "Mining Speed",
		NameTiny:    "MngSpd",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "mining",
		Description: "Increases the speed of mining blocks.",
	},
	{
		ID:                      "mining_spread",
		Name:                    "Mining Spread",
		NameLore:                "Mining Spread",
		NameShort:               "",
		NameTiny:                "MS",
		Symbol:                  "",
		Suffix:                  "",
		Color:                   "e",
		Category:                "mining",
		Description:             "Allows breaking adjacent blocks. Every 100 Mining Spread increases the amount of blocks broken. Does not work on Gemstones.",
		DisabledOnPrivateIsland: true,
	},
	{
		ID:          "gemstone_spread",
		Name:        "Gemstone Spread",
		NameLore:    "Gemstone Spread",
		NameShort:   "",
		NameTiny:    "GS",
		Symbol:      "",
		Suffix:      "",
		Color:       "e",
		Category:    "mining",
		Description: "Allows breaking adjacent Gemstone blocks.",
	},
	{
		ID:          "pristine",
		Name:        "Pristine",
		NameLore:    "Pristine",
		NameShort:   "Pristine",
		NameTiny:    "Prs",
		Symbol:      "",
		Suffix:      "",
		Color:       "5",
		Category:    "mining",
		Description: "Increases the chance to increase the quality of Gemstones when mined.",
	},
	{
		ID:                      "mining_fortune",
		Name:                    "Mining Fortune",
		NameLore:                "Mining Fortune",
		NameShort:               "Mining Fortune",
		NameTiny:                "MngFrt",
		Symbol:                  "",
		Suffix:                  "",
		Color:                   "6",
		Category:                "mining",
		Description:             "Increases the chance to get more mining drops. Every 100 Mining Fortune grants an additional drop.",
		DisabledOnPrivateIsland: true,
	},
	{
		ID:          "ore_fortune",
		Name:        "Ore Fortune",
		NameLore:    "Ore Fortune",
		NameShort:   "Ore Fortune",
		NameTiny:    "OreFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "mining",
		Description: "Increases Mining Fortune for Ores.",
	},
	{
		ID:          "block_fortune",
		Name:        "Block Fortune",
		NameLore:    "Block Fortune",
		NameShort:   "Block Fortune",
		NameTiny:    "BFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "mining",
		Description: "Increases Mining Fortune for Blocks.",
	},
	{
		ID:          "dwarven_metal_fortune",
		Name:        "Dwarven Metal Fortune",
		NameLore:    "Dwarven Metal Fortune",
		NameShort:   "Dwarven Metal Fortune",
		NameTiny:    "DMFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "mining",
		Description: "Increases Mining Fortune for Dwarven Metals.",
	},
	{
		ID:          "gemstone_fortune",
		Name:        "Gemstone Fortune",
		NameLore:    "Gemstone Fortune",
		NameShort:   "Gemstone Fortune",
		NameTiny:    "GFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "mining",
		Description: "Increases Mining Fortune for Gemstones.",
	},

	// ---------------------------------------------------------------------
	// Farming
	// ---------------------------------------------------------------------

	{
		ID:          "bonus_pest_chance",
		Name:        "Bonus Pest Chance",
		NameLore:    "Bonus Pest Chance",
		NameShort:   "Bonus Pest Chance",
		NameTiny:    "BPC",
		Symbol:      "",
		Suffix:      "",
		Color:       "2",
		Category:    "farming",
		Description: "Grants a chance for more Pests to spawn together. Every 100 Bonus Pest Chance adds another pest as well as increases the total amount of pests that can spawn before the player starts losing farming fortune.",
	},
	{
		ID:                      "overbloom",
		Name:                    "Overbloom",
		NameLore:                "Overbloom",
		NameShort:               "Overbloom",
		NameTiny:                "OB",
		Symbol:                  "",
		Suffix:                  "",
		Color:                   "a",
		Category:                "farming",
		Description:             "Increases the chance to drop Rare Crops.",
		DisabledOnPrivateIsland: true,
	},
	{
		ID:          "farming_fortune",
		Name:        "Farming Fortune",
		NameLore:    "Farming Fortune",
		NameShort:   "Farming Fortune",
		NameTiny:    "FrmFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases the chance to get more farming drops. Every 100 Farming Fortune grants an additional drop.",
	},
	{
		ID:          "wheat_fortune",
		Name:        "Wheat Fortune",
		NameLore:    "Wheat Fortune",
		NameShort:   "",
		NameTiny:    "WFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Wheat.",
	},
	{
		ID:          "carrot_fortune",
		Name:        "Carrot Fortune",
		NameLore:    "Carrot Fortune",
		NameShort:   "",
		NameTiny:    "CFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Carrots.",
	},
	{
		ID:          "potato_fortune",
		Name:        "Potato Fortune",
		NameLore:    "Potato Fortune",
		NameShort:   "",
		NameTiny:    "PFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Potatoes.",
	},
	{
		ID:          "pumpkin_fortune",
		Name:        "Pumpkin Fortune",
		NameLore:    "Pumpkin Fortune",
		NameShort:   "",
		NameTiny:    "PkFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Pumpkins.",
	},
	{
		ID:          "sugar_cane_fortune",
		Name:        "Sugar Cane Fortune",
		NameLore:    "Sugar Cane Fortune",
		NameShort:   "",
		NameTiny:    "SCFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Sugar Canes.",
	},
	{
		ID:          "melon_slice_fortune",
		Name:        "Melon Slice Fortune",
		NameLore:    "Melon Slice Fortune",
		NameShort:   "",
		NameTiny:    "MSFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Melon Slices.",
	},
	{
		ID:          "cactus_fortune",
		Name:        "Cactus Fortune",
		NameLore:    "Cactus Fortune",
		NameShort:   "",
		NameTiny:    "CFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Cactus.",
	},
	{
		ID:          "cocoa_beans_fortune",
		Name:        "Cocoa Beans Fortune",
		NameLore:    "Cocoa Beans Fortune",
		NameShort:   "",
		NameTiny:    "CBFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Cocoa Beans.",
	},
	{
		ID:          "mushroom_fortune",
		Name:        "Mushroom Fortune",
		NameLore:    "Mushroom Fortune",
		NameShort:   "",
		NameTiny:    "MsFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Mushrooms.",
	},
	{
		ID:          "nether_wart_fortune",
		Name:        "Nether Wart Fortune",
		NameLore:    "Nether Wart Fortune",
		NameShort:   "",
		NameTiny:    "NWFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Nether Warts.",
	},
	{
		ID:          "sunflower_fortune",
		Name:        "Sunflower Fortune",
		NameLore:    "Sunflower Fortune",
		NameShort:   "",
		NameTiny:    "SWFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Sunflowers.",
	},
	{
		ID:          "moonflower_fortune",
		Name:        "Moonflower Fortune",
		NameLore:    "Moonflower Fortune",
		NameShort:   "",
		NameTiny:    "MWFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Moonflowers.",
	},
	{
		ID:          "wild_rose_fortune",
		Name:        "Wild Rose Fortune",
		NameLore:    "Wild Rose Fortune",
		NameShort:   "",
		NameTiny:    "WRFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "farming",
		Description: "Increases Farming Fortune for Wild Roses.",
	},

	// ---------------------------------------------------------------------
	// Foraging
	// ---------------------------------------------------------------------

	{
		ID:                      "sweep",
		Name:                    "Sweep",
		NameLore:                "Sweep",
		NameShort:               "Sweep",
		NameTiny:                "Swp",
		Symbol:                  "",
		Suffix:                  "",
		Color:                   "2",
		Category:                "foraging",
		Description:             "Allows breaking more logs. The tougher the tree, the more Sweep it requires to break a log.",
		DisabledOnPrivateIsland: true,
	},
	{
		ID:          "foraging_fortune",
		Name:        "Foraging Fortune",
		NameLore:    "Foraging Fortune",
		NameShort:   "Foraging Fortune",
		NameTiny:    "FrgFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "foraging",
		Description: "Increases the chance to get more foraging drops. Every 100 Foraging Fortune grants an additional drop.",
	},
	{
		ID:          "fig_fortune",
		Name:        "Fig Fortune",
		NameLore:    "Fig Fortune",
		NameShort:   "Fig Fortune",
		NameTiny:    "FigFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "foraging",
		Description: "Increases Foraging Fortune for Fig Logs.",
	},
	{
		ID:          "mangrove_fortune",
		Name:        "Mangrove Fortune",
		NameLore:    "Mangrove Fortune",
		NameShort:   "Mangrove Fortune",
		NameTiny:    "MngFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "foraging",
		Description: "Increases Foraging Fortune for Mangrove Logs.",
	},

	// ---------------------------------------------------------------------
	// Fishing
	// ---------------------------------------------------------------------

	{
		ID:          "fishing_speed",
		Name:        "Fishing Speed",
		NameLore:    "Fishing Speed",
		NameShort:   "Fishing Speed",
		NameTiny:    "FS",
		Symbol:      "",
		Suffix:      "",
		Color:       "b",
		Category:    "fishing",
		Description: "Determines how fast the player can catch a fish.",
		Cap:         intPtr(300),
	},
	{
		ID:          "sea_creature_chance",
		Name:        "Sea Creature Chance",
		NameLore:    "Sea Creature Chance",
		NameShort:   "SC Chance",
		NameTiny:    "SCC",
		Symbol:      "",
		Suffix:      "%",
		Color:       "3",
		Category:    "fishing",
		Description: "Determines the chance of catching a Sea Creature.",
		Percent:     true,
	},
	{
		ID:          "double_hook_chance",
		Name:        "Double Hook Chance",
		NameLore:    "Double Hook Chance",
		NameShort:   "Double Hook Chance",
		NameTiny:    "DHC",
		Symbol:      "",
		Suffix:      "%",
		Color:       "9",
		Category:    "fishing",
		Description: "Grants a chance to catch an additional Sea Creature.",
		Percent:     true,
	},
	{
		ID:          "trophy_chance",
		Name:        "Trophy Chance",
		NameLore:    "Trophy Chance",
		NameShort:   "Trophy Chance",
		NameTiny:    "TC",
		Symbol:      "",
		Suffix:      "%",
		Color:       "6",
		Category:    "fishing",
		Description: "Increases the chance of catching a Trophy Fish.",
		Percent:     true,
	},
	{
		ID:          "treasure_chance",
		Name:        "Treasure Chance",
		NameLore:    "Treasure Chance",
		NameShort:   "Treasure Chance",
		NameTiny:    "TC",
		Symbol:      "",
		Suffix:      "%",
		Color:       "6",
		Category:    "fishing",
		Description: "Increases the chance of catching a Fishing Treasure.",
		Percent:     true,
	},

	// ---------------------------------------------------------------------
	// Hunting
	// ---------------------------------------------------------------------

	{
		ID:          "pull",
		Name:        "Pull",
		NameLore:    "Pull",
		NameShort:   "Pull",
		NameTiny:    "Pull",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "hunting",
		Description: "Dictates what fish can be grabbed and how long it takes using a Fishing Net.",
	},
	{
		ID:          "hunting_fortune",
		Name:        "Hunting Fortune",
		NameLore:    "Hunting Fortune",
		NameShort:   "Hunting Fortune",
		NameTiny:    "HntFrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "6",
		Category:    "hunting",
		Description: "Increases the chance to get more Attribute Shards. Every 100 Hunting Fortune grants an additional shard.",
	},

	// ---------------------------------------------------------------------
	// Miscellaneous
	// ---------------------------------------------------------------------

	{
		ID:          "speed",
		Name:        "Speed",
		NameLore:    "Speed",
		NameShort:   "Speed",
		NameTiny:    "Spd",
		Symbol:      "",
		Suffix:      "",
		Color:       "f",
		Category:    "misc",
		Description: "Increases player's movement speed.",
		Cap:         intPtr(400),
	},
	{
		ID:          "magic_find",
		Name:        "Magic Find",
		NameLore:    "Magic Find",
		NameShort:   "Magic Find",
		NameTiny:    "MF",
		Symbol:      "",
		Suffix:      "",
		Color:       "b",
		Category:    "misc",
		Description: "Increases the chance to find rare items from monsters and bosses.",
		Cap:         intPtr(900),
	},
	{
		ID:          "pet_luck",
		Name:        "Pet Luck",
		NameLore:    "Pet Luck",
		NameShort:   "Pet Luck",
		NameTiny:    "PL",
		Symbol:      "",
		Suffix:      "",
		Color:       "d",
		Category:    "misc",
		Description: "Increases the chance of obtaining a Pet from monsters and bosses.",
	},
	{
		ID:          "heat_resistance",
		Name:        "Heat Resistance",
		NameLore:    "Heat Resistance",
		NameShort:   "Heat Resistance",
		NameTiny:    "HRes",
		Symbol:      "",
		Suffix:      "",
		Color:       "c",
		Category:    "misc",
		Description: "Slows down Heat gained in Magma Fields.",
	},
	{
		ID:          "cold_resistance",
		Name:        "Cold Resistance",
		NameLore:    "Cold Resistance",
		NameShort:   "Cold Resistance",
		NameTiny:    "CRes",
		Symbol:      "",
		Suffix:      "",
		Color:       "b",
		Category:    "misc",
		Description: "Slows down Cold gained in Glacite Tunnels and Glacite Mineshafts.",
	},
	{
		ID:          "respiration",
		Name:        "Respiration",
		NameLore:    "Respiration",
		NameShort:   "Respiration",
		NameTiny:    "Resp",
		Symbol:      "",
		Suffix:      "",
		Color:       "b",
		Category:    "misc",
		Description: "Extends underwater breathing time.",
	},
	{
		ID:          "pressure_resistance",
		Name:        "Pressure Resistance",
		NameLore:    "Pressure Resistance",
		NameShort:   "Pressure Resistance",
		NameTiny:    "PRes",
		Symbol:      "",
		Suffix:      "",
		Color:       "b",
		Category:    "misc",
		Description: "Reduces the effects of Pressure when diving.",
	},
	{
		ID:          "tracking",
		Name:        "Tracking",
		NameLore:    "Tracking",
		NameShort:   "Tracking",
		NameTiny:    "Trk",
		Symbol:      "",
		Suffix:      "",
		Color:       "a",
		Category:    "misc",
		Description: "Increases the chance to find Elusive mobs.",
	},
	{
		ID:          "fear",
		Name:        "Fear",
		NameLore:    "Fear",
		NameShort:   "Fear",
		NameTiny:    "Fr",
		Symbol:      "",
		Suffix:      "",
		Color:       "5",
		Category:    "misc",
		Description: "Spawns Primal Fears more often and reduces damage dealt by them during the Great Spook.",
	},

	// ---------------------------------------------------------------------
	// Wisdom
	// ---------------------------------------------------------------------

	{
		ID:          "combat_wisdom",
		Name:        "Combat Wisdom",
		NameLore:    "Combat Wisdom",
		NameShort:   "Combat Wisdom",
		NameTiny:    "CW",
		Symbol:      "☯",
		Suffix:      "",
		Color:       "3",
		Category:    "wisdom",
		Description: "Increases how much Combat XP the player gains.",
	},
	{
		ID:          "farming_wisdom",
		Name:        "Farming Wisdom",
		NameLore:    "Farming Wisdom",
		NameShort:   "Farming Wisdom",
		NameTiny:    "FW",
		Symbol:      "☯",
		Suffix:      "",
		Color:       "3",
		Category:    "wisdom",
		Description: "Increases how much Farming XP the player gains.",
	},
	{
		ID:          "fishing_wisdom",
		Name:        "Fishing Wisdom",
		NameLore:    "Fishing Wisdom",
		NameShort:   "Fishing Wisdom",
		NameTiny:    "FW",
		Symbol:      "☯",
		Suffix:      "",
		Color:       "3",
		Category:    "wisdom",
		Description: "Increases how much Fishing XP the player gains.",
	},
	{
		ID:          "mining_wisdom",
		Name:        "Mining Wisdom",
		NameLore:    "Mining Wisdom",
		NameShort:   "Mining Wisdom",
		NameTiny:    "MW",
		Symbol:      "☯",
		Suffix:      "",
		Color:       "3",
		Category:    "wisdom",
		Description: "Increases how much Mining XP the player gains.",
	},
	{
		ID:          "foraging_wisdom",
		Name:        "Foraging Wisdom",
		NameLore:    "Foraging Wisdom",
		NameShort:   "Foraging Wisdom",
		NameTiny:    "FW",
		Symbol:      "☯",
		Suffix:      "",
		Color:       "3",
		Category:    "wisdom",
		Description: "Increases how much Foraging XP the player gains.",
	},
	{
		ID:          "enchanting_wisdom",
		Name:        "Enchanting Wisdom",
		NameLore:    "Enchanting Wisdom",
		NameShort:   "Enchanting Wisdom",
		NameTiny:    "EW",
		Symbol:      "☯",
		Suffix:      "",
		Color:       "3",
		Category:    "wisdom",
		Description: "Increases how much Enchanting XP the player gains.",
	},
	{
		ID:          "alchemy_wisdom",
		Name:        "Alchemy Wisdom",
		NameLore:    "Alchemy Wisdom",
		NameShort:   "Alchemy Wisdom",
		NameTiny:    "AW",
		Symbol:      "☯",
		Suffix:      "",
		Color:       "3",
		Category:    "wisdom",
		Description: "Increases how much Alchemy XP the player gains.",
	},
	{
		ID:          "carpentry_wisdom",
		Name:        "Carpentry Wisdom",
		NameLore:    "Carpentry Wisdom",
		NameShort:   "Carpentry Wisdom",
		NameTiny:    "CW",
		Symbol:      "☯",
		Suffix:      "",
		Color:       "3",
		Category:    "wisdom",
		Description: "Increases how much Carpentry XP the player gains.",
	},
	{
		ID:          "runecrafting_wisdom",
		Name:        "Runecrafting Wisdom",
		NameLore:    "Runecrafting Wisdom",
		NameShort:   "Runecrafting Wisdom",
		NameTiny:    "RW",
		Symbol:      "☯",
		Suffix:      "",
		Color:       "3",
		Category:    "wisdom",
		Description: "Increases how much Runecrafting XP the player gains.",
	},
	{
		ID:          "taming_wisdom",
		Name:        "Taming Wisdom",
		NameLore:    "Taming Wisdom",
		NameShort:   "Taming Wisdom",
		NameTiny:    "TW",
		Symbol:      "☯",
		Suffix:      "",
		Color:       "3",
		Category:    "wisdom",
		Description: "Increases how much Taming XP the player gains.",
	},
	{
		ID:          "social_wisdom",
		Name:        "Social Wisdom",
		NameLore:    "Social Wisdom",
		NameShort:   "Social Wisdom",
		NameTiny:    "SW",
		Symbol:      "☯",
		Suffix:      "",
		Color:       "3",
		Category:    "wisdom",
		Description: "Increases how much Social XP the player gains.",
	},
	{
		ID:          "hunting_wisdom",
		Name:        "Hunting Wisdom",
		NameLore:    "Hunting Wisdom",
		NameShort:   "Hunting Wisdom",
		NameTiny:    "HW",
		Symbol:      "☯",
		Suffix:      "",
		Color:       "3",
		Category:    "wisdom",
		Description: "Increases how much Hunting XP the player gains.",
	},

	// ---------------------------------------------------------------------
	// Rift
	// ---------------------------------------------------------------------

	{
		ID:          "rift_time",
		Name:        "Rift Time",
		NameLore:    "Rift Time",
		NameShort:   "Rift Time",
		NameTiny:    "RT",
		Symbol:      "",
		Suffix:      "",
		Color:       "a",
		Category:    "rift",
		Description: "Increases how long you can stay in the Rift before being kicked out.",
	},
	{
		ID:          "rift_damage",
		Name:        "Rift Damage",
		NameLore:    "Rift Damage",
		NameShort:   "Rift Damage",
		NameTiny:    "RD",
		Symbol:      "",
		Suffix:      "",
		Color:       "c",
		Category:    "rift",
		Description: "Increases player's damage in the Rift Dimension.",
	},
	{
		ID:          "rift_intelligence",
		Name:        "Rift Intelligence",
		NameLore:    "Rift Intelligence",
		NameShort:   "Rift Intelligence",
		NameTiny:    "RInt",
		Symbol:      "",
		Suffix:      "",
		Color:       "b",
		Category:    "rift",
		Description: "Increases player's Mana and Mana Regen in the Rift Dimension.",
	},
	{
		ID:          "mana_regen",
		Name:        "Mana Regen",
		NameLore:    "Mana Regen",
		NameShort:   "Mana Regen",
		NameTiny:    "MPR",
		Symbol:      "",
		Suffix:      "",
		Color:       "b",
		Category:    "rift",
		Description: "Increases the regeneration of Mana inside of the Rift, separate from Mana Regeneration of the Overworld.",
	},
	{
		ID:          "rift_speed",
		Name:        "Rift Speed",
		NameLore:    "Rift Speed",
		NameShort:   "Rift Speed",
		NameTiny:    "RSpd",
		Symbol:      "",
		Suffix:      "",
		Color:       "f",
		Category:    "rift",
		Description: "Increases player's movement speed in the Rift Dimension.",
	},
	{
		ID:          "hearts",
		Name:        "Hearts",
		NameLore:    "Hearts",
		NameShort:   "Hearts",
		NameTiny:    "Hrt",
		Symbol:      "",
		Suffix:      "",
		Color:       "c",
		Category:    "rift",
		Description: "Increases player's survivability against certain Rift creatures, most notably the Riftstalker Bloodfiend.",
	},
	{
		ID:          "damage",
		Name:        "Damage",
		NameLore:    "Damage",
		NameShort:   "Damage",
		NameTiny:    "DMG",
		Symbol:      "❁",
		Suffix:      "",
		Color:       "c",
		Category:    "combat",
		Description: "Represents the player's damage output.",
	},
}

// intPtr is used for optional stat caps.
func intPtr(value int) *int {
	return &value
}

// -------------------------------------------------------------------------
// Stat bonuses
// -------------------------------------------------------------------------

var STATS_BONUS = map[string]map[int]map[string]float64{
	// Skills
	"skill_farming": {
		1:  {"health": 2, "farming_fortune": 4},
		15: {"health": 3, "farming_fortune": 4},
		20: {"health": 4, "farming_fortune": 4},
		26: {"health": 5, "farming_fortune": 4},
	},
	"skill_mining": {
		1:  {"defense": 1, "mining_fortune": 4},
		15: {"defense": 2, "mining_fortune": 4},
	},
	"skill_combat": {
		1: {"critical_chance": 0.5},
	},
	"skill_foraging": {
		1:  {"strength": 1, "foraging_fortune": 4},
		15: {"strength": 2, "foraging_fortune": 4},
	},
	"skill_fishing": {
		1:  {"health": 2},
		15: {"health": 3},
		20: {"health": 4},
		26: {"health": 5},
	},
	"skill_enchanting": {
		1:  {"intelligence": 1, "ability_damage": 0.5},
		15: {"intelligence": 2, "ability_damage": 0.5},
	},
	"skill_alchemy": {
		1:  {"intelligence": 1},
		15: {"intelligence": 2},
	},
	"skill_taming": {
		1: {"pet_luck": 1},
	},
	"skill_dungeoneering": {
		1:  {"health": 2},
		51: {"health": 0},
	},
	"skill_social":       {},
	"skill_carpentry":    {1: {"health": 1}},
	"skill_runecrafting": {},

	// Slayers
	"slayer_zombie": {
		1: {"health": 2},
		3: {"health": 3},
		5: {"health": 4},
		7: {"health": 5},
		8: {"health": 5, "health_regen": 50},
		9: {"health": 6},
	},
	"slayer_spider": {
		1: {"critical_damage": 1},
		5: {"critical_damage": 2},
		7: {"critical_damage": 2},
		8: {"critical_damage": 3},
	},
	"slayer_wolf": {
		1: {"speed": 1},
		2: {"health": 2},
		3: {"speed": 1},
		4: {"health": 2},
		5: {"critical_damage": 1},
		6: {"health": 3},
		7: {"critical_damage": 2},
		8: {"speed": 1},
		9: {"health": 5},
	},
	"slayer_enderman": {
		1: {"health": 1},
		2: {"intelligence": 2},
		3: {"health": 2},
		4: {"true_defense": 1},
		5: {"health": 3},
		6: {"intelligence": 5},
		7: {"health": 4},
		8: {"intelligence": 4},
		9: {"health": 5},
	},
	"slayer_blaze": {
		1: {"health": 3},
		2: {"strength": 1},
		3: {"health": 4},
		4: {"true_defense": 1},
		5: {"health": 5},
		6: {"strength": 2},
		7: {"health": 6},
		8: {"true_defense": 2},
		9: {"health": 7},
	},
}

func GetBonusStats(level int, statsBonus map[int]map[string]float64) map[string]float64 {
	bonus := make(map[string]float64)

	if statsBonus == nil {
		return bonus
	}

	steps := make([]int, 0, len(statsBonus))
	for k := range statsBonus {
		steps = append(steps, k)
	}

	sort.Ints(steps)

	if len(steps) == 0 {
		return bonus
	}

	for x := steps[0]; x <= len(statsBonus); x++ {
		if level < x {
			break
		}

		step := steps[0]
		for _, s := range steps {
			if s <= x {
				step = s
			}
		}

		stepBonuses := statsBonus[step]
		for statName, value := range stepBonuses {
			bonus[statName] += value
		}
	}

	return bonus
}

func GetBonusStat(level int, key string, max int) map[string]float64 {
	bonus := make(map[string]float64)

	objOfLevelBonuses, ok := STATS_BONUS[key]
	if !ok {
		return bonus
	}

	steps := make([]int, 0, len(objOfLevelBonuses))
	for k := range objOfLevelBonuses {
		steps = append(steps, k)
	}

	sort.Ints(steps)

	if len(steps) == 0 {
		return bonus
	}

	for x := steps[0]; x <= max; x++ {
		if level < x {
			break
		}

		step := steps[0]
		for i := len(steps) - 1; i >= 0; i-- {
			if steps[i] <= x {
				step = steps[i]
				break
			}
		}

		stepBonuses := objOfLevelBonuses[step]
		for statName, value := range stepBonuses {
			bonus[statName] += value
		}
	}

	return bonus
}

func GetStatData(id string) (StatData, bool) {
	for _, stat := range STATS_DATA {
		if stat.ID == id {
			return stat, true
		}
	}

	return StatData{}, false
}
