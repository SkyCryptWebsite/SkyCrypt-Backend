package constants

import (
	"fmt"
	"os"
	"skycrypt/src/models"
	"slices"
	"time"

	"github.com/joho/godotenv"
)

// TODO: Resolve import cycle with helper.go
func getDomain() string {
	output := os.Getenv("DOMAIN")
	if output != "" {
		return output
	}

	_ = godotenv.Load()
	output = os.Getenv("DOMAIN")
	if output != "" {
		return output
	}

	return "https://sky.shiiyu.moe"
}

type Accessory struct {
	Name       string `json:"name"`
	SkyBlockID string `json:"skyblock_id"`
	Texture    string `json:"texture"`
	Rarity     string `json:"rarity"`
}

type specialAccessoryConstant struct {
	AllowsRecomb     bool              `json:"allows_recomb,omitempty"`
	Rarities         []string          `json:"rarities,omitempty"`
	CustomPrice      bool              `json:"custom_price,omitempty"`
	AllowsEnrichment bool              `json:"allows_enrichment,omitempty"`
	Upgrade          *accessoryUpgrade `json:"upgrade,omitempty"`
}

type accessoryUpgrade struct {
	Item string         `json:"item"`
	Cost map[string]int `json:"cost"`
}

type ExtraAccessory struct {
	ID      string `json:"id"`
	Texture string `json:"texture"`
	Name    string `json:"name"`
	Rarity  string `json:"rarity"`
}

var ACCESSORIES []models.ProcessedHypixelItem

func getAccessories() {
	var output []models.ProcessedHypixelItem
	for _, item := range ItemsSnapshot() {
		if item.Category == "accessory" {
			output = append(output, item)
		}
	}

	ACCESSORIES = output
	//fmt.Printf("[ACCESSORIES] Loaded %d accessories\n", len(ACCESSORIES))
}

