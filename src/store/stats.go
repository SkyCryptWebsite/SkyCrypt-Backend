package store

import (
	"math"
	"strings"
)

// Stat keys name what a value is: "skyblock_level", "networth", "skill:farming", "slayer:zombie",
// "dungeons:class:mage", "dungeons:catacombs:7:fastest_s_plus", "kuudra:infernal", "collection:WHEAT",
// "kills:zombie", "deaths:zombie", "pests:mite", and on profiles "coop_collection:WHEAT", "garden:level",
// "garden:milestone:WHEAT". Every key is its own leaderboard. The API layer turns keys into names and categories.

// statsVersion is stored with every value record. Bump it when extraction changes: the parent recomputes older
// records from the stored profiles (see backfill.go).
const statsVersion = 3

// API categories a player can hide. A hidden category keeps its last known values, which then show as outdated.
const (
	hiddenSkills      = "skills"
	hiddenCollections = "collections"
	hiddenInventory   = "inventory"
)

// hiddenCategory says which API toggle hides a key, if any.
func hiddenCategory(key string) string {
	switch {
	case strings.HasPrefix(key, "skill:"):
		return hiddenSkills
	case strings.HasPrefix(key, "collection:"):
		return hiddenCollections
	case key == "networth" || key == "networth_noncosmetic":
		return hiddenInventory
	}
	return ""
}

// Ascending reports whether lower values rank higher, which is the case for dungeon run times.
func Ascending(key string) bool {
	return strings.HasSuffix(key, ":fastest") || strings.HasSuffix(key, ":fastest_s") || strings.HasSuffix(key, ":fastest_s_plus")
}

// memberJSON is the part of a profile member the light tables and leaderboards read. jsoniter skips the rest.
type memberJSON struct {
	Profile *struct {
		DeletionNotice *struct{} `json:"deletion_notice"`
		CoopInvitation *struct {
			Confirmed bool `json:"confirmed"`
		} `json:"coop_invitation"`
	} `json:"profile"`
	Leveling *struct {
		Experience *float64 `json:"experience"`
	} `json:"leveling"`
	PlayerData *struct {
		Experience map[string]float64 `json:"experience"`
	} `json:"player_data"`
	Slayer *struct {
		SlayerBosses map[string]struct {
			XP *float64 `json:"xp"`
		} `json:"slayer_bosses"`
	} `json:"slayer"`
	Dungeons *struct {
		DungeonTypes  map[string]dungeonTypeJSON `json:"dungeon_types"`
		PlayerClasses map[string]struct {
			Experience *float64 `json:"experience"`
		} `json:"player_classes"`
	} `json:"dungeons"`
	NetherIsland *struct {
		KuudraCompletedTiers map[string]float64 `json:"kuudra_completed_tiers"`
		MagesReputation      *float64           `json:"mages_reputation"`
		BarbariansReputation *float64           `json:"barbarians_reputation"`
	} `json:"nether_island_player_data"`
	PlayerStats *struct {
		Kills  map[string]float64 `json:"kills"`
		Deaths map[string]float64 `json:"deaths"`
	} `json:"player_stats"`
	Collection map[string]float64 `json:"collection"`
	Inventory  *struct{}          `json:"inventory"`
}

type dungeonTypeJSON struct {
	Experience       *float64           `json:"experience"`
	TierCompletions  map[string]float64 `json:"tier_completions"`
	FastestTime      map[string]float64 `json:"fastest_time"`
	FastestTimeS     map[string]float64 `json:"fastest_time_s"`
	FastestTimeSPlus map[string]float64 `json:"fastest_time_s_plus"`
}

var kuudraTiers = []string{"none", "hot", "burning", "fiery", "infernal"}

