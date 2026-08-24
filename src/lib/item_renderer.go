package lib

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"
)

type RedirectError struct {
	URL string
}

func (e RedirectError) Error() string {
	return "redirect:" + e.URL
}

func RenderItem(itemID string, enabledPacks []string, returnBarrierIfNone bool) ([]byte, error) {
	appliedTexture, err := ResolveItemTexture(itemID, enabledPacks, returnBarrierIfNone)
	if err != nil {
		return nil, err
	}
	return resolveRenderedItemTexture(appliedTexture, returnBarrierIfNone)
}

func ResolveItemTexture(itemID string, enabledPacks []string, returnBarrierIfNone bool) (AppliedItemTexture, error) {
	itemID, damage := parseItemIdentifier(itemID)
	if itemID == "" {
		return AppliedItemTexture{}, fmt.Errorf("couldn't find the texture")
	}
	cacheKey := itemTextureResolutionCacheKey(itemID, damage, enabledPacks, returnBarrierIfNone)
	return resolveItemTextureSingleflight(cacheKey, func() (AppliedItemTexture, error) {
		return resolveItemTextureUncached(itemID, damage, enabledPacks, returnBarrierIfNone)
	})
}

func itemTextureResolutionCacheKey(itemID string, damage int, enabledPacks []string, returnBarrierIfNone bool) string {
	packSignature := packSignatureFromPackIDs(NormalizeEnabledPacks(enabledPacks))
	return fmt.Sprintf("%s|damage:%d|packs:%s|barrier:%t", strings.ToLower(strings.TrimSpace(itemID)), damage, packSignature, returnBarrierIfNone)
}

func resolveItemTextureSingleflight(cacheKey string, resolve func() (AppliedItemTexture, error)) (AppliedItemTexture, error) {
	if cached, ok := loadResolvedItemTexture(cacheKey); ok {
		return cached, nil
	}

	result, err, _ := itemTextureResolutionGroup.Do(cacheKey, func() (any, error) {
		if cached, ok := loadResolvedItemTexture(cacheKey); ok {
			return cached, nil
		}
		texture, err := resolve()
		if err != nil {
			return AppliedItemTexture{}, err
		}
		storeResolvedItemTexture(cacheKey, texture)
		return texture, nil
	})
	if err != nil {
		return AppliedItemTexture{}, err
	}
	return result.(AppliedItemTexture), nil
}

func loadResolvedItemTexture(cacheKey string) (AppliedItemTexture, bool) {
	resolvedItemTextureCacheMu.RLock()
	texture, ok := resolvedItemTextureCache[cacheKey]
	resolvedItemTextureCacheMu.RUnlock()
	return texture, ok
}

func storeResolvedItemTexture(cacheKey string, texture AppliedItemTexture) {
	resolvedItemTextureCacheMu.Lock()
	resolvedItemTextureCache[cacheKey] = texture
	resolvedItemTextureCacheMu.Unlock()
}

func clearResolvedItemTextureCache() {
	resolvedItemTextureCacheMu.Lock()
	resolvedItemTextureCache = make(map[string]AppliedItemTexture)
	resolvedItemTextureCacheMu.Unlock()
}

func resolveItemTextureUncached(itemID string, damage int, enabledPacks []string, returnBarrierIfNone bool) (AppliedItemTexture, error) {
	customItem, modelItem, err := itemTextureCandidates(itemID, damage)
	if err != nil {
		return AppliedItemTexture{}, err
	}

	if modelItem != nil {
		if texture := ApplyTexture(modelItem, enabledPacks); usableItemTexture(texture, false) {
			return texture, nil
		}
	}

	texture := ApplyTexture(customItem, enabledPacks)
	if !usableItemTexture(texture, returnBarrierIfNone) {
		return AppliedItemTexture{}, fmt.Errorf("couldn't find the texture")
	}
	return texture, nil
}

func parseItemIdentifier(itemID string) (string, int) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" || strings.HasPrefix(strings.ToLower(itemID), "minecraft:") {
		return itemID, 0
	}

	splitIndex := strings.LastIndex(itemID, ":")
	if splitIndex <= 0 || splitIndex == len(itemID)-1 {
		return itemID, 0
	}

	damage, err := utility.ParseInt(itemID[splitIndex+1:])
	if err != nil {
		return itemID, 0
	}
	return strings.TrimSpace(itemID[:splitIndex]), damage
}

