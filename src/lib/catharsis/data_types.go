package catharsis

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strconv"
	"strings"
)

type DataTypeResolver func(item models.TextureItem) (any, bool)

var dataTypeResolvers = map[string]DataTypeResolver{
	"skyblock_id":               resolveSkyblockID,
	"id":                        resolveID,
	"api_id":                    resolveAPIID,
	"uuid":                      resolveUUID,
	"timestamp":                 resolveTimestamp,
	"modifier":                  resolveModifier,
	"recombobulator":            resolveRecombobulator,
	"enchantments":              resolveEnchantments,
	"attributes":                resolveAttributes,
	"hot_potato_books":          resolveHotPotatoBooks,
	"art_of_war":                resolveArtOfWar,
	"art_of_peace":              resolveArtOfPeace,
	"boosters":                  resolveBoosters,
	"jalapeno_book":             resolveJalapenoBook,
	"midas_weapon_bid":          resolveMidasWeaponBid,
	"midas_weapon_added_coins":  resolveMidasWeaponAddedCoins,
	"enrichment":                resolveEnrichment,
	"quiver_arrow":              resolveQuiverArrow,
	"personal_compactor_items":  resolvePersonalCompactorItems,
	"personal_deletor_items":    resolvePersonalDeletorItems,
	"potion":                    resolvePotion,
	"potion_level":              resolvePotionLevel,
	"book_of_stats":             resolveBookOfStats,
	"applied_rune":              resolveAppliedRune,
	"used_rune":                 resolveUsedRune,
	"applied_dye":               resolveAppliedDye,
	"helmet_skin":               resolveHelmetSkin,
	"pet_data":                  resolvePetData,
	"absorb_logs":               resolveAbsorbLogs,
	"logs_cut":                  resolveLogsCut,
	"gilded_gifted_coins":       resolveGildedGiftedCoins,
	"abicase_model":             resolveAbicaseModel,
	"party_hat_color":           resolvePartyHatColor,
	"party_hat_year":            resolvePartyHatYear,
	"seconds_held":              resolveSecondsHeld,
	"bottle_of_jyrre_seconds":   resolveBottleOfJyrreSeconds,
	"rift_discrite_seconds":     resolveRiftDiscriteSeconds,
	"dungeon_item":              resolveDungeonItem,
	"star_count":                resolveStarCount,
	"necron_scrolls":            resolveNecronScrolls,
	"dungeon_tier":              resolveDungeonTier,
	"dungeon_quality":           resolveDungeonQuality,
	"wet_book":                  resolveWetBook,
	"hook":                      resolveHook,
	"line":                      resolveLine,
	"sinker":                    resolveSinker,
	"fungi_cutter_mode":         resolveFungiCutterMode,
	"crops_broken":              resolveCropsBroken,
	"cultivating_crops":         resolveCultivatingCrops,
	"tool_level":                resolveToolLevel,
	"tool_exp":                  resolveToolExp,
	"tool_overclocks":           resolveToolOverclocks,
	"pickonimbus_durability":    resolvePickonimbusDurability,
	"compact_blocks":            resolveCompactBlocks,
	"divan_powder_coating":      resolveDivanPowderCoating,
	"polarvoid":                 resolvePolarvoid,
	"power_ability_scroll":      resolvePowerAbilityScroll,
	"fuel_tank":                 resolveFuelTank,
	"engine":                    resolveEngine,
	"upgrade_module":            resolveUpgradeModule,
	"rarity":                    resolveRarity,
	"category":                  resolveCategory,
	"midas_weapon_paid":         resolveMidasWeaponPaid,
	"fuel":                      resolveFuel,
	"personal_accessory_active": resolvePersonalAccessoryActive,
	"has_dye_fallback":          resolveHasDyeFallback,
	"has_skin_fallback":         resolveHasSkinFallback,
}

// RegisterDataType allows packs/extensions to add custom catharsis data types at runtime.
func RegisterDataType(name string, resolver DataTypeResolver) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" || resolver == nil {
		return
	}

	dataTypeResolvers[key] = resolver
}

// ResolveDataType resolves a catharsis data_type value from item data.
func ResolveDataType(item models.TextureItem, dataType string) (any, bool) {
	resolver, ok := dataTypeResolvers[strings.ToLower(strings.TrimSpace(dataType))]
	if !ok {
		return nil, false
	}

	return resolver(item)
}

