package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	mr "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer"
	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

var ctx = context.Background()

var SkyCryptRender *mr.Renderer

var ITEM_TEXTURE_CACHE = make(map[string]AppliedItemTexture)
var itemTextureCacheMu sync.RWMutex
var renderedSkyBlockIndex = make(map[string]struct{})
var renderedSkyBlockIndexMu sync.RWMutex
var customResourcesOnce sync.Once
var customResourcesErr error
var resourcePackConfigsOnce sync.Once
var resourcePackConfigs []models.ResourcePackConfig
var resourcePackConfigsErr error

var vanillaTextureCache sync.Map
var vanillaAssetTextureCache sync.Map
var vanillaModelTextureCache sync.Map
var vanillaItemExistsCache sync.Map

var fallbackResourcePackIDs = []string{"FSR", "HYPIXEL_PLUS"}

type AppliedItemTexture struct {
	Texture     string
	TexturePack string
}

type ItemTextureInput struct {
	ID           string
	ItemModel    string
	SkyBlockID   string
	NumericID    int
	Damage       int
	Texture      string
	DisplayColor int
	SkullOwner   *skycrypttypes.SkullOwner
	Tag          any
}

type TextureApplyContext struct {
	DisabledPacks        map[string]struct{}
	EnabledPackIDs       []string
	PackSignature        string
	Domain               string
	DisableRuntimeRender bool
	DisableStats         bool
	Stats                *TextureApplyStats
}

type TextureApplyStats struct {
	Total                            int
	CacheHits                        int
	CacheMisses                      int
	StableSkyBlockHits               int
	StableKeyHits                    int
	LegacySkyBlockHits               int
	RawMapHits                       int
	RenderAttempts                   int
	RenderHits                       int
	RenderErrors                     int
	RenderDuration                   time.Duration
	RuntimeRenderSkipped             int
	RuntimeRenderSkippedDisabled     int
	RuntimeRenderSkippedRendererNil  int
	RuntimeRenderSkippedNoPacks      int
	RuntimeRenderSkippedGenericSkull int
	SkullFallbacks                   int
	HeadFallbacks                    int
	LeatherFallbacks                 int
	VanillaFallbacks                 int
	VanillaTextureFallbacks          int
	VanillaModelFallbacks            int
	NumericFallbacks                 int
	BarrierFallbacks                 int
	TotalDuration                    time.Duration
	CacheDuration                    time.Duration
	FallbackDuration                 time.Duration
	Samples                          []TextureDecisionSample
}

type TextureDecisionSample struct {
	SkyBlockID  string
	MinecraftID string
	ItemModel   string
	Texture     string
	TexturePack string
	StableKey   string
	Reason      string
	Duration    time.Duration
}

type resourcePackMeta struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Author   string `json:"author"`
	URL      string `json:"url"`
	Icon     string `json:"icon"`
	Priority int    `json:"priority"`
}

func ResourcePackConfigs() ([]models.ResourcePackConfig, error) {
	resourcePackConfigsOnce.Do(func() {
		appRoot, err := appRootDir()
		if err != nil {
			resourcePackConfigsErr = err
			return
		}

		resourcePackConfigs, resourcePackConfigsErr = loadResourcePackConfigs(filepath.Join(appRoot, "assets", "resourcepacks"))
	})

	return cloneResourcePackConfigs(resourcePackConfigs), resourcePackConfigsErr
}

func loadResourcePackConfigs(resourcePacksPath string) ([]models.ResourcePackConfig, error) {
	files, err := os.ReadDir(resourcePacksPath)
	if err != nil {
		return nil, err
	}

	configs := make([]models.ResourcePackConfig, 0, len(files))
	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		metaPath := filepath.Join(resourcePacksPath, file.Name(), "meta.json")
		metaFile, err := os.Open(metaPath)
		if err != nil {
			continue
		}

		var meta resourcePackMeta
		decodeErr := json.NewDecoder(metaFile).Decode(&meta)
		_ = metaFile.Close()
		if decodeErr != nil {
			continue
		}

		id := strings.TrimSpace(meta.ID)
		if id == "" || strings.EqualFold(id, "vanilla") {
			continue
		}
		url := strings.TrimSpace(meta.URL)
		author := strings.TrimSpace(meta.Author)
		icon := strings.TrimSpace(meta.Icon)
		if icon == "" {
			icon = fmt.Sprintf("%s/assets/resourcepacks/%s/pack.webp", utility.GetDomain(), file.Name())
		}
		priority := meta.Priority
		if priority == 0 {
			priority = defaultResourcePackPriority(id)
		}

		config := models.ResourcePackConfig{
			Id:       id,
			Name:     strings.TrimSpace(meta.Name),
			Version:  strings.TrimSpace(meta.Version),
			Priority: priority,
			Author:   author,
			Url:      url,
			Icon:     icon,
		}
		configs = append(configs, config)
	}

	sortResourcePackConfigs(configs)
	return configs, nil
}

func sortResourcePackConfigs(configs []models.ResourcePackConfig) {
	sort.SliceStable(configs, func(i, j int) bool {
		if configs[i].Priority != configs[j].Priority {
			return configs[i].Priority > configs[j].Priority
		}
		return configs[i].Id < configs[j].Id
	})
}

func defaultResourcePackPriority(packID string) int {
	switch canonicalPackAlias(packID) {
	case "fsr":
		return 100
	case "hplus":
		return 50
	default:
		return 0
	}
}

func cloneResourcePackConfigs(configs []models.ResourcePackConfig) []models.ResourcePackConfig {
	cloned := make([]models.ResourcePackConfig, len(configs))
	copy(cloned, configs)
	return cloned
}

func defaultResourcePackIDs() []string {
	configs, err := ResourcePackConfigs()
	if err == nil && len(configs) > 0 {
		ids := make([]string, 0, len(configs))
		for _, config := range configs {
			id := strings.TrimSpace(config.Id)
			if id != "" {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			return ids
		}
	}

	return append([]string(nil), fallbackResourcePackIDs...)
}

func defaultPackSignature() string {
	return strings.Join(defaultResourcePackIDs(), ",")
}

func NewTextureApplyContext(disabledPacksParam ...[]string) TextureApplyContext {
	disabledPacks := disabledPackSet(disabledPacksParam...)
	enabledPackIDs := enabledPackIDs(disabledPacks)
	signature := strings.Join(enabledPackIDs, ",")
	if signature == "" {
		signature = "vanilla"
	}

	return TextureApplyContext{
		DisabledPacks:  disabledPacks,
		EnabledPackIDs: enabledPackIDs,
		PackSignature:  signature,
		Domain:         utility.GetDomain(),
		Stats:          &TextureApplyStats{},
	}
}

func normalizeTextureApplyContext(textureCtx TextureApplyContext) TextureApplyContext {
	if textureCtx.DisabledPacks == nil {
		textureCtx.DisabledPacks = map[string]struct{}{}
	}
	if textureCtx.EnabledPackIDs == nil {
		textureCtx.EnabledPackIDs = enabledPackIDs(textureCtx.DisabledPacks)
	}
	if textureCtx.PackSignature == "" {
		if len(textureCtx.EnabledPackIDs) == 0 {
			textureCtx.PackSignature = "vanilla"
		} else {
			textureCtx.PackSignature = strings.Join(textureCtx.EnabledPackIDs, ",")
		}
	}
	if textureCtx.Domain == "" {
		textureCtx.Domain = utility.GetDomain()
	}
	if textureCtx.DisableStats {
		textureCtx.Stats = nil
	} else if textureCtx.Stats == nil {
		textureCtx.Stats = &TextureApplyStats{}
	}
	return textureCtx
}

func cachedItemTexture(id string, disabledPacks map[string]struct{}) (AppliedItemTexture, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return AppliedItemTexture{}, false
	}

	return cachedTextureForStableKey("skyblock:"+id, packSignatureFromDisabled(disabledPacks), enabledPackIDs(disabledPacks), disabledPacks, id)
}

func cachedTextureForStableKey(stableKey string, packSignature string, enabledPackIDs []string, disabledPacks map[string]struct{}, legacyKeys ...string) (AppliedItemTexture, bool) {
	stableKey = strings.TrimSpace(stableKey)
	if stableKey == "" || len(enabledPackIDs) == 0 {
		return AppliedItemTexture{}, false
	}

	seenKeys := map[string]struct{}{}
	if texture, ok := cachedTextureByKeyOnce(textureCacheKey(packSignature, stableKey), disabledPacks, seenKeys); ok {
		return texture, true
	}

	for _, packID := range enabledPackIDs {
		if texture, ok := cachedTextureByPackVariant(packID, stableKey, disabledPacks, seenKeys); ok {
			return texture, true
		}
	}

	if packSignature != defaultPackSignature() {
		if texture, ok := cachedTextureByKeyOnce(textureCacheKey(defaultPackSignature(), stableKey), disabledPacks, seenKeys); ok {
			return texture, true
		}
	}

	for _, legacyKey := range legacyKeys {
		if texture, ok := cachedTextureByKeyOnce(legacyKey, disabledPacks, seenKeys); ok {
			return texture, true
		}
	}
	return AppliedItemTexture{}, false
}

