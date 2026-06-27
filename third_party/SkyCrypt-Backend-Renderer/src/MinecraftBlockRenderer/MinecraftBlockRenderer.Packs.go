package minecraftblockrenderer

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	texturepacks "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/TexturePacks"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/assets"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var VanillaPackId = "vanilla"

var RendererVersion = "0.1.6"

type ItemRenderCapture struct {
	OriginalTarget      string
	NormalizedItemKey   string
	ItemInfo            *data.ItemInfo
	Model               *data.BlockModelInstance
	ModelCandidates     []string
	ResolvedModelName   *string
	CompositeModelNames []string
	SpecialModel        *data.ItemSpecialModelInfo
	FinalOptions        *BlockRenderOptions
}

// public ItemModelResolution? ToResolution() {
// 	if (string.IsNullOrWhiteSpace(NormalizedItemKey)) {
// 		return null;
// 	}

// 	return new ItemModelResolution(NormalizedItemKey, ItemInfo, Model, ModelCandidates, ResolvedModelName);
// }

func (r *ItemRenderCapture) ToResolution() *ItemModelResolution {
	if strings.TrimSpace(r.NormalizedItemKey) == "" {
		return nil
	}

	return &ItemModelResolution{
		LookupTarget:        r.NormalizedItemKey,
		ItemInfo:            r.ItemInfo,
		Model:               r.Model,
		ModelCandidates:     r.ModelCandidates,
		ResolvedModelName:   r.ResolvedModelName,
		CompositeModelNames: r.CompositeModelNames,
		SpecialModel:        r.SpecialModel,
	}
}

func NewItemRenderCapture(originalTarget string, normalizedItemKey string, itemInfo *data.ItemInfo, model *data.BlockModelInstance, modelCandidates []string, resolvedModelName *string, finalOptions *BlockRenderOptions) *ItemRenderCapture {
	if strings.TrimSpace(originalTarget) == "" {
		panic("originalTarget cannot be null or whitespace")
	}

	if strings.TrimSpace(normalizedItemKey) == "" {
		panic("normalizedItemKey cannot be null or whitespace")
	}

	if itemInfo == nil {
		panic("itemInfo cannot be nil")
	}

	if model == nil {
		panic("model cannot be nil")
	}

	if modelCandidates == nil {
		panic("modelCandidates cannot be nil")
	}

	return &ItemRenderCapture{
		OriginalTarget:    originalTarget,
		NormalizedItemKey: normalizedItemKey,
		ItemInfo:          itemInfo,
		Model:             model,
		ModelCandidates:   modelCandidates,
		ResolvedModelName: resolvedModelName,
		FinalOptions:      finalOptions,
	}
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) NormalizeModelIdentifier(identifier string) string {
	if identifier == "" {
		return identifier
	}

	trimmed := strings.TrimSpace(identifier)
	if strings.Contains(trimmed, ":") {
		return trimmed
	}

	trimmed = strings.TrimLeft(trimmed, "/")
	if trimmed == "" {
		return "minecraft:"
	}

	return "minecraft:" + trimmed
}

type RenderPackContext struct {
	AssetsRoot      string
	OverlayRoots    []OverlayRoot
	PackIds         []string
	PackStackHash   string
	Packs           []texturepacks.RegisteredResourcePack
	SearchRoots     []OverlaySearchRoot
	AssetNamespaces assets.AssetNamespaceRegistry
}

type OverlaySearchRoot struct {
	Path     string
	SourceId string
	Kind     string
}

func NewRenderPackContext(assetsRoot string, overlayRoots []OverlayRoot, packIds []string, packStackHash string, packs []texturepacks.RegisteredResourcePack, overrideRegistry *assets.AssetNamespaceRegistry) *RenderPackContext {
	context := &RenderPackContext{
		AssetsRoot:    assetsRoot,
		OverlayRoots:  overlayRoots,
		PackIds:       packIds,
		PackStackHash: packStackHash,
		Packs:         packs,
	}

	if overrideRegistry == nil {
		context.AssetNamespaces = *context.BuildAssetNamespaces()
	} else {
		context.AssetNamespaces = *overrideRegistry
	}

	return context

}

func (_renderPackContext *RenderPackContext) BuildAssetNamespaces() *assets.AssetNamespaceRegistry {
	registry := assets.NewAssetNamespaceRegistry()
	if strings.TrimSpace(_renderPackContext.AssetsRoot) != "" {
		RegisterNamespaceRoot(registry, "minecraft", _renderPackContext.AssetsRoot, VanillaPackId, true)
	}

	for _, overlay := range _renderPackContext.OverlayRoots {
		AddOverlayNamespaces(registry, overlay)
	}

	// Register provider-backed pack namespaces (zip-backed packs whose overlay paths
	// don't exist on the filesystem and were skipped by AddOverlayNamespaces above)
	for _, pack := range _renderPackContext.Packs {
		if pack.NamespaceProviders == nil {
			continue
		}

		RegisterProviderNamespaces(registry, pack)
	}

	return registry
}

