package data

import (
	"fmt"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/assets"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/global"
	"image/color"
	"os"
	"path/filepath"
	"strings"
)

var PreferredVariantKeys = []string{
	"",
	"inventory",
	"normal",
	"facing=north",
	"north=true",
	"axis=y",
	"half=lower",
	"type=bottom",
	"part=base",
}

var TexturePreferenceOrder = []string{
	"all",
	"layer0",
	"texture",
	"side",
	"top",
	"bottom",
	"front",
	"back",
	"north",
	"south",
	"east",
	"west",
	"up",
	"down",
	"particle",
}

type ItemDefinitionEntry struct {
	Name           string
	ModelReference string
	LayerTints     map[int]ItemTintInfo
	Selector       *ItemModelSelector
}

type MinecraftAssetLoader struct{}

func (m *MinecraftAssetLoader) LoadModelDefinitions(assetsRoot string, overlayRoots *[]string, assetNamespaces *assets.AssetNamespaceRegistry) map[string]BlockModelDefinition {

	// fmt.Printf("assetsRoot: %s, \noverlayRoots: %+v, \nassetNamespaces: %+v\n", assetsRoot, overlayRoots, assetNamespaces)
	namespaceRoots := BuildNamespaceRootList(assetsRoot, overlayRoots, assetNamespaces, "minecraft", true)
	definitions := make(map[string]BlockModelDefinition)

	hasAnyModels := false
	for _, namespaceRoot := range namespaceRoots {
		// fmt.Printf("Processing namespace root: %s (namespace: %s, source: %s)\n", namespaceRoot.Path, namespaceRoot.Namespace, namespaceRoot.SourceId)
		provider := *namespaceRoot.Provider
		if provider == nil {
			fmt.Printf("Warning: No resource provider found for namespace root at path '%s'. Skipping.\n", namespaceRoot.Path)
			continue
		}

		for _, modelDir := range m.EnumerateModelDirectoryNames(provider) {
			hasAnyModels = true
			emuratedFiles, err := provider.EnumerateFiles(modelDir, "*.json", true)
			if err != nil {
				fmt.Printf("Error enumerating files in directory %s: %v\n", modelDir, err)
				continue
			}

			for _, file := range emuratedFiles {
				relativePath := assets.ResourceProviderExtensionsInstance.GetRelativePath(file, modelDir)
				key := m.NormalizeModelKey(relativePath, namespaceRoot.Namespace)
				if strings.TrimSpace(key) == "" {
					continue
				}

				jsonContent, err := assets.ResourceProviderExtensionsInstance.ReadAllText(provider, file)
				if err != nil {
					fmt.Printf("Error reading file %s: %v\n", file, err)
					continue
				}

				var definition BlockModelDefinition
				if err := global.JSON.Unmarshal([]byte(jsonContent), &definition); err != nil {
					fmt.Printf("Error deserializing JSON from file %s: %v\n", file, err)
					continue
				}

				definitions[key] = definition
			}
		}

	}

	if !hasAnyModels {
		modelsRoot := filepath.Join(assetsRoot, "models")
		panic(fmt.Sprintf("Models directory not found at '%s'.", modelsRoot))
	}

	for key, definition := range m.GetBuiltinModelDefinitions() {
		if _, exists := definitions[key]; !exists {
			definitions[key] = definition
		}
	}

	return definitions
}

