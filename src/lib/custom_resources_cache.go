package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"skycrypt/src/models"
	"skycrypt/src/utility"
)

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
		priority := meta.Priority

		config := models.ResourcePackConfig{
			Id:       id,
			Name:     strings.TrimSpace(meta.Name),
			Version:  strings.TrimSpace(meta.Version),
			Priority: priority,
			Author:   author,
			Url:      url,
			Icon:     fmt.Sprintf("%s%s", utility.GetDomain(), meta.Icon),
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

	return []string{}
}

const orderedPackSignaturePrefix = "enabled-v7:"

func orderedPackSignature(packIDs []string) string {
	return orderedPackSignaturePrefix + strings.Join(packIDs, ",")
}

func defaultPackSignature() string {
	return orderedPackSignature(defaultResourcePackIDs())
}

func NewTextureApplyContext(enabledPacksParam ...[]string) TextureApplyContext {
	var enabledPackIDs []string
	if len(enabledPacksParam) > 0 {
		enabledPackIDs = NormalizeEnabledPacks(enabledPacksParam[0])
	} else {
		enabledPackIDs = defaultResourcePackIDs()
	}
	signature := orderedPackSignature(enabledPackIDs)

	return TextureApplyContext{
		EnabledPackIDs: enabledPackIDs,
		EnabledPackSet: enabledPackSet(enabledPackIDs),
		PackSignature:  signature,
		Domain:         utility.GetDomain(),
		Stats:          &TextureApplyStats{},
	}
}

func normalizeTextureApplyContext(textureCtx TextureApplyContext) TextureApplyContext {
	if len(textureCtx.EnabledPackIDs) == 0 {
		textureCtx.EnabledPackIDs = defaultResourcePackIDs()
	} else {
		textureCtx.EnabledPackIDs = NormalizeEnabledPacks(textureCtx.EnabledPackIDs)
	}
	if textureCtx.EnabledPackSet == nil {
		textureCtx.EnabledPackSet = enabledPackSet(textureCtx.EnabledPackIDs)
	}
	if textureCtx.PackSignature == "" {
		textureCtx.PackSignature = orderedPackSignature(textureCtx.EnabledPackIDs)
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

func cachedItemTexture(id string, textureCtx TextureApplyContext) (AppliedItemTexture, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return AppliedItemTexture{}, false
	}

	return cachedTextureForStableKey("skyblock:"+id, textureCtx.PackSignature, textureCtx.EnabledPackIDs, textureCtx.EnabledPackSet, id)
}

func cachedTextureForStableKey(stableKey string, packSignature string, enabledPackIDs []string, enabledPacks map[string]struct{}, legacyKeys ...string) (AppliedItemTexture, bool) {
	stableKey = strings.TrimSpace(stableKey)
	if stableKey == "" || len(enabledPackIDs) == 0 {
		return AppliedItemTexture{}, false
	}

	if texture, ok := cachedTextureForStableKeyInMemory(stableKey, packSignature, enabledPackIDs, enabledPacks, legacyKeys...); ok {
		return texture, true
	}
	if lazyReloadRenderedTextureIndex() {
		if texture, ok := cachedTextureForStableKeyInMemory(stableKey, packSignature, enabledPackIDs, enabledPacks, legacyKeys...); ok {
			return texture, true
		}
	}

	return AppliedItemTexture{}, false
}

func cachedTextureForStableKeyInMemory(stableKey string, packSignature string, enabledPackIDs []string, enabledPacks map[string]struct{}, legacyKeys ...string) (AppliedItemTexture, bool) {
	seenKeys := map[string]struct{}{}
	if texture, ok := cachedTextureByKeyOnce(textureCacheKey(packSignature, stableKey), enabledPacks, seenKeys); ok {
		return texture, true
	}

	for _, packID := range enabledPackIDs {
		if texture, ok := cachedTextureByPackVariant(packID, stableKey, enabledPacks, seenKeys); ok {
			return texture, true
		}
	}

	if packSignature == defaultPackSignature() {
		for _, legacyKey := range legacyKeys {
			if texture, ok := cachedTextureByKeyOnce(legacyKey, enabledPacks, seenKeys); ok {
				return texture, true
			}
		}
	}
	return AppliedItemTexture{}, false
}

func cachedTextureByKeyOnce(key string, enabledPacks map[string]struct{}, seenKeys map[string]struct{}) (AppliedItemTexture, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return AppliedItemTexture{}, false
	}
	if _, seen := seenKeys[key]; seen {
		return AppliedItemTexture{}, false
	}
	seenKeys[key] = struct{}{}
	return cachedTextureByKey(key, enabledPacks)
}

func cachedTextureByPackVariant(packID string, stableKey string, enabledPacks map[string]struct{}, seenKeys map[string]struct{}) (AppliedItemTexture, bool) {
	packID = strings.TrimSpace(packID)
	if packID == "" {
		return AppliedItemTexture{}, false
	}

	for _, alias := range texturePackAliases(packID) {
		texture, ok := cachedTextureByKeyOnce(textureCacheKey(alias, stableKey), enabledPacks, seenKeys)
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

func cachedTextureByKey(key string, enabledPacks map[string]struct{}) (AppliedItemTexture, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return AppliedItemTexture{}, false
	}

	itemTextureCacheMu.RLock()
	texture := itemTextureCache[key]
	itemTextureCacheMu.RUnlock()

	if !validCachedTexture(texture, enabledPacks) {
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
	itemTextureCache[textureCacheKey(packSignature, stableKey)] = texture
	for _, alias := range texturePackAliases(texture.TexturePack) {
		itemTextureCache[textureCacheKey(alias, stableKey)] = texture
	}
	if skyblockID := strings.TrimPrefix(stableKey, "skyblock:"); skyblockID != stableKey && skyblockID != "" {
		itemTextureCache[skyblockID] = texture
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
	length := len(itemTextureCache)
	itemTextureCacheMu.RUnlock()
	return length
}

func cachedTextureFromRawMap(itemMap map[string]any, textureCtx TextureApplyContext) (AppliedItemTexture, bool) {
	itemModel := itemModelFromItem(itemMap)
	if itemModel != "" {
		if cachedTexture, ok := cachedItemTexture(strings.TrimPrefix(itemModel, "minecraft:"), textureCtx); ok {
			return cachedTexture, true
		}
		return AppliedItemTexture{}, false
	}

	if skyblockID := textureString(itemMap, "skyblock_id", "skyblockId", "SkyblockID"); skyblockID != "" {
		if cachedTexture, ok := cachedItemTexture(skyblockID, textureCtx); ok {
			return cachedTexture, true
		}
	}

	if id := normalizeMinecraftItemID(textureString(itemMap, "id", "ID")); id != "" {
		if cachedTexture, ok := cachedItemTexture(strings.TrimPrefix(id, "minecraft:"), textureCtx); ok {
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

func textureCacheKey(packSignature string, stableKey string) string {
	packSignature = strings.TrimSpace(packSignature)
	if packSignature == "" {
		packSignature = defaultPackSignature()
	}
	return packSignature + "|" + stableKey
}

func validCachedTexture(texture AppliedItemTexture, enabledPacks map[string]struct{}) bool {
	return texture.Texture != "" && texturePackEnabled(texture.TexturePack, enabledPacks) && !isStaleVanillaChestParticleRender(texture.Texture)
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