func cachedTextureByKeyOnce(key string, disabledPacks map[string]struct{}, seenKeys map[string]struct{}) (AppliedItemTexture, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return AppliedItemTexture{}, false
	}
	if _, seen := seenKeys[key]; seen {
		return AppliedItemTexture{}, false
	}
	seenKeys[key] = struct{}{}
	return cachedTextureByKey(key, disabledPacks)
}

func cachedTextureByPackVariant(packID string, stableKey string, disabledPacks map[string]struct{}, seenKeys map[string]struct{}) (AppliedItemTexture, bool) {
	packID = strings.TrimSpace(packID)
	if packID == "" {
		return AppliedItemTexture{}, false
	}

	for _, alias := range texturePackAliases(packID) {
		texture, ok := cachedTextureByKeyOnce(textureCacheKey(alias, stableKey), disabledPacks, seenKeys)
		if !ok {
			continue
		}
		if !sameTexturePack(texture.TexturePack, packID) {
			continue
		}
		return texture, true
	}
	return AppliedItemTexture{}, false
}

func cachedTextureByKey(key string, disabledPacks map[string]struct{}) (AppliedItemTexture, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return AppliedItemTexture{}, false
	}

	itemTextureCacheMu.RLock()
	texture := ITEM_TEXTURE_CACHE[key]
	itemTextureCacheMu.RUnlock()

	if !validCachedTexture(texture, disabledPacks) {
		return AppliedItemTexture{}, false
	}

	return texture, true
}

func setCachedTextureForStableKey(packSignature string, stableKey string, texture AppliedItemTexture) {
	stableKey = strings.TrimSpace(stableKey)
	if stableKey == "" || texture.Texture == "" || isStaleVanillaChestParticleRender(texture.Texture) {
		return
	}
	packSignature = strings.TrimSpace(packSignature)
	if packSignature == "" {
		packSignature = strings.ToLower(strings.TrimSpace(texture.TexturePack))
	}
	if packSignature == "" {
		packSignature = defaultPackSignature()
	}

	itemTextureCacheMu.Lock()
	ITEM_TEXTURE_CACHE[textureCacheKey(packSignature, stableKey)] = texture
	for _, alias := range texturePackAliases(texture.TexturePack) {
		ITEM_TEXTURE_CACHE[textureCacheKey(alias, stableKey)] = texture
	}
	if skyblockID := strings.TrimPrefix(stableKey, "skyblock:"); skyblockID != stableKey && skyblockID != "" {
		ITEM_TEXTURE_CACHE[skyblockID] = texture
	}
	itemTextureCacheMu.Unlock()

	if skyblockID := strings.TrimPrefix(stableKey, "skyblock:"); skyblockID != stableKey && skyblockID != "" {
		rememberRenderedSkyBlockID(skyblockID, packSignature)
		if texture.TexturePack != "" && !strings.EqualFold(texture.TexturePack, packSignature) {
			rememberRenderedSkyBlockID(skyblockID, texture.TexturePack)
		}
	}
}

func itemTextureCacheLen() int {
	itemTextureCacheMu.RLock()
	length := len(ITEM_TEXTURE_CACHE)
	itemTextureCacheMu.RUnlock()
	return length
}

func ItemTextureCacheLen() int {
	return itemTextureCacheLen()
}

func cachedTextureFromRawMap(itemMap map[string]any, disabledPacks map[string]struct{}) (AppliedItemTexture, bool) {
	if skyblockID := textureString(itemMap, "skyblock_id", "skyblockId", "SkyblockID"); skyblockID != "" {
		if cachedTexture, ok := cachedItemTexture(skyblockID, disabledPacks); ok {
			return cachedTexture, true
		}
	}

	if id := normalizeMinecraftItemID(textureString(itemMap, "ItemModel", "item_model", "itemModel", "id", "ID")); id != "" {
		if cachedTexture, ok := cachedItemTexture(strings.TrimPrefix(id, "minecraft:"), disabledPacks); ok {
			return cachedTexture, true
		}
	}

	return AppliedItemTexture{}, false
}

func barrierTextureURL() string {
	return fmt.Sprintf("%s/assets/resourcepacks/Vanilla/assets/minecraft/textures/item/barrier.png", utility.GetDomain())
}

func normalizeTextureItem(item any) (map[string]any, bool) {
	if item == nil {
		return nil, false
	}

	data, err := json.Marshal(item)
	if err != nil {
		return nil, false
	}

	var itemMap map[string]any
	if err := json.Unmarshal(data, &itemMap); err != nil {
		return nil, false
	}
	if len(itemMap) == 0 {
		return nil, false
	}

	return itemMap, true
}

func textureValue(values map[string]any, keys ...string) (any, bool) {
	if values == nil {
		return nil, false
	}

	for _, key := range keys {
		for actualKey, value := range values {
			if strings.EqualFold(actualKey, key) {
				return value, true
			}
		}
	}

	return nil, false
}

func textureMap(values map[string]any, keys ...string) (map[string]any, bool) {
	value, ok := textureValue(values, keys...)
	if !ok {
		return nil, false
	}

	mapped, ok := value.(map[string]any)
	return mapped, ok
}

func textureString(values map[string]any, keys ...string) string {
	value, ok := textureValue(values, keys...)
	if !ok || value == nil {
		return ""
	}

	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strings.TrimSpace(strconv.FormatFloat(typed, 'f', -1, 64))
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func textureInt(values map[string]any, keys ...string) (int, bool) {
	value, ok := textureValue(values, keys...)
	if !ok || value == nil {
		return 0, false
	}

	switch typed := value.(type) {
	case int:
		return typed, true
	case *int:
		if typed != nil {
			return *typed, true
		}
	case int8:
		return int(typed), true
	case *int8:
		if typed != nil {
			return int(*typed), true
		}
	case int16:
		return int(typed), true
	case *int16:
		if typed != nil {
			return int(*typed), true
		}
	case int32:
		return int(typed), true
	case *int32:
		if typed != nil {
			return int(*typed), true
		}
	case int64:
		return int(typed), true
	case *int64:
		if typed != nil {
			return int(*typed), true
		}
	case uint:
		return int(typed), true
	case *uint:
		if typed != nil {
			return int(*typed), true
		}
	case uint8:
		return int(typed), true
	case *uint8:
		if typed != nil {
			return int(*typed), true
		}
	case uint16:
		return int(typed), true
	case *uint16:
		if typed != nil {
			return int(*typed), true
		}
	case uint32:
		return int(typed), true
	case *uint32:
		if typed != nil {
			return int(*typed), true
		}
	case uint64:
		return int(typed), true
	case *uint64:
		if typed != nil {
			return int(*typed), true
		}
	case float32:
		return int(typed), true
	case *float32:
		if typed != nil {
			return int(*typed), true
		}
	case float64:
		return int(typed), true
	case *float64:
		if typed != nil {
			return int(*typed), true
		}
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed), true
		}
	case *json.Number:
		if typed != nil {
			parsed, err := typed.Int64()
			if err == nil {
				return int(parsed), true
			}
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed, true
		}
	case *string:
		if typed != nil {
			parsed, err := strconv.Atoi(strings.TrimSpace(*typed))
			if err == nil {
				return parsed, true
			}
		}
	}

	return 0, false
}

func numericString(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	_, err := strconv.Atoi(value)
	return err == nil
}

func normalizeMinecraftItemID(id string) string {
	id = strings.TrimSpace(strings.ToLower(id))
	if id == "" {
		return ""
	}
	if numericString(id) {
		return ""
	}
	if !strings.Contains(id, ":") {
		return "minecraft:" + id
	}
	return id
}

func itemModelFromItem(itemMap map[string]any) string {
	if itemModel := normalizeMinecraftItemID(textureString(itemMap, "ItemModel", "item_model", "itemModel")); itemModel != "" {
		return itemModel
	}

	tag, ok := textureMap(itemMap, "tag", "Tag")
	if !ok {
		return ""
	}

	return normalizeMinecraftItemID(textureString(tag, "ItemModel", "item_model", "itemModel"))
}

func explicitMinecraftItemIDFromItem(itemMap map[string]any) string {
	if itemModel := itemModelFromItem(itemMap); itemModel != "" {
		return itemModel
	}
	return normalizeMinecraftItemID(textureString(itemMap, "id", "ID"))
}

func shouldUseLegacyNumericFallback(id string) bool {
	id = strings.TrimSpace(strings.ToLower(id))
	if id == "" || id == "minecraft:" || id == "minecraft:air" {
		return true
	}
	id = strings.TrimPrefix(id, "minecraft:")
	return numericString(id) || !vanillaItemResourceExists(id)
}

