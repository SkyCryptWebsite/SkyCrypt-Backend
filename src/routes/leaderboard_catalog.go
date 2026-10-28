package routes

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"skycrypt/src/constants"
	statsLeveling "skycrypt/src/stats/leveling"
	"skycrypt/src/store"
	"skycrypt/src/utility"
)

// The store knows boards by key ("skill:farming", "kills:zombie", ...). This file turns keys into what people see:
// a name, a category, a unit, and the shown value (a level for XP).

type boardCategory struct {
	ID   string
	Name string
}

// leaderboardCategories in the order they are listed.
var leaderboardCategories = []boardCategory{
	{"general", "General"},
	{"skills", "Skills"},
	{"slayers", "Slayers"},
	{"dungeons", "Dungeons"},
	{"crimson_isle", "Crimson Isle"},
	{"collections", "Collections"},
	{"coop_collections", "Coop Collections"},
	{"garden", "Garden"},
	{"pests", "Pests"},
	{"kills", "Kills"},
	{"deaths", "Deaths"},
}

type boardInfo struct {
	ID       string
	Name     string
	Category string
	// Group splits big categories: collection categories (Farming, Rift, ...), dungeon modes.
	Group string
	// Unit is level (value is a level, raw the XP), xp, amount, coins or time (milliseconds, lower is better).
	Unit      string
	Ascending bool
	// Profile boards rank whole profiles (coop collections, garden) instead of a player on a profile.
	Profile bool
	// levelType is the XP table for level units; capped by MaxedSkillCaps when the skill has one.
	levelType string
	divisor   float64
	order     int
}

// describeBoard returns how a key is shown, or false for keys that aren't leaderboards.
func describeBoard(key string) (boardInfo, bool) {
	info := boardInfo{ID: key, Ascending: store.Ascending(key)}
	parts := strings.Split(key, ":")

	switch parts[0] {
	case "skyblock_level":
		info.Name, info.Category, info.Unit, info.divisor = "SkyBlock Level", "general", "level", 100
	case "networth":
		info.Name, info.Category, info.Unit, info.order = "Networth", "general", "coins", 1
	case "networth_noncosmetic":
		info.Name, info.Category, info.Unit, info.order = "Non-Cosmetic Networth", "general", "coins", 2
	case "skill":
		if len(parts) != 2 {
			return info, false
		}
		info.Name, info.Category, info.Unit, info.levelType = utility.TitleCase(parts[1]), "skills", "level", parts[1]
	case "slayer":
		if len(parts) != 2 {
			return info, false
		}
		info.Category, info.Unit = "slayers", "xp"
		if parts[1] == "total" {
			info.Name, info.order = "Total Slayer XP", -1
		} else if slayer, ok := constants.SLAYER_INFO[parts[1]]; ok {
			info.Name = slayer.Name
		} else {
			info.Name = utility.TitleCase(parts[1])
		}
	case "dungeons":
		info.Category = "dungeons"
		if !describeDungeonBoard(&info, parts[1:]) {
			return info, false
		}
	case "kuudra":
		if len(parts) != 2 {
			return info, false
		}
		info.Category, info.Group, info.Unit = "crimson_isle", "Kuudra", "amount"
		if parts[1] == "total" {
			info.Name, info.order = "Total Kuudra Completions", -1
		} else if tier, ok := constants.KUUDRA_TIERS[parts[1]]; ok {
			info.Name, info.order = tier.Name+" Kuudra Completions", slices.Index(kuudraOrder, parts[1])
		} else {
			info.Name = utility.TitleCase(parts[1]) + " Kuudra Completions"
		}
	case "crimson":
		if len(parts) != 2 {
			return info, false
		}
		info.Category, info.Group, info.Unit = "crimson_isle", "Reputation", "amount"
		info.Name = map[string]string{"mages_reputation": "Mage Reputation", "barbarians_reputation": "Barbarian Reputation"}[parts[1]]
		if info.Name == "" {
			info.Name = utility.TitleCase(parts[1])
		}
	case "collection", "coop_collection":
		item := strings.Join(parts[1:], ":")
		if item == "" {
			return info, false
		}
		info.Name, info.Group = collectionName(item)
		info.Unit = "amount"
		if parts[0] == "collection" {
			info.Category = "collections"
		} else {
			info.Category, info.Profile = "coop_collections", true
		}
	case "garden":
		info.Category, info.Profile = "garden", true
		switch {
		case len(parts) == 2 && parts[1] == "level":
			info.Name, info.Unit, info.levelType, info.order = "Garden Level", "level", "garden", -1
		case len(parts) >= 3 && parts[1] == "milestone":
			crop := strings.Join(parts[2:], ":")
			name := constants.CROPS[crop]
			if name == "" {
				name = utility.TitleCase(crop)
			}
			info.Name, info.Group, info.Unit = name+" Milestone", "Crop Milestones", "level"
			info.levelType = "crop_milestone_" + constants.CROP_TO_ID[crop]
			if constants.CROP_TO_ID[crop] == "" {
				info.Unit, info.levelType = "amount", ""
			}
		default:
			return info, false
		}
	case "pests":
		if len(parts) != 2 {
			return info, false
		}
		info.Category, info.Unit = "pests", "amount"
		if parts[1] == "total" {
			info.Name, info.order = "Total Pests", -1
		} else {
			info.Name = mobName(parts[1])
		}
	case "kills", "deaths":
		if len(parts) != 2 {
			return info, false
		}
		info.Category, info.Unit = parts[0], "amount"
		if parts[1] == "total" {
			info.Name, info.order = "Total "+utility.TitleCase(parts[0]), -1
		} else {
			info.Name = mobName(parts[1])
		}
	default:
		return info, false
	}
	return info, info.Name != ""
}

