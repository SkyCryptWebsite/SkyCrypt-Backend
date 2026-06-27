package minecraftblockrenderer

import (
	"encoding/base64"
	"fmt"
	texturepacks "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/TexturePacks"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/assets"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"image"
	"image/draw"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type BlockFaceRenderOptions struct {
	Size    int
	Face    data.BlockFaceDirection
	PackIds []string
}

type AnimatedRenderedFrame struct {
	Image      *image.RGBA
	ResourceId ResourceIdResult
	Index      int
	DurationMs int
}

type AnimatedRenderedResource struct {
	Image      *image.RGBA
	ResourceId ResourceIdResult
	Frames     []AnimatedRenderedFrame
}

func CreateFromDataDirectory(dataDirectory string) (*MinecraftBlockRenderer, error) {
	if strings.TrimSpace(dataDirectory) == "" {
		return nil, fmt.Errorf("dataDirectory cannot be empty")
	}
	if info, err := os.Stat(dataDirectory); err != nil || !info.IsDir() {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("dataDirectory is not a directory: %s", dataDirectory)
	}
	return CreateFromMinecraftAssets(dataDirectory, nil, nil), nil
}

func CreateFromResourceProvider(provider assets.ResourceProvider) (*MinecraftBlockRenderer, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider cannot be nil")
	}

	namespaceRegistry := assets.NewAssetNamespaceRegistry()
	if provider.DirectoryExists("assets") {
		dirs, err := provider.EnumerateDirectories("assets", "*", false)
		if err != nil {
			return nil, err
		}
		for _, dir := range dirs {
			namespace := dir
			if idx := strings.LastIndex(namespace, "/"); idx >= 0 {
				namespace = namespace[idx+1:]
			}
			if strings.TrimSpace(namespace) == "" {
				continue
			}
			namespaceProvider := assets.NewSubPathResourceProvider(provider, dir)
			namespaceRegistry.AddNamespaceWithProvider(namespace, provider.RootPath()+"/"+dir, VanillaPackId, true, namespaceProvider)
			if namespaceProvider.DirectoryExists("textures") {
				textureProvider := assets.NewSubPathResourceProvider(namespaceProvider, "textures")
				namespaceRegistry.AddNamespaceWithProvider(namespace, provider.RootPath()+"/"+dir+"/textures", VanillaPackId, true, textureProvider)
			}
		}
	} else {
		namespaceRegistry.AddNamespaceWithProvider("minecraft", provider.RootPath(), VanillaPackId, true, provider)
		if provider.DirectoryExists("textures") {
			textureProvider := assets.NewSubPathResourceProvider(provider, "textures")
			namespaceRegistry.AddNamespaceWithProvider("minecraft", provider.RootPath()+"/textures", VanillaPackId, true, textureProvider)
		}
	}

	assetsRoot := provider.RootPath()
	packContext := NewRenderPackContext(assetsRoot, nil, nil, "", nil, namespaceRegistry)
	overlayPaths := []string{}

	modelResolver := data.NewBlockModelResolver(make(map[string]data.BlockModelDefinition)).LoadFromMinecraftAssets(assetsRoot, &overlayPaths, &packContext.AssetNamespaces)
	blockRegistry := data.BlockRegistryInstance.LoadFromMinecraftAssets(assetsRoot, modelResolver.Definitions, overlayPaths, &packContext.AssetNamespaces)
	itemRegistry := data.NewItemRegistry().LoadFromMinecraftAssets(assetsRoot, modelResolver.Definitions, overlayPaths, &packContext.AssetNamespaces)
	textureRepository := data.NewTextureRepository(assetsRoot, nil, overlayPaths, packContext.AssetNamespaces)

	return NewMinecraftBlockRenderer(modelResolver, textureRepository, blockRegistry, itemRegistry, assetsRoot, nil, nil, *packContext), nil
}

func (renderer *MinecraftBlockRenderer) GetKnownBlockNames() []string {
	if renderer == nil || renderer._blockRegistry == nil {
		return nil
	}
	names := renderer._blockRegistry.GetAllBlockNames()
	sort.Strings(names)
	return names
}

func (renderer *MinecraftBlockRenderer) GetKnownItemNames() []string {
	if renderer == nil || renderer._itemRegistry == nil {
		return nil
	}
	names := renderer._itemRegistry.GetAllItemNames()
	sort.Strings(names)
	return names
}