func disabledPackSet(disabledPacksParam ...[]string) map[string]struct{} {
	disabledPacks := map[string]struct{}{}
	if len(disabledPacksParam) == 0 {
		return disabledPacks
	}

	for _, pack := range NormalizeDisabledPacks(disabledPacksParam[0]) {
		disabledPacks[pack] = struct{}{}
	}

	return disabledPacks
}

func NormalizeDisabledPacks(disabledPacks []string) []string {
	if len(disabledPacks) == 0 {
		return nil
	}

	knownPacks := knownResourcePackAliases()
	disabledSet := map[string]struct{}{}
	for _, rawPack := range disabledPacks {
		for _, pack := range strings.Split(rawPack, ",") {
			canonicalPack := canonicalPackAlias(pack)
			if canonicalPack == "" {
				continue
			}
			if _, known := knownPacks[canonicalPack]; !known {
				continue
			}
			disabledSet[canonicalPack] = struct{}{}
		}
	}
	if len(disabledSet) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(disabledSet))
	for _, packID := range defaultResourcePackIDs() {
		if _, disabled := disabledSet[canonicalPackAlias(packID)]; disabled {
			normalized = append(normalized, packID)
		}
	}
	return normalized
}

func knownResourcePackAliases() map[string]struct{} {
	known := map[string]struct{}{}
	for _, packID := range defaultResourcePackIDs() {
		if canonicalPack := canonicalPackAlias(packID); canonicalPack != "" {
			known[canonicalPack] = struct{}{}
		}
	}
	return known
}

func canonicalPackAlias(packID string) string {
	packID = strings.ToLower(strings.TrimSpace(packID))
	packID = strings.ReplaceAll(packID, "-", "_")
	packID = strings.ReplaceAll(packID, " ", "_")
	switch packID {
	case "hypixel_plus", "hplus", "hypixel+":
		return "hplus"
	case "fur_sky_reborn", "fursky_reborn", "fursky", "fsr":
		return "fsr"
	default:
		return packID
	}
}

func texturePackAliases(packID string) []string {
	packID = strings.TrimSpace(packID)
	if packID == "" {
		return nil
	}

	aliases := []string{packID}
	switch canonicalPackAlias(packID) {
	case "hplus":
		aliases = append(aliases, "HYPIXEL_PLUS", "hypixel_plus", "hplus")
	case "fsr":
		aliases = append(aliases, "FSR", "fsr", "FURSKY_REBORN", "fursky_reborn")
	}

	seen := map[string]struct{}{}
	output := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		if _, ok := seen[alias]; ok {
			continue
		}
		seen[alias] = struct{}{}
		output = append(output, alias)
	}
	return output
}

func sameTexturePack(a string, b string) bool {
	return canonicalPackAlias(a) == canonicalPackAlias(b)
}

func enabledPackIDs(disabledPacks map[string]struct{}) []string {
	allPackIDs := defaultResourcePackIDs()
	enabled := make([]string, 0, len(allPackIDs))
	for _, packID := range allPackIDs {
		if packDisabled(packID, disabledPacks) {
			continue
		}
		enabled = append(enabled, packID)
	}
	return enabled
}

func packDisabled(packID string, disabledPacks map[string]struct{}) bool {
	if len(disabledPacks) == 0 {
		return false
	}
	canonicalPack := canonicalPackAlias(packID)
	if canonicalPack == "" {
		return false
	}
	for disabledPack := range disabledPacks {
		if canonicalPackAlias(disabledPack) == canonicalPack {
			return true
		}
	}
	return false
}

func packSignatureFromDisabled(disabledPacks map[string]struct{}) string {
	enabled := enabledPackIDs(disabledPacks)
	if len(enabled) == 0 {
		return "vanilla"
	}
	return strings.Join(enabled, ",")
}

func packSignatureFromPackIDs(packIDs []string) string {
	normalized := normalizePackIDs(packIDs)
	if len(normalized) == 0 {
		return "vanilla"
	}
	return strings.Join(normalized, ",")
}

func normalizePackIDs(packIDs []string) []string {
	if len(packIDs) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(packIDs))
	for _, packID := range packIDs {
		packID = strings.TrimSpace(packID)
		if packID == "" {
			continue
		}
		seenKey := canonicalPackAlias(packID)
		if _, ok := seen[seenKey]; ok {
			continue
		}
		seen[seenKey] = struct{}{}
		normalized = append(normalized, packID)
	}
	return normalized
}

func textureCacheKey(packSignature string, stableKey string) string {
	packSignature = strings.TrimSpace(packSignature)
	if packSignature == "" {
		packSignature = defaultPackSignature()
	}
	return packSignature + "|" + stableKey
}

func texturePackDisabled(texturePack string, disabledPacks map[string]struct{}) bool {
	return packDisabled(texturePack, disabledPacks)
}

func validCachedTexture(texture AppliedItemTexture, disabledPacks map[string]struct{}) bool {
	return texture.Texture != "" && !texturePackDisabled(texture.TexturePack, disabledPacks) && !isStaleVanillaChestParticleRender(texture.Texture)
}

