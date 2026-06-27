package minecraftblockrenderer

import (
	"fmt"
	"strconv"
	"strings"
)

func legacyNumericItemID(id int, damage int) (string, bool) {
	return getItemIdFromNumericId(id, damage)
}

func legacyStringItemID(id string, damage int) (string, bool) {
	base, parsedDamage, ok := parseLegacyItemString(id)
	if !ok {
		return "", false
	}
	wasNamespaced := strings.HasPrefix(strings.ToLower(strings.TrimSpace(base)), "minecraft:")

	effectiveDamage := damage
	if parsedDamage != nil {
		effectiveDamage = *parsedDamage
	}

	if numericID, err := strconv.Atoi(base); err == nil {
		return legacyNumericItemID(numericID, effectiveDamage)
	}

	normalized := normalizeLegacyItemAlias(base)
	if normalized == "" {
		return "", false
	}

	if normalized == "dye" || normalized == "ink_sack" {
		return legacyNumericItemID(351, effectiveDamage)
	}

	if wasNamespaced && !legacyNamespacedItemAliases[normalized] {
		return "", false
	}

	if numericID, ok := BUKKIT_TO_ID[strings.ToUpper(normalized)]; ok {
		return legacyNumericItemID(numericID, effectiveDamage)
	}

	if mapped, ok := MINECRAFT_BLOCK_MAPPINGS[fmt.Sprintf("%s:%d", normalized, effectiveDamage)]; ok {
		return mapped, true
	}

	return "", false
}

func parseLegacyItemString(id string) (base string, parsedDamage *int, ok bool) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", nil, false
	}

	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	base = trimmed

	if split := strings.LastIndex(base, ":"); split > 0 && split < len(base)-1 {
		suffix := strings.TrimSpace(base[split+1:])
		if damage, err := strconv.Atoi(suffix); err == nil {
			copyDamage := damage
			parsedDamage = &copyDamage
			base = strings.TrimSpace(base[:split])
		}
	}

	if parsedDamage == nil {
		if split := strings.LastIndex(base, "-"); split > 0 && split < len(base)-1 {
			suffix := strings.TrimSpace(base[split+1:])
			if damage, err := strconv.Atoi(suffix); err == nil {
				copyDamage := damage
				parsedDamage = &copyDamage
				base = strings.TrimSpace(base[:split])
			}
		}
	}

	base = strings.TrimSpace(base)
	if base == "" {
		return "", parsedDamage, false
	}
	return base, parsedDamage, true
}

func resolveLegacyItemName(itemID string, numericID *int, damage int) (string, bool) {
	if numericID != nil {
		if mapped, ok := legacyNumericItemID(*numericID, damage); ok {
			return mapped, true
		}
	}
	return legacyStringItemID(itemID, damage)
}