func itemTextureCandidates(itemID string, damage int) (map[string]any, map[string]any, error) {
	itemData, isSkyBlockItem := constants.GetItem(strings.ToUpper(itemID))
	if isSkyBlockItem && itemData.SkyblockID != "" {
		customItem, modelItem := skyBlockTextureCandidates(itemData, damage)
		return customItem, modelItem, nil
	}
	if neuItem, err := notenoughupdates.GetItem(strings.ToUpper(itemID)); err == nil {
		customItem, modelItem := neuTextureCandidates(itemID, neuItem, damage)
		return customItem, modelItem, nil
	}

	customItem, err := vanillaTextureCandidate(itemID, damage)
	return customItem, nil, err
}

func neuTextureCandidates(itemID string, neuItem models.NEUItem, damage int) (map[string]any, map[string]any) {
	itemDamage := neuItem.Damage
	if damage != 0 {
		itemDamage = damage
	}

	skyBlockID := strings.TrimSpace(neuItem.NEUId)
	if skyBlockID == "" {
		skyBlockID = strings.ToUpper(strings.TrimSpace(itemID))
	}

	tag := neuItem.NBT.ToMap()
	itemModel := strings.TrimSpace(neuItem.NBT.ItemModel)
	minecraftID, numericID := neuMinecraftItemID(neuItem.MinecraftId, itemModel, itemDamage)

	customItem := map[string]any{
		"id":          minecraftID,
		"tag":         tagWithoutItemModel(tag),
		"damage":      itemDamage,
		"item_id":     numericID,
		"skyblock_id": skyBlockID,
	}
	if itemModel == "" {
		return customItem, nil
	}

	return customItem, map[string]any{
		"id":          normalizeMinecraftItemID(itemModel),
		"ItemModel":   itemModel,
		"tag":         tag,
		"damage":      itemDamage,
		"item_id":     numericID,
		"skyblock_id": skyBlockID,
	}
}

func neuMinecraftItemID(minecraftID string, itemModel string, damage int) (string, int) {
	normalizedID := normalizeMinecraftItemID(minecraftID)
	lookupID := strings.ToUpper(strings.TrimPrefix(normalizedID, "minecraft:"))
	if lookupID == "SKULL" {
		lookupID = "SKULL_ITEM"
	}
	numericID := constants.BUKKIT_TO_ID[lookupID]
	if numericID != 0 {
		if mappedID := constants.GetVanillaItemId(constants.ItemModel{NumericId: numericID, ItemDamage: damage}); mappedID != "" {
			normalizedID = "minecraft:" + mappedID
		}
	}
	if normalizedID == "" && strings.TrimSpace(itemModel) != "" {
		normalizedID = normalizeMinecraftItemID(itemModel)
	}
	return normalizedID, numericID
}

func skyBlockTextureCandidates(itemData models.ProcessedHypixelItem, damage int) (map[string]any, map[string]any) {
	itemDamage := itemData.Damage
	if damage != 0 {
		itemDamage = damage
	}

	tag := any(map[string]any{"ExtraAttributes": map[string]any{"id": itemData.SkyblockID}})
	neuItemModel := ""
	if neuItem, err := notenoughupdates.GetItem(itemData.SkyblockID); err == nil {
		tag = neuItem.NBT.ToMap()
		neuItemModel = neuItem.NBT.ItemModel
	}

	vanillaItemID := constants.GetVanillaItemId(constants.ItemModel{NumericId: itemData.ItemId, ItemDamage: itemDamage})
	if vanillaItemID == "" {
		vanillaItemID = strings.ToLower(itemData.Material)
	}

	customItem := map[string]any{
		"id":          "minecraft:" + vanillaItemID,
		"tag":         tagWithoutItemModel(tag),
		"damage":      itemDamage,
		"item_id":     itemData.ItemId,
		"texture":     itemData.TextureId,
		"skyblock_id": itemData.SkyblockID,
	}

	itemModel := preferredItemModel(itemData.ItemModel, neuItemModel)
	if itemModel == "" {
		return customItem, nil
	}
	return customItem, map[string]any{
		"id":          normalizeMinecraftItemID(itemModel),
		"ItemModel":   itemModel,
		"tag":         tag,
		"damage":      itemDamage,
		"item_id":     itemData.ItemId,
		"texture":     itemData.TextureId,
		"skyblock_id": itemData.SkyblockID,
	}
}

func vanillaTextureCandidate(itemID string, damage int) (map[string]any, error) {
	vanillaID := strings.TrimPrefix(strings.ToLower(itemID), "minecraft:")
	if vanillaID == "" {
		return nil, fmt.Errorf("couldn't find the texture")
	}

	numericID := constants.BUKKIT_TO_ID[strings.ToUpper(vanillaID)]
	if numericID != 0 {
		mappedID := constants.GetVanillaItemId(constants.ItemModel{NumericId: numericID, ItemDamage: damage, ItemId: vanillaID})
		if mappedID != "" {
			vanillaID = mappedID
		}
	} else if !vanillaItemResourceExists(vanillaID) {
		return nil, fmt.Errorf("couldn't find the texture")
	}

	return map[string]any{
		"id":      "minecraft:" + vanillaID,
		"tag":     map[string]any{"ExtraAttributes": map[string]any{"id": ""}},
		"damage":  damage,
		"item_id": numericID,
	}, nil
}