func isStaleVanillaChestParticleRender(texture string) bool {
	normalized := strings.ToLower(strings.TrimSpace(texture))
	if normalized == "" || !strings.Contains(normalized, "/cache/rendered/") {
		return false
	}

	staleMarkers := []string{
		"model=minecraft_item_chest__tex1=minecraft_block_oak_planks",
		"model=minecraft_item_ender_chest__tex1=minecraft_block_obsidian",
		"model=minecraft_item_trapped_chest__tex1=minecraft_block_oak_planks",
	}
	for _, marker := range staleMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func rememberRenderedSkyBlockID(id string, packID string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	packID = strings.TrimSpace(packID)
	if packID == "" {
		packID = defaultPackSignature()
	}

	renderedSkyBlockIndexMu.Lock()
	renderedSkyBlockIndex[renderedSkyBlockIndexKey(packID, id)] = struct{}{}
	renderedSkyBlockIndexMu.Unlock()
}

func renderedSkyBlockIDKnown(id string, packID string) bool {
	id = strings.TrimSpace(id)
	packID = strings.TrimSpace(packID)
	if id == "" || packID == "" {
		return false
	}
	renderedSkyBlockIndexMu.RLock()
	_, ok := renderedSkyBlockIndex[renderedSkyBlockIndexKey(packID, id)]
	renderedSkyBlockIndexMu.RUnlock()
	if ok {
		return true
	}

	if strings.Contains(packID, ",") {
		_, ok := cachedTextureByKey(textureCacheKey(packID, "skyblock:"+id), nil)
		return ok
	}
	for _, alias := range texturePackAliases(packID) {
		texture, ok := cachedTextureByKey(textureCacheKey(alias, "skyblock:"+id), nil)
		if ok && sameTexturePack(texture.TexturePack, packID) {
			return true
		}
	}
	return false
}

func renderedSkyBlockIndexKey(packID string, id string) string {
	if !strings.Contains(packID, ",") {
		packID = canonicalPackAlias(packID)
	}
	return strings.TrimSpace(packID) + "|" + strings.TrimSpace(id)
}

func skyblockIDFromItem(itemMap map[string]any) string {
	if skyblockID := textureString(itemMap, "skyblock_id", "skyblockId", "SkyblockID"); skyblockID != "" {
		return skyblockID
	}

	tag, ok := textureMap(itemMap, "tag", "Tag")
	if !ok {
		return ""
	}

	extraAttributes, ok := textureMap(tag, "ExtraAttributes", "extraAttributes", "extra_attributes")
	if !ok {
		return ""
	}

	return textureString(extraAttributes, "id", "Id", "ID")
}

func vanillaRenderItem(itemMap map[string]any, id string) map[string]any {
	vanillaItem, ok := normalizeTextureItem(itemMap)
	if !ok {
		return nil
	}

	vanillaItem["id"] = id
	vanillaItem["skyblock_id"] = ""
	vanillaItem["skyblockId"] = ""
	vanillaItem["SkyblockID"] = ""

	tag, ok := textureMap(vanillaItem, "tag", "Tag")
	if !ok {
		return vanillaItem
	}

	extraAttributes, ok := textureMap(tag, "ExtraAttributes", "extraAttributes", "extra_attributes")
	if ok {
		extraAttributes["id"] = ""
		extraAttributes["Id"] = ""
		extraAttributes["ID"] = ""
	}

	return vanillaItem
}

func skullTextureHashFromValues(values map[string]any) string {
	skullOwner, ok := textureMap(values, "SkullOwner", "skullOwner", "skull_owner")
	if !ok {
		return ""
	}

	properties, ok := textureMap(skullOwner, "Properties", "properties")
	if !ok {
		return ""
	}

	texturesValue, ok := textureValue(properties, "textures", "Textures")
	if !ok {
		return ""
	}

	textures, ok := texturesValue.([]any)
	if !ok || len(textures) == 0 {
		return ""
	}

	textureEntry, ok := textures[0].(map[string]any)
	if !ok {
		return ""
	}

	value := textureString(textureEntry, "Value", "value")
	if value == "" {
		return ""
	}

	return utility.GetSkinHash(value)
}

func headTextureURL(texture string) AppliedItemTexture {
	texture = strings.TrimSpace(texture)
	if texture == "" {
		return AppliedItemTexture{}
	}

	if skinHash := utility.GetSkinHash(texture); skinHash != "" {
		texture = skinHash
	}

	return AppliedItemTexture{Texture: fmt.Sprintf("%s/api/head/%s", utility.GetDomain(), texture)}
}

func displayColorFromItem(itemMap map[string]any) string {
	tag, ok := textureMap(itemMap, "tag", "Tag")
	if !ok {
		return ""
	}

	display, ok := textureMap(tag, "display", "Display")
	if !ok {
		return ""
	}

	color, ok := textureInt(display, "color", "Color")
	if !ok || color == 0 {
		return ""
	}

	return fmt.Sprintf("%06X", color)
}

func vanillaTextureURL(id string) AppliedItemTexture {
	id = strings.TrimSpace(strings.ToLower(strings.TrimPrefix(id, "minecraft:")))
	if id == "" {
		return AppliedItemTexture{}
	}
	if cached, ok := vanillaTextureCache.Load(id); ok {
		return cached.(AppliedItemTexture)
	}

	publicPath := fmt.Sprintf("/assets/resourcepacks/Vanilla/assets/minecraft/textures/item/%s.png", id)
	appRoot, err := appRootDir()
	if err != nil {
		return AppliedItemTexture{}
	}

	localPath := filepath.Join(appRoot, filepath.FromSlash(strings.TrimPrefix(publicPath, "/")))
	if _, err := os.Stat(localPath); err != nil {
		vanillaTextureCache.Store(id, AppliedItemTexture{})
		return AppliedItemTexture{}
	}

	texture := AppliedItemTexture{Texture: utility.GetDomain() + publicPath}
	vanillaTextureCache.Store(id, texture)
	return texture
}

func vanillaAssetTextureURL(texturePath string) AppliedItemTexture {
	texturePath = strings.TrimSpace(strings.TrimPrefix(texturePath, "minecraft:"))
	if texturePath == "" {
		return AppliedItemTexture{}
	}
	if cached, ok := vanillaAssetTextureCache.Load(texturePath); ok {
		return cached.(AppliedItemTexture)
	}

	publicPath := fmt.Sprintf("/assets/resourcepacks/Vanilla/assets/minecraft/textures/%s.png", texturePath)
	appRoot, err := appRootDir()
	if err != nil {
		return AppliedItemTexture{}
	}

	localPath := filepath.Join(appRoot, filepath.FromSlash(strings.TrimPrefix(publicPath, "/")))
	if _, err := os.Stat(localPath); err != nil {
		vanillaAssetTextureCache.Store(texturePath, AppliedItemTexture{})
		return AppliedItemTexture{}
	}

	texture := AppliedItemTexture{Texture: utility.GetDomain() + publicPath}
	vanillaAssetTextureCache.Store(texturePath, texture)
	return texture
}

func vanillaBlockTextureURL(id string) AppliedItemTexture {
	id = strings.TrimSpace(strings.ToLower(strings.TrimPrefix(id, "minecraft:")))
	if id == "" {
		return AppliedItemTexture{}
	}

	for _, candidate := range []string{
		"block/" + id,
		"block/" + id + "_front",
		"block/" + id + "_top",
		"block/" + id + "_side",
		"block/" + id + "_pane_top",
	} {
		if texture := vanillaAssetTextureURL(candidate); texture.Texture != "" {
			return texture
		}
	}

	return AppliedItemTexture{}
}

func vanillaModelTextureURL(id string) AppliedItemTexture {
	id = strings.TrimSpace(strings.ToLower(strings.TrimPrefix(id, "minecraft:")))
	if id == "" {
		return AppliedItemTexture{}
	}
	if cached, ok := vanillaModelTextureCache.Load(id); ok {
		return cached.(AppliedItemTexture)
	}

	appRoot, err := appRootDir()
	if err != nil {
		return AppliedItemTexture{}
	}

	modelRefs := []string{}
	if itemDefinition := readVanillaJSON(filepath.Join(appRoot, "assets", "resourcepacks", "Vanilla", "assets", "minecraft", "items", id+".json")); itemDefinition != nil {
		if modelRef := findVanillaModelRef(itemDefinition); modelRef != "" {
			modelRefs = append(modelRefs, modelRef)
		}
	}
	modelRefs = append(modelRefs, "minecraft:item/"+id, "minecraft:block/"+id)

	seen := map[string]struct{}{}
	for _, modelRef := range modelRefs {
		modelRef = strings.TrimSpace(strings.TrimPrefix(modelRef, "minecraft:"))
		if modelRef == "" {
			continue
		}
		if _, ok := seen[modelRef]; ok {
			continue
		}
		seen[modelRef] = struct{}{}

		modelPath := filepath.Join(appRoot, "assets", "resourcepacks", "Vanilla", "assets", "minecraft", "models", filepath.FromSlash(modelRef+".json"))
		model := readVanillaJSON(modelPath)
		if model == nil {
			continue
		}
		if texture := textureFromVanillaModel(model); texture.Texture != "" {
			vanillaModelTextureCache.Store(id, texture)
			return texture
		}
	}

	texture := vanillaBlockTextureURL(id)
	vanillaModelTextureCache.Store(id, texture)
	return texture
}

func vanillaSpecialHeadTextureURL(id string) AppliedItemTexture {
	id = strings.TrimSpace(strings.ToLower(strings.TrimPrefix(id, "minecraft:")))
	switch id {
	case "zombie_head":
		return vanillaAssetTextureURL("entity/zombie/zombie")
	case "skeleton_skull":
		return vanillaAssetTextureURL("entity/skeleton/skeleton")
	case "wither_skeleton_skull":
		return vanillaAssetTextureURL("entity/skeleton/wither_skeleton")
	case "creeper_head":
		return vanillaAssetTextureURL("entity/creeper/creeper")
	case "dragon_head":
		return vanillaAssetTextureURL("entity/enderdragon/dragon")
	case "piglin_head":
		return vanillaAssetTextureURL("entity/piglin/piglin")
	default:
		return AppliedItemTexture{}
	}
}

func readVanillaJSON(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil
	}

	return decoded
}

func findVanillaModelRef(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		if modelRef, ok := typed["model"].(string); ok && strings.TrimSpace(modelRef) != "" {
			return modelRef
		}
		for _, key := range []string{"model", "cases", "entries"} {
			if modelRef := findVanillaModelRef(typed[key]); modelRef != "" {
				return modelRef
			}
		}
		for _, nested := range typed {
			if modelRef := findVanillaModelRef(nested); modelRef != "" {
				return modelRef
			}
		}
	case []any:
		for _, nested := range typed {
			if modelRef := findVanillaModelRef(nested); modelRef != "" {
				return modelRef
			}
		}
	}

	return ""
}

func textureFromVanillaModel(model map[string]any) AppliedItemTexture {
	texturesRaw, ok := textureMap(model, "textures")
	if !ok {
		return AppliedItemTexture{}
	}

	textures := map[string]string{}
	for key := range texturesRaw {
		if textureRef := textureString(texturesRaw, key); textureRef != "" {
			textures[key] = textureRef
		}
	}

	for _, key := range []string{"layer0", "all", "front", "pane", "edge", "side", "top", "particle", "texture"} {
		if texture := vanillaTextureFromModelRef(textures[key], textures); texture.Texture != "" {
			return texture
		}
	}
	for _, textureRef := range textures {
		if texture := vanillaTextureFromModelRef(textureRef, textures); texture.Texture != "" {
			return texture
		}
	}

	return AppliedItemTexture{}
}

func vanillaTextureFromModelRef(textureRef string, textures map[string]string) AppliedItemTexture {
	textureRef = strings.TrimSpace(textureRef)
	if textureRef == "" {
		return AppliedItemTexture{}
	}
	if strings.HasPrefix(textureRef, "#") {
		textureRef = textures[strings.TrimPrefix(textureRef, "#")]
	}

	textureRef = strings.TrimPrefix(textureRef, "minecraft:")
	return vanillaAssetTextureURL(textureRef)
}