func (renderer *MinecraftBlockRenderer) RenderBlockFace(blockName string, options BlockFaceRenderOptions) (*image.RGBA, error) {
	if renderer == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	if strings.TrimSpace(blockName) == "" {
		return nil, fmt.Errorf("blockName cannot be empty")
	}
	size := options.Size
	if size <= 0 {
		size = DefaultBlockRenderOptions().Size
	}

	blockOptions := DefaultBlockRenderOptions()
	blockOptions.Size = size
	blockOptions.PackIds = options.PackIds
	rendererForOptions, forwarded := renderer.ResolveRendererForOptions(blockOptions)

	modelName := blockName
	if rendererForOptions._blockRegistry != nil {
		if mapped, ok := rendererForOptions._blockRegistry.TryGetModel(blockName); ok && strings.TrimSpace(mapped) != "" {
			modelName = mapped
		}
	}
	model := rendererForOptions._modelResolver.Resolve(modelName)
	if model == nil {
		return nil, fmt.Errorf("model not found for block %s", blockName)
	}

	for _, element := range model.Elements {
		if face, ok := element.Faces[options.Face]; ok {
			textureId := ResolveTexture(face.Texture, model)
			return rendererForOptions.RenderGuiItemFromTextureId(textureId, &forwarded)
		}
	}

	return nil, fmt.Errorf("face %s not found on block %s", data.BlockFaceDirectionToString(options.Face), blockName)
}

func (renderer *MinecraftBlockRenderer) RenderGuiItemFromTextureId(textureId string, options *BlockRenderOptions) (*image.RGBA, error) {
	if renderer == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	if strings.TrimSpace(textureId) == "" {
		return nil, fmt.Errorf("textureId cannot be empty")
	}
	effective := MergeBlockRenderOptions(options)
	if fallbackItem, ok := resolveTextureIDFallbackItem(textureId); ok {
		rendered := renderer.RenderGuiItemWithResourceId(fallbackItem, &effective)
		if rendered == nil || rendered.Image == nil {
			return nil, fmt.Errorf("failed to render fallback item %s for texture id", fallbackItem)
		}
		return rendered.Image, nil
	}
	rendererForOptions, forwarded := renderer.ResolveRendererForOptions(effective)
	texture := rendererForOptions._textureRepository.GetTexture(textureId)
	if texture == nil || rendererForOptions._textureRepository.IsMissingTexture(texture) {
		return nil, fmt.Errorf("texture %q could not be resolved", textureId)
	}
	rendered, err := rendererForOptions.RenderFlatItem([]string{textureId}, forwarded, textureId)
	if err != nil {
		return nil, err
	}
	return &rendered, nil
}

func resolveTextureIDFallbackItem(textureId string) (string, bool) {
	decoded, ok := tryDecodeTextureDescriptor(textureId)
	if !ok {
		decoded = strings.TrimSpace(textureId)
	}
	if decoded == "" {
		return "", false
	}

	descriptor, rawQuery, _ := strings.Cut(decoded, "?")
	values, _ := url.ParseQuery(rawQuery)
	if base := strings.TrimSpace(values.Get("base")); base != "" {
		return base, true
	}

	lower := strings.ToLower(strings.TrimSpace(descriptor))
	if strings.HasPrefix(lower, "numeric:") {
		numeric := strings.TrimSpace(descriptor[len("numeric:"):])
		var id int
		if _, err := fmt.Sscanf(numeric, "%d", &id); err == nil {
			if mapped, ok := legacyNumericItemID(id, 0); ok {
				return "minecraft:" + mapped, true
			}
		}
	}

	if strings.HasPrefix(lower, "skyblock:") {
		return "", false
	}
	if strings.HasPrefix(lower, "custom:") {
		return "", false
	}
	if strings.HasPrefix(lower, "minecraft:") && !strings.Contains(lower, "/") {
		return descriptor, true
	}

	return "", false
}

func tryDecodeTextureDescriptor(textureId string) (string, bool) {
	trimmed := strings.TrimSpace(textureId)
	if trimmed == "" || strings.Contains(trimmed, ":") {
		return "", false
	}

	candidates := []string{
		strings.NewReplacer("-", "+", "_", "/").Replace(trimmed),
		trimmed,
	}
	for _, candidate := range candidates {
		padding := (4 - len(candidate)%4) % 4
		if padding > 0 {
			candidate += strings.Repeat("=", padding)
		}
		data, err := base64.StdEncoding.DecodeString(candidate)
		if err != nil {
			continue
		}
		decoded := strings.TrimSpace(string(data))
		if decoded == "" {
			continue
		}
		lower := strings.ToLower(decoded)
		if strings.HasPrefix(lower, "skyblock:") ||
			strings.HasPrefix(lower, "numeric:") ||
			strings.HasPrefix(lower, "custom:") ||
			strings.HasPrefix(lower, "minecraft:") {
			return decoded, true
		}
	}

	return "", false
}