func BuildNamespaceRootList(primaryRoot string, overlayRoots *[]string, assetNamespaces *assets.AssetNamespaceRegistry, namespaceName string, includeAllNamespaces bool) []*assets.AssetNamespaceRoot {
	if assetNamespaces != nil {
		var resolvedNamespaces []*assets.AssetNamespaceRoot
		if includeAllNamespaces {
			resolvedNamespaces = assetNamespaces.Roots()
			// fmt.Print("_________ Including all namespaces from registry:\n")
			// for _, root := range resolvedNamespaces {
			// 	fmt.Printf("  - %s (%s) Provider: %v %v\n", root.Namespace, root.Path, root.Provider == nil, (*root.Provider).RootPath())

			// }

		} else {
			resolvedNamespaces = assetNamespaces.ResolveRoots(namespaceName, true)
			// Console.WriteLine($"__________________Resolved {resolvedNamespaces.Count()} roots for namespace '{namespaceName}':");
			// fmt.Printf("__________________Resolved %d roots for namespace '%s':\n", len(resolvedNamespaces), namespaceName)
		}

		resolvedList := DeduplicateNamespaceRoots(resolvedNamespaces)
		if len(resolvedList) > 0 {
			// fmt.Printf("-!!!!!!!!!!!!!!!!!!!!!! Resolved namespace roots:\n")
			// for _, root := range resolvedList {
			// 	fmt.Printf("  - %s (%s)\n", root.Namespace, root.Path)
			// }

			return resolvedList
		}
	}

	dedupe := make(map[string]struct{})
	results := []*assets.AssetNamespaceRoot{}
	effectiveNamespace := namespaceName
	if effectiveNamespace == "" {
		effectiveNamespace = "minecraft"
	}

	tryAdd := func(candidate string) {
		if strings.TrimSpace(candidate) == "" {
			return
		}

		fullPath, err := filepath.Abs(candidate)
		if err != nil {
			fmt.Printf("Error resolving path %s: %v\n", candidate, err)
			return
		}
		info, err := os.Stat(fullPath)
		if err != nil || !info.IsDir() {
			return
		}

		if _, exists := dedupe[fullPath]; exists {
			return
		}

		dedupe[fullPath] = struct{}{}
		results = append(results, &assets.AssetNamespaceRoot{
			Namespace: effectiveNamespace,
			Path:      fullPath,
			SourceId:  "external",
			IsVanilla: false,
		})
	}

	tryAdd(primaryRoot)
	if overlayRoots != nil {
		for _, overlay := range *overlayRoots {
			tryAdd(overlay)
		}
	}

	return results
}

func DeduplicateNamespaceRoots(roots []*assets.AssetNamespaceRoot) []*assets.AssetNamespaceRoot {
	var results []*assets.AssetNamespaceRoot
	var seen = make(map[string]struct{})

	for _, root := range roots {
		if root.Path == "" {
			continue
		}

		path := root.Path

		namespaceKey := root.Namespace
		if namespaceKey == "" {
			namespaceKey = "minecraft"
		}

		identity := fmt.Sprintf("%s|%s", strings.ToLower(namespaceKey), strings.ToLower(path))
		if _, exists := seen[identity]; exists {
			continue
		}

		seen[identity] = struct{}{}

		var provider assets.ResourceProvider
		if root.Provider == nil {
			resourceProvider, err := assets.NewDirectoryResourceProvider(path)
			if err != nil {
				fmt.Printf("Error creating directory resource provider for path %s: %v\n", path, err)
				continue
			}
			provider = resourceProvider
		} else {
			provider = *root.Provider
		}

		results = append(results, &assets.AssetNamespaceRoot{
			Namespace: root.Namespace,
			Path:      path,
			SourceId:  root.SourceId,
			IsVanilla: root.IsVanilla,
			Provider:  &provider,
		})
	}

	return results
}

func (m *MinecraftAssetLoader) DirectoryExists(provider *assets.AssetNamespaceRoot, relativePath string) bool {
	if provider == nil {
		return false
	}

	fullPath := fmt.Sprintf("%s/%s", provider.Path, relativePath)
	info, err := os.Stat(fullPath)
	return err == nil && info.IsDir()
}

func (m *MinecraftAssetLoader) EnumerateModelDirectoryNames(provider assets.ResourceProvider) []string {
	var results []string
	if provider == nil {
		return results
	}

	if provider.DirectoryExists("models") {
		results = append(results, "models")
	}

	if provider.DirectoryExists("blockentities/blockModels") {
		results = append(results, "blockentities/blockModels")
	}

	return results
}