func vanillaItemResourceExists(id string) bool {
	id = strings.TrimSpace(strings.ToLower(strings.TrimPrefix(id, "minecraft:")))
	if id == "" {
		return false
	}
	if cached, ok := vanillaItemExistsCache.Load(id); ok {
		return cached.(bool)
	}
	if vanillaTextureURL(id).Texture != "" {
		vanillaItemExistsCache.Store(id, true)
		return true
	}
	if vanillaModelTextureURL(id).Texture != "" {
		vanillaItemExistsCache.Store(id, true)
		return true
	}

	appRoot, err := appRootDir()
	if err != nil {
		return false
	}

	for _, relativePath := range []string{
		fmt.Sprintf("assets/resourcepacks/Vanilla/assets/minecraft/items/%s.json", id),
		fmt.Sprintf("assets/resourcepacks/Vanilla/assets/minecraft/models/item/%s.json", id),
		fmt.Sprintf("assets/resourcepacks/Vanilla/assets/minecraft/models/block/%s.json", id),
	} {
		if _, err := os.Stat(filepath.Join(appRoot, filepath.FromSlash(relativePath))); err == nil {
			vanillaItemExistsCache.Store(id, true)
			return true
		}
	}

	vanillaItemExistsCache.Store(id, false)
	return false
}

func publicCacheTextureURL(texturePath string) string {
	texturePath = strings.TrimSpace(texturePath)
	if texturePath == "" {
		return ""
	}
	if strings.HasPrefix(texturePath, "http://") || strings.HasPrefix(texturePath, "https://") {
		return texturePath
	}

	normalizedPath := filepath.ToSlash(texturePath)
	if strings.HasPrefix(normalizedPath, "/cache/") {
		return utility.GetDomain() + normalizedPath
	}
	if strings.HasPrefix(normalizedPath, "cache/") {
		return fmt.Sprintf("%s/%s", utility.GetDomain(), normalizedPath)
	}
	if strings.HasPrefix(normalizedPath, "rendered/") {
		return fmt.Sprintf("%s/cache/%s", utility.GetDomain(), normalizedPath)
	}
	if cacheIndex := strings.Index(normalizedPath, "/cache/"); cacheIndex >= 0 {
		return utility.GetDomain() + normalizedPath[cacheIndex:]
	}

	return fmt.Sprintf("%s/cache/rendered/%s", utility.GetDomain(), filepath.Base(normalizedPath))
}

func preRenderSkyBlockItemIDs() []string {
	skyblockItems := constants.ItemsSnapshot()
	seen := map[string]struct{}{}
	itemIDs := make([]string, 0, len(skyblockItems))

	addID := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		itemIDs = append(itemIDs, id)
	}

	for _, item := range skyblockItems {
		addID(item.SkyblockID)
	}

	if appRoot, err := appRootDir(); err == nil {
		if entries, err := os.ReadDir(filepath.Join(appRoot, "NotEnoughUpdates-REPO", "items")); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				name := entry.Name()
				if strings.EqualFold(filepath.Ext(name), ".json") {
					addID(strings.TrimSuffix(name, filepath.Ext(name)))
				}
			}
		}
	}

	notenoughupdates.CACHED_NEU_ITEMS.Range(func(key, value any) bool {
		if keyID, ok := key.(string); ok {
			addID(keyID)
		}

		switch item := value.(type) {
		case models.NEUItem:
			addID(item.NEUId)
			if item.NBT.ExtraAttributes != nil {
				addID(item.NBT.ExtraAttributes.Id)
			}
		case *models.NEUItem:
			if item != nil {
				addID(item.NEUId)
				if item.NBT.ExtraAttributes != nil {
					addID(item.NBT.ExtraAttributes.Id)
				}
			}
		}

		return true
	})

	return itemIDs
}

func LoadRenderedTextureIndex(cacheDir string) (int, error) {
	if strings.TrimSpace(cacheDir) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return 0, fmt.Errorf("failed to get current working directory: %v", err)
		}
		cacheDir = filepath.Join(cwd, "cache")
	}

	renderedDir := filepath.Join(cacheDir, "rendered")
	files, err := os.ReadDir(renderedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	loaded := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()
		parts := strings.Split(fileName, "__")
		itemId := ""
		minecraftID := ""
		itemModel := ""
		texturePack := ""
		for _, part := range parts {
			if strings.HasPrefix(part, "skyblock=") {
				itemId = strings.TrimPrefix(part, "skyblock=")
			} else if strings.HasPrefix(part, "mc=") {
				minecraftID = debugFilenameIdentifier(strings.TrimPrefix(part, "mc="))
			} else if strings.HasPrefix(part, "itemmodel=") {
				itemModel = debugFilenameIdentifier(strings.TrimPrefix(part, "itemmodel="))
			} else if strings.HasPrefix(part, "pack=") {
				texturePack = strings.TrimSpace(strings.TrimPrefix(part, "pack="))
			}
		}

		texture := AppliedItemTexture{
			Texture:     publicCacheTextureURL(filepath.Join(renderedDir, fileName)),
			TexturePack: texturePack,
		}
		if !validCachedTexture(texture, nil) {
			continue
		}
		if itemId != "" && texturePack != "" {
			setCachedTextureForStableKey(texturePack, "skyblock:"+itemId, texture)
			loaded++
		}
		if texturePack != "" {
			if itemModel != "" {
				setCachedTextureForStableKey(texturePack, "itemmodel:"+normalizeMinecraftItemID(itemModel), texture)
			}
			if minecraftID != "" {
				setCachedTextureForStableKey(texturePack, fmt.Sprintf("mc:%s|damage:0|color:0", normalizeMinecraftItemID(minecraftID)), texture)
			}
		}
	}

	return loaded, nil
}

func debugFilenameIdentifier(value string) string {
	value = strings.TrimSuffix(strings.TrimSpace(value), ".webp")
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "minecraft_") {
		return "minecraft:" + strings.TrimPrefix(value, "minecraft_")
	}
	return strings.ReplaceAll(value, "_", ":")
}

func StartCustomResources() error {
	customResourcesOnce.Do(func() {
		customResourcesErr = startCustomResources()
	})
	return customResourcesErr
}

func startCustomResources() error {
	timeNow := time.Now()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %v", err)
	}

	cacheDir := filepath.Join(cwd, "cache")
	resourcePacksPath := filepath.Join(cwd, "assets", "resourcepacks")
	assetsPath := filepath.Join(resourcePacksPath, "Vanilla", "assets", "minecraft")
	renderedDir := filepath.Join(cacheDir, "rendered")
	_, renderedDirErr := os.Stat(renderedDir)
	if renderedDirErr != nil && !os.IsNotExist(renderedDirErr) {
		return fmt.Errorf("[CUSTOM_RESOURCES] Failed to stat cache directory %s: %v", renderedDir, renderedDirErr)
	}
	renderedDirExists := renderedDirErr == nil

	timeNowv2 := time.Now()
	loaded, err := LoadRenderedTextureIndex(cacheDir)
	if err != nil {
		return fmt.Errorf("[CUSTOM_RESOURCES] Failed to read cache directory: %v", err)
	}
	if os.Getenv("FIBER_PREFORK_CHILD") == "" || loaded > 0 {
		fmt.Printf("[CUSTOM_RESOURCES] Loaded %s cached textures from %s in %s\n", utility.AddCommas(loaded), renderedDir, time.Since(timeNowv2))
	}

	preloadRenderer := os.Getenv("FIBER_PREFORK_CHILD") == ""
	if err := InitRenderer(cacheDir, resourcePacksPath, assetsPath, preloadRenderer); err != nil {
		return err
	}

	fmt.Printf("[CUSTOM_RESOURCES] Started SkyCrypt renderer instance in %s\n", time.Since(timeNow))

	// Render textures only on main thread; generated files are shared in cache/rendered.
	if os.Getenv("FIBER_PREFORK_CHILD") == "" {
		if err := WarmConfiguredSkyBlockTextures(ctx, cacheDir, resourcePacksPath, assetsPath, preRenderSkyBlockItemIDs(), mr.PreRenderOptions{}); err != nil {
			return err
		}
	} else if !renderedDirExists && os.IsNotExist(renderedDirErr) {
		fmt.Printf("[CUSTOM_RESOURCES] cache/rendered is missing; child process will serve fallbacks until the main process warms textures\n")
	}

	if os.Getenv("FIBER_PREFORK_CHILD") == "" {
		fmt.Printf("[CUSTOM_RESOURCES] SkyCrypt renderer initialized successfully with %s textures in %s\n", utility.AddCommas(itemTextureCacheLen()), time.Since(timeNow))
	}

	return nil
}

func InitRenderer(cacheDir string, resourcePacksPath string, assetsPath string, preload bool) error {
	renderer, err := newRendererForPackIDs(cacheDir, resourcePacksPath, assetsPath, defaultResourcePackIDs(), preload)
	if err != nil {
		return fmt.Errorf("[CUSTOM_RESOURCES] Failed to initialize SkyCrypt renderer: %v", err)
	}
	SkyCryptRender = renderer
	return nil
}

func newRendererForPackIDs(cacheDir string, resourcePacksPath string, assetsPath string, packIDs []string, preload bool) (*mr.Renderer, error) {
	return mr.NewRendererWithOptions(mr.Options{
		AssetsRoot:        assetsPath,
		ResourcePacksRoot: resourcePacksPath,
		PackIDs:           packIDs,
		Preload:           preload,
		CacheDir:          cacheDir,
	})
}

