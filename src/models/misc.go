package models

type FairySouls struct {
	Found int `json:"found"`
	Total int `json:"total"`
}

type MiscOutput struct {
	Essence           []MiscEssence               `json:"essence"`
	Kills             MiscKills                   `json:"kills"`
	Gifts             MiscGifts                   `json:"gifts"`
	SeasonOfJerry     MiscSeasonOfJerry           `json:"season_of_jerry"`
	Dragons           MiscDragons                 `json:"dragons"`
	EndstoneProtector MiscEndstoneProtector       `json:"endstone_protector"`
	Damage            MiscDamage                  `json:"damage"`
	PetMilestones     map[string]MiscPetMilestone `json:"pet_milestones"`
	MythologicalEvent MiscMythologicalEvent       `json:"mythological_event"`
	ProfileUpgrades   MiscProfileUpgrades         `json:"profile_upgrades"`
	Auctions          MiscAuctions                `json:"auctions"`
	ClaimedItems      map[string]int64            `json:"claimed_items"`
	Uncategorized     map[string]any              `json:"uncategorized"`
}

type MiscAuctions struct {
	Bids        float64            `json:"bids"`
	HighestBid  float64            `json:"highest_bid"`
	Won         float64            `json:"won"`
	TotalBought map[string]float64 `json:"total_bought"`
	GoldSpent   float64            `json:"gold_spent"`
	Created     float64            `json:"created"`
	Fees        float64            `json:"fees"`
	TotalSold   map[string]float64 `json:"total_sold"`
	GoldEarned  float64            `json:"gold_earned"`
	NoBids      float64            `json:"no_bids"`
}

type MiscProfileUpgrades map[string]int

type MiscMythologicalEvent struct {
	Kills                 float64            `json:"kills"`
	BurrowsDugNext        map[string]float64 `json:"burrows_dug_next"`
	BurrowsDugCombat      map[string]float64 `json:"burrows_dug_combat"`
	BurrowsDugTreasure    map[string]float64 `json:"burrows_dug_treasure"`
	BurrowsChainsComplete map[string]float64 `json:"burrows_chains_complete"`
}

type MiscPetMilestone struct {
	Amount   int    `json:"amount"`
	Rarity   string `json:"rarity"`
	Total    int    `json:"total"`
	Progress string `json:"progress"`
}

type MiscDamage struct {
	HighestCriticalDamage float64 `json:"highest_critical_damage"`
}

type MiscEndstoneProtector struct {
	Kills  int `json:"kills"`
	Deaths int `json:"deaths"`
}

type MiscDragons struct {
	EnderCrystalsDestroyed int                `json:"ender_crystals_destroyed"`
	MostDamage             map[string]float64 `json:"most_damage"`
	FastestKill            map[string]float64 `json:"fastest_kill"`
	LastHits               map[string]float64 `json:"last_hits"`
	Deaths                 map[string]float64 `json:"deaths"`
}

type MiscSeasonOfJerry struct {
	MostSnowballsHit     int `json:"most_snowballs_hit"`
	MostDamageDealt      int `json:"most_damage_dealt"`
	MostMagmaDamageDealt int `json:"most_magma_damage_dealt"`
	MostCannonballsHit   int `json:"most_cannonballs_hit"`
}

type MiscEssence struct {
	Name    string `json:"name"`
	Texture string `json:"texture"`
	Amount  int    `json:"amount"`
}

type MiscKills struct {
	TotalKills  int        `json:"total_kills"`
	TotalDeaths int        `json:"total_deaths"`
	Kills       []MiscKill `json:"kills"`
	Deaths      []MiscKill `json:"deaths"`
}

type MiscKill struct {
	Name   string `json:"name"`
	Amount int    `json:"amount"`
}

type MiscGifts struct {
	Given    int `json:"given"`
	Received int `json:"received"`
}