var AccessoryUpgrades = [][]string{
	{"WOLF_TALISMAN", "WOLF_RING"},
	{"POTION_AFFINITY_TALISMAN", "RING_POTION_AFFINITY", "ARTIFACT_POTION_AFFINITY"},
	{"FEATHER_TALISMAN", "FEATHER_RING", "FEATHER_ARTIFACT"},
	{"SEA_CREATURE_TALISMAN", "SEA_CREATURE_RING", "SEA_CREATURE_ARTIFACT"},
	{"HEALING_TALISMAN", "HEALING_RING"},
	{"CANDY_TALISMAN", "CANDY_RING", "CANDY_ARTIFACT", "CANDY_RELIC"},
	{"INTIMIDATION_TALISMAN", "INTIMIDATION_RING", "INTIMIDATION_ARTIFACT", "INTIMIDATION_RELIC"},
	{"SPIDER_TALISMAN", "SPIDER_RING", "SPIDER_ARTIFACT"},
	{"RED_CLAW_TALISMAN", "RED_CLAW_RING", "RED_CLAW_ARTIFACT"},
	{"HUNTER_TALISMAN", "HUNTER_RING"},
	{"ZOMBIE_TALISMAN", "ZOMBIE_RING", "ZOMBIE_ARTIFACT"},
	{"BAT_TALISMAN", "BAT_RING", "BAT_ARTIFACT"},
	{"BROKEN_PIGGY_BANK", "CRACKED_PIGGY_BANK", "PIGGY_BANK"},
	{"SPEED_TALISMAN", "SPEED_RING", "SPEED_ARTIFACT", "SPEED_RELIC"},
	{"PERSONAL_COMPACTOR_4000", "PERSONAL_COMPACTOR_5000", "PERSONAL_COMPACTOR_6000", "PERSONAL_COMPACTOR_7000"},
	{"PERSONAL_DELETOR_4000", "PERSONAL_DELETOR_5000", "PERSONAL_DELETOR_6000", "PERSONAL_DELETOR_7000"},
	{"SCARF_STUDIES", "SCARF_THESIS", "SCARF_GRIMOIRE"},
	{"CAT_TALISMAN", "LYNX_TALISMAN", "CHEETAH_TALISMAN"},
	{"SHADY_RING", "CROOKED_ARTIFACT", "SEAL_OF_THE_FAMILY"},
	{"TREASURE_TALISMAN", "TREASURE_RING", "TREASURE_ARTIFACT"},
	{"BEASTMASTER_CREST_COMMON", "BEASTMASTER_CREST_UNCOMMON", "BEASTMASTER_CREST_RARE", "BEASTMASTER_CREST_EPIC", "BEASTMASTER_CREST_LEGENDARY"},
	{"RAGGEDY_SHARK_TOOTH_NECKLACE", "DULL_SHARK_TOOTH_NECKLACE", "HONED_SHARK_TOOTH_NECKLACE", "SHARP_SHARK_TOOTH_NECKLACE", "RAZOR_SHARP_SHARK_TOOTH_NECKLACE"},
	{"BAT_PERSON_TALISMAN", "BAT_PERSON_RING", "BAT_PERSON_ARTIFACT"},
	{"LUCKY_HOOF", "ETERNAL_HOOF"},
	{"WITHER_ARTIFACT", "WITHER_RELIC"},
	{"WEDDING_RING_0", "WEDDING_RING_2", "WEDDING_RING_4", "WEDDING_RING_7", "WEDDING_RING_9"},
	{"CAMPFIRE_TALISMAN_1", "CAMPFIRE_TALISMAN_4", "CAMPFIRE_TALISMAN_8", "CAMPFIRE_TALISMAN_13", "CAMPFIRE_TALISMAN_21"},
	{"JERRY_TALISMAN_GREEN", "JERRY_TALISMAN_BLUE", "JERRY_TALISMAN_PURPLE", "JERRY_TALISMAN_GOLDEN"},
	{"TITANIUM_TALISMAN", "TITANIUM_RING", "TITANIUM_ARTIFACT", "TITANIUM_RELIC"},
	{"BAIT_RING", "SPIKED_ATROCITY"},
	{"MASTER_SKULL_TIER_1", "MASTER_SKULL_TIER_2", "MASTER_SKULL_TIER_3", "MASTER_SKULL_TIER_4", "MASTER_SKULL_TIER_5", "MASTER_SKULL_TIER_6", "MASTER_SKULL_TIER_7"},
	{"SOULFLOW_PILE", "SOULFLOW_BATTERY", "SOULFLOW_SUPERCELL"},
	{"ENDER_ARTIFACT", "ENDER_RELIC"},
	{"POWER_TALISMAN", "POWER_RING", "POWER_ARTIFACT", "POWER_RELIC"},
	{"BINGO_TALISMAN", "BINGO_RING", "BINGO_ARTIFACT", "BINGO_RELIC"},
	{"BURSTSTOPPER_TALISMAN", "BURSTSTOPPER_ARTIFACT"},
	{"ODGERS_BRONZE_TOOTH", "ODGERS_SILVER_TOOTH", "ODGERS_GOLD_TOOTH", "ODGERS_DIAMOND_TOOTH"},
	{"GREAT_SPOOK_TALISMAN", "GREAT_SPOOK_RING", "GREAT_SPOOK_ARTIFACT"},
	{"DRACONIC_TALISMAN", "DRACONIC_RING", "DRACONIC_ARTIFACT"},
	{"BURNING_KUUDRA_CORE", "FIERY_KUUDRA_CORE", "INFERNAL_KUUDRA_CORE"},
	{"VACCINE_TALISMAN", "VACCINE_RING", "VACCINE_ARTIFACT"},
	{"WHITE_GIFT_TALISMAN", "GREEN_GIFT_TALISMAN", "BLUE_GIFT_TALISMAN", "PURPLE_GIFT_TALISMAN", "GOLD_GIFT_TALISMAN"},
	{"GLACIAL_TALISMAN", "GLACIAL_RING", "GLACIAL_ARTIFACT"},
	{"CROPIE_TALISMAN", "SQUASH_RING", "FERMENTO_ARTIFACT", "HELIANTHUS_RELIC"},
	{"KUUDRA_FOLLOWER_ARTIFACT", "KUUDRA_FOLLOWER_RELIC"},
	{"AGARIMOO_TALISMAN", "AGARIMOO_RING", "AGARIMOO_ARTIFACT"},
	{"BLOOD_DONOR_TALISMAN", "BLOOD_DONOR_RING", "BLOOD_DONOR_ARTIFACT"},
	{"LUSH_TALISMAN", "LUSH_RING", "LUSH_ARTIFACT"},
	{"ANITA_TALISMAN", "ANITA_RING", "ANITA_ARTIFACT"},
	{"PESTHUNTER_BADGE", "PESTHUNTER_RING", "PESTHUNTER_ARTIFACT", "PESTHUNTER_RELIC"},
	{"NIBBLE_CHOCOLATE_STICK", "SMOOTH_CHOCOLATE_BAR", "RICH_CHOCOLATE_CHUNK", "GANACHE_CHOCOLATE_SLAB", "PRESTIGE_CHOCOLATE_REALM"},
	{"EMERALD_RING", "EMERALD_ARTIFACT"},
	{"COIN_TALISMAN", "RING_OF_COINS", "ARTIFACT_OF_COINS", "RELIC_OF_COINS"},
	{"SCAVENGER_TALISMAN", "SCAVENGER_RING", "SCAVENGER_ARTIFACT"},
	{"MINERAL_TALISMAN", "GLOSSY_MINERAL_TALISMAN"},
	{"HASTE_RING", "HASTE_ARTIFACT"},
	{"IQ_POINT", "TWO_IQ_POINT"},
	{"CENTURY_TALISMAN", "CENTURY_RING", "CENTURY_ARTIFACT"},
	{"SEAL_TALISMAN", "SEAL_RING"},
	{"FROZEN_CHICKEN", "FRIED_FROZEN_CHICKEN"},
	{"JUNK_TALISMAN", "JUNK_RING", "JUNK_ARTIFACT"},
	{"BLUETOOTH_RING", "BLUERTOOTH_RING"},
	{"RESPIRATION_TALISMAN", "RESPIRATION_RING", "RESPIRATION_ARTIFACT"},
	{"SMALL_FISH_BOWL", "MEDIUM_FISH_BOWL", "LARGE_FISH_BOWL"},
	{"RUNEBLADE_TALISMAN", "RUNEBLADE_RING", "RUNEBLADE_ARTIFACT"},
	{"MOONGLADE_TALISMAN", "MOONGLADE_RING", "MOONGLADE_ARTIFACT"},
	{"PRESSURE_TALISMAN", "PRESSURE_RING", "PRESSURE_ARTIFACT"},
	{"EMPEROR_TALISMAN", "EMPEROR_RING", "EMPEROR_ARTIFACT"},
	{"SOUL_CAMPFIRE_TALISMAN_1", "SOUL_CAMPFIRE_TALISMAN_4", "SOUL_CAMPFIRE_TALISMAN_8", "SOUL_CAMPFIRE_TALISMAN_13", "SOUL_CAMPFIRE_TALISMAN_21"},
	{"TARANTULA_TALISMAN", "TARANTULA_RING"},
	{"ANGUISH_TALISMAN", "ANGUISH_RING", "ANGUISH_ARTIFACT"},
	{"POTATO_TALISMAN", "POTATO_RING"},
	{"BLOOD_GOD_CREST", "BLOOD_GOD_SIGIL"},
	{"KUUDRAS_KIDNEY", "KUUDRAS_LUNG", "KUUDRAS_HEART"},
	{"VOTER_BADGE", "VOTER_BADGE_VIP", "VOTER_BADGE_ELITE", "VOTER_BADGE_SUPREME"},
	{"BIOANALYSIS_TALISMAN", "BIOANALYSIS_RING", "BIOANALYSIS_ARTIFACT"},
	{"DAY_CRYSTAL", "SUNSHINE_CRYSTAL"},
	{"NIGHT_CRYSTAL", "MOONLIGHT_CRYSTAL"},
	{"WITCH_TALISMAN", "WITCH_RING", "WITCH_ARTIFACT"},
	{"ORGAN_DONOR_TALISMAN", "ORGAN_DONOR_RING", "ORGAN_DONOR_ARTIFACT"},
	{"COPPER_TALISMAN", "COPPER_RING", "COPPER_ARTIFACT"},
	{"GRATITUDE_TALISMAN", "GRATITUDE_RING", "GRATITUDE_ARTIFACT"},
	{"FRESHLY_BAKED_TALISMAN", "FRESHLY_BAKED_RING", "FRESHLY_BAKED_ARTIFACT", "FRESHLY_BAKED_RELIC", "FRESHLY_BAKED_HEIRLOOM"},
	{"APPLICANT_STATEMENT", "STUDENT_STUDIES", "MASTER_THESIS", "PHD_GRIMOIRE"},
	{"ACCRETION_TALISMAN", "ACCRETION_RING", "ACCRETION_ARTIFACT"},
	{"HONEYCOMB_TALISMAN", "HONEYCOMB_RING", "HONEYCOMB_ARTIFACT"},
	{"LUMBERJACK_TALISMAN", "LUMBERJACK_RING", "LUMBERJACK_ARTIFACT"},
}