func WarmConfiguredSkyBlockTextures(ctx context.Context, cacheDir string, resourcePacksPath string, assetsPath string, itemIDs []string, options mr.PreRenderOptions) error {
	for _, packID := range defaultResourcePackIDs() {
		renderer := SkyCryptRender
		if len(rendererPackIDs(renderer)) != 1 || rendererPackIDs(renderer)[0] != packID {
			var err error
			renderer, err = newRendererForPackIDs(cacheDir, resourcePacksPath, assetsPath, []string{packID}, false)
			if err != nil {
				return fmt.Errorf("[CUSTOM_RESOURCES] Failed to initialize %s renderer: %v", packID, err)
			}
		}

		if err := warmMissingSkyBlockTexturesWithRenderer(ctx, renderer, []string{packID}, itemIDs, options); err != nil {
			return err
		}
	}
	return nil
}

func WarmMissingSkyBlockTextures(ctx context.Context, itemIDs []string, options mr.PreRenderOptions) error {
	if SkyCryptRender == nil {
		return fmt.Errorf("[CUSTOM_RESOURCES] renderer is not initialized")
	}
	return warmMissingSkyBlockTexturesWithRenderer(ctx, SkyCryptRender, rendererPackIDs(SkyCryptRender), itemIDs, options)
}

func warmMissingSkyBlockTexturesWithRenderer(ctx context.Context, renderer *mr.Renderer, packIDs []string, itemIDs []string, options mr.PreRenderOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if renderer == nil {
		return fmt.Errorf("[CUSTOM_RESOURCES] renderer is not initialized")
	}
	packSignature := packSignatureFromPackIDs(packIDs)
	indexPackID := packSignature
	if len(packIDs) == 1 {
		indexPackID = strings.TrimSpace(packIDs[0])
	}

	missing := make([]string, 0, len(itemIDs))
	for _, id := range itemIDs {
		id = strings.TrimSpace(id)
		if id == "" || renderedSkyBlockIDKnown(id, indexPackID) {
			continue
		}
		missing = append(missing, id)
	}

	if options.Workers <= 0 {
		options.Workers = runtime.GOMAXPROCS(0)
		if options.Workers > 4 {
			options.Workers = 4
		}
		if options.Workers < 1 {
			options.Workers = 1
		}
	}

	timeNow := time.Now()
	fmt.Printf("[CUSTOM_RESOURCES] Pre-rendering %s missing SkyBlock items for packs %s with %d workers...\n", utility.AddCommas(len(missing)), packSignature, options.Workers)
	if len(missing) == 0 {
		fmt.Printf("[CUSTOM_RESOURCES] Pre-rendered 0 SkyBlock items for packs %s, skipped 0, failed 0 in %s\n", packSignature, time.Since(timeNow))
		return nil
	}

	output, err := renderer.PreRenderSkyBlockItemIDs(ctx, missing, options)
	if err != nil {
		return fmt.Errorf("[CUSTOM_RESOURCES] Failed to pre-render SkyBlock items for packs %s: %v", packSignature, err)
	}

	for _, item := range output.Entries {
		if item.Error == "" && !item.Skipped {
			setCachedTextureForStableKey(packSignature, "skyblock:"+item.InputID, AppliedItemTexture{
				Texture:     publicCacheTextureURL(item.Path),
				TexturePack: item.TexturePackID,
			})
		}
	}

	fmt.Printf("[CUSTOM_RESOURCES] Pre-rendered %d SkyBlock items for packs %s, skipped %d, failed %d in %s\n", output.Succeeded, packSignature, output.Skipped, output.Failed, time.Since(timeNow))
	return nil
}

func rendererPackIDs(renderer *mr.Renderer) []string {
	if renderer == nil {
		return nil
	}
	return normalizePackIDs(renderer.PackIDs())
}

func stableTextureKeysFromInput(input ItemTextureInput) []string {
	keys := make([]string, 0, 4)
	if skullKey := skullIdentityFromInput(input); skullKey != "" {
		if skyblockID := strings.TrimSpace(input.SkyBlockID); skyblockID != "" {
			keys = append(keys, "skyblock:"+skyblockID+"|"+skullKey)
		}
		keys = append(keys, skullKey)
		return keys
	}

	if skyblockID := strings.TrimSpace(input.SkyBlockID); skyblockID != "" {
		keys = append(keys, "skyblock:"+skyblockID)
	}
	if itemModel := normalizeMinecraftItemID(input.ItemModel); itemModel != "" {
		keys = append(keys, "itemmodel:"+itemModel)
	}
	if id := normalizeMinecraftItemID(input.ID); id != "" {
		keys = append(keys, fmt.Sprintf("mc:%s|damage:%d|color:%d", id, input.Damage, input.DisplayColor))
	}
	if skinHash := skullTextureHashFromOwner(input.SkullOwner); skinHash != "" {
		keys = append(keys, "skull:"+skinHash)
	}
	return keys
}

type textureCacheLookupDetail struct {
	Reason    string
	StableKey string
}

func cachedTextureForInputDetailed(input ItemTextureInput, textureCtx TextureApplyContext) (AppliedItemTexture, bool, textureCacheLookupDetail) {
	skyblockID := strings.TrimSpace(input.SkyBlockID)
	if texture, ok := cachedStableSkyBlockTexture(skyblockID, textureCtx); ok {
		return texture, true, textureCacheLookupDetail{
			Reason:    "stable_skyblock_cache",
			StableKey: "skyblock:" + skyblockID,
		}
	}

	hasSkullIdentity := skullIdentityFromInput(input) != ""
	for _, stableKey := range stableTextureKeysFromInput(input) {
		if skyblockID != "" && stableKey == "skyblock:"+skyblockID {
			continue
		}
		if texture, ok := cachedTextureForStableKey(stableKey, textureCtx.PackSignature, textureCtx.EnabledPackIDs, textureCtx.DisabledPacks); ok {
			return texture, true, textureCacheLookupDetail{
				Reason:    "stable_key_cache",
				StableKey: stableKey,
			}
		}
	}
	if skyblockID != "" && !hasSkullIdentity {
		if texture, ok := cachedItemTexture(skyblockID, textureCtx.DisabledPacks); ok {
			return texture, true, textureCacheLookupDetail{
				Reason:    "legacy_skyblock_cache",
				StableKey: skyblockID,
			}
		}
	}
	return AppliedItemTexture{}, false, textureCacheLookupDetail{}
}

func cachedStableSkyBlockTexture(skyblockID string, textureCtx TextureApplyContext) (AppliedItemTexture, bool) {
	skyblockID = strings.TrimSpace(skyblockID)
	if skyblockID == "" {
		return AppliedItemTexture{}, false
	}
	return cachedTextureForStableKey("skyblock:"+skyblockID, textureCtx.PackSignature, textureCtx.EnabledPackIDs, textureCtx.DisabledPacks)
}

func setCachedTextureForInput(input ItemTextureInput, textureCtx TextureApplyContext, texture AppliedItemTexture) {
	if texture.Texture == "" {
		return
	}
	for _, stableKey := range stableTextureKeysFromInput(input) {
		setCachedTextureForStableKey(textureCtx.PackSignature, stableKey, texture)
	}
}

func itemTextureInputRenderMap(input ItemTextureInput) map[string]any {
	id := normalizeMinecraftItemID(input.ID)
	if id == "" && input.NumericID > 0 {
		if mapped := constants.GetVanillaItemId(constants.ItemModel{NumericId: input.NumericID, ItemDamage: input.Damage}); mapped != "" {
			id = "minecraft:" + mapped
		}
	}
	if id == "" && input.ItemModel != "" {
		id = normalizeMinecraftItemID(input.ItemModel)
	}

	tag := input.Tag
	if tag == nil {
		tag = map[string]any{"ExtraAttributes": map[string]any{"id": input.SkyBlockID}}
	}

	itemMap := map[string]any{
		"id":          id,
		"tag":         tag,
		"damage":      input.Damage,
		"item_id":     input.NumericID,
		"skyblock_id": input.SkyBlockID,
	}
	if input.ItemModel != "" {
		itemMap["ItemModel"] = input.ItemModel
	}
	if input.Texture != "" {
		itemMap["texture"] = input.Texture
	}
	return itemMap
}