func (m *MinecraftAssetLoader) NormalizeModelKey(relativePath string, namespaceName string) string {
	if strings.TrimSpace(relativePath) == "" {
		return ""
	}

	normalized := strings.ReplaceAll(relativePath, "\\", "/")
	normalized = strings.TrimSpace(normalized)

	normalized = strings.TrimSuffix(normalized, ".json")

	normalized = strings.TrimLeft(normalized, "./")

	if strings.HasPrefix(normalized, "block/") {
		normalized = normalized[6:]
	} else if strings.HasPrefix(normalized, "blocks/") {
		normalized = normalized[7:]
	}

	normalized = strings.TrimLeft(normalized, "/")

	if strings.TrimSpace(namespaceName) != "" &&
		!strings.EqualFold(namespaceName, "minecraft") &&
		!strings.HasPrefix(strings.ToLower(normalized), strings.ToLower(namespaceName)+":") {
		normalized = fmt.Sprintf("%s:%s", namespaceName, normalized)
	}

	return strings.ToLower(normalized)
}

func (m *MinecraftAssetLoader) GetBuiltinModelDefinitions() map[string]BlockModelDefinition {
	return map[string]BlockModelDefinition{
		"builtin/generated": {},
		"builtin/entity":    {},
		"builtin/missing": {
			Textures: map[string]string{
				"particle": "minecraft:block/missingno",
			},
		},
	}
}

func MinecraftAssetLoaderLoadBlockInfos(assetsRoot string, modelDefinitions map[string]BlockModelDefinition, overlayRoots []string, assetNamespaces *assets.AssetNamespaceRegistry) []BlockInfo {
	if strings.TrimSpace(assetsRoot) == "" {
		panic("assetsRoot cannot be null or whitespace")
	}

	if modelDefinitions == nil {
		panic("modelDefinitions cannot be null")
	}

	namespaceRoots := BuildNamespaceRootList(assetsRoot, &overlayRoots, assetNamespaces, "minecraft", false)
	// fmt.Printf("namespaceRoots: %+v\n", namespaceRoots)
	var entries []BlockInfo

	hasAnyBlockstates := false
	for _, root := range namespaceRoots {
		provider := root.Provider
		if provider == nil {
			fmt.Printf("Warning: No resource provider found for namespace root at path '%s'. Skipping.\n", root.Path)
			continue
		}

		names := EnumerateBlockstateDirectoryNames(*provider)
		// fmt.Printf("names: %+v\n", names)
		for _, bsDir := range names {
			hasAnyBlockstates = true
			emuratedFiles, err := (*provider).EnumerateFiles(bsDir, "*.json", true)
			if err != nil {
				fmt.Printf("Error enumerating files in directory %s: %v\n", bsDir, err)
				continue
			}

			for _, file := range emuratedFiles {
				relativePath := assets.ResourceProviderExtensionsInstance.GetRelativePath(file, bsDir)
				blockName := NormalizeBlockStateName(relativePath)

				jsonContent, err := assets.ResourceProviderExtensionsInstance.ReadAllText(*provider, file)
				if err != nil {
					fmt.Printf("Error reading file %s: %v\n", file, err)
					continue
				}

				var blockStateData map[string]interface{}
				if err := global.JSON.Unmarshal([]byte(jsonContent), &blockStateData); err != nil {
					fmt.Printf("Error deserializing JSON from file %s: %v\n", file, err)
					continue
				}

				modelReference := ResolveDefaultModel(blockName, blockStateData, modelDefinitions)
				textureReference := ResolveRepresentativeTexture(modelReference, modelDefinitions)

				entries = append(entries, BlockInfo{
					Name:       blockName,
					BlockState: &blockName,
					Model:      modelReference,
					Texture:    textureReference,
				})

				// Console.WriteLine($"Loaded blockstate for '{blockName}' with model reference '{modelReference}' and texture reference '{textureReference}'.");
				// fmt.Printf("Loaded blockstate for '%s' with model reference '%s' and texture reference '%s'\n", blockName, *modelReference, *textureReference)
			}
		}
	}

	if !hasAnyBlockstates {
		blockstatesRoot := fmt.Sprintf("%s/blockstates", assetsRoot)
		panic("Blockstates directory not found at '" + blockstatesRoot + "'.")
	}

	return entries
}