var ignoredAccessories = []string{
	"BINGO_HEIRLOOM", "LUCK_TALISMAN", "TALISMAN_OF_SPACE", "RING_OF_SPACE",
	"MASTER_SKULL_TIER_8", "MASTER_SKULL_TIER_9", "MASTER_SKULL_TIER_10",
	"COMPASS_TALISMAN", "ARTIFACT_OF_SPACE", "GRIZZLY_PAW", "ETERNAL_CRYSTAL", "OLD_BOOT",
}

var ACCESSORY_ALIASES = map[string][]string{
	"WEDDING_RING_0":            {"WEDDING_RING_1"},
	"WEDDING_RING_2":            {"WEDDING_RING_3"},
	"WEDDING_RING_4":            {"WEDDING_RING_5", "WEDDING_RING_6"},
	"WEDDING_RING_7":            {"WEDDING_RING_8"},
	"CAMPFIRE_TALISMAN_1":       {"CAMPFIRE_TALISMAN_2", "CAMPFIRE_TALISMAN_3"},
	"CAMPFIRE_TALISMAN_4":       {"CAMPFIRE_TALISMAN_5", "CAMPFIRE_TALISMAN_6", "CAMPFIRE_TALISMAN_7"},
	"CAMPFIRE_TALISMAN_8":       {"CAMPFIRE_TALISMAN_9", "CAMPFIRE_TALISMAN_10", "CAMPFIRE_TALISMAN_11", "CAMPFIRE_TALISMAN_12"},
	"CAMPFIRE_TALISMAN_13":      {"CAMPFIRE_TALISMAN_14", "CAMPFIRE_TALISMAN_15", "CAMPFIRE_TALISMAN_16", "CAMPFIRE_TALISMAN_17", "CAMPFIRE_TALISMAN_18", "CAMPFIRE_TALISMAN_19", "CAMPFIRE_TALISMAN_20"},
	"CAMPFIRE_TALISMAN_21":      {"CAMPFIRE_TALISMAN_22", "CAMPFIRE_TALISMAN_23", "CAMPFIRE_TALISMAN_24", "CAMPFIRE_TALISMAN_25", "CAMPFIRE_TALISMAN_26", "CAMPFIRE_TALISMAN_27", "CAMPFIRE_TALISMAN_28", "CAMPFIRE_TALISMAN_29"},
	"PARTY_HAT_CRAB":            {"PARTY_HAT_CRAB_ANIMATED", "PARTY_HAT_SLOTH", "BALLOON_HAT_2024", "BALLOON_HAT_2025", "CAKE_HAT_2026"},
	"DANTE_TALISMAN":            {"DANTE_RING"},
	"SOUL_CAMPFIRE_TALISMAN_1":  {"SOUL_CAMPFIRE_TALISMAN_2", "SOUL_CAMPFIRE_TALISMAN_3"},
	"SOUL_CAMPFIRE_TALISMAN_4":  {"SOUL_CAMPFIRE_TALISMAN_5", "SOUL_CAMPFIRE_TALISMAN_6", "SOUL_CAMPFIRE_TALISMAN_7"},
	"SOUL_CAMPFIRE_TALISMAN_8":  {"SOUL_CAMPFIRE_TALISMAN_9", "SOUL_CAMPFIRE_TALISMAN_10", "SOUL_CAMPFIRE_TALISMAN_11", "SOUL_CAMPFIRE_TALISMAN_12"},
	"SOUL_CAMPFIRE_TALISMAN_13": {"SOUL_CAMPFIRE_TALISMAN_14", "SOUL_CAMPFIRE_TALISMAN_15", "SOUL_CAMPFIRE_TALISMAN_16", "SOUL_CAMPFIRE_TALISMAN_17", "SOUL_CAMPFIRE_TALISMAN_18", "SOUL_CAMPFIRE_TALISMAN_19", "SOUL_CAMPFIRE_TALISMAN_20"},
	"SOUL_CAMPFIRE_TALISMAN_21": {"SOUL_CAMPFIRE_TALISMAN_22", "SOUL_CAMPFIRE_TALISMAN_23", "SOUL_CAMPFIRE_TALISMAN_24", "SOUL_CAMPFIRE_TALISMAN_25", "SOUL_CAMPFIRE_TALISMAN_26", "SOUL_CAMPFIRE_TALISMAN_27", "SOUL_CAMPFIRE_TALISMAN_28", "SOUL_CAMPFIRE_TALISMAN_29"},
}

