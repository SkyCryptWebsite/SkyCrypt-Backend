package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skycrypt/src/utility"
)

type vanillaAssetIndex struct {
	root       string
	items      map[string]string
	models     map[string]string
	textures   map[string]string
	assetFiles map[string]struct{}
}

func currentVanillaAssetIndex() *vanillaAssetIndex {
	appRoot, err := appRootDir()
	if err != nil {
		return nil
	}
	vanillaRoot := filepath.Join(appRoot, "assets", "resourcepacks", "Vanilla", "assets", "minecraft")

	vanillaAssetsMu.RLock()
	index := vanillaAssets
	vanillaAssetsMu.RUnlock()
	if index != nil && index.root == vanillaRoot {
		return index
	}

	built := buildVanillaAssetIndex(vanillaRoot)
	vanillaAssetsMu.Lock()
	if vanillaAssets == nil || vanillaAssets.root != vanillaRoot {
		vanillaAssets = built
	} else {
		built = vanillaAssets
	}
	vanillaAssetsMu.Unlock()
	return built
}

func buildVanillaAssetIndex(root string) *vanillaAssetIndex {
	index := &vanillaAssetIndex{
		root:       root,
		items:      make(map[string]string),
		models:     make(map[string]string),
		textures:   make(map[string]string),
		assetFiles: make(map[string]struct{}),
	}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		relative = filepath.ToSlash(relative)
		index.assetFiles[relative] = struct{}{}
		withoutExtension := strings.TrimSuffix(relative, filepath.Ext(relative))
		switch {
		case strings.HasPrefix(withoutExtension, "items/") && strings.EqualFold(filepath.Ext(relative), ".json"):
			index.items[strings.TrimPrefix(withoutExtension, "items/")] = path
		case strings.HasPrefix(withoutExtension, "models/") && strings.EqualFold(filepath.Ext(relative), ".json"):
			index.models[strings.TrimPrefix(withoutExtension, "models/")] = path
		case strings.HasPrefix(withoutExtension, "textures/") && strings.EqualFold(filepath.Ext(relative), ".png"):
			index.textures[strings.TrimPrefix(withoutExtension, "textures/")] = path
		}
		return nil
	})
	return index
}

func normalizedVanillaID(id string) string {
	return strings.TrimSpace(strings.ToLower(strings.TrimPrefix(id, "minecraft:")))
}

func vanillaTextureURLFromPath(path string) AppliedItemTexture {
	if strings.TrimSpace(path) == "" {
		return AppliedItemTexture{}
	}
	index := currentVanillaAssetIndex()
	if index == nil {
		return AppliedItemTexture{}
	}
	relative, err := filepath.Rel(index.root, path)
	if err != nil {
		return AppliedItemTexture{}
	}
	return AppliedItemTexture{Texture: utility.GetDomain() + "/assets/resourcepacks/Vanilla/assets/minecraft/" + filepath.ToSlash(relative)}
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
	id = normalizedVanillaID(id)
	if id == "" {
		return AppliedItemTexture{}
	}
	index := currentVanillaAssetIndex()
	if index == nil {
		return AppliedItemTexture{}
	}
	return vanillaTextureURLFromPath(index.textures["item/"+id])
}

func vanillaAssetTextureURL(texturePath string) AppliedItemTexture {
	texturePath = normalizedVanillaID(texturePath)
	if texturePath == "" {
		return AppliedItemTexture{}
	}
	index := currentVanillaAssetIndex()
	if index == nil {
		return AppliedItemTexture{}
	}
	return vanillaTextureURLFromPath(index.textures[texturePath])
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
	id = normalizedVanillaID(id)
	if id == "" {
		return AppliedItemTexture{}
	}
	index := currentVanillaAssetIndex()
	if index == nil {
		return AppliedItemTexture{}
	}

	modelRefs := []string{}
	if itemPath := index.items[id]; itemPath != "" {
		if itemDefinition := readVanillaJSON(itemPath); itemDefinition != nil {
			if modelRef := findVanillaModelRef(itemDefinition); modelRef != "" {
				modelRefs = append(modelRefs, modelRef)
			}
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

		modelPath := index.models[modelRef]
		if modelPath == "" {
			continue
		}
		model := readVanillaJSON(modelPath)
		if model == nil {
			continue
		}
		if texture := textureFromVanillaModel(model); texture.Texture != "" {
			return texture
		}
	}

	texture := vanillaBlockTextureURL(id)
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
	id = normalizedVanillaID(id)
	if id == "" {
		return false
	}
	index := currentVanillaAssetIndex()
	if index == nil {
		return false
	}
	if _, ok := index.items[id]; ok {
		return true
	}
	if _, ok := index.models["item/"+id]; ok {
		return true
	}
	if _, ok := index.models["block/"+id]; ok {
		return true
	}
	_, ok := index.textures["item/"+id]
	return ok
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