// IsDataTypePresent mirrors catharsis:is_data_type_present semantics.
func IsDataTypePresent(item models.TextureItem, dataType string) bool {
	_, ok := ResolveDataType(item, dataType)
	return ok
}

// MatchDataType checks whether a resolved data_type equals a target "when" value.
func MatchDataType(item models.TextureItem, dataType string, when any) bool {
	value, ok := ResolveDataType(item, dataType)
	if !ok {
		return false
	}

	return valuesEqual(value, when)
}

// MatchDataTypeThreshold checks whether a numeric data type is >= threshold.
func MatchDataTypeThreshold(item models.TextureItem, dataType string, threshold float64) bool {
	value, ok := ResolveDataType(item, dataType)
	if !ok {
		return false
	}

	number, ok := toFloat64(value)
	if !ok {
		return false
	}

	return number >= threshold
}

func resolveModifier(item models.TextureItem) (any, bool) {
	return getExtraString(item, "modifier")
}

func resolveSkyblockID(item models.TextureItem) (any, bool) {
	return getExtraString(item, "id")
}

func resolveID(item models.TextureItem) (any, bool) {
	return resolveSkyblockID(item)
}

func resolveAPIID(item models.TextureItem) (any, bool) {
	id, ok := getExtraString(item, "id")
	if !ok {
		return nil, false
	}

	switch strings.ToUpper(id) {
	case "RUNE", "UNIQUE_RUNE":
		if runeValue, ok := resolveAppliedRune(item); ok {
			return "rune:" + fmt.Sprintf("%v", runeValue), true
		}
	case "PET":
		if petData, ok := resolvePetData(item); ok {
			if petMap, isMap := petData.(map[string]any); isMap {
				petId := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", petMap["id"])))
				rarity := strings.TrimSpace(strings.ToUpper(fmt.Sprintf("%v", petMap["rarity"])))
				if petId != "" && rarity != "" {
					return "pet:" + petId + ":" + rarity, true
				}
			}
		}
	}

	return id, true
}

func resolveUUID(item models.TextureItem) (any, bool) {
	return getExtraString(item, "uuid")
}

func resolveTimestamp(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "timestamp")
}

func resolveCategory(item models.TextureItem) (any, bool) {
	id, ok := getExtraString(item, "id")
	if !ok {
		return nil, false
	}

	itemData, exists := constants.ITEMS[strings.ToUpper(id)]
	if !exists || strings.TrimSpace(itemData.Category) == "" {
		return nil, false
	}

	return strings.ToLower(itemData.Category), true
}

func resolveRecombobulator(item models.TextureItem) (any, bool) {
	number, ok := firstExtraNumber(item, "rarity_upgrades")
	if !ok {
		return nil, false
	}

	return number > 0, true
}

func resolveEnchantments(item models.TextureItem) (any, bool) {
	return resolveStringIntMap(item, "enchantments")
}

func resolveAttributes(item models.TextureItem) (any, bool) {
	return resolveStringIntMap(item, "attributes")
}

func resolveHotPotatoBooks(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "hot_potato_count")
}

func resolveArtOfWar(item models.TextureItem) (any, bool) {
	number, ok := firstExtraNumber(item, "art_of_war_count", "art_of_war")
	if !ok {
		return nil, false
	}

	return number > 0, true
}

func resolveArtOfPeace(item models.TextureItem) (any, bool) {
	if b, ok := firstExtraBool(item, "art_of_peace", "artOfPeaceApplied"); ok {
		return b, true
	}

	number, ok := firstExtraNumber(item, "artOfPeaceApplied")
	if !ok {
		return nil, false
	}

	return number > 0, true
}

func resolveBoosters(item models.TextureItem) (any, bool) {
	value, ok := firstExtra(item, "boosters")
	if !ok {
		return nil, false
	}

	entries, ok := toStringSlice(value)
	if !ok {
		return nil, false
	}

	boosters := make([]string, 0, len(entries))
	for _, booster := range entries {
		trimmed := strings.TrimSpace(strings.ToUpper(booster))
		if trimmed == "" {
			continue
		}

		boosters = append(boosters, trimmed+"_BOOSTER")
	}

	return boosters, true
}