func usableItemTexture(texture AppliedItemTexture, returnBarrierIfNone bool) bool {
	return texture.Texture != "" && (returnBarrierIfNone || !strings.HasSuffix(texture.Texture, "/barrier.png"))
}

func preferredItemModel(hypixelItemModel string, neuItemModel string) string {
	if itemModel := strings.TrimSpace(hypixelItemModel); itemModel != "" {
		return itemModel
	}
	return strings.TrimSpace(neuItemModel)
}

func tagWithoutItemModel(tag any) any {
	tagMap, ok := normalizeTextureItem(tag)
	if !ok {
		return tag
	}
	delete(tagMap, "ItemModel")
	delete(tagMap, "item_model")
	delete(tagMap, "itemModel")
	return tagMap
}

func resolveRenderedItemTexture(appliedTexture AppliedItemTexture, returnBarrierIfNone bool) ([]byte, error) {
	if !usableItemTexture(appliedTexture, returnBarrierIfNone) {
		return nil, fmt.Errorf("couldn't find the texture")
	}

	if strings.Contains(appliedTexture.Texture, "/api/") {
		return nil, RedirectError{URL: appliedTexture.Texture}
	}

	if strings.Contains(appliedTexture.Texture, "cache/") || strings.Contains(appliedTexture.Texture, "assets/") {
		texturePath, local, err := localStaticTexturePath(appliedTexture.Texture)
		if err != nil {
			return nil, err
		}
		if !local {
			return nil, RedirectError{URL: appliedTexture.Texture}
		}

		textureBytes, err := os.ReadFile(texturePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read texture %s: %w", texturePath, err)
		}
		return textureBytes, nil
	}

	return nil, RedirectError{URL: fmt.Sprintf("%s/assets/resourcepacks/Vanilla/assets/minecraft/textures/item/barrier.png", utility.GetDomain())}
}

func localStaticTexturePath(texture string) (string, bool, error) {
	parsedTexture, err := url.Parse(strings.TrimSpace(texture))
	if err != nil {
		return "", false, fmt.Errorf("failed to parse texture path %q: %w", texture, err)
	}
	if parsedTexture.IsAbs() && parsedTexture.Host != "" {
		localDomain, err := url.Parse(utility.GetDomain())
		if err != nil {
			return "", false, fmt.Errorf("failed to parse local domain %q: %w", utility.GetDomain(), err)
		}
		if localDomain.Host != "" && !strings.EqualFold(parsedTexture.Host, localDomain.Host) {
			return "", false, nil
		}
	}

	texturePath := parsedTexture.Path
	if texturePath == "" {
		texturePath = texture
	}
	texturePath = filepath.ToSlash(texturePath)
	pathIsFilesystemAbsolute := !parsedTexture.IsAbs() && filepath.IsAbs(filepath.FromSlash(texturePath))

	for _, root := range []string{"cache", "assets"} {
		localPath, keepAbsolute := staticPathFromRoot(texturePath, root, pathIsFilesystemAbsolute)
		if localPath == "" {
			continue
		}
		if keepAbsolute {
			return filepath.Clean(filepath.FromSlash(texturePath)), true, nil
		}

		appRoot, err := appRootDir()
		if err != nil {
			return "", false, err
		}
		return filepath.Join(appRoot, filepath.FromSlash(localPath)), true, nil
	}
	return "", false, nil
}

func staticPathFromRoot(texturePath string, root string, pathIsFilesystemAbsolute bool) (string, bool) {
	rootPath := "/" + root
	normalizedPath := path.Clean("/" + strings.TrimLeft(texturePath, "/"))
	if normalizedPath == rootPath || strings.HasPrefix(normalizedPath, rootPath+"/") {
		return strings.TrimPrefix(normalizedPath, "/"), false
	}

	rootSegment := "/" + root + "/"
	rootIndex := strings.Index(normalizedPath, rootSegment)
	if rootIndex < 0 {
		return "", false
	}

	candidatePath := path.Clean("/" + normalizedPath[rootIndex+1:])
	if candidatePath == rootPath || strings.HasPrefix(candidatePath, rootPath+"/") {
		return strings.TrimPrefix(candidatePath, "/"), pathIsFilesystemAbsolute
	}
	return "", false
}