// memberStatValues reads every ranked value of a member and which hideable categories are hidden right now.
// Values of 0 are left out: nobody is ranked for something they never did.
func memberStatValues(m *memberJSON) (map[string]int64, map[string]bool) {
	values := map[string]int64{}
	set := func(key string, value float64) {
		if value > 0 && !math.IsNaN(value) {
			values[key] = int64(min(math.Round(value), math.MaxInt64))
		}
	}
	hidden := map[string]bool{}

	if m.Leveling != nil && m.Leveling.Experience != nil {
		set("skyblock_level", *m.Leveling.Experience)
	}

	if m.PlayerData == nil || m.PlayerData.Experience == nil {
		hidden[hiddenSkills] = true
	} else {
		for key, xp := range m.PlayerData.Experience {
			skill, ok := strings.CutPrefix(key, "SKILL_")
			// SKILL_DUNGEONEERING is unused (Catacombs XP is elsewhere); SKILL_FORAGING_extra_level_cap isn't XP.
			if !ok || skill == "DUNGEONEERING" || strings.ContainsRune(skill, '_') {
				continue
			}
			set("skill:"+strings.ToLower(skill), xp)
		}
	}

	if m.Slayer != nil {
		total := 0.0
		for boss, data := range m.Slayer.SlayerBosses {
			if data.XP != nil {
				set("slayer:"+boss, *data.XP)
				total += *data.XP
			}
		}
		set("slayer:total", total)
	}

	if m.Dungeons != nil {
		if catacombs, ok := m.Dungeons.DungeonTypes["catacombs"]; ok && catacombs.Experience != nil {
			set("dungeons:catacombs", *catacombs.Experience)
		}
		for class, data := range m.Dungeons.PlayerClasses {
			if data.Experience != nil {
				set("dungeons:class:"+class, *data.Experience)
			}
		}
		for dungeon, data := range m.Dungeons.DungeonTypes {
			total := 0.0
			for floor, count := range data.TierCompletions {
				if floor == "total" {
					continue
				}
				set("dungeons:"+dungeon+":"+floor+":completions", count)
				total += count
			}
			set("dungeons:"+dungeon+":total:completions", total)
			for metric, times := range map[string]map[string]float64{
				"fastest": data.FastestTime, "fastest_s": data.FastestTimeS, "fastest_s_plus": data.FastestTimeSPlus,
			} {
				for floor, ms := range times {
					if floor != "best" {
						set("dungeons:"+dungeon+":"+floor+":"+metric, ms)
					}
				}
			}
		}
	}

	if m.NetherIsland != nil {
		total := 0.0
		for _, tier := range kuudraTiers {
			count := m.NetherIsland.KuudraCompletedTiers[tier]
			set("kuudra:"+tier, count)
			total += count
		}
		set("kuudra:total", total)
		if m.NetherIsland.MagesReputation != nil {
			set("crimson:mages_reputation", *m.NetherIsland.MagesReputation)
		}
		if m.NetherIsland.BarbariansReputation != nil {
			set("crimson:barbarians_reputation", *m.NetherIsland.BarbariansReputation)
		}
	}

	if m.PlayerStats != nil {
		pests := 0.0
		for mob, count := range m.PlayerStats.Kills {
			if pest, ok := strings.CutPrefix(mob, "pest_"); ok {
				set("pests:"+pest, count)
				pests += count
				continue
			}
			set("kills:"+mob, count)
		}
		set("pests:total", pests)
		for mob, count := range m.PlayerStats.Deaths {
			set("deaths:"+mob, count)
		}
	}

	if m.Collection == nil {
		hidden[hiddenCollections] = true
	} else {
		for item, amount := range m.Collection {
			set("collection:"+item, amount)
		}
	}

	if m.Inventory == nil {
		hidden[hiddenInventory] = true
	}
	return values, hidden
}

// gardenJSON is the part of /v2/skyblock/garden the profile boards read.
type gardenJSON struct {
	Garden *struct {
		GardenExperience   *float64           `json:"garden_experience"`
		ResourcesCollected map[string]float64 `json:"resources_collected"`
	} `json:"garden"`
}

func gardenStatValues(g *gardenJSON) map[string]int64 {
	values := map[string]int64{}
	if g.Garden == nil {
		return values
	}
	if g.Garden.GardenExperience != nil && *g.Garden.GardenExperience > 0 {
		values["garden:level"] = int64(math.Round(*g.Garden.GardenExperience))
	}
	for crop, amount := range g.Garden.ResourcesCollected {
		if amount > 0 {
			values["garden:milestone:"+crop] = int64(math.Round(amount))
		}
	}
	return values
}