func EnumerateBlockstateDirectoryNames(provider assets.ResourceProvider) []string {
	var results []string
	if provider == nil {
		fmt.Printf("Provider is nil, cannot enumerate blockstate directories.\n")
		return results
	}

	if provider.DirectoryExists("blockstates") {
		results = append(results, "blockstates")
	}

	if provider.DirectoryExists("blockentities/blockStates") {
		results = append(results, "blockentities/blockStates")
	}

	return results
}

func NormalizeBlockStateName(relativePath string) string {
	normalized := strings.ReplaceAll(relativePath, "\\", "/")
	if strings.HasSuffix(strings.ToLower(normalized), ".json") {
		normalized = normalized[:len(normalized)-5]
	}

	return strings.Trim(normalized, "/")
}

func ResolveDefaultModel(blockName string, blockStateData map[string]interface{}, modelDefinitions map[string]BlockModelDefinition) *string {
	if variants, ok := blockStateData["variants"].(map[string]interface{}); ok {
		if resolved := ResolveFromVariants(variants, modelDefinitions); resolved != nil {
			return resolved
		}
	}

	if multipart, ok := blockStateData["multipart"].([]interface{}); ok {
		if resolved := ResolveFromMultipart(blockName, multipart, modelDefinitions); resolved != nil {
			return resolved
		}
	}

	if normalized, exists := ModelExists(fmt.Sprintf("minecraft:block/%s", blockName), modelDefinitions); exists {
		formatted := FormatModelReference(fmt.Sprintf("minecraft:block/%s", blockName), normalized)
		return &formatted
	}

	if normalized, exists := ModelExists(blockName, modelDefinitions); exists {
		formatted := FormatModelReference("", normalized)
		return &formatted
	}

	return nil
}

func ResolveFromVariants(variants map[string]interface{}, modelDefinitions map[string]BlockModelDefinition) *string {
	for _, key := range PreferredVariantKeys {
		if variantElement, exists := variants[key]; exists {
			modelRef := ExtractModelReference(variantElement)
			if normalized, exists := ModelExists(modelRef, modelDefinitions); exists {
				formatted := FormatModelReference(modelRef, normalized)
				return &formatted
			}
		}
	}

	for _, propertyValue := range variants {
		modelRef := ExtractModelReference(propertyValue)
		if normalized, exists := ModelExists(modelRef, modelDefinitions); exists {
			formatted := FormatModelReference(modelRef, normalized)
			return &formatted
		}
	}

	return nil
}

func ExtractModelReference(element interface{}) string {
	switch v := element.(type) {
	case map[string]interface{}:
		if modelProperty, exists := v["model"]; exists {
			if modelStr, ok := modelProperty.(string); ok {
				return modelStr
			}
			return ExtractModelReference(modelProperty)
		}

		if baseProperty, exists := v["base"]; exists {
			if baseStr, ok := baseProperty.(string); ok {
				return baseStr
			}
		}

	case []interface{}:
		for _, entry := range v {
			candidate := ExtractModelReference(entry)
			if strings.TrimSpace(candidate) != "" {
				return candidate
			}
		}
	}

	return ""
}

func ResolveFromMultipart(blockName string, multipart []interface{}, modelDefinitions map[string]BlockModelDefinition) *string {
	for _, candidate := range EnumerateMultipartCandidates(blockName) {
		if normalized, exists := ModelExists(candidate, modelDefinitions); exists {
			formatted := FormatModelReference(candidate, normalized)
			return &formatted
		}
	}

	for _, part := range multipart {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}

		applyElement, exists := partMap["apply"]
		if !exists {
			continue
		}

		modelRef := ExtractModelReference(applyElement)
		if normalized, exists := ModelExists(modelRef, modelDefinitions); exists {
			formatted := FormatModelReference(modelRef, normalized)
			return &formatted
		}
	}

	return nil
}

func ModelExists(reference string, modelDefinitions map[string]BlockModelDefinition) (string, bool) {
	normalized := NormalizeModelReference(reference)
	if strings.TrimSpace(normalized) == "" {
		return "", false
	}

	_, exists := modelDefinitions[strings.ToLower(normalized)]
	return normalized, exists
}

