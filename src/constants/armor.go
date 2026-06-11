package constants

type armorSet struct {
	Pieces []string `json:"pieces"`
	Name   string   `json:"name"`
}

var SPECIAL_SETS = []armorSet{
	{
		Pieces: []string{"SKELETON_HELMET", "GUARDIAN_CHESTPLATE", "CREEPER_LEGGINGS", "SPIDER_BOOTS"},
		Name:   "Monster Hunter Armor",
	},
	{
		Pieces: []string{"SKELETON_HELMET", "GUARDIAN_CHESTPLATE", "CREEPER_LEGGINGS", "TARANTULA_BOOTS"},
		Name:   "Monster Raider Armor",
	},
	{
		Pieces: []string{"SPONGE_HELMET", "SPONGE_CHESTPLATE", "SPONGE_LEGGINGS", "SPONGE_BOOTS"},
		Name:   "Sponge Armor",
	},
	{
		Pieces: []string{"FAIRY_HELMET", "FAIRY_CHESTPLATE", "FAIRY_LEGGINGS", "FAIRY_BOOTS"},
		Name:   "Fairy Armor",
	},
	{
		Pieces: []string{"DIVER_HELMET", "DIVER_CHESTPLATE", "DIVER_LEGGINGS", "DIVER_BOOTS"},
		Name:   "Diver Armor",
	},
	{
		Pieces: []string{"LEAFLET_HELMET", "LEAFLET_CHESTPLATE", "LEAFLET_LEGGINGS", "LEAFLET_BOOTS"},
		Name:   "Leaflet Armor",
	},
	{
		Pieces: []string{"MASTIFF_HELMET", "MASTIFF_CHESTPLATE", "MASTIFF_LEGGINGS", "MASTIFF_BOOTS"},
		Name:   "Mastiff Armor",
	},
	{
		Pieces: []string{"ADAPTIVE_HELMET", "ADAPTIVE_CHESTPLATE", "ADAPTIVE_LEGGINGS", "ADAPTIVE_BOOTS"},
		Name:   "Adaptive Armor",
	},
	{
		Pieces: []string{"TAURUS_HELMET", "FLAMING_CHESTPLATE", "MOOGMA_LEGGINGS", "SLUG_BOOTS"},
		Name:   "Lava Sea Creature Armor",
	},
}

var WARDROBE_SLOT_OFFSET = map[string]int{
	"helmet":     0,
	"chestplate": 1,
	"leggings":   2,
	"boots":      3,
}
