package constants

var FARMING_MEDALS = []string{"bronze", "silver", "gold", "platinum", "diamond"}

var CROPS = map[string]string{
	"INK_SACK:3":          "Cocoa Beans",
	"POTATO_ITEM":         "Potato",
	"CARROT_ITEM":         "Carrot",
	"CACTUS":              "Cactus",
	"SUGAR_CANE":          "Sugar Cane",
	"MUSHROOM_COLLECTION": "Mushroom",
	"PUMPKIN":             "Pumpkin",
	"NETHER_STALK":        "Nether Wart",
	"WHEAT":               "Wheat",
	"MELON":               "Melon",
	"MOONFLOWER":          "Moonflower",
	"WILD_ROSE":           "Wild Rose",
	"DOUBLE_PLANT":        "Sunflower",
}

var CROP_TO_ID = map[string]string{
	"WHEAT":               "WHEAT",
	"CARROT_ITEM":         "CARROT",
	"POTATO_ITEM":         "POTATO",
	"MELON":               "MELON",
	"PUMPKIN":             "PUMPKIN",
	"SUGAR_CANE":          "SUGAR_CANE",
	"CACTUS":              "CACTUS",
	"MUSHROOM_COLLECTION": "MUSHROOM",
	"NETHER_STALK":        "NETHER_WART",
	"INK_SACK:3":          "COCOA_BEANS",
	"MOONFLOWER":          "MOONFLOWER",
	"WILD_ROSE":           "WILD_ROSE",
	"SUNFLOWER":           "SUNFLOWER",
}

var CROP_TEXTURES = map[string]string{
	"MUSHROOM_COLLECTION": "BROWN_MUSHROOM",
	"WHEAT":               "WHEAT",
	"CARROT_ITEM":         "CARROT",
	"POTATO_ITEM":         "POTATO",
	"MELON":               "MELON",
	"PUMPKIN":             "PUMPKIN",
	"SUGAR_CANE":          "SUGAR_CANE",
	"CACTUS":              "CACTUS",
	"NETHER_STALK":        "NETHER_WART",
	"INK_SACK:3":          "COCOA_BEANS",
	"MOONFLOWER":          "BLUE_ORCHID",
	"WILD_ROSE":           "ROSE_BUSH",
	"DOUBLE_PLANT":        "SUNFLOWER",
}

type GardenUpgrade struct {
	Texture  string
	Name     string
	MaxLevel int
}

var GARDEN_UPGRADES = map[string]GardenUpgrade{
	"growth_speed": {
		Texture:  "WHEAT_SEEDS",
		Name:     "Growth Speed",
		MaxLevel: 9,
	},
	"plot_limit": {
		Texture:  "GRASS_BLOCK",
		Name:     "Plot Limit",
		MaxLevel: 2,
	},
	"yield": {
		Texture:  "FLOWER_POT",
		Name:     "Plot Yield",
		MaxLevel: 9,
	},
}

var MAX_GARDEN_CHIPS = 20

var GARDEN_CHIPS = []string{
	"cropshot",
	"sowledge",
	"mechamind",
	"overdrive",
	"vermin_vaporizer",
	"quickdraw",
	"hypercharge",
	"evergreen",
	"rarefinder",
	"synthesis",
}

var MAX_DNA_ANALYSIS_MILESTONE = 6