func resolveJalapenoBook(item models.TextureItem) (any, bool) {
	number, ok := firstExtraNumber(item, "jalapeno_count", "jalapeno_book")
	if !ok {
		return nil, false
	}

	return number > 0, true
}

func resolveMidasWeaponBid(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "winning_bid")
}

func resolveMidasWeaponAddedCoins(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "additional_coins")
}

func resolveEnrichment(item models.TextureItem) (any, bool) {
	id, ok := getExtraString(item, "talisman_enrichment")
	if !ok {
		return nil, false
	}

	return "talisman_enrichment_" + id, true
}

func resolveQuiverArrow(item models.TextureItem) (any, bool) {
	return firstExtraBool(item, "quiver_arrow")
}

func resolvePersonalCompactorItems(item models.TextureItem) (any, bool) {
	return firstExtra(item, "personal_compactor_items", "personal_compactor")
}

func resolvePersonalDeletorItems(item models.TextureItem) (any, bool) {
	return firstExtra(item, "personal_deletor_items", "personal_deletor")
}

func resolvePotion(item models.TextureItem) (any, bool) {
	return getFirstExtraString(item, "potion")
}

func resolvePotionLevel(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "potion_level")
}

func resolveBookOfStats(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "book_of_stats", "stats_book")
}

func resolveAppliedRune(item models.TextureItem) (any, bool) {
	runes, ok := getStringIntMap(item, "runes")
	if !ok || len(runes) == 0 {
		return nil, false
	}

	for name, level := range runes {
		return strings.ToLower(name) + ":" + strconv.Itoa(level), true
	}

	return nil, false
}

func resolveUsedRune(item models.TextureItem) (any, bool) {
	runeValue, ok := resolveAppliedRune(item)
	if !ok {
		return nil, false
	}

	return "rune:" + fmt.Sprintf("%v", runeValue), true
}

func resolveAppliedDye(item models.TextureItem) (any, bool) {
	return getExtraString(item, "dye_item")
}

func resolveHelmetSkin(item models.TextureItem) (any, bool) {
	return getExtraString(item, "skin")
}

func resolvePetData(item models.TextureItem) (any, bool) {
	petInfo, ok := getFirstExtraString(item, "petInfo")
	if !ok {
		return nil, false
	}

	petMap := map[string]any{}
	if err := jsonUnmarshal([]byte(petInfo), &petMap); err != nil {
		return nil, false
	}

	if tier, ok := petMap["tier"]; ok {
		petMap["rarity"] = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", tier)))
	}

	if t, ok := petMap["type"]; ok {
		petMap["id"] = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", t)))
	}

	return petMap, true
}

func resolveAbsorbLogs(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "absorb_logs_chopped")
}

func resolveLogsCut(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "logs_cut")
}

func resolveGildedGiftedCoins(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "gilded_gifted_coins")
}

func resolveAbicaseModel(item models.TextureItem) (any, bool) {
	return getFirstExtraString(item, "abicase_model", "model")
}

func resolvePartyHatColor(item models.TextureItem) (any, bool) {
	return getFirstExtraString(item, "party_hat_color")
}

func resolvePartyHatYear(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "party_hat_year")
}

func resolveSecondsHeld(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "seconds_held")
}

func resolveBottleOfJyrreSeconds(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "bottle_of_jyrre_seconds")
}

func resolveRiftDiscriteSeconds(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "rift_discrite_seconds")
}

func resolveDungeonItem(item models.TextureItem) (any, bool) {
	if b, ok := firstExtraBool(item, "dungeon_item"); ok {
		return b, true
	}

	number, ok := firstExtraNumber(item, "dungeon_item")
	if !ok {
		return nil, false
	}

	return number > 0, true
}

func resolveStarCount(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "upgrade_level", "dungeon_item_level")
}

func resolveNecronScrolls(item models.TextureItem) (any, bool) {
	value, ok := firstExtra(item, "ability_scroll")
	if !ok {
		return nil, false
	}

	entries, ok := toStringSlice(value)
	if !ok {
		return nil, false
	}

	for _, entry := range entries {
		if strings.EqualFold(entry, "ULTIMATE_WITHER_SCROLL") {
			return []string{"WITHER_SHIELD_SCROLL", "SHADOW_WARP_SCROLL", "IMPLOSION_SCROLL"}, true
		}
	}

	return entries, true
}