var extraAccessories = []ExtraAccessory{
	/*
	  {
	    id: "ID",
	    texture: "TEXTURE",
	    name: "NAME",
	    rarity: "RARITY",
	  },
	*/
}

var SPECIAL_ACCESSORIES = map[string]specialAccessoryConstant{
	"BOOK_OF_PROGRESSION": {
		AllowsRecomb:     false,
		Rarities:         []string{"uncommon", "rare", "epic", "legendary", "mythic"},
		AllowsEnrichment: true,
	},
	"PANDORAS_BOX": {
		AllowsRecomb:     false,
		Rarities:         []string{"uncommon", "rare", "epic", "legendary", "mythic"},
		AllowsEnrichment: true,
	},
	"TRAPPER_CREST": {
		AllowsRecomb:     true,
		Rarities:         []string{"uncommon", "rare"},
		AllowsEnrichment: true,
	},
	"PULSE_RING": {
		AllowsRecomb: true,
		Rarities:     []string{"rare", "epic", "legendary"},
		CustomPrice:  true,
		Upgrade: &accessoryUpgrade{
			Item: "THUNDER_IN_A_BOTTLE",
			Cost: map[string]int{
				"rare":      3,
				"epic":      20,
				"legendary": 100,
			},
		},
		AllowsEnrichment: true,
	},
	"POWER_RELIC": {
		AllowsRecomb:     true,
		Rarities:         []string{"legendary"},
		CustomPrice:      true,
		AllowsEnrichment: true,
	},
	"RIFT_PRISM": {
		AllowsRecomb:     false,
		AllowsEnrichment: true,
	},
	"HOCUS_POCUS_CIPHER": {
		AllowsRecomb:     true,
		AllowsEnrichment: false,
	},
	"RUNEBOOK": {
		AllowsRecomb:     false,
		AllowsEnrichment: true,
	},
	"VOTER_BADGE_SUPREME": {
		AllowsRecomb:     false,
		AllowsEnrichment: true,
	},
	"SAFETY_BADGE": {
		AllowsRecomb:     false,
		AllowsEnrichment: true,
	},
}

