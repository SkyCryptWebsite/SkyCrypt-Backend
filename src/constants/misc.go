package constants

import (
	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

type essence struct {
	Name    string
	Texture string
}

var ESSENCE = map[string]essence{
	"ice": {
		Name:    "Ice",
		Texture: "/api/head/ddba642efffa13ec3730eafc5914ab68115c1f998803f74452e2e0cd26af0b8",
	},
	"wither": {
		Name:    "Wither",
		Texture: "/api/head/c4db4adfa9bf48ff5d41707ae34ea78bd2371659fcd8cd8934749af4cce9b",
	},
	"spider": {
		Name:    "Spider",
		Texture: "/api/head/16617131250e578333a441fdf4a5b8c62163640a9d06cd67db89031d03accf6",
	},
	"undead": {
		Name:    "Undead",
		Texture: "/api/head/71d7c816fc8c636d7f50a93a0ba7aaeff06c96a561645e9eb1bef391655c531",
	},
	"diamond": {
		Name:    "Diamond",
		Texture: "/api/head/964f25cfff754f287a9838d8efe03998073c22df7a9d3025c425e3ed7ff52c20",
	},
	"dragon": {
		Name:    "Dragon",
		Texture: "/api/head/33ff416aa8bec1665b92701fbe68a4effff3d06ed9147454fa77712dd6079b33",
	},
	"gold": {
		Name:    "Gold",
		Texture: "/api/head/8816606260779b23ed15f87c56c932240db745f86f683d1f4deb83a4a125fa7b",
	},
	"crimson": {
		Name:    "Crimson",
		Texture: "/api/head/67c41930f8ff0f2b0430e169ae5f38e984df1244215705c6f173862844543e9d",
	},
}

var RACE_NAMES = map[string]string{
	"crystal_core":    "Crystal Core",
	"giant_mushroom":  "Giant Mushroom",
	"precursor_ruins": "Precursor Ruins",
	"foraging_race":   "Foraging",
	"end_race":        "End",
	"chicken_race_2":  "Chicken",
	"rift_race":       "Rift",
}

var MILESTONE_RARITIES = []string{"common", "uncommon", "rare", "epic", "legendary"}

var PET_MILESTONES = map[string][]int{
	"sea_creatures_killed": {250, 1000, 2500, 5000, 10000},
	"ores_mined":           {2500, 7500, 20000, 100000, 250000},
}

var PROFILE_UPGRADES = map[string]int{
	"island_size":     10,
	"minion_slots":    5,
	"guests_count":    5,
	"coop_slots":      3,
	"coins_allowance": 5,
}

var CLAIMABLE_ITEMS = map[string]string{
	"claimed_potato_talisman":       "Potato Talisman",
	"claimed_potato_basket":         "Potato Basket",
	"claim_potato_war_silver_medal": "Silver Medal (Potato War)",
	"claim_potato_war_crown":        "Crown (Potato War)",
	"skyblock_free_cookie":          "Free Booster Cookie",
	"claimed_century_cake":          "Century Cake",
	"claimed_century_cake200":       "Century Cake (Year 200)",
}

var BANK_COOLDOWN = map[int]string{
	1: "20 minutes",
	2: "5 minutes",
	3: "None",
}

type consumableData struct {
	Name      string
	Texture   string
	Amount    func(userProfile *skycrypttypes.Member) int
	MaxAmount int
}

var CONSUMABLES = []consumableData{
	{
		Name:    "Teleporter Pill",
		Texture: "/api/item/TELEPORTER_PILL",
		Amount: func(userProfile *skycrypttypes.Member) int {
			if userProfile.ItemData.TeleporterPillConsumed {
				return 1
			}
			return 0
		},
		MaxAmount: 1,
	},
	{
		Name:    "Metaphysical Serums Drank",
		Texture: "/api/item/METAPHYSICAL_SERUM",
		Amount: func(userProfile *skycrypttypes.Member) int {
			return userProfile.Experimentation.SerumsDrank
		},
		MaxAmount: 3,
	},
	{
		Name:    "Reaper Peppers Eaten",
		Texture: "/api/item/REAPER_PEPPER",
		Amount: func(userProfile *skycrypttypes.Member) int {
			return userProfile.PlayerData.ReaperPeppersEaten
		},
		MaxAmount: 5,
	},
	{
		Name:    "McGrubber's Burgers Eaten",
		Texture: "/api/item/MCGRUBBER_BURGER",
		Amount: func(userProfile *skycrypttypes.Member) int {
			return userProfile.Rift.Castle.GrubberStacks
		},
		MaxAmount: 5,
	},
	{
		Name:    "Wriggling Larvae Eaten",
		Texture: "/api/item/WRIGGLING_LARVA",
		Amount: func(userProfile *skycrypttypes.Member) int {
			return userProfile.Garden.LarvaConsumed
		},
		MaxAmount: 5,
	},
	{
		Name:    "Refined Bottles of Jyrre Drank",
		Texture: "/api/item/REFINED_BOTTLE_OF_JYRRE",
		Amount: func(userProfile *skycrypttypes.Member) int {
			return userProfile.WinterPlayerData.RefinedJyrreUses
		},
		MaxAmount: 5,
	},
	/*{
		Name:    "Vial of Venom",
		Texture: "/api/item/VIAL_OF_VENOM",
		Amount: func(userProfile *skycrypttypes.Member) int {
			// INFO: Missing from the API
		},
		MaxAmount: 5,
	},*/
	/*{
		Name:    "Festering Maggot",
		Texture: "/api/item/FESTERING_MAGGOT",
		Amount: func(userProfile *skycrypttypes.Member) int {
			// INFO: Missing from the API
		},
		MaxAmount: 5,
	},*/
	{
		Name:    "Refined Dark Cacao Truffles Consumed",
		Texture: "/api/item/REFINED_DARK_CACAO_TRUFFLE",
		Amount: func(userProfile *skycrypttypes.Member) int {
			return userProfile.Events.Easter.RefinedDarkCacaoTruffles
		},
		MaxAmount: 5,
	},
	/*{
		Name:    "Spotlite",
		Texture: "/api/item/SPOTLITE",
		Amount: func(userProfile *skycrypttypes.Member) int {
			// INFO: Missing from the API
		},
		MaxAmount: 5,
	},*/
	/*{
		Name:    "Dwarven O's Ore Oats",
		Texture: "/api/item/DWARVEN_OS_ORE_OATS",
		Amount: func(userProfile *skycrypttypes.Member) int {
			// INFO: Missing from the API
			return 0
		},
		MaxAmount: 5,
	},*/
	/*{
		Name:    "Dwarven O's Block Bran",
		Texture: "/api/item/DWARVEN_OS_BLOCK_BRAN",
		Amount: func(userProfile *skycrypttypes.Member) int {
			// INFO: Missing from the API
			return 0
		},
		MaxAmount: 5,
	},*/
	/*{
		Name:    "Dwarven O's Gemstone Grahams",
		Texture: "/api/item/DWARVEN_OS_GEMSTONE_GRAHAMS",
		Amount: func(userProfile *skycrypttypes.Member) int {
			// INFO: Missing from the API
			return 0
		},
		MaxAmount: 5,
	},*/
	/*{
		Name:    "Dwarven O's Metallic Minis",
		Texture: "/api/item/DWARVEN_OS_METALLIC_MINIS",
		Amount: func(userProfile *skycrypttypes.Member) int {
			// INFO: Missing from the API
			return 0
		},
		MaxAmount: 5,
	},*/
	/*{
		Name:    "Moby-Duck: Collector's Edition",
		Texture: "/api/item/MOBY_DUCK",
		Amount: func(userProfile *skycrypttypes.Member) int {
			// INFO: Missing from the API
			return 0
		},
		MaxAmount: 1,
	},*/
	/*{
		Name:    "Brain Food",
		Texture: "/api/item/BRAIN_FOOD",
		Amount: func(userProfile *skycrypttypes.Member) int {
			// INFO: Missing from the API
			return 0
		},
		MaxAmount: 5,
	},*/
	/*{
		Name:    "Filled Rosewater Flask",
		Texture: "/api/item/FILLED_ROSEWATER_FLASK",
		Amount: func(userProfile *skycrypttypes.Member) int {
			// INFO: Missing from the API
			return 0
		},
		MaxAmount: 10,
	},*/
}