func resolveDungeonTier(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "dungeon_tier", "item_tier")
}

func resolveDungeonQuality(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "dungeon_quality", "baseStatBoostPercentage")
}

func resolveWetBook(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "wet_book", "wet_book_count")
}

func resolveHook(item models.TextureItem) (any, bool) {
	return resolveFishingPart(item, "hook")
}

func resolveLine(item models.TextureItem) (any, bool) {
	return resolveFishingPart(item, "line")
}

func resolveSinker(item models.TextureItem) (any, bool) {
	return resolveFishingPart(item, "sinker")
}

func resolveFungiCutterMode(item models.TextureItem) (any, bool) {
	return getExtraString(item, "fungi_cutter_mode")
}

func resolveCropsBroken(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "mined_crops")
}

func resolveCultivatingCrops(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "cultivating_crops", "farmed_cultivating")
}

func resolveToolLevel(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "tool_level", "levelable_lvl")
}

func resolveToolExp(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "tool_exp", "levelable_exp")
}

func resolveToolOverclocks(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "tool_overclocks", "levelable_overclocks")
}

func resolvePickonimbusDurability(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "pickonimbus_durability")
}

func resolveCompactBlocks(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "compact_blocks")
}

func resolveDivanPowderCoating(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "divan_powder_coating")
}

func resolvePolarvoid(item models.TextureItem) (any, bool) {
	return firstExtraNumber(item, "polarvoid")
}

func resolvePowerAbilityScroll(item models.TextureItem) (any, bool) {
	return getFirstExtraString(item, "power_ability_scroll")
}

func resolveFuelTank(item models.TextureItem) (any, bool) {
	return getFirstExtraString(item, "fuel_tank")
}

func resolveEngine(item models.TextureItem) (any, bool) {
	return getFirstExtraString(item, "engine")
}

func resolveUpgradeModule(item models.TextureItem) (any, bool) {
	return getFirstExtraString(item, "upgrade_module")
}

func resolveRarity(item models.TextureItem) (any, bool) {
	id, ok := getExtraString(item, "id")
	if !ok {
		return nil, false
	}

	itemData, exists := constants.ITEMS[strings.ToUpper(id)]
	if !exists || strings.TrimSpace(itemData.Rarity) == "" {
		return nil, false
	}

	return strings.ToLower(itemData.Rarity), true
}

func resolveMidasWeaponPaid(item models.TextureItem) (any, bool) {
	var total float64
	var hasAny bool

	for _, key := range []string{"winning_bid", "additional_coins"} {
		value, ok := getExtra(item, key)
		if !ok {
			continue
		}

		number, ok := toFloat64(value)
		if !ok {
			continue
		}

		total += number
		hasAny = true
	}

	if !hasAny {
		return nil, false
	}

	return total, true
}

func resolveFuel(item models.TextureItem) (any, bool) {
	for _, key := range []string{"drill_fuel", "fuel"} {
		value, ok := getExtra(item, key)
		if !ok {
			continue
		}

		number, ok := toFloat64(value)
		if ok {
			return number, true
		}
	}

	return nil, false
}

func resolvePersonalAccessoryActive(item models.TextureItem) (any, bool) {
	for _, key := range []string{"personal_accessory_active", "is_active", "active"} {
		value, ok := getExtra(item, key)
		if !ok {
			continue
		}

		if b, ok := toBool(value); ok {
			return b, true
		}
	}

	for _, key := range []string{"personal_compactor_items", "personal_deletor_items", "personal_compactor", "personal_deletor"} {
		value, ok := getExtra(item, key)
		if !ok {
			continue
		}

		if hasEntries(value) {
			return true, true
		}
	}

	return nil, false
}

func resolveHasDyeFallback(item models.TextureItem) (any, bool) {
	_, hasDye := getExtraString(item, "dye_item")
	return hasDye, true
}

func resolveHasSkinFallback(item models.TextureItem) (any, bool) {
	_, hasSkin := getExtraString(item, "skin")
	return hasSkin, true
}

func getExtra(item models.TextureItem, key string) (any, bool) {
	if item.Tag.ExtraAttributes == nil {
		return nil, false
	}

	value, ok := item.Tag.ExtraAttributes[key]
	if !ok || value == nil {
		return nil, false
	}

	return value, true
}