func NormalizeModelReference(reference string) string {
	if strings.TrimSpace(reference) == "" {
		return ""
	}

	normalized := strings.TrimSpace(reference)
	if strings.HasPrefix(strings.ToLower(normalized), "minecraft:") {
		normalized = normalized[10:]
	}

	normalized = strings.ReplaceAll(normalized, "\\", "/")
	normalized = strings.TrimLeft(normalized, "/")

	if strings.HasPrefix(strings.ToLower(normalized), "block/") {
		normalized = normalized[6:]
	} else if strings.HasPrefix(strings.ToLower(normalized), "blocks/") {
		normalized = normalized[7:]
	}

	return normalized
}

func FormatModelReference(originalReference string, normalized string) string {
	if strings.TrimSpace(originalReference) != "" {
		return originalReference
	}

	if strings.HasPrefix(strings.ToLower(normalized), "item/") {
		return "minecraft:" + normalized
	}

	if strings.HasPrefix(strings.ToLower(normalized), "builtin/") {
		return normalized
	}

	return fmt.Sprintf("minecraft:block/%s", normalized)
}

func EnumerateMultipartCandidates(blockName string) []string {
	candidates := []string{
		fmt.Sprintf("minecraft:block/%s_inventory", blockName),
		fmt.Sprintf("minecraft:block/%s_item", blockName),
		fmt.Sprintf("minecraft:block/%s", blockName),
		fmt.Sprintf("minecraft:block/%s_post", blockName),
		fmt.Sprintf("minecraft:block/%s_center", blockName),
		fmt.Sprintf("minecraft:block/%s_side", blockName),
		fmt.Sprintf("minecraft:block/%s_floor", blockName),
		fmt.Sprintf("minecraft:block/%s_top", blockName),
		fmt.Sprintf("minecraft:block/%s_bottom", blockName),
	}

	return candidates
}

func ResolveRepresentativeTexture(modelReference *string, models map[string]BlockModelDefinition) *string {
	if modelReference == nil || strings.TrimSpace(*modelReference) == "" {
		return nil
	}

	normalized := NormalizeModelReference(*modelReference)
	if strings.TrimSpace(normalized) == "" {
		return nil
	}

	definition, exists := models[normalized]
	if !exists {
		return nil
	}

	return ResolvePrimaryTexture(definition, models)
}

func ResolvePrimaryTexture(definition BlockModelDefinition, models map[string]BlockModelDefinition) *string {
	if len(definition.Textures) > 0 {
		for _, key := range TexturePreferenceOrder {
			if value, exists := definition.Textures[key]; exists && strings.TrimSpace(value) != "" {
				return &value
			}
		}

		for _, value := range definition.Textures {
			if strings.TrimSpace(value) != "" {
				return &value
			}
		}
	}

	if definition.Parent != nil && strings.TrimSpace(*definition.Parent) != "" {
		parentKey := NormalizeModelReference(*definition.Parent)
		if strings.TrimSpace(parentKey) != "" {
			if parentDefinition, exists := models[parentKey]; exists {
				return ResolvePrimaryTexture(parentDefinition, models)
			}
		}
	}

	return nil
}