func normalizeLegacyItemAlias(id string) string {
	normalized := strings.TrimSpace(id)
	if normalized == "" {
		return ""
	}
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	normalized = strings.Trim(normalized, "/")
	lower := strings.ToLower(normalized)
	if strings.HasPrefix(lower, "minecraft:") {
		normalized = normalized[len("minecraft:"):]
		lower = strings.ToLower(normalized)
	}
	if strings.HasPrefix(lower, "item/") {
		normalized = normalized[len("item/"):]
	} else if strings.HasPrefix(lower, "items/") {
		normalized = normalized[len("items/"):]
	}
	normalized = strings.Trim(normalized, "/")
	normalized = strings.ToLower(normalized)
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

var legacyNamespacedItemAliases = map[string]bool{
	"dye":                      true,
	"ink_sack":                 true,
	"wool":                     true,
	"wood":                     true,
	"log":                      true,
	"log2":                     true,
	"leaves":                   true,
	"leaves2":                  true,
	"sapling":                  true,
	"stone":                    true,
	"dirt":                     true,
	"sand":                     true,
	"sandstone":                true,
	"red_sandstone":            true,
	"stone_slab":               true,
	"stone_slab2":              true,
	"wooden_slab":              true,
	"double_stone_slab":        true,
	"double_stone_slab2":       true,
	"double_wooden_slab":       true,
	"stained_glass":            true,
	"stained_glass_pane":       true,
	"stained_clay":             true,
	"stained_hardened_clay":    true,
	"carpet":                   true,
	"concrete":                 true,
	"concrete_powder":          true,
	"skull":                    true,
	"skull_item":               true,
	"monster_egg":              true,
	"monster_eggs":             true,
	"red_flower":               true,
	"yellow_flower":            true,
	"long_grass":               true,
	"double_plant":             true,
	"prismarine":               true,
	"quartz_block":             true,
	"coal":                     true,
	"golden_apple":             true,
	"raw_fish":                 true,
	"cooked_fish":              true,
	"fish":                     true,
	"banner":                   true,
	"standing_banner":          true,
	"wall_banner":              true,
	"anvil":                    true,
	"cobble_wall":              true,
	"cobblestone_wall":         true,
	"leaves_2":                 true,
	"wood_step":                true,
	"wood_double_step":         true,
	"smooth_brick":             true,
	"red_rose":                 true,
	"silver_shulker_box":       true,
	"silver_glazed_terracotta": true,
}

var BUKKIT_TO_ID = map[string]int{
	"AIR":                          0,
	"STONE":                        1,
	"GRASS":                        2,
	"DIRT":                         3,
	"COBBLESTONE":                  4,
	"WOOD":                         5,
	"SAPLING":                      6,
	"BEDROCK":                      7,
	"WATER":                        8,
	"STATIONARY_WATER":             9,
	"LAVA":                         10,
	"STATIONARY_LAVA":              11,
	"SAND":                         12,
	"GRAVEL":                       13,
	"GOLD_ORE":                     14,
	"IRON_ORE":                     15,
	"COAL_ORE":                     16,
	"LOG":                          17,
	"LEAVES":                       18,
	"SPONGE":                       19,
	"GLASS":                        20,
	"LAPIS_ORE":                    21,
	"LAPIS_BLOCK":                  22,
	"DISPENSER":                    23,
	"SANDSTONE":                    24,
	"NOTE_BLOCK":                   25,
	"BED_BLOCK":                    26,
	"POWERED_RAIL":                 27,
	"DETECTOR_RAIL":                28,
	"PISTON_STICKY_BASE":           29,
	"WEB":                          30,
	"LONG_GRASS":                   31,
	"DEAD_BUSH":                    32,
	"PISTON_BASE":                  33,
	"PISTON_EXTENSION":             34,
	"WOOL":                         35,
	"PISTON_MOVING_PIECE":          36,
	"YELLOW_FLOWER":                37,
	"RED_ROSE":                     38,
	"BROWN_MUSHROOM":               39,
	"RED_MUSHROOM":                 40,
	"GOLD_BLOCK":                   41,
	"IRON_BLOCK":                   42,
	"DOUBLE_STEP":                  43,
	"STEP":                         44,
	"BRICK":                        45,
	"TNT":                          46,
	"BOOKSHELF":                    47,
	"MOSSY_COBBLESTONE":            48,
	"OBSIDIAN":                     49,
	"TORCH":                        50,
	"FIRE":                         51,
	"MOB_SPAWNER":                  52,
	"WOOD_STAIRS":                  53,
	"CHEST":                        54,
	"REDSTONE_WIRE":                55,
	"DIAMOND_ORE":                  56,
	"DIAMOND_BLOCK":                57,
	"WORKBENCH":                    58,
	"CROPS":                        59,
	"SOIL":                         60,
	"FURNACE":                      61,
	"BURNING_FURNACE":              62,
	"SIGN_POST":                    63,
	"WOODEN_DOOR":                  64,
	"LADDER":                       65,
	"RAILS":                        66,
	"COBBLESTONE_STAIRS":           67,
	"WALL_SIGN":                    68,
	"LEVER":                        69,
	"STONE_PLATE":                  70,
	"IRON_DOOR_BLOCK":              71,
	"WOOD_PLATE":                   72,
	"REDSTONE_ORE":                 73,
	"GLOWING_REDSTONE_ORE":         74,
	"REDSTONE_TORCH_OFF":           75,
	"REDSTONE_TORCH_ON":            76,
	"STONE_BUTTON":                 77,
	"SNOW":                         78,
	"ICE":                          79,
	"SNOW_BLOCK":                   80,
	"CACTUS":                       81,
	"CLAY":                         82,
	"SUGAR_CANE_BLOCK":             83,
	"JUKEBOX":                      84,
	"FENCE":                        85,
	"PUMPKIN":                      86,
	"NETHERRACK":                   87,
	"SOUL_SAND":                    88,
	"GLOWSTONE":                    89,
	"PORTAL":                       90,
	"JACK_O_LANTERN":               91,
	"CAKE_BLOCK":                   92,
	"DIODE_BLOCK_OFF":              93,
	"DIODE_BLOCK_ON":               94,
	"STAINED_GLASS":                95,
	"TRAP_DOOR":                    96,
	"MONSTER_EGGS":                 97,
	"SMOOTH_BRICK":                 98,
	"HUGE_MUSHROOM_1":              99,
	"HUGE_MUSHROOM_2":              100,
	"IRON_FENCE":                   101,
	"THIN_GLASS":                   102,
	"MELON_BLOCK":                  103,
	"PUMPKIN_STEM":                 104,
	"MELON_STEM":                   105,
	"VINE":                         106,
	"FENCE_GATE":                   107,
	"BRICK_STAIRS":                 108,
	"SMOOTH_STAIRS":                109,
	"MYCEL":                        110,
	"WATER_LILY":                   111,
	"NETHER_BRICK":                 112,
	"NETHER_FENCE":                 113,
	"NETHER_BRICK_STAIRS":          114,
	"NETHER_WARTS":                 115,
	"ENCHANTMENT_TABLE":            116,
	"BREWING_STAND":                117,
	"CAULDRON":                     118,
	"ENDER_PORTAL":                 119,
	"ENDER_PORTAL_FRAME":           120,
	"ENDER_STONE":                  121,
	"DRAGON_EGG":                   122,
	"REDSTONE_LAMP_OFF":            123,
	"REDSTONE_LAMP_ON":             124,
	"WOOD_DOUBLE_STEP":             125,
	"WOOD_STEP":                    126,
	"COCOA":                        127,
	"SANDSTONE_STAIRS":             128,
	"EMERALD_ORE":                  129,
	"ENDER_CHEST":                  130,
	"TRIPWIRE_HOOK":                131,
	"TRIPWIRE":                     132,
	"EMERALD_BLOCK":                133,
	"SPRUCE_WOOD_STAIRS":           134,
	"BIRCH_WOOD_STAIRS":            135,
	"JUNGLE_WOOD_STAIRS":           136,
	"COMMAND":                      137,
	"BEACON":                       138,
	"COBBLE_WALL":                  139,
	"FLOWER_POT":                   140,
	"CARROT":                       141,
	"POTATO":                       142,
	"WOOD_BUTTON":                  143,
	"SKULL":                        144,
	"ANVIL":                        145,
	"TRAPPED_CHEST":                146,
	"GOLD_PLATE":                   147,
	"IRON_PLATE":                   148,
	"REDSTONE_COMPARATOR_OFF":      149,
	"REDSTONE_COMPARATOR_ON":       150,
	"DAYLIGHT_DETECTOR":            151,
	"REDSTONE_BLOCK":               152,
	"QUARTZ_ORE":                   153,
	"HOPPER":                       154,
	"QUARTZ_BLOCK":                 155,
	"QUARTZ_STAIRS":                156,
	"ACTIVATOR_RAIL":               157,
	"DROPPER":                      158,
	"STAINED_CLAY":                 159,
	"STAINED_GLASS_PANE":           160,
	"LEAVES_2":                     161,
	"LOG_2":                        162,
	"ACACIA_STAIRS":                163,
	"DARK_OAK_STAIRS":              164,
	"SLIME_BLOCK":                  165,
	"BARRIER":                      166,
	"IRON_TRAPDOOR":                167,
	"PRISMARINE":                   168,
	"SEA_LANTERN":                  169,
	"HAY_BLOCK":                    170,
	"CARPET":                       171,
	"HARD_CLAY":                    172,
	"COAL_BLOCK":                   173,
	"PACKED_ICE":                   174,
	"DOUBLE_PLANT":                 175,
	"STANDING_BANNER":              176,
	"WALL_BANNER":                  177,
	"DAYLIGHT_DETECTOR_INVERTED":   178,
	"RED_SANDSTONE":                179,
	"RED_SANDSTONE_STAIRS":         180,
	"DOUBLE_STONE_SLAB2":           181,
	"STONE_SLAB2":                  182,
	"SPRUCE_FENCE_GATE":            183,
	"BIRCH_FENCE_GATE":             184,
	"JUNGLE_FENCE_GATE":            185,
	"DARK_OAK_FENCE_GATE":          186,
	"ACACIA_FENCE_GATE":            187,
	"SPRUCE_FENCE":                 188,
	"BIRCH_FENCE":                  189,
	"JUNGLE_FENCE":                 190,
	"DARK_OAK_FENCE":               191,
	"ACACIA_FENCE":                 192,
	"SPRUCE_DOOR":                  193,
	"BIRCH_DOOR":                   194,
	"JUNGLE_DOOR":                  195,
	"ACACIA_DOOR":                  196,
	"DARK_OAK_DOOR":                197,
	"END_ROD":                      198,
	"CHORUS_PLANT":                 199,
	"CHORUS_FLOWER":                200,
	"PURPUR_BLOCK":                 201,
	"PURPUR_PILLAR":                202,
	"PURPUR_STAIRS":                203,
	"PURPUR_DOUBLE_SLAB":           204,
	"PURPUR_SLAB":                  205,
	"END_BRICKS":                   206,
	"BEETROOT_BLOCK":               207,
	"GRASS_PATH":                   208,
	"END_GATEWAY":                  209,
	"COMMAND_REPEATING":            210,
	"COMMAND_CHAIN":                211,
	"FROSTED_ICE":                  212,
	"MAGMA":                        213,
	"NETHER_WART_BLOCK":            214,
	"RED_NETHER_BRICK":             215,
	"BONE_BLOCK":                   216,
	"STRUCTURE_VOID":               217,
	"OBSERVER":                     218,
	"WHITE_SHULKER_BOX":            219,
	"ORANGE_SHULKER_BOX":           220,
	"MAGENTA_SHULKER_BOX":          221,
	"LIGHT_BLUE_SHULKER_BOX":       222,
	"YELLOW_SHULKER_BOX":           223,
	"LIME_SHULKER_BOX":             224,
	"PINK_SHULKER_BOX":             225,
	"GRAY_SHULKER_BOX":             226,
	"SILVER_SHULKER_BOX":           227,
	"CYAN_SHULKER_BOX":             228,
	"PURPLE_SHULKER_BOX":           229,
	"BLUE_SHULKER_BOX":             230,
	"BROWN_SHULKER_BOX":            231,
	"GREEN_SHULKER_BOX":            232,
	"RED_SHULKER_BOX":              233,
	"BLACK_SHULKER_BOX":            234,
	"WHITE_GLAZED_TERRACOTTA":      235,
	"ORANGE_GLAZED_TERRACOTTA":     236,
	"MAGENTA_GLAZED_TERRACOTTA":    237,
	"LIGHT_BLUE_GLAZED_TERRACOTTA": 238,
	"YELLOW_GLAZED_TERRACOTTA":     239,
	"LIME_GLAZED_TERRACOTTA":       240,
	"PINK_GLAZED_TERRACOTTA":       241,
	"GRAY_GLAZED_TERRACOTTA":       242,
	"SILVER_GLAZED_TERRACOTTA":     243,
	"CYAN_GLAZED_TERRACOTTA":       244,
	"PURPLE_GLAZED_TERRACOTTA":     245,
	"BLUE_GLAZED_TERRACOTTA":       246,
	"BROWN_GLAZED_TERRACOTTA":      247,
	"GREEN_GLAZED_TERRACOTTA":      248,
	"RED_GLAZED_TERRACOTTA":        249,
	"BLACK_GLAZED_TERRACOTTA":      250,
	"CONCRETE":                     251,
	"CONCRETE_POWDER":              252,
	"STRUCTURE_BLOCK":              255,
	"IRON_SPADE":                   256,
	"IRON_PICKAXE":                 257,
	"IRON_AXE":                     258,
	"FLINT_AND_STEEL":              259,
	"APPLE":                        260,
	"BOW":                          261,
	"ARROW":                        262,
	"COAL":                         263,
	"DIAMOND":                      264,
	"IRON_INGOT":                   265,
	"GOLD_INGOT":                   266,
	"IRON_SWORD":                   267,
	"WOOD_SWORD":                   268,
	"WOOD_SPADE":                   269,
	"WOOD_PICKAXE":                 270,
	"WOOD_AXE":                     271,
	"STONE_SWORD":                  272,
	"STONE_SPADE":                  273,
	"STONE_PICKAXE":                274,
	"STONE_AXE":                    275,
	"DIAMOND_SWORD":                276,
	"DIAMOND_SPADE":                277,
	"DIAMOND_PICKAXE":              278,
	"DIAMOND_AXE":                  279,
	"STICK":                        280,
	"BOWL":                         281,
	"MUSHROOM_SOUP":                282,
	"GOLD_SWORD":                   283,
	"GOLD_SPADE":                   284,
	"GOLD_PICKAXE":                 285,
	"GOLD_AXE":                     286,
	"STRING":                       287,
	"FEATHER":                      288,
	"SULPHUR":                      289,
	"WOOD_HOE":                     290,
	"STONE_HOE":                    291,
	"IRON_HOE":                     292,
	"DIAMOND_HOE":                  293,
	"GOLD_HOE":                     294,
	"SEEDS":                        295,
	"WHEAT":                        296,
	"BREAD":                        297,
	"LEATHER_HELMET":               298,
	"LEATHER_CHESTPLATE":           299,
	"LEATHER_LEGGINGS":             300,
	"LEATHER_BOOTS":                301,
	"CHAINMAIL_HELMET":             302,
	"CHAINMAIL_CHESTPLATE":         303,
	"CHAINMAIL_LEGGINGS":           304,
	"CHAINMAIL_BOOTS":              305,
	"IRON_HELMET":                  306,
	"IRON_CHESTPLATE":              307,
	"IRON_LEGGINGS":                308,
	"IRON_BOOTS":                   309,
	"DIAMOND_HELMET":               310,
	"DIAMOND_CHESTPLATE":           311,
	"DIAMOND_LEGGINGS":             312,
	"DIAMOND_BOOTS":                313,
	"GOLD_HELMET":                  314,
	"GOLD_CHESTPLATE":              315,
	"GOLD_LEGGINGS":                316,
	"GOLD_BOOTS":                   317,
	"FLINT":                        318,
	"PORK":                         319,
	"GRILLED_PORK":                 320,
	"PAINTING":                     321,
	"GOLDEN_APPLE":                 322,
	"SIGN":                         323,
	"WOOD_DOOR":                    324,
	"BUCKET":                       325,
	"WATER_BUCKET":                 326,
	"LAVA_BUCKET":                  327,
	"MINECART":                     328,
	"SADDLE":                       329,
	"IRON_DOOR":                    330,
	"REDSTONE":                     331,
	"SNOW_BALL":                    332,
	"BOAT":                         333,
	"LEATHER":                      334,
	"MILK_BUCKET":                  335,
	"CLAY_BRICK":                   336,
	"CLAY_BALL":                    337,
	"SUGAR_CANE":                   338,
	"PAPER":                        339,
	"BOOK":                         340,
	"SLIME_BALL":                   341,
	"STORAGE_MINECART":             342,
	"POWERED_MINECART":             343,
	"EGG":                          344,
	"COMPASS":                      345,
	"FISHING_ROD":                  346,
	"WATCH":                        347,
	"GLOWSTONE_DUST":               348,
	"RAW_FISH":                     349,
	"COOKED_FISH":                  350,
	"INK_SACK":                     351,
	"BONE":                         352,
	"SUGAR":                        353,
	"CAKE":                         354,
	"BED":                          355,
	"DIODE":                        356,
	"COOKIE":                       357,
	"MAP":                          358,
	"SHEARS":                       359,
	"MELON":                        360,
	"PUMPKIN_SEEDS":                361,
	"MELON_SEEDS":                  362,
	"RAW_BEEF":                     363,
	"COOKED_BEEF":                  364,
	"RAW_CHICKEN":                  365,
	"COOKED_CHICKEN":               366,
	"ROTTEN_FLESH":                 367,
	"ENDER_PEARL":                  368,
	"BLAZE_ROD":                    369,
	"GHAST_TEAR":                   370,
	"GOLD_NUGGET":                  371,
	"NETHER_STALK":                 372,
	"POTION":                       373,
	"GLASS_BOTTLE":                 374,
	"SPIDER_EYE":                   375,
	"FERMENTED_SPIDER_EYE":         376,
	"BLAZE_POWDER":                 377,
	"MAGMA_CREAM":                  378,
	"BREWING_STAND_ITEM":           379,
	"CAULDRON_ITEM":                380,
	"EYE_OF_ENDER":                 381,
	"SPECKLED_MELON":               382,
	"MONSTER_EGG":                  383,
	"EXP_BOTTLE":                   384,
	"FIREBALL":                     385,
	"BOOK_AND_QUILL":               386,
	"WRITTEN_BOOK":                 387,
	"EMERALD":                      388,
	"ITEM_FRAME":                   389,
	"FLOWER_POT_ITEM":              390,
	"CARROT_ITEM":                  391,
	"POTATO_ITEM":                  392,
	"BAKED_POTATO":                 393,
	"POISONOUS_POTATO":             394,
	"EMPTY_MAP":                    395,
	"GOLDEN_CARROT":                396,
	"SKULL_ITEM":                   397,
	"CARROT_STICK":                 398,
	"NETHER_STAR":                  399,
	"PUMPKIN_PIE":                  400,
	"FIREWORK":                     401,
	"FIREWORK_CHARGE":              402,
	"ENCHANTED_BOOK":               403,
	"REDSTONE_COMPARATOR":          404,
	"NETHER_BRICK_ITEM":            405,
	"QUARTZ":                       406,
	"EXPLOSIVE_MINECART":           407,
	"HOPPER_MINECART":              408,
	"PRISMARINE_SHARD":             409,
	"PRISMARINE_CRYSTALS":          410,
	"RABBIT":                       411,
	"COOKED_RABBIT":                412,
	"RABBIT_STEW":                  413,
	"RABBIT_FOOT":                  414,
	"RABBIT_HIDE":                  415,
	"ARMOR_STAND":                  416,
	"IRON_BARDING":                 417,
	"GOLD_BARDING":                 418,
	"DIAMOND_BARDING":              419,
	"LEASH":                        420,
	"NAME_TAG":                     421,
	"COMMAND_MINECART":             422,
	"MUTTON":                       423,
	"COOKED_MUTTON":                424,
	"BANNER":                       425,
	"END_CRYSTAL":                  426,
	"SPRUCE_DOOR_ITEM":             427,
	"BIRCH_DOOR_ITEM":              428,
	"JUNGLE_DOOR_ITEM":             429,
	"ACACIA_DOOR_ITEM":             430,
	"DARK_OAK_DOOR_ITEM":           431,
	"CHORUS_FRUIT":                 432,
	"CHORUS_FRUIT_POPPED":          433,
	"BEETROOT":                     434,
	"BEETROOT_SEEDS":               435,
	"BEETROOT_SOUP":                436,
	"DRAGONS_BREATH":               437,
	"SPLASH_POTION":                438,
	"SPECTRAL_ARROW":               439,
	"TIPPED_ARROW":                 440,
	"LINGERING_POTION":             441,
	"SHIELD":                       442,
	"ELYTRA":                       443,
	"BOAT_SPRUCE":                  444,
	"BOAT_BIRCH":                   445,
	"BOAT_JUNGLE":                  446,
	"BOAT_ACACIA":                  447,
	"BOAT_DARK_OAK":                448,
	"TOTEM":                        449,
	"SHULKER_SHELL":                450,
	"IRON_NUGGET":                  452,
	"KNOWLEDGE_BOOK":               453,
	"GOLD_RECORD":                  2256,
	"GREEN_RECORD":                 2257,
	"RECORD_3":                     2258,
	"RECORD_4":                     2259,
	"RECORD_5":                     2260,
	"RECORD_6":                     2261,
	"RECORD_7":                     2262,
	"RECORD_8":                     2263,
	"RECORD_9":                     2264,
	"RECORD_10":                    2265,
	"RECORD_11":                    2266,
	"RECORD_12":                    2267,
}

// Credits: https://github.com/ViaVersion/ViaVersion/blob/fe4e4075200484fbc1d7a194d032d0eb70d99cdb/common/src/main/resources/assets/viaversion/data/blockIds1.12to1.13.json
var MinecraftBlockMapping = map[string][]string{
	"anvil":                      {"anvil", "chipped_anvil", "damaged_anvil"},
	"banner":                     {"white_banner", "orange_banner", "magenta_banner", "light_blue_banner", "yellow_banner", "lime_banner", "pink_banner", "gray_banner", "light_gray_banner", "cyan_banner", "purple_banner", "blue_banner", "brown_banner", "green_banner", "red_banner", "black_banner"},
	"bed":                        {"white_bed", "orange_bed", "magenta_bed", "light_blue_bed", "yellow_bed", "lime_bed", "pink_bed", "gray_bed", "light_gray_bed", "cyan_bed", "purple_bed", "blue_bed", "brown_bed", "green_bed", "red_bed", "black_bed"},
	"boat":                       {"oak_boat"},
	"brick_block":                {"bricks"},
	"brown_mushroom_block":       {"brown_mushroom_block", "mushroom_stem", "red_mushroom_block"},
	"carpet":                     {"white_carpet", "orange_carpet", "magenta_carpet", "light_blue_carpet", "yellow_carpet", "lime_carpet", "pink_carpet", "gray_carpet", "light_gray_carpet", "cyan_carpet", "purple_carpet", "blue_carpet", "brown_carpet", "green_carpet", "red_carpet", "black_carpet"},
	"chorus_fruit_popped":        {"popped_chorus_fruit"},
	"coal":                       {"coal", "charcoal"},
	"cobblestone_wall":           {"cobblestone_wall", "mossy_cobblestone_wall"},
	"comparator":                 {"comparator"},
	"concrete":                   {"white_concrete", "orange_concrete", "magenta_concrete", "light_blue_concrete", "yellow_concrete", "lime_concrete", "pink_concrete", "gray_concrete", "light_gray_concrete", "cyan_concrete", "purple_concrete", "blue_concrete", "brown_concrete", "green_concrete", "red_concrete", "black_concrete"},
	"concrete_powder":            {"white_concrete_powder", "orange_concrete_powder", "magenta_concrete_powder", "light_blue_concrete_powder", "yellow_concrete_powder", "lime_concrete_powder", "pink_concrete_powder", "gray_concrete_powder", "light_gray_concrete_powder", "cyan_concrete_powder", "purple_concrete_powder", "blue_concrete_powder", "brown_concrete_powder", "green_concrete_powder", "red_concrete_powder", "black_concrete_powder"},
	"cooked_fish":                {"cooked_cod", "cooked_salmon"},
	"daylight_detector":          {"daylight_detector"},
	"daylight_detector_inverted": {"daylight_detector"},
	"deadbush":                   {"dead_bush"},
	"dirt":                       {"dirt", "coarse_dirt", "podzol"},
	"double_plant":               {"sunflower", "lilac", "tall_grass", "large_fern", "rose_bush", "peony"},
	"double_purpur_slab":         {"purpur_slab"},
	"double_stone_slab":          {"stone_slab", "sandstone_slab", "petrified_oak_slab", "cobblestone_slab", "brick_slab", "stone_brick_slab", "nether_brick_slab", "quartz_slab", "smooth_stone", "smooth_sandstone", "smooth_quartz"},
	"double_stone_slab2":         {"red_sandstone_slab", "smooth_red_sandstone"},
	"double_wooden_slab":         {"oak_slab", "spruce_slab", "birch_slab", "jungle_slab", "acacia_slab", "dark_oak_slab"},
	"dye":                        {"bone_meal", "orange_dye", "magenta_dye", "light_blue_dye", "dandelion_yellow", "lime_dye", "pink_dye", "gray_dye", "light_gray_dye", "cyan_dye", "purple_dye", "lapis_lazuli", "cocoa_beans", "cactus_green", "rose_red", "ink_sac"},
	"end_bricks":                 {"end_stone_bricks"},
	"fence":                      {"oak_fence"},
	"fence_gate":                 {"oak_fence_gate"},
	"firework_charge":            {"firework_star"},
	"fireworks":                  {"firework_rocket"},
	"fish":                       {"cod", "salmon", "tropical_fish", "pufferfish"},
	"flower_pot":                 {"flower_pot", "potted_poppy", "potted_dandelion", "potted_oak_sapling", "potted_spruce_sapling", "potted_birch_sapling", "potted_jungle_sapling", "potted_red_mushroom", "potted_brown_mushroom", "potted_cactus", "potted_dead_bush", "potted_fern", "potted_acacia_sapling", "potted_dark_oak_sapling", "potted_blue_orchid", "potted_allium", "potted_azure_bluet", "potted_red_tulip", "potted_orange_tulip", "potted_white_tulip", "potted_pink_tulip", "potted_oxeye_daisy"},
	"flowing_lava":               {"lava"},
	"flowing_water":              {"water"},
	"furnace":                    {"furnace"},
	"golden_apple":               {"golden_apple", "enchanted_golden_apple"},
	"golden_rail":                {"powered_rail"},
	"grass":                      {"grass_block"},
	"hardened_clay":              {"terracotta"},
	"lava":                       {"lava"},
	"leaves":                     {"oak_leaves", "spruce_leaves", "birch_leaves", "jungle_leaves"},
	"leaves2":                    {"acacia_leaves", "dark_oak_leaves"},
	"lit_furnace":                {"furnace"},
	"lit_pumpkin":                {"jack_o_lantern"},
	"lit_redstone_lamp":          {"redstone_lamp"},
	"lit_redstone_ore":           {"redstone_ore"},
	"log":                        {"oak_log", "spruce_log", "birch_log", "jungle_log", "oak_wood", "spruce_wood", "birch_wood", "jungle_wood"},
	"log2":                       {"acacia_log", "dark_oak_log", "acacia_wood", "dark_oak_wood"},
	"magma":                      {"magma_block"},
	"melon":                      {"melon_slice"},
	"melon_block":                {"melon"},
	"melon_stem":                 {"melon_stem", "attached_melon_stem"},
	"mob_spawner":                {"spawner"},
	"monster_egg":                {"infested_stone", "infested_cobblestone", "infested_stone_bricks", "infested_mossy_stone_bricks", "infested_cracked_stone_bricks", "infested_chiseled_stone_bricks"},
	"nether_brick":               {"nether_bricks"},
	"netherbrick":                {"nether_brick"},
	"noteblock":                  {"note_block"},
	"piston_extension":           {"moving_piston"},
	"planks":                     {"oak_planks", "spruce_planks", "birch_planks", "jungle_planks", "acacia_planks", "dark_oak_planks"},
	"portal":                     {"nether_portal"},
	"powered_comparator":         {"comparator"},
	"powered_repeater":           {"repeater"},
	"prismarine":                 {"prismarine", "prismarine_bricks", "dark_prismarine"},
	"pumpkin":                    {"carved_pumpkin"},
	"pumpkin_stem":               {"pumpkin_stem", "attached_pumpkin_stem"},
	"purpur_slab":                {"purpur_slab"},
	"quartz_block":               {"quartz_block", "chiseled_quartz_block", "quartz_pillar"},
	"quartz_ore":                 {"nether_quartz_ore"},
	"record_11":                  {"music_disc_11"},
	"record_13":                  {"music_disc_13"},
	"record_blocks":              {"music_disc_blocks"},
	"record_cat":                 {"music_disc_cat"},
	"record_chirp":               {"music_disc_chirp"},
	"record_far":                 {"music_disc_far"},
	"record_mall":                {"music_disc_mall"},
	"record_mellohi":             {"music_disc_mellohi"},
	"record_stal":                {"music_disc_stal"},
	"record_strad":               {"music_disc_strad"},
	"record_wait":                {"music_disc_wait"},
	"record_ward":                {"music_disc_ward"},
	"red_flower":                 {"poppy", "blue_orchid", "allium", "azure_bluet", "red_tulip", "orange_tulip", "white_tulip", "pink_tulip", "oxeye_daisy"},
	"red_mushroom_block":         {"brown_mushroom_block", "mushroom_stem", "red_mushroom_block"},
	"red_nether_brick":           {"red_nether_bricks"},
	"red_sandstone":              {"red_sandstone", "chiseled_red_sandstone", "cut_sandstone"},
	"redstone_lamp":              {"redstone_lamp"},
	"redstone_ore":               {"redstone_ore"},
	"redstone_torch":             {"redstone_wall_torch", "redstone_torch"},
	"reeds":                      {"sugar_cane"},
	"repeater":                   {"repeater"},
	"sand":                       {"sand", "red_sand"},
	"sandstone":                  {"sandstone", "chiseled_sandstone", "cut_sandstone"},
	"sapling":                    {"oak_sapling", "spruce_sapling", "birch_sapling", "jungle_sapling", "acacia_sapling", "dark_oak_sapling"},
	"sign":                       {"sign"},
	"silver_glazed_terracotta":   {"light_gray_glazed_terracotta"},
	"silver_shulker_box":         {"light_gray_shulker_box"},
	"skull":                      {"skeleton_skull", "skeleton_wall_skull", "wither_skeleton_skull", "wither_skeleton_wall_skull", "zombie_head", "zombie_wall_head", "player_head", "player_wall_head", "creeper_head", "creeper_wall_head", "dragon_head", "dragon_wall_head"},
	"slime":                      {"slime_block"},
	"snow":                       {"snow_block"},
	"snow_layer":                 {"snow"},
	"spawn_egg":                  {"bat_spawn_egg", "blaze_spawn_egg", "cave_spider_spawn_egg", "chicken_spawn_egg", "cow_spawn_egg", "creeper_spawn_egg", "donkey_spawn_egg", "elder_guardian_spawn_egg", "enderman_spawn_egg", "endermite_spawn_egg", "evoker_spawn_egg", "ghast_spawn_egg", "guardian_spawn_egg", "horse_spawn_egg", "husk_spawn_egg", "llama_spawn_egg", "magma_cube_spawn_egg", "mooshroom_spawn_egg", "mule_spawn_egg", "ocelot_spawn_egg", "parrot_spawn_egg", "pig_spawn_egg", "polar_bear_spawn_egg", "rabbit_spawn_egg", "sheep_spawn_egg", "shulker_spawn_egg", "silverfish_spawn_egg", "skeleton_spawn_egg", "skeleton_horse_spawn_egg", "slime_spawn_egg", "spider_spawn_egg", "squid_spawn_egg", "stray_spawn_egg", "vex_spawn_egg", "villager_spawn_egg", "vindicator_spawn_egg", "witch_spawn_egg", "wither_skeleton_spawn_egg", "wolf_spawn_egg", "zombie_spawn_egg", "zombie_horse_spawn_egg", "zombie_pigman_spawn_egg", "zombie_villager_spawn_egg"},
	"speckled_melon":             {"glistering_melon_slice"},
	"sponge":                     {"sponge", "wet_sponge"},
	"stained_glass":              {"white_stained_glass", "orange_stained_glass", "magenta_stained_glass", "light_blue_stained_glass", "yellow_stained_glass", "lime_stained_glass", "pink_stained_glass", "gray_stained_glass", "light_gray_stained_glass", "cyan_stained_glass", "purple_stained_glass", "blue_stained_glass", "brown_stained_glass", "green_stained_glass", "red_stained_glass", "black_stained_glass"},
	"stained_glass_pane":         {"white_stained_glass_pane", "orange_stained_glass_pane", "magenta_stained_glass_pane", "light_blue_stained_glass_pane", "yellow_stained_glass_pane", "lime_stained_glass_pane", "pink_stained_glass_pane", "gray_stained_glass_pane", "light_gray_stained_glass_pane", "cyan_stained_glass_pane", "purple_stained_glass_pane", "blue_stained_glass_pane", "brown_stained_glass_pane", "green_stained_glass_pane", "red_stained_glass_pane", "black_stained_glass_pane"},
	"stained_hardened_clay":      {"white_terracotta", "orange_terracotta", "magenta_terracotta", "light_blue_terracotta", "yellow_terracotta", "lime_terracotta", "pink_terracotta", "gray_terracotta", "light_gray_terracotta", "cyan_terracotta", "purple_terracotta", "blue_terracotta", "brown_terracotta", "green_terracotta", "red_terracotta", "black_terracotta"},
	"standing_banner":            {"white_banner", "orange_banner", "magenta_banner", "light_blue_banner", "yellow_banner", "lime_banner", "pink_banner", "gray_banner", "light_gray_banner", "cyan_banner", "purple_banner", "blue_banner", "brown_banner", "green_banner", "red_banner", "black_banner"},
	"standing_sign":              {"sign"},
	"stone":                      {"stone", "granite", "polished_granite", "diorite", "polished_diorite", "andesite", "polished_andesite"},
	"stone_slab":                 {"stone_slab", "sandstone_slab", "petrified_oak_slab", "cobblestone_slab", "brick_slab", "stone_brick_slab", "nether_brick_slab", "quartz_slab", "smooth_stone", "smooth_sandstone", "smooth_quartz"},
	"stone_slab2":                {"red_sandstone_slab", "smooth_red_sandstone"},
	"stone_stairs":               {"cobblestone_stairs"},
	"stonebrick":                 {"stone_bricks", "mossy_stone_bricks", "cracked_stone_bricks", "chiseled_stone_bricks"},
	"tallgrass":                  {"dead_bush", "grass", "fern"},
	"torch":                      {"wall_torch", "torch"},
	"trapdoor":                   {"oak_trapdoor"},
	"unlit_redstone_torch":       {"redstone_wall_torch", "redstone_torch"},
	"unpowered_comparator":       {"comparator"},
	"unpowered_repeater":         {"repeater"},
	"wall_banner":                {"white_wall_banner", "orange_wall_banner", "magenta_wall_banner", "light_blue_wall_banner", "yellow_wall_banner", "lime_wall_banner", "pink_wall_banner", "gray_wall_banner", "light_gray_wall_banner", "cyan_wall_banner", "purple_wall_banner", "blue_wall_banner", "brown_wall_banner", "green_wall_banner", "red_wall_banner", "black_wall_banner"},
	"water":                      {"water"},
	"waterlily":                  {"lily_pad"},
	"web":                        {"cobweb"},
	"wooden_button":              {"oak_button"},
	"wooden_door":                {"oak_door"},
	"wooden_pressure_plate":      {"oak_pressure_plate"},
	"wooden_slab":                {"oak_slab", "spruce_slab", "birch_slab", "jungle_slab", "acacia_slab", "dark_oak_slab"},
	"wool":                       {"white_wool", "orange_wool", "magenta_wool", "light_blue_wool", "yellow_wool", "lime_wool", "pink_wool", "gray_wool", "light_gray_wool", "cyan_wool", "purple_wool", "blue_wool", "brown_wool", "green_wool", "red_wool", "black_wool"},
	"yellow_flower":              {"dandelion"},
}

// Credits: https://github.com/ptlthg/MinecraftRenderer/blob/84f80c0da351be56a00b2c563e7d4df22573be68/MinecraftRenderer/Hypixel/LegacyItemMappings.cs#L28
func getItemIdFromNumericId(numericID int, damage int) (string, bool) {
	var itemID string

	switch numericID {
	//case 0:
	//itemID = "air"
	case 1:
		switch damage {
		case 1:
			itemID = "granite"
		case 2:
			itemID = "polished_granite"
		case 3:
			itemID = "diorite"
		case 4:
			itemID = "polished_diorite"
		case 5:
			itemID = "andesite"
		case 6:
			itemID = "polished_andesite"
		default:
			itemID = "stone"
		}
	case 2:
		itemID = "grass_block"
	case 3:
		switch damage {
		case 1:
			itemID = "coarse_dirt"
		case 2:
			itemID = "podzol"
		default:
			itemID = "dirt"
		}
	case 4:
		itemID = "cobblestone"
	case 5:
		switch damage {
		case 1:
			itemID = "spruce_planks"
		case 2:
			itemID = "birch_planks"
		case 3:
			itemID = "jungle_planks"
		case 4:
			itemID = "acacia_planks"
		case 5:
			itemID = "dark_oak_planks"
		default:
			itemID = "oak_planks"
		}
	case 6:
		switch damage {
		case 1:
			itemID = "spruce_sapling"
		case 2:
			itemID = "birch_sapling"
		case 3:
			itemID = "jungle_sapling"
		case 4:
			itemID = "acacia_sapling"
		case 5:
			itemID = "dark_oak_sapling"
		default:
			itemID = "oak_sapling"
		}
	case 7:
		itemID = "bedrock"
	case 8:
		itemID = "flowing_water"
	case 9:
		itemID = "water"
	case 10:
		itemID = "flowing_lava"
	case 11:
		itemID = "lava"
	case 12:
		if damage == 1 {
			itemID = "red_sand"
		} else {
			itemID = "sand"
		}
	case 13:
		itemID = "gravel"
	case 14:
		itemID = "gold_ore"
	case 15:
		itemID = "iron_ore"
	case 16:
		itemID = "coal_ore"
	case 17:
		switch damage {
		case 1, 5, 9:
			itemID = "spruce_log"
		case 2, 6, 10:
			itemID = "birch_log"
		case 3, 7, 11:
			itemID = "jungle_log"
		case 13:
			itemID = "spruce_wood"
		case 14:
			itemID = "birch_wood"
		case 15:
			itemID = "jungle_wood"
		default:
			itemID = "oak_log"
		}
	case 18:
		switch damage {
		case 1:
			itemID = "spruce_leaves"
		case 2:
			itemID = "birch_leaves"
		case 3:
			itemID = "jungle_leaves"
		default:
			itemID = "oak_leaves"
		}
	case 19:
		if damage == 1 {
			itemID = "wet_sponge"
		} else {
			itemID = "sponge"
		}
	case 20:
		itemID = "glass"
	case 21:
		itemID = "lapis_ore"
	case 22:
		itemID = "lapis_block"
	case 23:
		itemID = "dispenser"
	case 24:
		switch damage {
		case 1:
			itemID = "cut_sandstone"
		case 2:
			itemID = "chiseled_sandstone"
		case 3:
			itemID = "smooth_sandstone"
		default:
			itemID = "sandstone"
		}
	case 25:
		itemID = "note_block"
	case 26:
		itemID = "red_bed"
	case 27:
		itemID = "golden_rail"
	case 28:
		itemID = "detector_rail"
	case 29:
		itemID = "sticky_piston"
	case 30:
		itemID = "cobweb"
	case 31:
		switch damage {
		case 1:
			itemID = "tall_grass"
		case 2:
			itemID = "fern"
		default:
			itemID = "dead_bush"
		}
	case 32:
		itemID = "dead_bush"
	case 33:
		itemID = "piston"
	case 34:
		itemID = "piston_head"
	case 35:
		colors := []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black"}
		if damage >= 0 && damage < 16 {
			itemID = "" + colors[damage] + "_wool"
		} else {
			itemID = "white_wool"
		}
	case 37:
		itemID = "dandelion"
	case 38:
		switch damage {
		case 1:
			itemID = "blue_orchid"
		case 2:
			itemID = "allium"
		case 3:
			itemID = "azure_bluet"
		case 4:
			itemID = "red_tulip"
		case 5:
			itemID = "orange_tulip"
		case 6:
			itemID = "white_tulip"
		case 7:
			itemID = "pink_tulip"
		case 8:
			itemID = "oxeye_daisy"
		case 9:
			itemID = "cornflower"
		case 10:
			itemID = "lily_of_the_valley"
		default:
			itemID = "poppy"
		}
	case 39:
		itemID = "brown_mushroom"
	case 40:
		itemID = "red_mushroom"
	case 41:
		itemID = "gold_block"
	case 42:
		itemID = "iron_block"
	case 43:
		names := []string{"stone", "sandstone", "oak", "cobblestone", "brick", "stone_brick", "nether_brick", "quartz"}
		if damage >= 0 && int(damage) < len(names) {
			itemID = "double_" + names[damage] + "_slab"
		} else {
			itemID = "double_stone_slab"
		}
	case 44:
		names := []string{"stone", "sandstone", "oak", "cobblestone", "brick", "stone_brick", "nether_brick", "quartz"}
		if damage >= 0 && int(damage) < len(names) {
			itemID = "" + names[damage] + "_slab"
		} else {
			itemID = "stone_slab"
		}
	case 45:
		itemID = "brick_block"
	case 46:
		itemID = "tnt"
	case 47:
		itemID = "bookshelf"
	case 48:
		itemID = "mossy_cobblestone"
	case 49:
		itemID = "obsidian"
	case 50:
		itemID = "torch"
	case 51:
		itemID = "fire"
	case 52:
		itemID = "mob_spawner"
	case 53:
		itemID = "oak_stairs"
	case 54:
		itemID = "chest"
	case 55:
		itemID = "redstone_wire"
	case 56:
		itemID = "diamond_ore"
	case 57:
		itemID = "diamond_block"
	case 58:
		itemID = "crafting_table"
	case 59:
		itemID = "wheat_block"
	case 60:
		itemID = "farmland"
	case 61:
		itemID = "furnace"
	case 62:
		itemID = "lit_furnace"
	case 63:
		itemID = "oak_sign"
	case 64:
		itemID = "oak_door"
	case 65:
		itemID = "ladder"
	case 66:
		itemID = "rail"
	case 67:
		itemID = "stone_stairs"
	case 68:
		itemID = "oak_wall_sign"
	case 69:
		itemID = "lever"
	case 70:
		itemID = "stone_pressure_plate"
	case 71:
		itemID = "iron_door_block"
	case 72:
		itemID = "oak_pressure_plate"
	case 73:
		itemID = "redstone_ore"
	case 74:
		itemID = "lit_redstone_ore"
	case 75:
		itemID = "unlit_redstone_torch"
	case 76:
		itemID = "redstone_torch"
	case 77:
		itemID = "stone_button"
	case 78:
		itemID = "snow_layer"
	case 79:
		itemID = "ice"
	case 80:
		itemID = "snow_block"
	case 81:
		itemID = "cactus"
	case 82:
		itemID = "clay"
	case 83:
		itemID = "reeds"
	case 84:
		itemID = "jukebox"
	case 85:
		itemID = "oak_fence"
	case 86:
		itemID = "pumpkin"
	case 87:
		itemID = "netherrack"
	case 88:
		itemID = "soul_sand"
	case 89:
		itemID = "glowstone"
	case 90:
		itemID = "portal"
	case 91:
		itemID = "jack_o_lantern"
	case 92:
		itemID = "cake_block"
	case 93:
		itemID = "unpowered_repeater"
	case 94:
		itemID = "powered_repeater"
	case 95:
		colors := []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black"}
		if damage >= 0 && damage < 16 {
			itemID = "" + colors[damage] + "_stained_glass"
		} else {
			itemID = "white_stained_glass"
		}
	case 96:
		itemID = "oak_trapdoor"
	case 97:
		switch damage {
		case 1:
			itemID = "infested_cobblestone"
		case 2:
			itemID = "infested_stone_bricks"
		case 3:
			itemID = "infested_mossy_stone_bricks"
		case 4:
			itemID = "infested_cracked_stone_bricks"
		case 5:
			itemID = "infested_chiseled_stone_bricks"
		default:
			itemID = "infested_stone"
		}
	case 98:
		switch damage {
		case 1:
			itemID = "mossy_stone_bricks"
		case 2:
			itemID = "cracked_stone_bricks"
		case 3:
			itemID = "chiseled_stone_bricks"
		default:
			itemID = "stone_bricks"
		}
	case 99:
		itemID = "brown_mushroom_block"
	case 100:
		itemID = "red_mushroom_block"
	case 101:
		itemID = "iron_bars"
	case 102:
		itemID = "glass_pane"
	case 103:
		itemID = "melon"
	case 104:
		itemID = "pumpkin_stem"
	case 105:
		itemID = "melon_stem"
	case 106:
		itemID = "vine"
	case 107:
		itemID = "oak_fence_gate"
	case 108:
		itemID = "brick_stairs"
	case 109:
		itemID = "stone_brick_stairs"
	case 110:
		itemID = "mycelium"
	case 111:
		itemID = "lily_pad"
	case 112:
		itemID = "nether_brick"
	case 113:
		itemID = "nether_brick_fence"
	case 114:
		itemID = "nether_brick_stairs"
	case 115:
		itemID = "nether_wart_block"
	case 116:
		itemID = "enchanting_table"
	case 117:
		itemID = "brewing_stand"
	case 118:
		itemID = "cauldron"
	case 119:
		itemID = "end_portal"
	case 120:
		itemID = "end_portal_frame"
	case 121:
		itemID = "end_stone"
	case 122:
		itemID = "dragon_egg"
	case 123:
		itemID = "redstone_lamp"
	case 124:
		itemID = "lit_redstone_lamp"
	case 125:
		names := []string{"oak", "spruce", "birch", "jungle", "acacia", "dark_oak"}
		if damage >= 0 && int(damage) < len(names) {
			itemID = "double_" + names[damage] + "_slab"
		} else {
			itemID = "double_oak_slab"
		}
	case 126:
		names := []string{"oak", "spruce", "birch", "jungle", "acacia", "dark_oak"}
		if damage >= 0 && int(damage) < len(names) {
			itemID = "" + names[damage] + "_slab"
		} else {
			itemID = "oak_slab"
		}
	case 127:
		itemID = "cocoa"
	case 128:
		itemID = "sandstone_stairs"
	case 129:
		itemID = "emerald_ore"
	case 130:
		itemID = "ender_chest"
	case 131:
		itemID = "tripwire_hook"
	case 132:
		itemID = "tripwire"
	case 133:
		itemID = "emerald_block"
	case 134:
		itemID = "spruce_stairs"
	case 135:
		itemID = "birch_stairs"
	case 136:
		itemID = "jungle_stairs"
	case 137:
		itemID = "command_block"
	case 138:
		itemID = "beacon"
	case 139:
		if damage == 1 {
			itemID = "mossy_cobblestone_wall"
		} else {
			itemID = "cobblestone_wall"
		}
	case 140:
		itemID = "flower_pot"
	case 141:
		itemID = "carrot"
	case 142:
		itemID = "potato"
	case 143:
		itemID = "wooden_button"
	case 144:
		itemID = "skeleton_skull"
	case 145:
		itemID = "anvil"
	case 146:
		itemID = "trapped_chest"
	case 147:
		itemID = "light_weighted_pressure_plate"
	case 148:
		itemID = "heavy_weighted_pressure_plate"
	case 149:
		itemID = "unpowered_comparator"
	case 150:
		itemID = "powered_comparator"
	case 151:
		itemID = "daylight_detector"
	case 152:
		itemID = "redstone_block"
	case 153:
		itemID = "quartz_ore"
	case 154:
		itemID = "hopper"
	case 155:
		switch damage {
		case 1:
			itemID = "chiseled_quartz_block"
		case 2:
			itemID = "quartz_pillar"
		default:
			itemID = "quartz_block"
		}
	case 156:
		itemID = "quartz_stairs"
	case 157:
		itemID = "activator_rail"
	case 158:
		itemID = "dropper"
	case 159:
		colors := []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black"}
		if damage >= 0 && damage < 16 {
			itemID = "" + colors[damage] + "_terracotta"
		} else {
			itemID = "white_terracotta"
		}
	case 160:
		colors := []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black"}
		if damage >= 0 && damage < 16 {
			itemID = "" + colors[damage] + "_stained_glass_pane"
		} else {
			itemID = "white_stained_glass_pane"
		}
	case 161:
		if damage == 1 {
			itemID = "dark_oak_leaves"
		} else {
			itemID = "acacia_leaves"
		}
	case 162:
		if damage == 1 {
			itemID = "dark_oak_log"
		} else {
			itemID = "acacia_log"
		}
	case 163:
		itemID = "acacia_stairs"
	case 164:
		itemID = "dark_oak_stairs"
	case 165:
		itemID = "slime_block"
	case 166:
		itemID = "barrier"
	case 167:
		itemID = "iron_trapdoor"
	case 168:
		switch damage {
		case 1:
			itemID = "prismarine_bricks"
		case 2:
			itemID = "dark_prismarine"
		default:
			itemID = "prismarine"
		}
	case 169:
		itemID = "sea_lantern"
	case 170:
		itemID = "hay_block"
	case 171:
		colors := []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black"}
		if damage >= 0 && damage < 16 {
			itemID = "" + colors[damage] + "_carpet"
		} else {
			itemID = "white_carpet"
		}
	case 172:
		itemID = "terracotta"
	case 173:
		itemID = "coal_block"
	case 174:
		itemID = "packed_ice"
	case 175:
		switch damage {
		case 1:
			itemID = "lilac"
		case 2:
			itemID = "tall_grass"
		case 3:
			itemID = "large_fern"
		case 4:
			itemID = "rose_bush"
		case 5:
			itemID = "peony"
		default:
			itemID = "sunflower"
		}
	case 176:
		itemID = "standing_banner"
	case 177:
		itemID = "wall_banner"
	case 178:
		itemID = "daylight_detector_inverted"
	case 179:
		switch damage {
		case 1:
			itemID = "chiseled_red_sandstone"
		case 2:
			itemID = "smooth_red_sandstone"
		default:
			itemID = "red_sandstone"
		}
	case 180:
		itemID = "red_sandstone_stairs"
	case 181:
		itemID = "double_stone_slab2"
	case 182:
		itemID = "stone_slab2"
	case 183:
		itemID = "spruce_fence_gate"
	case 184:
		itemID = "birch_fence_gate"
	case 185:
		itemID = "jungle_fence_gate"
	case 186:
		itemID = "dark_oak_fence_gate"
	case 187:
		itemID = "acacia_fence_gate"
	case 188:
		itemID = "spruce_fence"
	case 189:
		itemID = "birch_fence"
	case 190:
		itemID = "jungle_fence"
	case 191:
		itemID = "dark_oak_fence"
	case 192:
		itemID = "acacia_fence"
	case 193:
		itemID = "spruce_door_block"
	case 194:
		itemID = "birch_door_block"
	case 195:
		itemID = "jungle_door_block"
	case 196:
		itemID = "acacia_door_block"
	case 197:
		itemID = "dark_oak_door_block"
	case 198:
		itemID = "end_rod"
	case 199:
		itemID = "chorus_plant"
	case 200:
		itemID = "chorus_flower"
	case 201:
		itemID = "purpur_block"
	case 202:
		itemID = "purpur_pillar"
	case 203:
		itemID = "purpur_stairs"
	case 204:
		itemID = "purpur_double_slab"
	case 205:
		itemID = "purpur_slab"
	case 206:
		itemID = "end_bricks"
	case 207:
		itemID = "beetroots"
	case 208:
		itemID = "grass_path"
	case 209:
		itemID = "end_gateway"
	case 210:
		itemID = "repeating_command_block"
	case 211:
		itemID = "chain_command_block"
	case 212:
		itemID = "frosted_ice"
	case 213:
		itemID = "magma"
	case 214:
		itemID = "nether_wart_block"
	case 215:
		itemID = "red_nether_brick"
	case 216:
		itemID = "bone_block"
	case 217:
		itemID = "structure_void"
	case 218:
		itemID = "observer"
	case 219:
		itemID = "white_shulker_box"
	case 220:
		itemID = "orange_shulker_box"
	case 221:
		itemID = "magenta_shulker_box"
	case 222:
		itemID = "light_blue_shulker_box"
	case 223:
		itemID = "yellow_shulker_box"
	case 224:
		itemID = "lime_shulker_box"
	case 225:
		itemID = "pink_shulker_box"
	case 226:
		itemID = "gray_shulker_box"
	case 227:
		itemID = "light_gray_shulker_box"
	case 228:
		itemID = "cyan_shulker_box"
	case 229:
		itemID = "purple_shulker_box"
	case 230:
		itemID = "blue_shulker_box"
	case 231:
		itemID = "brown_shulker_box"
	case 232:
		itemID = "green_shulker_box"
	case 233:
		itemID = "red_shulker_box"
	case 234:
		itemID = "black_shulker_box"
	case 235:
		itemID = "white_glazed_terracotta"
	case 236:
		itemID = "orange_glazed_terracotta"
	case 237:
		itemID = "magenta_glazed_terracotta"
	case 238:
		itemID = "light_blue_glazed_terracotta"
	case 239:
		itemID = "yellow_glazed_terracotta"
	case 240:
		itemID = "lime_glazed_terracotta"
	case 241:
		itemID = "pink_glazed_terracotta"
	case 242:
		itemID = "gray_glazed_terracotta"
	case 243:
		itemID = "light_gray_glazed_terracotta"
	case 244:
		itemID = "cyan_glazed_terracotta"
	case 245:
		itemID = "purple_glazed_terracotta"
	case 246:
		itemID = "blue_glazed_terracotta"
	case 247:
		itemID = "brown_glazed_terracotta"
	case 248:
		itemID = "green_glazed_terracotta"
	case 249:
		itemID = "red_glazed_terracotta"
	case 250:
		itemID = "black_glazed_terracotta"
	case 251:
		colors := []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black"}
		if damage >= 0 && damage < 16 {
			itemID = "" + colors[damage] + "_concrete"
		} else {
			itemID = "white_concrete"
		}
	case 252:
		colors := []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black"}
		if damage >= 0 && damage < 16 {
			itemID = "" + colors[damage] + "_concrete_powder"
		} else {
			itemID = "white_concrete_powder"
		}
	case 253, 254:
		itemID = "air"
	case 255:
		itemID = "structure_block"
	case 256:
		itemID = "iron_shovel"
	case 257:
		itemID = "iron_pickaxe"
	case 258:
		itemID = "iron_axe"
	case 259:
		itemID = "flint_and_steel"
	case 260:
		itemID = "apple"
	case 261:
		itemID = "bow"
	case 262:
		itemID = "arrow"
	case 263:
		itemID = "coal"
	case 264:
		itemID = "diamond"
	case 265:
		itemID = "iron_ingot"
	case 266:
		itemID = "gold_ingot"
	case 267:
		itemID = "iron_sword"
	case 268:
		itemID = "wooden_sword"
	case 269:
		itemID = "wooden_shovel"
	case 270:
		itemID = "wooden_pickaxe"
	case 271:
		itemID = "wooden_axe"
	case 272:
		itemID = "stone_sword"
	case 273:
		itemID = "stone_shovel"
	case 274:
		itemID = "stone_pickaxe"
	case 275:
		itemID = "stone_axe"
	case 276:
		itemID = "diamond_sword"
	case 277:
		itemID = "diamond_shovel"
	case 278:
		itemID = "diamond_pickaxe"
	case 279:
		itemID = "diamond_axe"
	case 280:
		itemID = "stick"
	case 281:
		itemID = "bowl"
	case 282:
		itemID = "mushroom_stew"
	case 283:
		itemID = "golden_sword"
	case 284:
		itemID = "golden_shovel"
	case 285:
		itemID = "golden_pickaxe"
	case 286:
		itemID = "golden_axe"
	case 287:
		itemID = "string"
	case 288:
		itemID = "feather"
	case 289:
		itemID = "gunpowder"
	case 290:
		itemID = "wooden_hoe"
	case 291:
		itemID = "stone_hoe"
	case 292:
		itemID = "iron_hoe"
	case 293:
		itemID = "diamond_hoe"
	case 294:
		itemID = "golden_hoe"
	case 295:
		itemID = "wheat_seeds"
	case 296:
		itemID = "wheat"
	case 297:
		itemID = "bread"
	case 298:
		itemID = "leather_helmet"
	case 299:
		itemID = "leather_chestplate"
	case 300:
		itemID = "leather_leggings"
	case 301:
		itemID = "leather_boots"
	case 302:
		itemID = "chainmail_helmet"
	case 303:
		itemID = "chainmail_chestplate"
	case 304:
		itemID = "chainmail_leggings"
	case 305:
		itemID = "chainmail_boots"
	case 306:
		itemID = "iron_helmet"
	case 307:
		itemID = "iron_chestplate"
	case 308:
		itemID = "iron_leggings"
	case 309:
		itemID = "iron_boots"
	case 310:
		itemID = "diamond_helmet"
	case 311:
		itemID = "diamond_chestplate"
	case 312:
		itemID = "diamond_leggings"
	case 313:
		itemID = "diamond_boots"
	case 314:
		itemID = "golden_helmet"
	case 315:
		itemID = "golden_chestplate"
	case 316:
		itemID = "golden_leggings"
	case 317:
		itemID = "golden_boots"
	case 318:
		itemID = "flint"
	case 319:
		itemID = "porkchop"
	case 320:
		itemID = "cooked_porkchop"
	case 321:
		itemID = "painting"
	case 322:
		if damage == 1 {
			itemID = "enchanted_golden_apple"
		} else {
			itemID = "golden_apple"
		}
	case 323:
		itemID = "oak_sign"
	case 324:
		itemID = "oak_door"
	case 325:
		itemID = "bucket"
	case 326:
		itemID = "water_bucket"
	case 327:
		itemID = "lava_bucket"
	case 328:
		itemID = "minecart"
	case 329:
		itemID = "saddle"
	case 330:
		itemID = "iron_door"
	case 331:
		itemID = "redstone"
	case 332:
		itemID = "snowball"
	case 333:
		itemID = "oak_boat"
	case 334:
		itemID = "leather"
	case 335:
		itemID = "milk_bucket"
	case 336:
		itemID = "brick"
	case 337:
		itemID = "clay_ball"
	case 338:
		itemID = "sugar_cane"
	case 339:
		itemID = "paper"
	case 340:
		itemID = "book"
	case 341:
		itemID = "slime_ball"
	case 342:
		itemID = "chest_minecart"
	case 343:
		itemID = "furnace_minecart"
	case 344:
		itemID = "egg"
	case 345:
		itemID = "compass"
	case 346:
		itemID = "fishing_rod"
	case 347:
		itemID = "clock"
	case 348:
		itemID = "glowstone_dust"
	case 349:
		switch damage {
		case 1:
			itemID = "salmon"
		case 2:
			itemID = "tropical_fish"
		case 3:
			itemID = "pufferfish"
		default:
			itemID = "cod"
		}
	case 350:
		if damage == 1 {
			itemID = "cooked_salmon"
		} else {
			itemID = "cooked_cod"
		}
	case 351:
		switch damage {
		case 1:
			itemID = "red_dye"
		case 2:
			itemID = "green_dye"
		case 3:
			itemID = "cocoa_beans"
		case 4:
			itemID = "lapis_lazuli"
		case 5:
			itemID = "purple_dye"
		case 6:
			itemID = "cyan_dye"
		case 7:
			itemID = "light_gray_dye"
		case 8:
			itemID = "gray_dye"
		case 9:
			itemID = "pink_dye"
		case 10:
			itemID = "lime_dye"
		case 11:
			itemID = "yellow_dye"
		case 12:
			itemID = "light_blue_dye"
		case 13:
			itemID = "magenta_dye"
		case 14:
			itemID = "orange_dye"
		case 15:
			itemID = "bone_meal"
		default:
			itemID = "ink_sac"
		}
	case 352:
		itemID = "bone"
	case 353:
		itemID = "sugar"
	case 354:
		itemID = "cake"
	case 355:
		itemID = "bed"
	case 356:
		itemID = "repeater"
	case 357:
		itemID = "cookie"
	case 358:
		itemID = "filled_map"
	case 359:
		itemID = "shears"
	case 360:
		itemID = "melon_slice"
	case 361:
		itemID = "pumpkin_seeds"
	case 362:
		itemID = "melon_seeds"
	case 363:
		itemID = "beef"
	case 364:
		itemID = "cooked_beef"
	case 365:
		itemID = "chicken"
	case 366:
		itemID = "cooked_chicken"
	case 367:
		itemID = "rotten_flesh"
	case 368:
		itemID = "ender_pearl"
	case 369:
		itemID = "blaze_rod"
	case 370:
		itemID = "ghast_tear"
	case 371:
		itemID = "gold_nugget"
	case 372:
		itemID = "nether_wart"
	case 373:
		itemID = "potion"
	case 374:
		itemID = "glass_bottle"
	case 375:
		itemID = "spider_eye"
	case 376:
		itemID = "fermented_spider_eye"
	case 377:
		itemID = "blaze_powder"
	case 378:
		itemID = "magma_cream"
	case 379:
		itemID = "brewingstand"
	case 380:
		itemID = "cauldron"
	case 381:
		itemID = "ender_eye"
	case 382:
		itemID = "glistering_melon_slice"
	case 383:
		switch damage {
		case 4:
			itemID = "elder_guardian_spawn_egg"
		case 5:
			itemID = "wither_skeleton_spawn_egg"
		case 6:
			itemID = "stray_spawn_egg"
		case 23:
			itemID = "husk_spawn_egg"
		case 27:
			itemID = "zombie_villager_spawn_egg"
		case 28:
			itemID = "skeleton_horse_spawn_egg"
		case 29:
			itemID = "zombie_horse_spawn_egg"
		case 31:
			itemID = "donkey_spawn_egg"
		case 32:
			itemID = "mule_spawn_egg"
		case 34:
			itemID = "evoker_spawn_egg"
		case 35:
			itemID = "vex_spawn_egg"
		case 36:
			itemID = "vindicator_spawn_egg"
		case 50:
			itemID = "creeper_spawn_egg"
		case 51:
			itemID = "skeleton_spawn_egg"
		case 52:
			itemID = "spider_spawn_egg"
		case 54:
			itemID = "zombie_spawn_egg"
		case 55:
			itemID = "slime_spawn_egg"
		case 56:
			itemID = "ghast_spawn_egg"
		case 57:
			itemID = "zombie_pigman_spawn_egg"
		case 58:
			itemID = "enderman_spawn_egg"
		case 59:
			itemID = "cave_spider_spawn_egg"
		case 60:
			itemID = "silverfish_spawn_egg"
		case 61:
			itemID = "blaze_spawn_egg"
		case 62:
			itemID = "magma_cube_spawn_egg"
		case 65:
			itemID = "bat_spawn_egg"
		case 66:
			itemID = "witch_spawn_egg"
		case 67:
			itemID = "endermite_spawn_egg"
		case 68:
			itemID = "guardian_spawn_egg"
		case 69:
			itemID = "shulker_spawn_egg"
		case 90:
			itemID = "pig_spawn_egg"
		case 91:
			itemID = "sheep_spawn_egg"
		case 92:
			itemID = "cow_spawn_egg"
		case 93:
			itemID = "chicken_spawn_egg"
		case 94:
			itemID = "squid_spawn_egg"
		case 95:
			itemID = "wolf_spawn_egg"
		case 96:
			itemID = "mooshroom_spawn_egg"
		case 98:
			itemID = "ocelot_spawn_egg"
		case 100:
			itemID = "horse_spawn_egg"
		case 101:
			itemID = "rabbit_spawn_egg"
		case 102:
			itemID = "polar_bear_spawn_egg"
		case 103:
			itemID = "llama_spawn_egg"
		case 105:
			itemID = "parrot_spawn_egg"
		case 120:
			itemID = "villager_spawn_egg"
		default:
			itemID = "zombie_spawn_egg"
		}
	case 384:
		itemID = "experience_bottle"
	case 385:
		itemID = "fire_charge"
	case 386:
		itemID = "writable_book"
	case 387:
		itemID = "written_book"
	case 388:
		itemID = "emerald"
	case 389:
		itemID = "item_frame"
	case 390:
		itemID = "flower_pot"
	case 391:
		itemID = "carrot"
	case 392:
		itemID = "potato"
	case 393:
		itemID = "baked_potato"
	case 394:
		itemID = "poisonous_potato"
	case 395:
		itemID = "map"
	case 396:
		itemID = "golden_carrot"
	case 397:
		switch damage {
		case 0:
			itemID = "skeleton_skull"
		case 1:
			itemID = "wither_skeleton_skull"
		case 2:
			itemID = "zombie_head"
		case 4:
			itemID = "creeper_head"
		case 5:
			itemID = "dragon_head"
		default:
			itemID = "player_head"
		}
	case 398:
		itemID = "carrot_on_a_stick"
	case 399:
		itemID = "nether_star"
	case 400:
		itemID = "pumpkin_pie"
	case 401:
		itemID = "firework_rocket"
	case 402:
		itemID = "firework_star"
	case 403:
		itemID = "enchanted_book"
	case 404:
		itemID = "comparator"
	case 405:
		itemID = "nether_brick"
	case 406:
		itemID = "quartz"
	case 407:
		itemID = "tnt_minecart"
	case 408:
		itemID = "hopper_minecart"
	case 409:
		itemID = "prismarine_shard"
	case 410:
		itemID = "prismarine_crystals"
	case 411:
		itemID = "rabbit"
	case 412:
		itemID = "cooked_rabbit"
	case 413:
		itemID = "rabbit_stew"
	case 414:
		itemID = "rabbit_foot"
	case 415:
		itemID = "rabbit_hide"
	case 416:
		itemID = "armor_stand"
	case 417:
		itemID = "iron_horse_armor"
	case 418:
		itemID = "golden_horse_armor"
	case 419:
		itemID = "diamond_horse_armor"
	case 420:
		itemID = "lead"
	case 421:
		itemID = "name_tag"
	case 422:
		itemID = "command_block_minecart"
	case 423:
		itemID = "mutton"
	case 424:
		itemID = "cooked_mutton"
	case 425:
		itemID = "white_banner"
	case 426:
		itemID = "end_crystal"
	case 427:
		itemID = "spruce_door"
	case 428:
		itemID = "birch_door"
	case 429:
		itemID = "jungle_door"
	case 430:
		itemID = "acacia_door"
	case 431:
		itemID = "dark_oak_door"
	case 432:
		itemID = "chorus_fruit"
	case 433:
		itemID = "chorus_fruit_popped"
	case 434:
		itemID = "beetroot"
	case 435:
		itemID = "beetroot_seeds"
	case 436:
		itemID = "beetroot_soup"
	case 437:
		itemID = "dragon_breath"
	case 438:
		itemID = "splash_potion"
	case 439:
		itemID = "spectral_arrow"
	case 440:
		itemID = "tipped_arrow"
	case 441:
		itemID = "lingering_potion"
	case 442:
		itemID = "shield"
	case 443:
		itemID = "elytra"
	case 444:
		itemID = "spruce_boat"
	case 445:
		itemID = "birch_boat"
	case 446:
		itemID = "jungle_boat"
	case 447:
		itemID = "acacia_boat"
	case 448:
		itemID = "dark_oak_boat"
	case 2256:
		itemID = "music_disc_13"
	case 2257:
		itemID = "music_disc_cat"
	case 2258:
		itemID = "music_disc_blocks"
	case 2259:
		itemID = "music_disc_chirp"
	case 2260:
		itemID = "music_disc_far"
	case 2261:
		itemID = "music_disc_mall"
	case 2262:
		itemID = "music_disc_mellohi"
	case 2263:
		itemID = "music_disc_stal"
	case 2264:
		itemID = "music_disc_strad"
	case 2265:
		itemID = "music_disc_ward"
	case 2266:
		itemID = "music_disc_11"
	case 2267:
		itemID = "music_disc_wait"
	default:
		return "", false
	}

	return itemID, itemID != ""
}

var MINECRAFT_BLOCK_MAPPINGS = map[string]string{}

func init() {
	for originalItemId, items := range MinecraftBlockMapping {
		for itemIndex, itemId := range items {
			MINECRAFT_BLOCK_MAPPINGS[fmt.Sprintf("%s:%d", originalItemId, itemIndex)] = itemId
		}
	}
}

type ItemModel struct {
	ItemId     string
	NumericId  int
	ItemDamage int
}

func GetVanillaItemId(item ItemModel) string {
	itemId, exists := getItemIdFromNumericId(item.NumericId, item.ItemDamage)
	if exists && itemId != "" {
		return itemId
	}

	if item.ItemId != "" {
		if _, exists := MINECRAFT_BLOCK_MAPPINGS[fmt.Sprintf("%s:%d", item.ItemId, item.ItemDamage)]; exists {
			return MINECRAFT_BLOCK_MAPPINGS[fmt.Sprintf("%s:%d", item.ItemId, item.ItemDamage)]
		}
	}

	return item.ItemId
}