func (renderer *MinecraftBlockRenderer) RenderAnimatedGuiItemWithResourceId(itemName string, options *BlockRenderOptions) (*AnimatedRenderedResource, error) {
	if renderer == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	effective := MergeBlockRenderOptions(options)
	rendererForOptions, forwarded := renderer.ResolveRendererForOptions(effective)
	base := rendererForOptions.RenderGuiItemWithResourceId(itemName, &forwarded)
	if base == nil {
		return nil, fmt.Errorf("failed to render item %s", itemName)
	}

	result := &AnimatedRenderedResource{
		Image:      base.Image,
		ResourceId: base.ResourceId,
	}

	animations := make(map[string]*data.TextureAnimation)
	maxFrames := 0
	for _, textureId := range base.ResourceId.Textures {
		if animation, ok := rendererForOptions._textureRepository.GetAnimation(textureId); ok && len(animation.Frames) > 0 {
			animations[rendererForOptions._textureRepository.NormalizeTextureId(textureId)] = animation
			if len(animation.Frames) > maxFrames {
				maxFrames = len(animation.Frames)
			}
		}
	}
	if maxFrames == 0 {
		result.Frames = []AnimatedRenderedFrame{{
			Image:      base.Image,
			ResourceId: base.ResourceId,
			Index:      0,
			DurationMs: 0,
		}}
		return result, nil
	}

	for frameIndex := 0; frameIndex < maxFrames; frameIndex++ {
		overrideFrames := make(map[string]data.TextureAnimationFrame)
		duration := 0
		for textureId, animation := range animations {
			frame := animation.Frames[frameIndex%len(animation.Frames)]
			overrideFrames[textureId] = frame
			if frame.DurationMs > duration {
				duration = frame.DurationMs
			}
		}

		var rendered *RenderedResource
		rendererForOptions._textureRepository.WithAnimationOverride(&data.AnimationOverride{
			Frames:         overrideFrames,
			CacheKeySuffix: fmt.Sprintf("frame-%d", frameIndex),
		}, func() {
			rendered = rendererForOptions.RenderGuiItemWithResourceId(itemName, &forwarded)
		})
		if rendered == nil {
			continue
		}
		result.Frames = append(result.Frames, AnimatedRenderedFrame{
			Image:      rendered.Image,
			ResourceId: rendered.ResourceId,
			Index:      frameIndex,
			DurationMs: duration,
		})
	}

	return result, nil
}

func (renderer *MinecraftBlockRenderer) ReloadResourcePacks() error {
	if renderer == nil || renderer._packRegistry == nil {
		return nil
	}
	oldSources := append([]texturepacks.RegistrationSource(nil), renderer._packRegistry.RegistrationSources...)
	registry := texturepacks.NewTexturePackRegistry()
	for _, source := range oldSources {
		if source.RegisterSinglePack {
			if _, err := registry.RegisterPack(source.Path); err != nil {
				return err
			}
			continue
		}
		registry.RegisterAllPacks(source.Path, source.SearchRecursively)
	}
	renderer._packRegistry = registry
	renderer._packRendererCacheMu.Lock()
	renderer._packRendererCache = make(map[string]*MinecraftBlockRenderer)
	renderer._packRendererCacheMu.Unlock()
	renderer._skyblockItemDefinitionsMu.Lock()
	renderer._skyblockItemDefinitions = nil
	renderer._skyblockItemDefinitionsMu.Unlock()
	return nil
}

func (renderer *MinecraftBlockRenderer) GetTexturePackIcon(packId string) (*image.RGBA, error) {
	if renderer == nil || renderer._packRegistry == nil {
		return nil, fmt.Errorf("renderer has no texture pack registry")
	}
	pack, ok := renderer._packRegistry.Packs[packId]
	if !ok {
		return nil, fmt.Errorf("unknown texture pack id %s", packId)
	}
	if pack.Provider != nil && pack.Provider.FileExists("pack.png") {
		reader, err := pack.Provider.OpenRead("pack.png")
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return decodeRGBA(reader)
	}
	iconPath := filepath.Join(pack.RootPath, "pack.png")
	if _, err := os.Stat(iconPath); err != nil {
		return nil, err
	}
	file, err := os.Open(iconPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return decodeRGBA(file)
}

func (renderer *MinecraftBlockRenderer) TextureIsMissing(textureID string) bool {
	if renderer == nil || renderer._textureRepository == nil {
		return true
	}
	texture := renderer._textureRepository.GetTexture(textureID)
	return texture == nil || renderer._textureRepository.IsMissingTexture(texture)
}

func decodeRGBA(reader io.Reader) (*image.RGBA, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, bounds.Min, draw.Src)
	return rgba, nil
}
