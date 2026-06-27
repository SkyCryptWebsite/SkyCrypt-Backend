package minecraftblockrenderer

import (
	"fmt"
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	texturepacks "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/TexturePacks"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/imagecache"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ColorMap maps common color names to RGB values (case-insensitive keys expected).
var ColorMap = map[string]color.NRGBA{
	"white":      {R: 249, G: 255, B: 254, A: 255},
	"orange":     {R: 249, G: 128, B: 29, A: 255},
	"magenta":    {R: 199, G: 78, B: 189, A: 255},
	"light_blue": {R: 58, G: 179, B: 218, A: 255},
	"yellow":     {R: 254, G: 216, B: 61, A: 255},
	"lime":       {R: 128, G: 199, B: 31, A: 255},
	"pink":       {R: 243, G: 139, B: 170, A: 255},
	"gray":       {R: 71, G: 79, B: 82, A: 255},
	"light_gray": {R: 157, G: 157, B: 151, A: 255},
	"cyan":       {R: 22, G: 156, B: 156, A: 255},
	"purple":     {R: 137, G: 50, B: 184, A: 255},
	"blue":       {R: 60, G: 68, B: 170, A: 255},
	"brown":      {R: 131, G: 84, B: 50, A: 255},
	"green":      {R: 94, G: 124, B: 22, A: 255},
	"red":        {R: 176, G: 46, B: 38, A: 255},
	"black":      {R: 29, G: 29, B: 33, A: 255},
}

var (
	biomeTintOnce   sync.Once
	biomeTintConfig *data.BiomeTintConfiguration
)

var BiomeTints *data.BiomeTintConfiguration

func init() {
	biomeTintOnce.Do(func() {
		biomeTintConfig = data.LoadDefault()
		BiomeTints = biomeTintConfig
	})
}

const (
	ConstantTintStrength = 1.45
	ColorTintBlend       = 0.82
)

type BiomeTintKind int

const (
	BiomeTintKindGrass BiomeTintKind = iota
	BiomeTintKindFoliage
	BiomeTintKindDryFoliage
)

// DefaultBiomeTintCoordinates maps BiomeTintKind to (Temperature, Downfall).
var DefaultBiomeTintCoordinates = map[BiomeTintKind]struct{ Temperature, Downfall float64 }{
	BiomeTintKindGrass:      {0.5, 1.0},
	BiomeTintKindFoliage:    {0.5, 1.0},
	BiomeTintKindDryFoliage: {0.5, 0.25},
}

type SkullResolverContext struct {
	ItemId       string
	ItemData     *data.ItemRenderData
	CustomDataId *string
	Profile      *nbt.NbtCompound
	CustomData   *nbt.NbtCompound
}

type MinecraftBlockRenderer struct {
	_modelResolver             *data.BlockModelResolver
	_textureRepository         *data.TextureRepository
	_blockRegistry             *data.BlockRegistry
	_itemRegistry              *data.ItemRegistry
	_packContext               RenderPackContext
	_assetsDirectory           string
	_cacheDirectory            string
	_playerSkinCacheDirectory  string
	_baseOverlayRoots          []OverlayRoot
	_packRegistry              *texturepacks.TexturePackRegistry
	_packRendererCache         map[string]*MinecraftBlockRenderer
	_packRendererCacheMu       sync.Mutex
	_biomeTintedTextureCache   map[string]image.RGBA
	_biomeTintedTextureMu      sync.RWMutex
	_playerSkinCache           map[string]*image.RGBA
	_playerSkinCacheMu         sync.RWMutex
	_skyblockItemDefinitions   map[string]skyblockItemDefinitionCacheEntry
	_skyblockItemDefinitionsMu sync.RWMutex
}

type ItemRenderData = data.ItemRenderData

func (renderer *MinecraftBlockRenderer) GetLayerTint(context SkullResolverContext, layerIndex int) (tintColor int, hasTint bool) {
	if context.ItemData != nil && context.ItemData.GetLayerTint != nil {
		return context.ItemData.GetLayerTint(layerIndex)
	}

	return 0, false
}

func (renderer *MinecraftBlockRenderer) IsDefault(context SkullResolverContext) bool {
	if context.ItemData != nil && context.ItemData.IsDefault != nil {
		if !context.ItemData.IsDefault() {
			return false
		}
	}

	return context.CustomDataId == nil &&
		context.CustomData == nil &&
		context.Profile == nil
}

type BlockRenderOptions struct {
	Size                      int
	YawInDegrees              float64
	PitchInDegrees            float64
	RollInDegrees             float64
	PerspectiveAmount         float64
	UseGuiTransform           bool
	Padding                   float64
	AdditionalScale           float64
	AdditionalTranslation     data.Vector3
	OverrideGuiTransform      *data.TransformDefinition
	PackIds                   []string
	ItemData                  *data.ItemRenderData
	SkullTextureResolver      func(context SkullResolverContext) *string
	EnableAntiAliasing        bool
	CustomTextureFallbackItem string
	CustomTextureFallbackData *data.ItemRenderData
}

func DefaultBlockRenderOptions() BlockRenderOptions {
	return BlockRenderOptions{
		Size:               512,
		YawInDegrees:       0,
		PitchInDegrees:     0,
		RollInDegrees:      0,
		PerspectiveAmount:  0,
		UseGuiTransform:    true,
		Padding:            0.12,
		AdditionalScale:    1,
		EnableAntiAliasing: false,
	}
}

func MergeBlockRenderOptions(options *BlockRenderOptions) BlockRenderOptions {
	effective := DefaultBlockRenderOptions()
	if options == nil {
		return effective
	}
	if options.Size != 0 {
		effective.Size = options.Size
	}
	if options.YawInDegrees != 0 {
		effective.YawInDegrees = options.YawInDegrees
	}
	if options.PitchInDegrees != 0 {
		effective.PitchInDegrees = options.PitchInDegrees
	}
	if options.RollInDegrees != 0 {
		effective.RollInDegrees = options.RollInDegrees
	}
	if options.PerspectiveAmount != 0 {
		effective.PerspectiveAmount = options.PerspectiveAmount
	}
	if options.Padding != 0 {
		effective.Padding = options.Padding
	}
	if options.AdditionalScale != 0 {
		effective.AdditionalScale = options.AdditionalScale
	}
	if options.AdditionalTranslation != (data.Vector3{}) {
		effective.AdditionalTranslation = options.AdditionalTranslation
	}
	if options.OverrideGuiTransform != nil {
		effective.OverrideGuiTransform = options.OverrideGuiTransform
	}
	if len(options.PackIds) > 0 {
		effective.PackIds = options.PackIds
	}
	if options.ItemData != nil {
		effective.ItemData = options.ItemData
	}
	if options.SkullTextureResolver != nil {
		effective.SkullTextureResolver = options.SkullTextureResolver
	}
	if options.EnableAntiAliasing {
		effective.EnableAntiAliasing = true
	}
	if strings.TrimSpace(options.CustomTextureFallbackItem) != "" {
		effective.CustomTextureFallbackItem = options.CustomTextureFallbackItem
	}
	if options.CustomTextureFallbackData != nil {
		effective.CustomTextureFallbackData = options.CustomTextureFallbackData
	}
	return effective
}

func CreateFromMinecraftAssets(
	assetsDirectory string,
	texturePackRegistry *texturepacks.TexturePackRegistry,
	defaultPackIds []string) *MinecraftBlockRenderer {
	if assetsDirectory == "" {
		panic("assetsDirectory cannot be null or whitespace")
	}

	overlayRoots := DiscoverOverlayRoots(assetsDirectory)
	var defaultPackStack *texturepacks.TexturePackStack
	if texturePackRegistry != nil && len(defaultPackIds) > 0 {
		defaultPackStack = texturePackRegistry.BuildPackStack(defaultPackIds)
	}

	var packContext = RenderPackContextCreate(&assetsDirectory, overlayRoots, defaultPackStack)
	overlayPaths := make([]string, 0)
	for _, root := range packContext.OverlayRoots {
		found := false
		for _, path := range overlayPaths {
			if path == root.Path {
				found = true
				break
			}
		}

		if !found {
			overlayPaths = append(overlayPaths, root.Path)
		}
	}

	var modelResolver = data.NewBlockModelResolver(make(map[string]data.BlockModelDefinition)).LoadFromMinecraftAssets(assetsDirectory, &overlayPaths, &packContext.AssetNamespaces)
	var blockRegistry = data.NewBlockRegistry().LoadFromMinecraftAssets(assetsDirectory, modelResolver.Definitions, overlayPaths, &packContext.AssetNamespaces)
	var itemRegistry = data.NewItemRegistry().LoadFromMinecraftAssets(assetsDirectory, modelResolver.Definitions, overlayPaths, &packContext.AssetNamespaces)
	textureRoot := filepath.Join(assetsDirectory, "textures")
	if _, err := os.Stat(textureRoot); os.IsNotExist(err) {
		textureRoot = assetsDirectory
	}

	// fmt.Print("WTATIAWTABTAOJTNIA_ textureRoot: ", textureRoot, "\n")

	var textureRepository = data.NewTextureRepository(textureRoot, nil, overlayPaths, packContext.AssetNamespaces)

	return NewMinecraftBlockRenderer(modelResolver, textureRepository, blockRegistry, itemRegistry, assetsDirectory, overlayRoots, texturePackRegistry, *packContext)
}

func NewMinecraftBlockRenderer(BlockModelResolver *data.BlockModelResolver, TextureRepository *data.TextureRepository, BlockRegistry *data.BlockRegistry, ItemRegistry *data.ItemRegistry, AssetsDirectory string, BaseOverlayRoots []OverlayRoot, PackRegistry *texturepacks.TexturePackRegistry, PackContext RenderPackContext) *MinecraftBlockRenderer {
	return &MinecraftBlockRenderer{
		_modelResolver:           BlockModelResolver,
		_textureRepository:       TextureRepository,
		_blockRegistry:           BlockRegistry,
		_itemRegistry:            ItemRegistry,
		_assetsDirectory:         AssetsDirectory,
		_baseOverlayRoots:        BaseOverlayRoots,
		_packRegistry:            PackRegistry,
		_packContext:             PackContext,
		_packRendererCache:       make(map[string]*MinecraftBlockRenderer),
		_biomeTintedTextureCache: make(map[string]image.RGBA),
		_playerSkinCache:         make(map[string]*image.RGBA),
	}
}

type OverlayRoot struct {
	Path     string
	SourceId string
	Kind     string
}

func DiscoverOverlayRoots(assetsDirectory string) []OverlayRoot {
	var overlays []OverlayRoot

	parent := getParentDirectory(assetsDirectory)

	tryAdd := func(candidateDir string) {
		if strings.TrimSpace(candidateDir) == "" {
			return
		}

		fullPath, err := filepath.Abs(candidateDir)
		if err != nil {
			return
		}

		if info, err := os.Stat(fullPath); err != nil || !info.IsDir() {
			return
		}

		for _, root := range overlays {
			if root.Path == fullPath {
				return
			}
		}

		sourceId := "customdata_" + fmt.Sprint(len(overlays))
		overlays = append(overlays, OverlayRoot{Path: fullPath, SourceId: sourceId, Kind: "CustomData"})
	}

	// Check for customdata next to the executable (deployed with the library)
	assemblyLocation, err := os.Executable()
	if err == nil && assemblyLocation != "" {
		assemblyDir := filepath.Dir(assemblyLocation)
		tryAdd(filepath.Join(assemblyDir, "customdata"))
	}

	// Check parent directory of assets for customdata
	if parent != nil {
		tryAdd(filepath.Join(*parent, "customdata"))
	}

	// Check inside assets directory for customdata
	tryAdd(filepath.Join(assetsDirectory, "customdata"))

	return overlays
}

func getParentDirectory(path string) *string {
	if path == "" {
		return nil
	}

	parent := filepath.Dir(path)
	if parent == "." || parent == "/" {
		return nil
	}

	return &parent
}

func (renderer *MinecraftBlockRenderer) SetCacheDirectory(cacheDir string) {
	if renderer == nil {
		return
	}

	cacheDir = strings.TrimSpace(cacheDir)
	renderer._cacheDirectory = cacheDir
	if cacheDir == "" {
		renderer._playerSkinCacheDirectory = ""
		if renderer._textureRepository != nil {
			renderer._textureRepository.SetDiskCacheDirectory("", "")
		}
		return
	}

	if err := imagecache.EnsureCacheVersion(cacheDir, imagecache.CacheFormatVersion, "rendered", "player_skins", "derived"); err != nil {
		fmt.Printf("warning: failed to prepare renderer cache directory %s: %v\n", cacheDir, err)
	}

	renderer._playerSkinCacheDirectory = filepath.Join(cacheDir, "player_skins")
	if renderer._textureRepository != nil {
		namespace := renderer._packContext.PackStackHash
		if strings.TrimSpace(namespace) == "" {
			namespace = VanillaPackId
		}
		renderer._textureRepository.SetDiskCacheDirectory(filepath.Join(cacheDir, "derived"), namespace)
	}

	renderer._packRendererCacheMu.Lock()
	for _, cached := range renderer._packRendererCache {
		cached.SetCacheDirectory(cacheDir)
	}
	renderer._packRendererCacheMu.Unlock()
}

func (renderer *MinecraftBlockRenderer) NormalizeResourceKey(identifier *string) string {
	if identifier == nil || *identifier == "" {
		return ""
	}
	normalized := *identifier
	normalized = filepath.ToSlash(normalized)
	if strings.HasPrefix(normalized, "#") {
		return ""
	}

	stateSeperator := strings.Index(normalized, "[")
	if stateSeperator >= 0 {
		normalized = normalized[:stateSeperator]
	}

	colonIndex := strings.Index(normalized, ":")
	if colonIndex >= 0 {
		normalized = normalized[colonIndex+1:]
	}

	normalized = strings.TrimLeft(normalized, "/")
	if strings.HasPrefix(strings.ToLower(normalized), "textures/") {
		normalized = normalized[9:]
	}

	if strings.HasPrefix(strings.ToLower(normalized), "models/") {
		normalized = normalized[7:]
	}

	if strings.HasPrefix(strings.ToLower(normalized), "block/") {
		normalized = normalized[6:]
	} else if strings.HasPrefix(strings.ToLower(normalized), "blocks/") {
		normalized = normalized[7:]
	} else if strings.HasPrefix(strings.ToLower(normalized), "item/") {
		normalized = normalized[5:]
	} else if strings.HasPrefix(strings.ToLower(normalized), "items/") {
		normalized = normalized[6:]
	}

	normalized = strings.Trim(normalized, "/")
	slashIndex := strings.LastIndex(normalized, "/")
	if slashIndex >= 0 {
		normalized = normalized[slashIndex+1:]
	}

	return strings.ToLower(normalized)
}

func (renderer *MinecraftBlockRenderer) TryGetConstantTint(textureId string, blockName *string) *color.RGBA {
	textureKey := renderer.NormalizeResourceKey(&textureId)
	constantColors := BiomeTints.ConstantColors
	if color, found := constantColors[textureKey]; found {
		return &color
	}

	if blockName != nil {
		blockKey := renderer.NormalizeResourceKey(blockName)
		if color, found := constantColors[blockKey]; found {
			return &color
		}
	}

	return nil
}

func (renderer *MinecraftBlockRenderer) TryGetBiomeTintKind(textureId string, blockName string) *BiomeTintKind {
	textureKey := renderer.NormalizeResourceKey(&textureId)
	blockKey := renderer.NormalizeResourceKey(&blockName)
	isItemTexture := renderer.IsLikelyItemTexture(textureId)

	config := BiomeTints

	if isItemTexture {
		if _, found := config.ItemTintExclusions[textureKey]; found {
			return nil
		}
		if blockKey != "" {
			if _, found := config.ItemTintExclusions[blockKey]; found {
				return nil
			}
		}
	}

	if _, found := config.DryFoliageTextures[textureKey]; found {
		result := BiomeTintKindDryFoliage
		return &result
	}

	if blockKey != "" {
		if _, found := config.DryFoliageBlocks[blockKey]; found {
			result := BiomeTintKindDryFoliage
			return &result
		}
	}

	if _, found := config.GrassTextures[textureKey]; found {
		result := BiomeTintKindGrass
		return &result
	}
	if blockKey != "" {
		if _, found := config.GrassBlocks[blockKey]; found {
			result := BiomeTintKindGrass
			return &result
		}
	}

	if _, found := config.FoliageTextures[textureKey]; found {
		result := BiomeTintKindFoliage
		return &result
	}
	if blockKey != "" {
		if _, found := config.FoliageBlocks[blockKey]; found {
			result := BiomeTintKindFoliage
			return &result
		}
	}

	return nil
}

func (renderer *MinecraftBlockRenderer) IsLikelyItemTexture(identifier string) bool {
	if identifier == "" {
		return false
	}

	normalized := strings.ReplaceAll(identifier, "\\", "/")
	if strings.HasPrefix(strings.ToLower(normalized), "item/") ||
		strings.HasPrefix(strings.ToLower(normalized), "items/") {
		return true
	}

	if strings.HasPrefix(strings.ToLower(normalized), "textures/item/") {
		return true
	}

	lowerNormalized := strings.ToLower(normalized)
	return strings.Contains(lowerNormalized, "/item/") ||
		strings.Contains(lowerNormalized, ":item/") ||
		strings.Contains(lowerNormalized, "/items/") ||
		strings.Contains(lowerNormalized, ":items/")
}

func (renderer *MinecraftBlockRenderer) GetBiomeTintedTexture(textureId string, kind BiomeTintKind) *image.RGBA {
	cacheKey := fmt.Sprintf("%s|%d", renderer.NormalizeResourceKey(&textureId), kind)

	renderer._biomeTintedTextureMu.RLock()
	if renderer._biomeTintedTextureCache != nil {
		if cached, found := renderer._biomeTintedTextureCache[cacheKey]; found {
			renderer._biomeTintedTextureMu.RUnlock()
			return &cached
		}
	}
	renderer._biomeTintedTextureMu.RUnlock()

	renderer._biomeTintedTextureMu.Lock()
	if renderer._biomeTintedTextureCache == nil {
		renderer._biomeTintedTextureCache = make(map[string]image.RGBA)
	}
	renderer._biomeTintedTextureMu.Unlock()

	var colormap *image.RGBA
	switch kind {
	case BiomeTintKindGrass:
		colormap = renderer._textureRepository.GrassColorMap
	case BiomeTintKindFoliage:
		colormap = renderer._textureRepository.FoliageColorMap
	case BiomeTintKindDryFoliage:
		colormap = renderer._textureRepository.DryFoliageColorMap
	default:
		return renderer._textureRepository.GetTexture(textureId)
	}

	if colormap == nil {
		return renderer._textureRepository.GetTexture(textureId)
	}

	tintColor := SampleBiomeTintColor(colormap, kind)
	tinted := ApplyBiomeTint(renderer._textureRepository.GetTexture(textureId), tintColor)
	renderer._biomeTintedTextureMu.Lock()
	renderer._biomeTintedTextureCache[cacheKey] = *tinted
	renderer._biomeTintedTextureMu.Unlock()

	return tinted
}

func SampleBiomeTintColor(colormap *image.RGBA, kind BiomeTintKind) color.RGBA {
	coordinates, found := DefaultBiomeTintCoordinates[kind]
	if !found {
		coordinates = struct{ Temperature, Downfall float64 }{0.5, 1.0}
	}

	temperature := clampFloat64(coordinates.Temperature, 0, 1)
	downfall := clampFloat64(coordinates.Downfall, 0, 1)
	rainfall := clampFloat64(downfall*temperature, 0, 1)

	x := int(clampFloat64(float64(colormap.Bounds().Dx()-1)*(1-temperature), 0, float64(colormap.Bounds().Dx()-1)))
	y := int(clampFloat64(float64(colormap.Bounds().Dy()-1)*(1-rainfall), 0, float64(colormap.Bounds().Dy()-1)))

	return colormap.At(x, y).(color.RGBA)
}

func clampFloat64(v, low, high float64) float64 {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func ApplyBiomeTint(baseTexture *image.RGBA, tintColor color.RGBA) *image.RGBA {
	bounds := baseTexture.Bounds()
	tinted := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			originalPixel := baseTexture.At(x, y).(color.RGBA)
			if originalPixel.A == 0 {
				tinted.Set(x, y, originalPixel)
				continue
			}

			r := uint8(float64(originalPixel.R) * float64(tintColor.R) / 255)
			g := uint8(float64(originalPixel.G) * float64(tintColor.G) / 255)
			b := uint8(float64(originalPixel.B) * float64(tintColor.B) / 255)
			a := originalPixel.A

			tinted.Set(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	return tinted
}

func (renderer *MinecraftBlockRenderer) RenderBlock(blockName string, options BlockRenderOptions) *image.RGBA {
	if blockName == "" {
		return nil
	}

	effectiveOptions := MergeBlockRenderOptions(&options)
	rendererForOptions, forwardedOptions := renderer.ResolveRendererForOptions(effectiveOptions)
	return rendererForOptions.RenderBlockInternal(blockName, forwardedOptions)
}

func (renderer *MinecraftBlockRenderer) RenderBlockInternal(blockName string, options BlockRenderOptions) *image.RGBA {
	modelName := blockName
	if mappedModel, found := renderer._blockRegistry.TryGetModel(blockName); found && mappedModel != "" {
		modelName = mappedModel
	}

	model := renderer._modelResolver.Resolve(modelName)

	// Console.WriteLine($"Rendering block '{blockName}' using model '{modelName}' with {model.Elements.Count} elements.");
	// fmt.Printf("Rendering block '%s' using model '%s' with %d elements\n", blockName, modelName, len(model.Elements))

	return renderer.RenderModel(model, options, &blockName)
}

func (renderer *MinecraftBlockRenderer) RenderItem(itemName string, itemData *data.ItemRenderData, options *BlockRenderOptions) *image.RGBA {
	effectiveOptions := MergeBlockRenderOptions(options)
	if itemData != nil {
		effectiveOptions.ItemData = itemData
	}

	rendererForOptions, forwardedOptions := renderer.ResolveRendererForOptions(effectiveOptions)
	return rendererForOptions.RenderGuiItemInternal(itemName, &forwardedOptions, nil)
}