var MAGICAL_POWER = map[string]int{
	"common":       3,
	"uncommon":     5,
	"rare":         8,
	"epic":         12,
	"legendary":    16,
	"mythic":       22,
	"special":      3,
	"very_special": 5,
}

var ENRICHMENT_TO_STAT = map[string]string{
	"walk_speed": "speed",
}

func findInAliases(id string) bool {
	for _, aliases := range ACCESSORY_ALIASES {
		if slices.Contains(aliases, id) {
			return true
		}
	}
	return false
}

func GetAllAccessories() []Accessory {
	var output []Accessory

	for _, item := range ACCESSORIES {
		if slices.Contains(ignoredAccessories, item.SkyblockID) {
			continue
		}

		if item.Origin == "RIFT" && !item.RiftTransferrable {
			continue
		}

		if findInAliases(item.SkyblockID) {
			continue
		}

		texturePath := ""
		if item.Texture != "" {
			texturePath = fmt.Sprintf("%s/api/head/%s", getDomain(), item.Texture)
		} else {
			texturePath = fmt.Sprintf("%s/api/item/%s:%d", getDomain(), item.Material, item.Damage)
		}

		accessory := Accessory{
			SkyBlockID: item.SkyblockID,
			Texture:    texturePath,
			Rarity:     item.Rarity,
			Name:       item.Name,
		}

		output = append(output, accessory)

		if specialAccessory, exists := SPECIAL_ACCESSORIES[item.SkyblockID]; exists && len(specialAccessory.Rarities) > 0 {
			for _, rarity := range specialAccessory.Rarities {
				specialAccessoryItem := Accessory{
					SkyBlockID: item.SkyblockID,
					Texture:    texturePath,
					Rarity:     rarity,
					Name:       item.Name,
				}
				output = append(output, specialAccessoryItem)
				// fmt.Printf("[ACCESSORIES] Added special accessory %s with rarity %s\n", specialAccessoryItem.SkyBlockID, specialAccessoryItem.Rarity)
			}
		}
	}

	for _, extra := range extraAccessories {
		output = append(output, Accessory{
			SkyBlockID: extra.ID,
			Texture:    extra.Texture,
			Rarity:     extra.Rarity,
			Name:       extra.Name,
		})
	}

	return output
}