func RegisterNamespaceRoot(registry *assets.AssetNamespaceRegistry, namespaceName string, path string, sourceId string, isVanilla bool) {
	registry.AddNamespace(namespaceName, path, sourceId, isVanilla)
	// Console.WriteLine($"----------------Registered namespace root: {namespaceName} ({path})");
	// fmt.Printf("----------------Registered namespace root: %s (%s)\n", namespaceName, path)

	texturesPath := filepath.Join(path, "textures")
	if fi, err := os.Stat(texturesPath); err == nil && fi.IsDir() {
		// Console.WriteLine($"-_---------------Registered namespace textures root: {namespaceName} ({texturesPath})");
		// fmt.Printf("-_--------------Registered namespace textures root: %s (%s)\n", namespaceName, texturesPath)

		registry.AddNamespace(namespaceName, texturesPath, sourceId, isVanilla)
	}
}

func AddOverlayNamespaces(registry *assets.AssetNamespaceRegistry, overlay OverlayRoot) {
	if strings.TrimSpace(overlay.Path) == "" {
		return
	}

	if fi, err := os.Stat(overlay.Path); err != nil || !fi.IsDir() {
		return
	}

	normalized := overlay.Path
	assetsDirectory := filepath.Join(normalized, "assets")
	if fi, err := os.Stat(assetsDirectory); err == nil && fi.IsDir() {
		files, err := os.ReadDir(assetsDirectory)
		if err == nil {
			for _, file := range files {
				if file.IsDir() {
					namespaceName := file.Name()
					RegisterNamespaceRoot(registry, namespaceName, filepath.Join(assetsDirectory, namespaceName), overlay.SourceId, overlay.Kind == "vanilla")
				}
			}
		}
		return
	}

	if strings.EqualFold(filepath.Base(filepath.Dir(normalized)), "assets") {
		namespaceName := filepath.Base(normalized)
		RegisterNamespaceRoot(registry, namespaceName, normalized, overlay.SourceId, overlay.Kind == "vanilla")
		return
	}

	if fi, err := os.Stat(filepath.Join(normalized, "models")); err == nil && fi.IsDir() {
		registry.AddNamespace("minecraft", normalized, overlay.SourceId, overlay.Kind == "vanilla")
		return
	}
	if fi, err := os.Stat(filepath.Join(normalized, "textures")); err == nil && fi.IsDir() {
		registry.AddNamespace("minecraft", normalized, overlay.SourceId, overlay.Kind == "vanilla")
		return
	}

	registry.AddNamespace("minecraft", normalized, overlay.SourceId, overlay.Kind == "vanilla")
}

func RegisterProviderNamespaces(registry *assets.AssetNamespaceRegistry, pack texturepacks.RegisteredResourcePack) {
	if pack.NamespaceProviders != nil {
		for namespaceName, nsProvider := range pack.NamespaceProviders {
			displayPath := pack.RootPath
			registry.AddNamespaceWithProvider(namespaceName, displayPath, pack.Id, false, nsProvider)

			if nsProvider.DirectoryExists("textures") {
				texturesProvider := assets.NewSubPathResourceProvider(nsProvider, "textures")
				registry.AddNamespaceWithProvider(namespaceName, displayPath+"/textures", pack.Id, false, texturesProvider)
			}
		}
	}

	// Register catharsis overlay namespace providers (higher priority, registered after base)
	if pack.OverlayNamespaceProviders != nil {
		for _, overlay := range pack.OverlayNamespaceProviders {
			namespaceName := overlay.Namespace
			displayPath := overlay.DisplayPath
			nsProvider := overlay.Provider

			registry.AddNamespaceWithProvider(namespaceName, displayPath, pack.Id, false, nsProvider)

			if nsProvider.DirectoryExists("textures") {
				texturesProvider := assets.NewSubPathResourceProvider(nsProvider, "textures")
				registry.AddNamespaceWithProvider(namespaceName, displayPath+"/textures", pack.Id, false, texturesProvider)
			}
		}
	}
}

func RenderPackContextCreate(assetsDirectory *string, baseOverlayRoots []OverlayRoot, packStack *texturepacks.TexturePackStack) *RenderPackContext {
	overlays := make([]OverlayRoot, len(baseOverlayRoots))
	copy(overlays, baseOverlayRoots)

	if packStack != nil {
		for _, overlay := range packStack.OverlayRoots {
			overlays = append(overlays, OverlayRoot{
				Path:     overlay.Path,
				SourceId: overlay.PackId,
				Kind:     "resource_pack",
			})
		}
	}

	assetsRoot := ""
	if assetsDirectory != nil {
		assetsRoot = *assetsDirectory
	}

	var packIds []string
	packStackHash := ""
	var packs []texturepacks.RegisteredResourcePack
	if packStack != nil {
		packStackHash = packStack.Fingerprint
		packs = packStack.Packs
		packIds = make([]string, 0, len(packStack.Packs))
		for _, pack := range packStack.Packs {
			packIds = append(packIds, pack.Id)
		}
	}

	return NewRenderPackContext(assetsRoot, overlays, packIds, packStackHash, packs, nil)

}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ResolveRendererForOptions(options BlockRenderOptions) (*MinecraftBlockRenderer, BlockRenderOptions) {
	var forwardedOptions BlockRenderOptions
	if _minecraftBlockRenderer._packRegistry == nil || options.PackIds == nil || len(options.PackIds) == 0 {
		forwardedOptions = options
		return _minecraftBlockRenderer, forwardedOptions
	}

	if len(_minecraftBlockRenderer._packContext.PackIds) > 0 && PackSequencesEqual(options.PackIds, _minecraftBlockRenderer._packContext.PackIds) {
		forwardedOptions = options
		forwardedOptions.PackIds = nil
		return _minecraftBlockRenderer, forwardedOptions
	}

	renderer := _minecraftBlockRenderer.GetRendererForPackStack(options.PackIds)
	forwardedOptions = options
	forwardedOptions.PackIds = nil
	return renderer, forwardedOptions
}