// mobName names a kills/deaths key. Hypixel keys some mobs by level ("agarimoo_35"), which reads as "Agarimoo (Lv 35)".
func mobName(key string) string {
	if name, ok := constants.MOB_NAMES[key]; ok {
		return name
	}
	if i := strings.LastIndexByte(key, '_'); i > 0 {
		if level, err := strconv.Atoi(key[i+1:]); err == nil {
			base := key[:i]
			name, ok := constants.MOB_NAMES[base]
			if !ok {
				name = utility.TitleCase(base)
			}
			return fmt.Sprintf("%s (Lv %d)", name, level)
		}
	}
	return utility.TitleCase(key)
}

var kuudraOrder = []string{"none", "hot", "burning", "fiery", "infernal"}

// describeDungeonBoard handles dungeons:catacombs, dungeons:class:<class> and
// dungeons:<catacombs|master_catacombs>:<floor>:<completions|fastest|fastest_s|fastest_s_plus>.
func describeDungeonBoard(info *boardInfo, parts []string) bool {
	switch {
	case len(parts) == 1 && parts[0] == "catacombs":
		info.Name, info.Group, info.Unit, info.levelType, info.order = "Catacombs", "Catacombs", "level", "dungeoneering", -2
		return true
	case len(parts) == 2 && parts[0] == "class":
		info.Name, info.Group, info.Unit, info.levelType = utility.TitleCase(parts[1]), "Classes", "level", "dungeoneering"
		return true
	case len(parts) != 3:
		return false
	}

	dungeon, floor, metric := parts[0], parts[1], parts[2]
	master := dungeon == "master_catacombs"
	if dungeon != "catacombs" && !master {
		return false
	}
	info.Group = "Catacombs"
	if master {
		info.Group = "Master Mode"
	}

	var label string
	switch {
	case floor == "total":
		label = "Total"
		info.order = -1
	case floor == "0" && !master:
		label = "Entrance"
	case master:
		label = "M" + floor
	default:
		label = "F" + floor
	}
	if floor != "total" {
		var n int
		_, _ = fmt.Sscan(floor, &n)
		info.order = n * 10
	}

	switch metric {
	case "completions":
		info.Name, info.Unit = label+" Completions", "amount"
	case "fastest":
		info.Name, info.Unit = label+" Fastest Time", "time"
		info.order += 1
	case "fastest_s":
		info.Name, info.Unit = label+" Fastest S Time", "time"
		info.order += 2
	case "fastest_s_plus":
		info.Name, info.Unit = label+" Fastest S+ Time", "time"
		info.order += 3
	default:
		return false
	}
	return true
}

// collectionName finds an item's name and collection category (Farming, Mining, Rift, ...).
func collectionName(item string) (string, string) {
	for _, category := range constants.Collections() {
		for _, collection := range category.Collections {
			if collection.Id == item {
				return collection.Name, category.Name
			}
		}
	}
	return utility.TitleCase(item), "Other"
}

// boardValue is the shown number: a level with progress for XP boards, the raw amount otherwise.
func boardValue(info boardInfo, raw int64) float64 {
	switch {
	case info.levelType != "":
		extra := &statsLeveling.ExtraSkillData{Type: info.levelType}
		// Boards rank by XP, so the cap is the highest a player can unlock (farming and taming need upgrades for 60).
		if maxCap, ok := constants.MaxedSkillCaps[info.levelType]; ok {
			extra.Cap = &maxCap
		}
		level := statsLeveling.GetLevelByXp(int(raw), extra)
		return math.Round(level.LevelWithProgress*100) / 100
	case info.divisor > 0:
		return float64(raw) / info.divisor
	default:
		return float64(raw)
	}
}

// sortBoards orders boards for listing: by category, group, explicit order, then name.
func sortBoards(boards []boardInfo) {
	categoryOrder := map[string]int{}
	for i, c := range leaderboardCategories {
		categoryOrder[c.ID] = i
	}
	slices.SortFunc(boards, func(a, b boardInfo) int {
		if c := categoryOrder[a.Category] - categoryOrder[b.Category]; c != 0 {
			return c
		}
		if c := strings.Compare(a.Group, b.Group); c != 0 {
			return c
		}
		if c := a.order - b.order; c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
}