func itemTextureInputFromMap(itemMap map[string]any) ItemTextureInput {
	itemModel := itemModelFromItem(itemMap)
	id := explicitMinecraftItemIDFromItem(itemMap)
	numericID, _ := textureInt(itemMap, "item_id", "itemId", "itemID")
	damage, _ := textureInt(itemMap, "item_damage", "itemDamage", "damage", "Damage")
	displayColor := 0
	if armorColor := textureString(itemMap, "armor_color", "armorColor"); armorColor != "" {
		if parsed, err := strconv.ParseInt(strings.TrimPrefix(armorColor, "#"), 16, 32); err == nil {
			displayColor = int(parsed)
		}
	} else if color, ok := textureInt(itemMap, "display_color", "displayColor"); ok {
		displayColor = color
	} else if colorHex := displayColorFromItem(itemMap); colorHex != "" {
		if parsed, err := strconv.ParseInt(colorHex, 16, 32); err == nil {
			displayColor = int(parsed)
		}
	}
	tag, _ := textureValue(itemMap, "tag", "Tag")

	return ItemTextureInput{
		ID:           id,
		ItemModel:    itemModel,
		SkyBlockID:   skyblockIDFromItem(itemMap),
		NumericID:    numericID,
		Damage:       damage,
		Texture:      textureString(itemMap, "texture", "Texture"),
		DisplayColor: displayColor,
		Tag:          tag,
	}
}

func skullIdentityFromInput(input ItemTextureInput) string {
	if input.SkullOwner != nil {
		if id := strings.TrimSpace(input.SkullOwner.ID); id != "" {
			return "skullid:" + id
		}
		if skinHash := skullTextureHashFromOwner(input.SkullOwner); skinHash != "" {
			return "skull:" + skinHash
		}
	}

	switch tag := input.Tag.(type) {
	case *skycrypttypes.Tag:
		if tag == nil || tag.SkullOwner == nil {
			return ""
		}
		if id := strings.TrimSpace(tag.SkullOwner.ID); id != "" {
			return "skullid:" + id
		}
		if skinHash := skullTextureHashFromOwner(tag.SkullOwner); skinHash != "" {
			return "skull:" + skinHash
		}
		return ""
	case skycrypttypes.Tag:
		if tag.SkullOwner == nil {
			return ""
		}
		if id := strings.TrimSpace(tag.SkullOwner.ID); id != "" {
			return "skullid:" + id
		}
		if skinHash := skullTextureHashFromOwner(tag.SkullOwner); skinHash != "" {
			return "skull:" + skinHash
		}
		return ""
	case skycrypttypes.TextureItemExtraAttributes:
		if tag.SkullOwner == nil {
			return ""
		}
		if id := strings.TrimSpace(tag.SkullOwner.ID); id != "" {
			return "skullid:" + id
		}
		if skinHash := skullTextureHashFromOwner(tag.SkullOwner); skinHash != "" {
			return "skull:" + skinHash
		}
		return ""
	case *skycrypttypes.TextureItemExtraAttributes:
		if tag == nil || tag.SkullOwner == nil {
			return ""
		}
		if id := strings.TrimSpace(tag.SkullOwner.ID); id != "" {
			return "skullid:" + id
		}
		if skinHash := skullTextureHashFromOwner(tag.SkullOwner); skinHash != "" {
			return "skull:" + skinHash
		}
		return ""
	}

	if _, ok := input.Tag.(map[string]any); ok {
		if tagID := skullOwnerIDFromTag(input.Tag); tagID != "" {
			return "skullid:" + tagID
		}
		if skinHash := skullTextureHashFromTag(input.Tag); skinHash != "" {
			return "skull:" + skinHash
		}
	}
	return ""
}

func skullTextureHashFromOwner(skullOwner *skycrypttypes.SkullOwner) string {
	if skullOwner == nil || len(skullOwner.Properties.Textures) == 0 {
		return ""
	}
	return utility.GetSkinHash(skullOwner.Properties.Textures[0].Value)
}

func skullFallbackTexture(input ItemTextureInput, textureCtx TextureApplyContext) AppliedItemTexture {
	if texture := headTextureURL(input.Texture); texture.Texture != "" {
		return texture
	}
	if skinHash := skullTextureHashFromOwner(input.SkullOwner); skinHash != "" {
		return AppliedItemTexture{Texture: fmt.Sprintf("%s/api/head/%s", textureCtx.Domain, skinHash)}
	}
	if skinHash := skullTextureHashFromTag(input.Tag); skinHash != "" {
		return AppliedItemTexture{Texture: fmt.Sprintf("%s/api/head/%s", textureCtx.Domain, skinHash)}
	}
	return AppliedItemTexture{}
}

const textureDecisionSampleLimit = 50

func textureDebugTraceEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("ITEM_PROCESSING_DEBUG")), "trace")
}

func addTextureDecisionSample(stats *TextureApplyStats, sample TextureDecisionSample) {
	if stats == nil {
		return
	}
	if len(stats.Samples) < textureDecisionSampleLimit {
		stats.Samples = append(stats.Samples, sample)
		return
	}

	shortestIndex := 0
	for index := 1; index < len(stats.Samples); index++ {
		if stats.Samples[index].Duration < stats.Samples[shortestIndex].Duration {
			shortestIndex = index
		}
	}
	if sample.Duration > stats.Samples[shortestIndex].Duration {
		stats.Samples[shortestIndex] = sample
	}
}

func recordTextureDecision(stats *TextureApplyStats, input ItemTextureInput, texture AppliedItemTexture, reason string, stableKey string, duration time.Duration) {
	if stats == nil {
		return
	}

	sample := TextureDecisionSample{
		SkyBlockID:  strings.TrimSpace(input.SkyBlockID),
		MinecraftID: normalizeMinecraftItemID(input.ID),
		ItemModel:   normalizeMinecraftItemID(input.ItemModel),
		Texture:     texture.Texture,
		TexturePack: texture.TexturePack,
		StableKey:   stableKey,
		Reason:      reason,
		Duration:    duration,
	}

	if !strings.HasPrefix(reason, "cache_hit") || duration >= time.Millisecond {
		addTextureDecisionSample(stats, sample)
	}

	if textureDebugTraceEnabled() {
		fmt.Printf(
			"[TEXTURE_DEBUG] reason=%s skyblock_id=%q minecraft_id=%q item_model=%q texture_pack=%q duration=%s texture=%q\n",
			reason,
			sample.SkyBlockID,
			sample.MinecraftID,
			sample.ItemModel,
			sample.TexturePack,
			duration,
			sample.Texture,
		)
	}
}

func recordTextureCacheHit(stats *TextureApplyStats, detail textureCacheLookupDetail) {
	if stats == nil {
		return
	}
	stats.CacheHits++
	switch detail.Reason {
	case "stable_skyblock_cache":
		stats.StableSkyBlockHits++
	case "stable_key_cache":
		stats.StableKeyHits++
	case "legacy_skyblock_cache":
		stats.LegacySkyBlockHits++
	case "raw_map_cache":
		stats.RawMapHits++
	}
}

func recordRuntimeRenderSkip(stats *TextureApplyStats, renderer *mr.Renderer, textureCtx TextureApplyContext, isGenericSkullInput bool) {
	if stats == nil {
		return
	}
	stats.RuntimeRenderSkipped++
	if textureCtx.DisableRuntimeRender {
		stats.RuntimeRenderSkippedDisabled++
		return
	}
	if renderer == nil {
		stats.RuntimeRenderSkippedRendererNil++
		return
	}
	if len(textureCtx.EnabledPackIDs) == 0 {
		stats.RuntimeRenderSkippedNoPacks++
		return
	}
	if isGenericSkullInput {
		stats.RuntimeRenderSkippedGenericSkull++
	}
}

func skullOwnerIDFromTag(tag any) string {
	if tag == nil {
		return ""
	}
	tagMap, ok := tag.(map[string]any)
	if !ok {
		normalized, ok := normalizeTextureItem(tag)
		if !ok {
			return ""
		}
		tagMap = normalized
	}
	skullOwner, ok := textureMap(tagMap, "SkullOwner", "skullOwner", "skull_owner")
	if !ok {
		return ""
	}
	return textureString(skullOwner, "Id", "ID", "id")
}

func skullTextureHashFromTag(tag any) string {
	if tag == nil {
		return ""
	}
	tagMap, ok := tag.(map[string]any)
	if !ok {
		normalized, ok := normalizeTextureItem(tag)
		if !ok {
			return ""
		}
		tagMap = normalized
	}
	return skullTextureHashFromValues(tagMap)
}