func PackSequencesEqual(candidate []string, baseline []string) bool {
	if len(candidate) != len(baseline) {
		return false
	}

	for i := 0; i < len(candidate); i++ {
		if !strings.EqualFold(candidate[i], baseline[i]) {
			return false
		}
	}

	return true
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetRendererForPackStack(packIds []string) *MinecraftBlockRenderer {
	if _minecraftBlockRenderer._packRegistry == nil {
		panic("This renderer was created without a texture pack registry and cannot resolve pack combinations.")
	}

	if strings.TrimSpace(_minecraftBlockRenderer._packContext.AssetsRoot) == "" {
		panic("Texture pack rendering requires a renderer created from Minecraft assets (not aggregated data files).")
	}

	packStack := _minecraftBlockRenderer._packRegistry.BuildPackStack(packIds)
	return _minecraftBlockRenderer.GetRendererForPackStackWithContext(packStack)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetRendererForPackStackWithContext(packStack *texturepacks.TexturePackStack) *MinecraftBlockRenderer {
	_minecraftBlockRenderer._packRendererCacheMu.Lock()
	defer _minecraftBlockRenderer._packRendererCacheMu.Unlock()

	if _minecraftBlockRenderer._packRendererCache == nil {
		_minecraftBlockRenderer._packRendererCache = make(map[string]*MinecraftBlockRenderer)
	}

	cacheKey := packStack.Fingerprint
	if cached, exists := _minecraftBlockRenderer._packRendererCache[cacheKey]; exists {
		return cached
	}

	renderer := _minecraftBlockRenderer.CreatePackRenderer(packStack)
	// fmt.Printf("cacheKey: %+v\nrendered: %+v\n", cacheKey, renderer != nil)

	_minecraftBlockRenderer._packRendererCache[cacheKey] = renderer
	return renderer
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) CreatePackRenderer(packStack *texturepacks.TexturePackStack) *MinecraftBlockRenderer {
	packContext := RenderPackContextCreate(&_minecraftBlockRenderer._assetsDirectory, _minecraftBlockRenderer._baseOverlayRoots, packStack)

	overlayPaths := make([]string, len(packContext.OverlayRoots))
	for i, root := range packContext.OverlayRoots {
		overlayPaths[i] = root.Path
	}

	modelResolver := data.NewBlockModelResolver(make(map[string]data.BlockModelDefinition)).LoadFromMinecraftAssets(_minecraftBlockRenderer._assetsDirectory, &overlayPaths, &packContext.AssetNamespaces)
	blockRegistry := data.NewBlockRegistry().LoadFromMinecraftAssets(_minecraftBlockRenderer._assetsDirectory, modelResolver.Definitions, overlayPaths, &packContext.AssetNamespaces)
	itemRegistry := data.NewItemRegistry().LoadFromMinecraftAssets(_minecraftBlockRenderer._assetsDirectory, modelResolver.Definitions, overlayPaths, &packContext.AssetNamespaces)
	textureRoot := filepath.Join(_minecraftBlockRenderer._assetsDirectory, "textures")
	if _, err := os.Stat(textureRoot); os.IsNotExist(err) {
		textureRoot = _minecraftBlockRenderer._assetsDirectory
	}
	textureRepository := data.NewTextureRepository(textureRoot, nil, overlayPaths, packContext.AssetNamespaces)

	renderer := NewMinecraftBlockRenderer(modelResolver, textureRepository, blockRegistry, itemRegistry, _minecraftBlockRenderer._packContext.AssetsRoot, _minecraftBlockRenderer._baseOverlayRoots, _minecraftBlockRenderer._packRegistry, *packContext)
	if strings.TrimSpace(_minecraftBlockRenderer._cacheDirectory) != "" {
		renderer.SetCacheDirectory(_minecraftBlockRenderer._cacheDirectory)
	}
	return renderer
}

type PacksOverlayRoot struct {
	Path     string
	SourceId string
	Kind     string
}

type PacksOverlayRootKind = string

const (
	OverlayRootKind_CustomData   PacksOverlayRootKind = "custom_data"
	OverlayRootKind_ResourcePack PacksOverlayRootKind = "resource_pack"
	OverlayRootKind_Vanilla      PacksOverlayRootKind = "vanilla"
)

type RenderedResource struct {
	Image      *image.RGBA
	ResourceId ResourceIdResult
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ComputeResourceIdInternal(target string, options BlockRenderOptions, preResolvedItem *ItemModelResolution) *ResourceIdResult {
	normalizedTarget := strings.TrimSpace(target)
	lookupTarget := normalizedTarget
	namespaceSeparator := strings.Index(lookupTarget, ":")
	if namespaceSeparator >= 0 {
		lookupTarget = lookupTarget[namespaceSeparator+1:]
	}

	var modelPath *string = nil
	var primaryModelIdentifier *string = nil
	resolvedTextures := make(map[string]struct{})
	var variantKey string

	hasItemRegistry := _minecraftBlockRenderer._itemRegistry != nil
	itemInfo := _minecraftBlockRenderer._itemRegistry.GetItemInfo(lookupTarget)
	hasItemInfo := itemInfo != nil
	if preResolvedItem != nil && strings.EqualFold(preResolvedItem.LookupTarget, lookupTarget) {
		if preResolvedItem.ItemInfo != nil {
			itemInfo = preResolvedItem.ItemInfo
			hasItemInfo = true
		}
	}

	shouldTreatAsItem := hasItemRegistry && (hasItemInfo || options.ItemData != nil)

	processItem := func(info *data.ItemInfo) {
		if _minecraftBlockRenderer._itemRegistry == nil {
			variantKey = "literal:" + normalizedTarget
			return
		}

		var referenceModel *string = nil
		if model, exists := _minecraftBlockRenderer._itemRegistry.TryGetModel(lookupTarget); exists && strings.TrimSpace(model) != "" {
			m := (_minecraftBlockRenderer.NormalizeModelIdentifier(model))
			referenceModel = &m
		}

		var effectiveModel *data.BlockModelInstance = nil
		var effectiveModelIdentifier *string = nil
		var modelCandidates []string = nil
		var resolvedModelName *string = nil
		var compositeModelNames []string = nil
		var specialModel *data.ItemSpecialModelInfo = nil

		if preResolvedItem != nil && strings.EqualFold(preResolvedItem.LookupTarget, lookupTarget) {
			effectiveModel = preResolvedItem.Model
			modelCandidates = preResolvedItem.ModelCandidates
			resolvedModelName = preResolvedItem.ResolvedModelName
			compositeModelNames = preResolvedItem.CompositeModelNames
			specialModel = preResolvedItem.SpecialModel
			if preResolvedItem.ItemInfo != nil {
				info = preResolvedItem.ItemInfo
			}
		} else {
			// Always use ResolveItemModel for consistent resolution logic
			// (it handles selectors, Firmament models, and all other item model types)
			effectiveModel, modelCandidates, resolvedModelName, compositeModelNames, specialModel = _minecraftBlockRenderer.ResolveItemModel(lookupTarget, info, options)
		}

		effectiveModelIdentifier = resolvedModelName
		if effectiveModelIdentifier == nil && effectiveModel != nil {
			effectiveModelIdentifier = &effectiveModel.Name
		}

		if len(compositeModelNames) > 1 {
			var identifiers []string
			for _, compositeModelName := range compositeModelNames {
				if strings.TrimSpace(compositeModelName) == "" {
					continue
				}
				compositeModelNameCopy := compositeModelName
				compositeModel := _minecraftBlockRenderer.ResolveModelOrNull(&compositeModelNameCopy)
				if compositeModel == nil {
					continue
				}

				identifier := _minecraftBlockRenderer.NormalizeModelIdentifier(compositeModelName)
				identifiers = append(identifiers, identifier)
				if primaryModelIdentifier == nil {
					primaryModelIdentifier = &identifier
				}
				for _, texture := range _minecraftBlockRenderer.CollectResolvedTextures(compositeModel) {
					resolvedTextures[texture] = struct{}{}
				}
			}

			if len(identifiers) > 0 {
				compositePath := "composite:" + strings.Join(identifiers, "+")
				modelPath = &compositePath
				referenceModel = &identifiers[0]
			}
		}

		if modelPath == nil && effectiveModel != nil {
			input := effectiveModelIdentifier
			if input == nil {
				input = &effectiveModel.Name
			}

			identifier := _minecraftBlockRenderer.NormalizeModelIdentifier(*input)
			primaryModelIdentifier = &identifier
			modelPath = &identifier
			referenceModel = &identifier
			for _, texture := range _minecraftBlockRenderer.CollectResolvedTextures(effectiveModel) {
				resolvedTextures[texture] = struct{}{}
			}
		} else if info != nil && info.Texture != nil {
			resolvedTextures[*info.Texture] = struct{}{}
		}

		if specialModel != nil && strings.EqualFold(specialModel.Type, "minecraft:chest") {
			if chestTexture := specialChestEntityTexture(specialModel.Texture); chestTexture != "" {
				texture := _minecraftBlockRenderer._textureRepository.GetTexture(chestTexture)
				if texture != nil && !_minecraftBlockRenderer._textureRepository.IsMissingTexture(texture) {
					resolvedTextures = map[string]struct{}{
						chestTexture: {},
					}
					marker := specialModelResourceMarker(specialModel)
					modelPath = &marker
					primaryModelIdentifier = &marker
					referenceModel = &marker
				}
			}
		}

		if options.ItemData != nil && options.ItemData.CustomData != nil {
			if headTexture, exists := _minecraftBlockRenderer.GetHeadTextureOverride(options.ItemData.CustomData); exists {
				resolvedTextures[headTexture] = struct{}{}
			}
		}

		if options.ItemData != nil && options.ItemData.Profile != nil {
			if profileTexture, exists := _minecraftBlockRenderer.GetHeadTextureOverride(options.ItemData.Profile); exists {
				resolvedTextures[profileTexture] = struct{}{}
			}
		}

		if len(resolvedTextures) == 0 && referenceModel != nil {
			resolvedTextures[*referenceModel] = struct{}{}
		}

		itemDataKey := ""
		if options.ItemData != nil {
			itemDataKey = BuildItemRenderDataKey(options.ItemData)
		}

		if modelPath == nil {
			modelPath = referenceModel
		}

		if modelPath == nil {
			modelPath = &normalizedTarget
		}

		textures := make([]string, 0, len(resolvedTextures))
		for texture := range resolvedTextures {
			textures = append(textures, texture)
		}

		variantKey = fmt.Sprintf("item:%s:%s:%s:%s", normalizedTarget, *modelPath, JoinTextures(textures), itemDataKey)

		_ = modelCandidates
	}

	if shouldTreatAsItem {
		processItem(itemInfo)
	} else if blockModelPath, exists := _minecraftBlockRenderer._blockRegistry.TryGetModel(lookupTarget); exists && strings.TrimSpace(blockModelPath) != "" {
		modelPath = &blockModelPath
		model := _minecraftBlockRenderer._modelResolver.Resolve(blockModelPath)
		modelPath = &model.Name
		for _, texture := range _minecraftBlockRenderer.CollectResolvedTextures(model) {
			resolvedTextures[texture] = struct{}{}
		}

		textures := make([]string, 0, len(resolvedTextures))
		for texture := range resolvedTextures {
			textures = append(textures, texture)
		}

		variantKey = fmt.Sprintf("block:%s:%s:%s", normalizedTarget, model.Name, JoinTextures(textures))
	} else if hasItemRegistry {
		processItem(itemInfo)
	} else {
		variantKey = "literal:" + normalizedTarget
	}

	sourcePackId := _minecraftBlockRenderer.DetermineSourcePackId(modelPath, resolvedTextures)
	if strings.EqualFold(sourcePackId, VanillaPackId) && primaryModelIdentifier != nil {
		if modelPackId, exists := _minecraftBlockRenderer.TryResolvePackFromAsset(*primaryModelIdentifier, "models", ".json"); exists {
			sourcePackId = modelPackId
		}
	}

	descriptor := fmt.Sprintf("%s|%s|%s", RendererVersion, _minecraftBlockRenderer._packContext.PackStackHash, variantKey)
	resourceId := ComputeResourceIdHash(descriptor)
	textures := make([]string, 0, len(resolvedTextures))
	for texture := range resolvedTextures {
		textures = append(textures, texture)
	}

	return &ResourceIdResult{
		ResourceId:    resourceId,
		SourcePackId:  sourcePackId,
		PackStackHash: _minecraftBlockRenderer._packContext.PackStackHash,
		Model:         modelPath,
		Textures:      textures,
	}

}

// private static string JoinTextures(IReadOnlyCollection<string> textures) {
// 	if (textures.Count == 0) {
// 		return string.Empty;
// 	}

//		return string.Join(',', textures.OrderBy(static t => t, StringComparer.OrdinalIgnoreCase));
//	}
func JoinTextures(textures []string) string {
	if len(textures) == 0 {
		return ""
	}

	sort.SliceStable(textures, func(i, j int) bool {
		return strings.Compare(textures[i], textures[j]) < 0
	})

	return strings.Join(textures, ",")
}

type ResourceIdResult struct {
	ResourceId    string
	SourcePackId  string
	PackStackHash string
	Model         *string
	Textures      []string
}

type ItemModelResolution struct {
	LookupTarget        string
	ItemInfo            *data.ItemInfo
	Model               *data.BlockModelInstance
	ModelCandidates     []string
	ResolvedModelName   *string
	CompositeModelNames []string
	SpecialModel        *data.ItemSpecialModelInfo
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) CollectResolvedTextures(model *data.BlockModelInstance) []string {
	set := make(map[string]struct{})
	for _, texture := range model.Textures {
		resolved := ResolveTexture(texture, model)
		if strings.TrimSpace(resolved) != "" {
			set[resolved] = struct{}{}
		}
	}

	var textures []string
	for texture := range set {
		textures = append(textures, texture)
	}

	return textures
}

func rgbaToHex(c *color.RGBA) string {
	return fmt.Sprintf("%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

func BuildItemRenderDataKey(data *data.ItemRenderData) string {
	if data == nil {
		return ""
	}
	customKey := "none"
	if data.CustomData != nil {
		customKey = BuildResourceRelevantCustomDataKey(data.CustomData)
	}
	if data.Layer0Tint == nil &&
		!data.DisableDefaultLayer0Tint &&
		len(data.AdditionalLayerTints) == 0 &&
		(customKey == "" || customKey == "none") &&
		data.Profile == nil {
		return ""
	}

	builder := strings.Builder{}

	if data.Layer0Tint != nil {
		builder.WriteString("l0=")
		builder.WriteString(rgbaToHex(data.Layer0Tint))
	} else {
		builder.WriteString("l0=none")
	}

	builder.WriteString(";disable=")
	if data.DisableDefaultLayer0Tint {
		builder.WriteString("1")
	} else {
		builder.WriteString("0")
	}

	if data.AdditionalLayerTints != nil && len(data.AdditionalLayerTints) > 0 {
		keys := make([]int, 0, len(data.AdditionalLayerTints))
		for k := range data.AdditionalLayerTints {
			keys = append(keys, k)
		}
		sort.Ints(keys)

		for _, key := range keys {
			layer := data.AdditionalLayerTints[key]
			builder.WriteString(";l")
			builder.WriteString(strconv.Itoa(key))
			builder.WriteString("=")
			builder.WriteString(rgbaToHex(layer))
		}
	}

	if data.CustomData != nil {
		builder.WriteString(";custom=")
		builder.WriteString(customKey)
	} else {
		builder.WriteString(";custom=none")
	}

	if data.Profile != nil {
		builder.WriteString(";profile=")
		builder.WriteString(BuildProfileKey(data.Profile))
	} else {
		builder.WriteString(";profile=none")
	}

	return builder.String()
}

func BuildProfileKey(profile *nbt.NbtCompound) string {
	propertiesTag, exists := profile.Get("properties")
	if exists {
		if properties, ok := propertiesTag.(*nbt.NbtList); ok {
			for i := 0; i < properties.Count(); i++ {
				entry := properties.At(i)
				propertyCompound := nbtCompoundFromTag(entry)
				if propertyCompound == nil {
					continue
				}

				nameTag, nameExists := propertyCompound.Get("name")
				nameValue, isNbtString := nbtStringValue(nameTag)
				valueTag, valueExists := propertyCompound.Get("value")
				valueString, isValueNbtString := nbtStringValue(valueTag)

				if nameExists && isNbtString && valueExists && isValueNbtString &&
					strings.EqualFold(nameValue, "textures") &&
					strings.TrimSpace(valueString) != "" {
					hash := sha256.Sum256([]byte(valueString))
					return hex.EncodeToString(hash[:])
				}
			}
		}
	}

	if idTag, exists := profile.Get("id"); exists {
		if formatted := FormatNbtValue(idTag); strings.TrimSpace(formatted) != "" {
			return formatted
		}
	}

	return "none"
}

func FormatNbtValue(tag nbt.NbtTag) string {
	if tag == nil {
		return ""
	}

	switch v := tag.(type) {
	case *nbt.NbtString:
		return v.Value
	case nbt.NbtString:
		return v.Value
	case *nbt.NbtByte:
		return strconv.FormatInt(int64(v.Value), 10)
	case nbt.NbtByte:
		return strconv.FormatInt(int64(v.Value), 10)
	case *nbt.NbtShort:
		return strconv.FormatInt(int64(v.Value), 10)
	case nbt.NbtShort:
		return strconv.FormatInt(int64(v.Value), 10)
	case *nbt.NbtInt:
		return strconv.FormatInt(int64(v.Value), 10)
	case nbt.NbtInt:
		return strconv.FormatInt(int64(v.Value), 10)
	case *nbt.NbtLong:
		return strconv.FormatInt(v.Value, 10)
	case nbt.NbtLong:
		return strconv.FormatInt(v.Value, 10)
	case *nbt.NbtFloat:
		return strconv.FormatFloat(float64(v.Value), 'f', -1, 32)
	case nbt.NbtFloat:
		return strconv.FormatFloat(float64(v.Value), 'f', -1, 32)
	case *nbt.NbtDouble:
		return strconv.FormatFloat(v.Value, 'f', -1, 64)
	case nbt.NbtDouble:
		return strconv.FormatFloat(v.Value, 'f', -1, 64)
	case *nbt.NbtCompound:
		return "{" + BuildCustomDataKey(v) + "}"
	case *nbt.NbtList:
		var items []string
		for i := 0; i < v.Count(); i++ {
			items = append(items, FormatNbtValue(v.At(i)))
		}
		return "[" + strings.Join(items, ",") + "]"
	case *nbt.NbtIntArray:
		var items []string
		for _, val := range v.Values {
			items = append(items, strconv.FormatInt(int64(val), 10))
		}
		return "[" + strings.Join(items, ",") + "]"
	case *nbt.NbtLongArray:
		var items []string
		for _, val := range v.Values {
			items = append(items, strconv.FormatInt(val, 10))
		}
		return "[" + strings.Join(items, ",") + "]"
	case *nbt.NbtByteArray:
		var items []string
		for _, val := range v.Values {
			items = append(items, strconv.FormatInt(int64(val), 10))
		}
		return "[" + strings.Join(items, ",") + "]"
	default:
		return ""
	}
}

func nbtCompoundFromTag(tag nbt.NbtTag) *nbt.NbtCompound {
	switch v := tag.(type) {
	case *nbt.NbtCompound:
		return v
	default:
		return nil
	}
}

func nbtStringValue(tag nbt.NbtTag) (string, bool) {
	switch v := tag.(type) {
	case *nbt.NbtString:
		return v.Value, true
	case nbt.NbtString:
		return v.Value, true
	default:
		return "", false
	}
}

func BuildCustomDataKey(compound *nbt.NbtCompound) string {
	var segments []string
	for key, value := range compound.Items() {
		formattedValue := FormatNbtValue(value)
		segments = append(segments, key+"="+formattedValue)
	}

	if len(segments) == 0 {
		return "none"
	}

	sort.SliceStable(segments, func(i, j int) bool {
		return strings.Compare(segments[i], segments[j]) < 0
	})

	return strings.Join(segments, "|")
}

func BuildResourceRelevantCustomDataKey(compound *nbt.NbtCompound) string {
	if compound == nil {
		return "none"
	}

	var segments []string
	for key, value := range compound.Items() {
		if !isResourceRelevantCustomDataKey(key) {
			if nested := nbtCompoundFromTag(value); nested != nil {
				nestedKey := BuildResourceRelevantCustomDataKey(nested)
				if nestedKey != "empty" && nestedKey != "none" {
					segments = append(segments, key+"={"+nestedKey+"}")
				}
			}
			continue
		}
		formattedValue := FormatNbtValue(value)
		if strings.TrimSpace(formattedValue) != "" {
			segments = append(segments, key+"="+formattedValue)
		}
	}

	if len(segments) == 0 {
		return "empty"
	}

	sort.SliceStable(segments, func(i, j int) bool {
		return strings.Compare(segments[i], segments[j]) < 0
	})

	return strings.Join(segments, "|")
}

func isResourceRelevantCustomDataKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "texture", "textures", "skin", "profile", "minecraft:profile", "skullowner":
		return true
	default:
		return strings.Contains(normalized, "texture")
	}
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) DetermineSourcePackId(modelPath *string, textureIds map[string]struct{}) string {
	if modelPath != nil {
		if packId, exists := _minecraftBlockRenderer.TryResolvePackFromAsset(*modelPath, "models", ".json"); exists {
			return packId
		}
	}

	for textureId := range textureIds {
		if packId, exists := _minecraftBlockRenderer.TryResolvePackFromAsset(textureId, "textures", ".png"); exists {
			return packId
		}
	}

	searchRoots := _minecraftBlockRenderer._packContext.SearchRoots
	if modelPath != nil {
		normalizedModel := _minecraftBlockRenderer.NormalizeModelPath(*modelPath)
		if strings.TrimSpace(normalizedModel) != "" {
			for i := len(searchRoots) - 1; i >= 0; i-- {
				root := searchRoots[i]
				if root.Kind == OverlayRootKind_Vanilla {
					continue
				}

				candidate := filepath.Join(root.Path, "models", normalizedModel+".json")
				if _, err := os.Stat(candidate); err == nil {
					return root.SourceId
				}
			}
		}
	}

	for textureId := range textureIds {
		normalizedTexture := _minecraftBlockRenderer.NormalizeTexturePath(textureId)
		if strings.TrimSpace(normalizedTexture) == "" {
			continue
		}

		for i := len(searchRoots) - 1; i >= 0; i-- {
			root := searchRoots[i]
			if root.Kind == OverlayRootKind_Vanilla {
				continue
			}

			candidate := filepath.Join(root.Path, "textures", normalizedTexture+".png")
			if _, err := os.Stat(candidate); err == nil {
				return root.SourceId
			}
		}
	}

	return VanillaPackId
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) NormalizeModelPath(modelPath string) string {
	if strings.TrimSpace(modelPath) == "" {
		return ""
	}

	normalized := strings.ReplaceAll(modelPath, "\\", "/")
	normalized = strings.TrimSpace(normalized)

	if strings.HasPrefix(normalized, "minecraft:") {
		normalized = normalized[10:]
	}

	normalized = strings.TrimLeft(normalized, "/")
	if strings.HasPrefix(normalized, "models/") {
		normalized = normalized[7:]
	}

	return normalized
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryResolvePackFromAsset(assetId string, category string, extension string) (string, bool) {
	packId := VanillaPackId
	if strings.TrimSpace(assetId) == "" {
		return packId, false
	}

	namespaceName, relativePath := _minecraftBlockRenderer.NormalizeAssetPath(assetId)
	if strings.TrimSpace(relativePath) == "" {
		return packId, false
	}

	if strings.HasPrefix(relativePath, category+"/") {
		relativePath = relativePath[len(category)+1:]
	}

	if strings.HasSuffix(relativePath, extension) {
		relativePath = relativePath[:len(relativePath)-len(extension)]
	}

	relativePath = strings.ReplaceAll(relativePath, "/", string(filepath.Separator))

	roots := _minecraftBlockRenderer._packContext.AssetNamespaces.ResolveRoots(namespaceName, true)
	for i := len(roots) - 1; i >= 0; i-- {
		root := roots[i]
		basePath := root.Path
		var candidate string

		if strings.HasSuffix(basePath, category) {
			candidate = filepath.Join(basePath, relativePath+extension)
		} else {
			candidate = filepath.Join(basePath, category, relativePath+extension)
		}

		if _, err := os.Stat(candidate); err == nil {
			return root.SourceId, !root.IsVanilla
		}

		// Check provider-backed roots (e.g. .cats archives or ZIP packs)
		if root.Provider != nil && fmt.Sprintf("%T", root.Provider) != "*assets.DirectoryResourceProvider" {
			providerRelative := strings.ReplaceAll(relativePath, string(filepath.Separator), "/")
			var providerCandidate string
			if strings.HasSuffix(basePath, category) {
				providerCandidate = providerRelative + extension
			} else {
				providerCandidate = category + "/" + providerRelative + extension
			}

			if (*root.Provider).FileExists(providerCandidate) {
				return root.SourceId, !root.IsVanilla
			}
		}
	}

	return packId, false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) NormalizeAssetPath(assetId string) (string, string) {
	normalized := strings.ReplaceAll(assetId, "\\", "/")
	normalized = strings.TrimSpace(normalized)
	if strings.TrimSpace(normalized) == "" {
		return "minecraft", ""
	}

	namespaceName := "minecraft"
	colonIndex := strings.Index(normalized, ":")
	if colonIndex >= 0 {
		namespaceName = normalized[:colonIndex]
		normalized = normalized[colonIndex+1:]
	}

	normalized = strings.TrimLeft(normalized, "/")
	return namespaceName, normalized
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) NormalizeTexturePath(textureId string) string {
	if strings.TrimSpace(textureId) == "" {
		return ""
	}

	normalized := strings.ReplaceAll(textureId, "\\", "/")
	normalized = strings.TrimSpace(normalized)
	if strings.HasPrefix(normalized, "minecraft:") {
		normalized = normalized[10:]
	}

	normalized = strings.TrimLeft(normalized, "/")
	if strings.HasPrefix(normalized, "textures/") {
		normalized = normalized[9:]
	}

	if strings.HasSuffix(normalized, ".png") {
		normalized = normalized[:len(normalized)-4]
	}

	return normalized
}

func ComputeResourceIdHash(input string) string {
	sha := sha256.New()
	sha.Write([]byte(input))
	hash := sha.Sum(nil)
	return base32.StdEncoding.EncodeToString(hash)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) PreloadRegisteredPacks(includeDefaultPackStack bool) {
	var stacksToPreload [][]string
	if includeDefaultPackStack && len(_minecraftBlockRenderer._packContext.PackIds) > 0 {
		stacksToPreload = append(stacksToPreload, _minecraftBlockRenderer._packContext.PackIds)
	}

	if _minecraftBlockRenderer._packRegistry == nil {
		return
	}

	for _, pack := range _minecraftBlockRenderer.GetRegisteredPacks() {
		stacksToPreload = append(stacksToPreload, []string{pack.Id})
	}

	_minecraftBlockRenderer.PreloadTexturePackStacks(stacksToPreload)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) PreloadTexturePackStacks(packStacks [][]string) {
	if _minecraftBlockRenderer._packRegistry == nil {
		for _, stack := range packStacks {
			if len(stack) > 0 {
				panic("This renderer was created without a texture pack registry and cannot preload pack combinations.")
			}
		}

		return
	}

	seenStacks := make(map[string]struct{})
	for _, packIds := range packStacks {
		if len(packIds) == 0 {
			continue
		}
		if len(_minecraftBlockRenderer._packContext.PackIds) > 0 && PackSequencesEqual(packIds, _minecraftBlockRenderer._packContext.PackIds) {
			continue
		}

		stack := _minecraftBlockRenderer._packRegistry.BuildPackStack(packIds)
		if _, exists := seenStacks[stack.Fingerprint]; exists {
			continue
		}

		seenStacks[stack.Fingerprint] = struct{}{}
		// fmt.Printf("Preloading renderer for texture pack stack: %s\n", stack.Fingerprint)
		_minecraftBlockRenderer.GetRendererForPackStackWithContext(stack)
		// Console.WriteLine($"Preloading renderer for texture pack stack: {stack.Fingerprint}");

	}
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetRegisteredPacks() []texturepacks.RegisteredResourcePack {
	if _minecraftBlockRenderer == nil || _minecraftBlockRenderer._packRegistry == nil {
		return nil
	}
	output := make([]texturepacks.RegisteredResourcePack, 0, len(_minecraftBlockRenderer._packRegistry.Packs))
	for _, pack := range _minecraftBlockRenderer._packRegistry.Packs {
		output = append(output, pack)
	}

	return output
}

// public IReadOnlyList<LoadedResourcePackInfo> GetLoadedResourcePacks() {
// 	EnsureNotDisposed();

// 	var packs = GetLoadedResourcePackSnapshot()
// 		.OrderBy(static pack => pack.DisplayName, StringComparer.OrdinalIgnoreCase)
// 		.ThenBy(static pack => pack.Id, StringComparer.OrdinalIgnoreCase)
// 		.ToArray();
// 	var results = new List<LoadedResourcePackInfo>(packs.Length);
// 	foreach (var pack in packs) {
// 		results.Add(new LoadedResourcePackInfo(pack, LoadPackIcon(pack)));
// 	}

//		return results;
//	}
func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetLoadedResourcePacks() []LoadedResourcePackInfo {
	packs := _minecraftBlockRenderer.GetRegisteredPacks()
	sort.SliceStable(packs, func(i, j int) bool {
		if strings.EqualFold(packs[i].DisplayName, packs[j].DisplayName) {
			return strings.Compare(packs[i].Id, packs[j].Id) < 0
		}
		return strings.Compare(packs[i].DisplayName, packs[j].DisplayName) < 0
	})

	results := make([]LoadedResourcePackInfo, len(packs))
	for i, pack := range packs {
		results[i] = LoadedResourcePackInfo{
			Pack:    pack,
			Icon:    _minecraftBlockRenderer.LoadPackIcon(pack),
			Display: fmt.Sprintf("%s (%s)", pack.DisplayName, pack.Id),
		}
	}

	return results
}

type LoadedResourcePackInfo struct {
	Pack    texturepacks.RegisteredResourcePack
	Meta    texturepacks.ResourcePackMeta
	Icon    *image.RGBA
	Display string
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) LoadPackIcon(pack texturepacks.RegisteredResourcePack) *image.RGBA {
	if pack.Provider != nil {
		if !(pack.Provider).FileExists("pack.png") {
			return nil
		}

		stream, err := (pack.Provider).OpenRead("pack.png")
		if err != nil {
			return nil
		}
		defer stream.Close()

		img, err := png.Decode(stream)
		if err != nil {
			return nil
		}

		rgbaImg := image.NewRGBA(img.Bounds())
		draw.Draw(rgbaImg, img.Bounds(), img, image.Point{0, 0}, draw.Src)
		return rgbaImg
	}

	iconPath := filepath.Join(pack.RootPath, "pack.png")
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(iconPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return nil
	}

	rgbaImg := image.NewRGBA(img.Bounds())
	draw.Draw(rgbaImg, img.Bounds(), img, image.Point{0, 0}, draw.Src)
	return rgbaImg
}