func MinecraftAssetsLoadItemInfosFrom(assetsRoot string, models map[string]BlockModelDefinition, overlayRoots []string, assetNamespaces *assets.AssetNamespaceRegistry) []ItemInfo {
	entries := make(map[string]ItemInfo)

	for key, definition := range models {
		if !strings.HasPrefix(strings.ToLower(key), "item/") {
			continue
		}

		itemName := key[5:]
		if strings.TrimSpace(itemName) == "" || IsTemplateItem(itemName) {
			continue
		}

		texture := ResolvePrimaryTexture(definition, models)
		info, exists := entries[itemName]
		if !exists {
			info = ItemInfo{Name: itemName}
		}

		info.Model = &key
		if (info.Texture == nil || strings.TrimSpace(*info.Texture) == "") && texture != nil && strings.TrimSpace(*texture) != "" {
			info.Texture = texture
		}

		entries[itemName] = info

	}

	for _, entry := range EnumerateItemDefinitions(assetsRoot, overlayRoots, assetNamespaces) {
		itemName := entry.Name
		modelReference := entry.ModelReference
		if strings.TrimSpace(itemName) == "" || IsTemplateItem(itemName) {
			continue
		}

		info, exists := entries[itemName]
		if !exists {
			info = ItemInfo{Name: itemName}
			entries[itemName] = info
		}

		if strings.TrimSpace(modelReference) != "" {
			info.Model = &modelReference

			if info.Texture == nil || strings.TrimSpace(*info.Texture) == "" {
				normalized := NormalizeModelReference(modelReference)
				if strings.TrimSpace(normalized) != "" {
					if definition, exists := models[normalized]; exists {
						texture := ResolvePrimaryTexture(definition, models)
						if texture != nil && strings.TrimSpace(*texture) != "" {
							info.Texture = texture
						}
					}
				}
			}
		}

		if entry.Selector != nil {
			info.Selector = *entry.Selector
		}

		if len(entry.LayerTints) > 0 {
			if info.LayerTints == nil {
				info.LayerTints = make(map[int]ItemTintInfo)
			}

			for layerIndex, tintInfo := range entry.LayerTints {
				info.LayerTints[layerIndex] = tintInfo
			}
		}

		entries[itemName] = info
	}

	var result []ItemInfo
	for _, info := range entries {
		result = append(result, info)
	}

	return result
}

func IsTemplateItem(itemName string) bool {
	if strings.HasPrefix(strings.ToLower(itemName), "template_") {
		return true
	}

	lower := strings.ToLower(itemName)
	return lower == "generated" || lower == "handheld" || lower == "handheld_rod" || lower == "handheld_mace"
}

func EnumerateItemDefinitions(assetsRoot string, overlayRoots []string, assetNamespaces *assets.AssetNamespaceRegistry) []ItemDefinitionEntry {
	namespaceRoots := BuildNamespaceRootList(assetsRoot, &overlayRoots, assetNamespaces, "minecraft", true)
	var entries []ItemDefinitionEntry

	for _, nsRoot := range namespaceRoots {
		provider := *nsRoot.Provider
		if provider == nil || !provider.DirectoryExists("items") {
			continue
		}

		emuratedFiles, err := provider.EnumerateFiles("items", "*.json", true)
		if err != nil {
			fmt.Printf("Error enumerating files in directory %s: %v\n", "items", err)
			continue
		}

		for _, file := range emuratedFiles {
			relativePath, err := provider.GetRelativePath(file, "items")
			if err != nil {
				fmt.Printf("Error getting relative path for file %s: %v\n", file, err)
				continue
			}

			itemName := NormalizeItemName(relativePath)
			if strings.TrimSpace(itemName) == "" {
				continue
			}

			jsonContent, err := provider.ReadAllText(file)
			if err != nil {
				fmt.Printf("Error reading file %s: %v\n", file, err)
				continue
			}

			var itemData map[string]interface{}
			if err := global.JSON.Unmarshal([]byte(jsonContent), &itemData); err != nil {
				fmt.Printf("Error deserializing JSON from file %s: %v\n", file, err)
				continue
			}

			tintMap := make(map[int]ItemTintInfo)
			ExtractTintInfoFromDefinition(itemData, tintMap)
			selector, err := ParseItemModelSelectorFromJSON([]byte(jsonContent))
			if err != nil {
				selector = nil
			}
			modelReference := ResolveModelReferenceFromItemDefinition(itemData)

			entry := ItemDefinitionEntry{
				Name:           itemName,
				ModelReference: modelReference,
				LayerTints:     tintMap,
				Selector:       &selector,
			}

			entries = append(entries, entry)
		}
	}

	return entries
}
func NormalizeItemName(relativePath string) string {
	normalized := strings.ReplaceAll(relativePath, "\\", "/")
	if strings.HasSuffix(strings.ToLower(normalized), ".json") {
		normalized = normalized[:len(normalized)-5]
	}

	return strings.Trim(normalized, "/")
}

