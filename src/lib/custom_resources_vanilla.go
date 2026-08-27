package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skycrypt/src/utility"
)

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
	texturePath := ""
	switch id {
	case "zombie_head":
		texturePath = "entity/zombie/zombie"
	case "skeleton_skull":
		texturePath = "entity/skeleton/skeleton"
	case "wither_skeleton_skull":
		texturePath = "entity/skeleton/wither_skeleton"
	case "creeper_head":
		texturePath = "entity/creeper/creeper"
	case "dragon_head":
		return vanillaAssetTextureURL("entity/enderdragon/dragon")
	case "piglin_head":
		return vanillaAssetTextureURL("entity/piglin/piglin")
	default:
		return AppliedItemTexture{}
	}

	appRoot, err := appRootDir()
	if err != nil {
		return AppliedItemTexture{}
	}
	sourcePath := filepath.Join(appRoot, "assets", "resourcepacks", "Vanilla", "assets", "minecraft", "textures", filepath.FromSlash(texturePath+".png"))
	cacheID := "minecraft_" + id
	if rendered := RenderLocalHead(cacheID, sourcePath); len(rendered) == 0 {
		return AppliedItemTexture{}
	}
	return AppliedItemTexture{Texture: publicCacheTextureURL(filepath.Join(CACHE_DIR, "heads", cacheID+".png"))}
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
	vanillaItemExistsCacheMu.RLock()
	cached, ok := vanillaItemExistsCache[id]
	vanillaItemExistsCacheMu.RUnlock()
	if ok {
		return cached
	}
	if vanillaTextureURL(id).Texture != "" {
		storeVanillaItemResourceExists(id, true)
		return true
	}
	if vanillaModelTextureURL(id).Texture != "" {
		storeVanillaItemResourceExists(id, true)
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
			storeVanillaItemResourceExists(id, true)
			return true
		}
	}

	storeVanillaItemResourceExists(id, false)
	return false
}

func storeVanillaItemResourceExists(id string, exists bool) {
	vanillaItemExistsCacheMu.Lock()
	vanillaItemExistsCache[id] = exists
	vanillaItemExistsCacheMu.Unlock()
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