func firstExtra(item models.TextureItem, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := getExtra(item, key); ok {
			return value, true
		}
	}

	return nil, false
}

func getFirstExtraString(item models.TextureItem, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := getExtraString(item, key); ok {
			return value, true
		}
	}

	return "", false
}

func firstExtraNumber(item models.TextureItem, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := getExtra(item, key)
		if !ok {
			continue
		}

		if number, ok := toFloat64(value); ok {
			return number, true
		}
	}

	return 0, false
}

func firstExtraBool(item models.TextureItem, keys ...string) (bool, bool) {
	for _, key := range keys {
		value, ok := getExtra(item, key)
		if !ok {
			continue
		}

		if b, ok := toBool(value); ok {
			return b, true
		}
	}

	return false, false
}

func resolveStringIntMap(item models.TextureItem, key string) (any, bool) {
	return getStringIntMap(item, key)
}

func getStringIntMap(item models.TextureItem, key string) (map[string]int, bool) {
	value, ok := getExtra(item, key)
	if !ok {
		return nil, false
	}

	switch data := value.(type) {
	case map[string]int:
		if len(data) == 0 {
			return nil, false
		}
		return data, true
	case map[string]any:
		out := make(map[string]int, len(data))
		for k, raw := range data {
			number, ok := toFloat64(raw)
			if !ok {
				continue
			}
			out[k] = int(number)
		}

		if len(out) == 0 {
			return nil, false
		}

		return out, true
	}

	return nil, false
}

func toStringSlice(value any) ([]string, bool) {
	switch entries := value.(type) {
	case []string:
		return entries, true
	case []any:
		out := make([]string, 0, len(entries))
		for _, entry := range entries {
			str := strings.TrimSpace(fmt.Sprintf("%v", entry))
			if str != "" {
				out = append(out, str)
			}
		}
		return out, len(out) > 0
	default:
		return nil, false
	}
}

func resolveFishingPart(item models.TextureItem, key string) (any, bool) {
	value, ok := getExtra(item, key)
	if !ok {
		return nil, false
	}

	partMap, ok := value.(map[string]any)
	if !ok {
		if fallback, ok := value.(map[string]interface{}); ok {
			partMap = map[string]any(fallback)
		} else {
			return nil, false
		}
	}

	uuidValue := strings.TrimSpace(fmt.Sprintf("%v", partMap["uuid"]))
	partName := strings.TrimSpace(fmt.Sprintf("%v", partMap["part"]))
	if uuidValue == "" && partName == "" {
		return nil, false
	}

	return strings.ToLower(uuidValue + ":" + partName), true
}

var jsonUnmarshal = func(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func getExtraString(item models.TextureItem, key string) (string, bool) {
	value, ok := getExtra(item, key)
	if !ok {
		return "", false
	}

	str := strings.TrimSpace(fmt.Sprintf("%v", value))
	if str == "" {
		return "", false
	}

	return strings.ToLower(str), true
}

func valuesEqual(a any, b any) bool {
	if av, aok := toFloat64(a); aok {
		if bv, bok := toFloat64(b); bok {
			return math.Abs(av-bv) < 0.0000001
		}
	}

	if ab, aok := toBool(a); aok {
		if bb, bok := toBool(b); bok {
			return ab == bb
		}
	}

	return strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", a)), strings.TrimSpace(fmt.Sprintf("%v", b)))
}

func toFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return parsed, true
		}
	}

	return 0, false
}

func toBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		trimmed := strings.TrimSpace(strings.ToLower(v))
		if trimmed == "true" {
			return true, true
		}
		if trimmed == "false" {
			return false, true
		}
	case int, int8, int16, int32, int64:
		n, _ := toFloat64(v)
		return n != 0, true
	case uint, uint8, uint16, uint32, uint64:
		n, _ := toFloat64(v)
		return n != 0, true
	case float32, float64:
		n, _ := toFloat64(v)
		return n != 0, true
	}

	return false, false
}

func hasEntries(value any) bool {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return false
	}

	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return v.Len() > 0
	}

	return utility.RemoveNonAscii(strings.TrimSpace(fmt.Sprintf("%v", value))) != ""
}