func ExtractTintInfoFromDefinition(element interface{}, target map[int]ItemTintInfo) {
	switch v := element.(type) {
	case map[string]interface{}:
		if tintsProperty, exists := v["tints"]; exists {
			if tintsArray, ok := tintsProperty.([]interface{}); ok {
				for index, tintElement := range tintsArray {
					if _, exists := target[index]; !exists {
						tintInfo := CreateTintInfo(tintElement)
						if tintInfo != nil {
							target[index] = *tintInfo
						}
					}
				}
			}
		}

		for _, propertyValue := range v {
			if propertyValue != nil {
				ExtractTintInfoFromDefinition(propertyValue, target)
			}
		}

	case []interface{}:
		for _, entry := range v {
			ExtractTintInfoFromDefinition(entry, target)
		}

	default:
		return
	}
}

func CreateTintInfo(tintElement interface{}) *ItemTintInfo {
	tintMap, ok := tintElement.(map[string]interface{})
	if !ok {
		return nil
	}

	var tintInfo ItemTintInfo
	if typeProperty, exists := tintMap["type"]; exists {
		if typeStr, ok := typeProperty.(string); ok {
			switch strings.ToLower(typeStr) {
			case "minecraft:dye":
				tintInfo.Kind = ItemTintKindDye
			case "minecraft:constant":
				tintInfo.Kind = ItemTintKindConstant
			case "":
				tintInfo.Kind = ItemTintKindUnspecified
			default:
				tintInfo.Kind = ItemTintKindUnknown
			}
		}
	}

	switch tintInfo.Kind {
	case ItemTintKindDye:
		tintInfo.DefaultColor = TryReadColor(tintMap, "default")
	case ItemTintKindConstant:
		tintInfo.DefaultColor = TryReadColor(tintMap, "value")
	default:
		tintInfo.DefaultColor = TryReadColor(tintMap, "default")
		if tintInfo.DefaultColor == nil {
			tintInfo.DefaultColor = TryReadColor(tintMap, "value")
		}
	}

	return &tintInfo
}

func TryReadColor(element map[string]interface{}, propertyName string) *color.RGBA {
	property, exists := element[propertyName]
	if !exists {
		return nil
	}

	switch v := property.(type) {
	case float64:
		return ConvertFloat64ToRGBA(v)
	case string:
		return ParseColorString(v)
	default:
		return nil
	}
}

func ConvertFloat64ToRGBA(value float64) *color.RGBA {
	if value < 0 {
		value = 0
	} else if value > 4294967295 {
		value = 4294967295
	}

	intValue := uint32(value)
	r := uint8((intValue >> 16) & 0xFF)
	g := uint8((intValue >> 8) & 0xFF)
	b := uint8(intValue & 0xFF)
	a := uint8((intValue >> 24) & 0xFF)

	return &color.RGBA{R: r, G: g, B: b, A: a}
}

func ParseColorString(value string) *color.RGBA {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	text := strings.TrimSpace(value)
	if strings.HasPrefix(text, "#") {
		text = text[1:]
	} else if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		text = text[2:]
	}

	if len(text) != 6 && len(text) != 8 {
		return nil
	}

	var hexValue uint64
	var err error
	if hexValue, err = ParseHexStringToUint64(text); err != nil {
		return nil
	}

	r := uint8((hexValue >> 16) & 0xFF)
	g := uint8((hexValue >> 8) & 0xFF)
	b := uint8(hexValue & 0xFF)
	a := uint8(255)
	if len(text) == 8 {
		a = uint8((hexValue >> 24) & 0xFF)
	}

	return &color.RGBA{R: r, G: g, B: b, A: a}
}

func ParseHexStringToUint64(text string) (uint64, error) {
	var value uint64
	_, err := fmt.Sscanf(text, "%x", &value)
	return value, err
}

func ResolveModelReferenceFromItemDefinition(itemData map[string]interface{}) string {
	if modelProperty, exists := itemData["model"]; exists {
		reference := ExtractModelReference(modelProperty)
		if strings.TrimSpace(reference) != "" {
			return reference
		}
	}

	if components, exists := itemData["components"].(map[string]interface{}); exists {
		if componentModel, exists := components["minecraft:model"]; exists {
			reference := ExtractModelReference(componentModel)
			if strings.TrimSpace(reference) != "" {
				return reference
			}
		}
	}

	return ""
}

var MinecraftAssetLoaderInstance = &MinecraftAssetLoader{}
