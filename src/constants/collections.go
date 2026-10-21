package constants

import (
	"skycrypt/src/models"
	"sync/atomic"
)

// collections is replaced whole on reload, never edited in place, so readers need no lock.
var collections atomic.Pointer[models.ProcessedHypixelCollection]

func Collections() models.ProcessedHypixelCollection {
	if current := collections.Load(); current != nil {
		return *current
	}
	return models.ProcessedHypixelCollection{}
}

func SetCollections(value models.ProcessedHypixelCollection) {
	collections.Store(&value)
}

var COLLECTION_ICONS = map[string]string{
	"farming":  "/api/item/GOLDEN_HOE",
	"mining":   "/api/item/STONE_PICKAXE",
	"combat":   "/api/item/STONE_SWORD",
	"foraging": "/api/item/JUNGLE_SAPLING",
	"fishing":  "/api/item/FISHING_ROD",
	"rift":     "/api/item/MYCELIUM",
}

type bossCollection struct {
	Name        string `json:"name"`
	Texture     string `json:"texture"`
	Collections []int  `json:"collections"`
}

var BOSS_COLLECTIONS = []bossCollection{
	{
		Name:        "Bonzo",
		Texture:     "/api/head/12716ecbf5b8da00b05f316ec6af61e8bd02805b21eb8e440151468dc656549c",
		Collections: []int{25, 50, 100, 150, 250, 1000},
	},
	{
		Name:        "Scarf",
		Texture:     "/api/head/7de7bbbdf22bfe17980d4e20687e386f11d59ee1db6f8b4762391b79a5ac532d",
		Collections: []int{25, 50, 100, 150, 250, 1000},
	},
	{
		Name:        "The Professor",
		Texture:     "/api/head/9971cee8b833a62fc2a612f3503437fdf93cad692d216b8cf90bbb0538c47dd8",
		Collections: []int{25, 50, 100, 150, 250, 1000},
	},
	{
		Name:        "Thorn",
		Texture:     "/api/head/8b6a72138d69fbbd2fea3fa251cabd87152e4f1c97e5f986bf685571db3cc0",
		Collections: []int{50, 100, 150, 250, 400, 1000},
	},
	{
		Name:        "Livid",
		Texture:     "/api/head/c1007c5b7114abec734206d4fc613da4f3a0e99f71ff949cedadc99079135a0b",
		Collections: []int{50, 100, 150, 250, 500, 750, 1000},
	},
	{
		Name:        "Sadan",
		Texture:     "/api/head/fa06cb0c471c1c9bc169af270cd466ea701946776056e472ecdaeb49f0f4a4dc",
		Collections: []int{50, 100, 150, 250, 500, 750, 1000},
	},
	{
		Name:        "Necron",
		Texture:     "/api/head/a435164c05cea299a3f016bbbed05706ebb720dac912ce4351c2296626aecd9a",
		Collections: []int{50, 100, 150, 250, 500, 750, 1000},
	},
	{
		Name:        "Kuudra",
		Texture:     "/api/head/82ee25414aa7efb4a2b4901c6e33e5eaa705a6ab212ebebfd6a4de984125c7a0",
		Collections: []int{10, 100, 500, 2000, 5000},
	},
}