func GetMaxAccessories() []Accessory {
	allAccessories := GetAllAccessories()
	var maxAccessories []Accessory

	for _, item := range allAccessories {
		upgradeList := GetUpgradeList(item.SkyBlockID)

		if len(upgradeList) == 0 || upgradeList[len(upgradeList)-1] == item.SkyBlockID {
			maxAccessories = append(maxAccessories, item)
		}
	}

	return maxAccessories
}

func GetUniqueAccessoriesCount() int {
	maxAccessories := GetMaxAccessories()
	uniqueIDs := make(map[string]bool)

	for _, accessory := range maxAccessories {
		uniqueIDs[accessory.SkyBlockID] = true
	}

	return len(uniqueIDs)
}

func GetRecombableAccessoriesCount() int {
	maxAccessories := GetMaxAccessories()
	uniqueIDs := make(map[string]bool)

	for _, accessory := range maxAccessories {
		var special, exists = SPECIAL_ACCESSORIES[accessory.SkyBlockID]
		if exists && !special.AllowsRecomb {
			continue
		}

		uniqueIDs[accessory.SkyBlockID] = true
	}

	return len(uniqueIDs)
}

func GetUpgradeList(id string) []string {
	for _, upgradeChain := range AccessoryUpgrades {
		if slices.Contains(upgradeChain, id) {
			return upgradeChain
		}
	}

	return []string{}
}

func GetBaseIdFromAlias(id string) string {
	for base, aliases := range ACCESSORY_ALIASES {
		if slices.Contains(aliases, id) {
			return base
		}
	}
	return id
}

func init() {
	go func() {
		getAccessories()
		for len(ACCESSORIES) == 0 {
			time.Sleep(1 * time.Second)
			getAccessories()
		}

		ticker := time.NewTicker(60 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			getAccessories()
		}
	}()
}
