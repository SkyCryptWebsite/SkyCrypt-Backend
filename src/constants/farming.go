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
	"DOUBLE_PLANT":        "SUNFLOWER",
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

var GREENHOUSE_MUTATIONS = []string{
    "veilshroom",
    "ashwreath",
    "shellfruit",
    "duskbloom",
    "shadevine",
    "jerryflower",
    "chorus_fruit",
    "creambloom",
    "lonelily",
    "chloronite",
    "witherbloom",
    "magic_jellybean",
    "cindershade",
    "cheesebite",
    "plantboy_advance",
    "scourroot",
    "startlevine",
    "choconut",
    "thunderling",
    "do_not_eat_shroom",
    "chocoberry",
    "puffercloud",
    "blastberry",
    "glasscorn",
    "stoplight_petal",
    "all_in_aloe",
    "thornshade",
    "fleshtrap",
    "noctilume",
    "zombud",
    "phantomleaf",
    "coalroot",
    "dustgrain",
    "godseed",
    "snoozling",
    "soggybud",
    "gloomgourd",
    "turtlellini",
    "timestalk",
    "devourer",
}