func ApplyTextureInput(input ItemTextureInput, textureCtx TextureApplyContext) AppliedItemTexture {
	timeNow := time.Now()
	textureCtx = normalizeTextureApplyContext(textureCtx)
	stats := textureCtx.Stats
	if stats != nil {
		stats.Total++
	}

	finish := func(reason string, stableKey string, texture AppliedItemTexture) AppliedItemTexture {
		duration := time.Since(timeNow)
		if stats != nil {
			stats.TotalDuration += duration
		}
		recordTextureDecision(stats, input, texture, reason, stableKey, duration)
		return texture
	}

	cacheStart := time.Now()
	cachedTexture, ok, cacheDetail := cachedTextureForInputDetailed(input, textureCtx)
	cacheDuration := time.Since(cacheStart)
	if stats != nil {
		stats.CacheDuration += cacheDuration
	}
	if ok {
		recordTextureCacheHit(stats, cacheDetail)
		return finish("cache_hit:"+cacheDetail.Reason, cacheDetail.StableKey, cachedTexture)
	}
	if stats != nil {
		stats.CacheMisses++
	}

	fallbackStart := time.Now()
	finishFallback := func(reason string, stableKey string, texture AppliedItemTexture) AppliedItemTexture {
		if stats != nil {
			stats.FallbackDuration += time.Since(fallbackStart)
		}
		return finish(reason, stableKey, texture)
	}

	skullFallback := skullFallbackTexture(input, textureCtx)
	id := normalizeMinecraftItemID(input.ID)
	if id == "" && input.ItemModel != "" {
		id = normalizeMinecraftItemID(input.ItemModel)
	}
	isGenericSkullInput := skullFallback.Texture != "" && strings.TrimSpace(input.SkyBlockID) == "" && id == "minecraft:player_head"

	itemMap := map[string]any(nil)
	var renderErr error
	renderer := SkyCryptRender
	canRuntimeRender := renderer != nil && len(textureCtx.EnabledPackIDs) > 0 && !isGenericSkullInput && !textureCtx.DisableRuntimeRender
	if !canRuntimeRender {
		recordRuntimeRenderSkip(stats, renderer, textureCtx, isGenericSkullInput)
	}
	if canRuntimeRender {
		itemMap = itemTextureInputRenderMap(input)
		renderStart := time.Now()
		customTexture, err := renderer.RenderItemNBTWithPackIDs(itemMap, textureCtx.EnabledPackIDs)
		renderDuration := time.Since(renderStart)
		if stats != nil {
			stats.RenderAttempts++
			stats.RenderDuration += renderDuration
		}
		if err == nil && customTexture != nil && customTexture.Path != "" {
			outputTexture := AppliedItemTexture{
				Texture:     publicCacheTextureURL(customTexture.Path),
				TexturePack: customTexture.TexturePackID,
			}
			if !texturePackDisabled(outputTexture.TexturePack, textureCtx.DisabledPacks) {
				if skullFallback.Texture == "" || outputTexture.TexturePack != "" && outputTexture.TexturePack != "vanilla" {
					if stats != nil {
						stats.RenderHits++
					}
					setCachedTextureForInput(input, textureCtx, outputTexture)
					return finishFallback("runtime_render_hit", "", outputTexture)
				}
			}
		} else if err != nil {
			if stats != nil {
				stats.RenderErrors++
			}
			renderErr = err
		}

		if renderErr != nil && id != "" && vanillaItemResourceExists(id) {
			if vanillaItem := vanillaRenderItem(itemMap, id); vanillaItem != nil {
				renderStart = time.Now()
				customTexture, err = renderer.RenderItemNBTWithPackIDs(vanillaItem, textureCtx.EnabledPackIDs)
				renderDuration = time.Since(renderStart)
				if stats != nil {
					stats.RenderAttempts++
					stats.RenderDuration += renderDuration
				}
				if err == nil && customTexture != nil && customTexture.Path != "" {
					outputTexture := AppliedItemTexture{
						Texture:     publicCacheTextureURL(customTexture.Path),
						TexturePack: customTexture.TexturePackID,
					}
					if !texturePackDisabled(outputTexture.TexturePack, textureCtx.DisabledPacks) {
						if skullFallback.Texture == "" || outputTexture.TexturePack != "" && outputTexture.TexturePack != "vanilla" {
							if stats != nil {
								stats.RenderHits++
							}
							setCachedTextureForInput(input, textureCtx, outputTexture)
							return finishFallback("runtime_vanilla_render_hit", "", outputTexture)
						}
					}
				} else if err != nil {
					if stats != nil {
						stats.RenderErrors++
					}
					renderErr = err
				}
			}
		}
	}

	if skullFallback.Texture != "" {
		if stats != nil {
			stats.SkullFallbacks++
			if strings.Contains(skullFallback.Texture, "/api/head/") {
				stats.HeadFallbacks++
			}
		}
		return finishFallback("skull_head_fallback", "", skullFallback)
	}

	if input.NumericID >= 298 && input.NumericID <= 301 {
		armorType := constants.ARMOR_TYPES[input.NumericID-298]
		armorColor := ""
		if input.DisplayColor != 0 {
			armorColor = fmt.Sprintf("%06X", input.DisplayColor)
		}
		if armorColor == "" && input.SkyBlockID != "" {
			if item, ok := constants.GetItem(input.SkyBlockID); ok && item.Color != "" {
				defaultHexColor := item.Color
				armorColor = defaultHexColor
			}
		}
		if armorColor == "" {
			armorColor = "A06540"
		}
		texture := AppliedItemTexture{Texture: fmt.Sprintf("%s/api/leather/%s/%s", textureCtx.Domain, armorType, armorColor)}
		if stats != nil {
			stats.LeatherFallbacks++
		}
		return finishFallback("leather_fallback", "", texture)
	}

	if input.NumericID > 0 && shouldUseLegacyNumericFallback(id) {
		textureID := constants.GetVanillaItemId(constants.ItemModel{
			NumericId:  input.NumericID,
			ItemDamage: input.Damage,
		})
		if textureID != "" && "minecraft:"+textureID != id {
			if stats != nil {
				stats.NumericFallbacks++
			}
			input.ID = "minecraft:" + textureID
			input.NumericID = 0
			texture := ApplyTextureInput(input, textureCtx)
			return finishFallback("numeric_fallback", "", texture)
		}
	}

	if vanillaTexture := vanillaTextureURL(id); vanillaTexture.Texture != "" {
		if stats != nil {
			stats.VanillaFallbacks++
			stats.VanillaTextureFallbacks++
		}
		return finishFallback("vanilla_texture_fallback", "", vanillaTexture)
	}
	if vanillaTexture := vanillaModelTextureURL(id); vanillaTexture.Texture != "" {
		if stats != nil {
			stats.VanillaFallbacks++
			stats.VanillaModelFallbacks++
		}
		return finishFallback("vanilla_model_fallback", "", vanillaTexture)
	}
	if vanillaTexture := vanillaSpecialHeadTextureURL(id); vanillaTexture.Texture != "" {
		if stats != nil {
			stats.VanillaFallbacks++
			stats.VanillaModelFallbacks++
		}
		return finishFallback("vanilla_special_head_fallback", "", vanillaTexture)
	}

	if renderErr != nil {
		fmt.Printf("[CUSTOM_RESOURCES] RenderItemNBT failed id=%q skyblock_id=%q error=%v\n", id, input.SkyBlockID, renderErr)
	}

	fmt.Printf("[CUSTOM_RESOURCES] No texture found for item id=%q skyblock_id=%q item_id=%d damage=%d\n", id, input.SkyBlockID, input.NumericID, input.Damage)
	texture := AppliedItemTexture{Texture: barrierTextureURL()}
	if stats != nil {
		stats.BarrierFallbacks++
	}
	return finishFallback("barrier_fallback", "", texture)
}

func ApplyTexture(item any, disabledPacksParam ...[]string) AppliedItemTexture {
	timeNow := time.Now()
	textureCtx := NewTextureApplyContext(disabledPacksParam...)
	stats := textureCtx.Stats
	disabledPacks := textureCtx.DisabledPacks
	if itemMap, ok := item.(map[string]any); ok {
		input := itemTextureInputFromMap(itemMap)
		if skullIdentityFromInput(input) == "" {
			if cachedTexture, ok := cachedTextureFromRawMap(itemMap, disabledPacks); ok {
				if stats != nil {
					stats.Total++
					recordTextureCacheHit(stats, textureCacheLookupDetail{Reason: "raw_map_cache"})
					duration := time.Since(timeNow)
					stats.TotalDuration += duration
					stats.CacheDuration += duration
					recordTextureDecision(stats, input, cachedTexture, "cache_hit:raw_map_cache", "", duration)
				}
				return cachedTexture
			}
		}
		cacheStart := time.Now()
		cachedTexture, ok, cacheDetail := cachedTextureForInputDetailed(input, textureCtx)
		cacheDuration := time.Since(cacheStart)
		if stats != nil {
			stats.CacheDuration += cacheDuration
		}
		if ok {
			if stats != nil {
				stats.Total++
				recordTextureCacheHit(stats, cacheDetail)
				duration := time.Since(timeNow)
				stats.TotalDuration += duration
				recordTextureDecision(stats, input, cachedTexture, "cache_hit:"+cacheDetail.Reason, cacheDetail.StableKey, duration)
			}
			return cachedTexture
		}
		return ApplyTextureInput(input, textureCtx)
	}

	itemMap, ok := normalizeTextureItem(item)
	if !ok {
		return AppliedItemTexture{Texture: barrierTextureURL()}
	}
	return ApplyTextureInput(itemTextureInputFromMap(itemMap), textureCtx)
}
