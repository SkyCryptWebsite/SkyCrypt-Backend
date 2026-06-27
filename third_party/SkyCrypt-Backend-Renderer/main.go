package renderer

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	mbr "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/MinecraftBlockRenderer"
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	texturepacks "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/TexturePacks"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/imagecache"
)

const (
	DefaultSize = 128
)

type Options struct {
	AssetsRoot        string
	ResourcePacksRoot string
	PackIDs           []string
	Size              int
	Preload           bool
	CacheDir          string
}

type RenderedItem struct {
	Path          string
	TexturePackID string
}

type PreRenderOptions struct {
	Workers   int
	Overwrite bool
}

type PreRenderedItem struct {
	InputID       string
	Path          string
	TexturePackID string
	Error         string
	Skipped       bool
}

type PreRenderResult struct {
	Entries   []PreRenderedItem
	Succeeded int
	Failed    int
	Skipped   int
}

type Renderer struct {
	renderer *mbr.MinecraftBlockRenderer
	packIDs  []string
	size     int
	cacheDir string

	renderCache   map[string]RenderedItem
	renderCacheMu sync.RWMutex
	inflight      map[string]*renderInflight
	inflightMu    sync.Mutex
}

type renderInflight struct {
	wg   sync.WaitGroup
	item *RenderedItem
	err  error
}

type renderDebugInfo struct {
	SkyBlockID  string
	MinecraftID string
	ItemModel   string
	DisplayName string
}

func NewRenderer(options Options) (*Renderer, error) {
	return NewRendererWithOptions(options)
}

func NewRendererWithOptions(options Options) (*Renderer, error) {
	if options.AssetsRoot == "" {
		return nil, fmt.Errorf("assets root is required")
	}
	if options.ResourcePacksRoot == "" {
		return nil, fmt.Errorf("resource packs root is required")
	}
	if len(options.PackIDs) == 0 {
		return nil, fmt.Errorf("at least one pack id is required")
	}
	cacheDir := strings.TrimSpace(options.CacheDir)
	if cacheDir == "" {
		return nil, fmt.Errorf("cache dir is required")
	}

	size := options.Size
	if size <= 0 {
		size = DefaultSize
	}

	packIDs := append([]string(nil), options.PackIDs...)

	registry := texturepacks.NewTexturePackRegistry()
	registry.RegisterAllPacks(options.ResourcePacksRoot, false)

	if err := imagecache.EnsureCacheVersion(cacheDir, imagecache.CacheFormatVersion, "rendered", "player_skins", "derived"); err != nil {
		return nil, err
	}

	blockRenderer := mbr.CreateFromMinecraftAssets(options.AssetsRoot, registry, packIDs)
	blockRenderer.SetCacheDirectory(cacheDir)
	if options.Preload {
		blockRenderer.PreloadTexturePackStacks([][]string{packIDs})
	}

	return &Renderer{
		renderer:      blockRenderer,
		packIDs:       packIDs,
		size:          size,
		cacheDir:      cacheDir,
		renderCache:   make(map[string]RenderedItem),
		inflight:      make(map[string]*renderInflight),
		renderCacheMu: sync.RWMutex{},
		inflightMu:    sync.Mutex{},
	}, nil
}

func (r *Renderer) Size() int {
	if r == nil || r.size <= 0 {
		return DefaultSize
	}
	return r.size
}

func (r *Renderer) PackIDs() []string {
	if r == nil {
		return nil
	}
	return append([]string(nil), r.packIDs...)
}

func (r *Renderer) MinecraftRenderer() *mbr.MinecraftBlockRenderer {
	if r == nil {
		return nil
	}
	return r.renderer
}

func (r *Renderer) RenderSkyBlockItemID(id string) (*RenderedItem, error) {
	return r.cachedSkyBlockItemID(id, false)
}

func (r *Renderer) RenderSkyBlockItemIDWithPackIDs(id string, packIDs []string) (*RenderedItem, error) {
	if len(packIDs) == 0 {
		return r.RenderSkyBlockItemID(id)
	}
	return r.cachedSkyBlockItemIDWithPackIDs(id, packIDs, false)
}

func (r *Renderer) RenderItemNBT(item any) (*RenderedItem, error) {
	return r.RenderItemNBTWithPackIDs(item, r.PackIDs())
}

func (r *Renderer) RenderItemNBTWithPackIDs(item any, packIDs []string) (*RenderedItem, error) {
	if r == nil || r.renderer == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	options := r.renderOptionsWithPackIDs(packIDs)
	debugInfo := debugInfoFromItemInput(item)
	return r.cachedRenderedItem(
		false,
		debugInfo,
		func() (*mbr.ResourceIdResult, error) {
			return r.renderer.ComputeResourceIdFromNBT(item, options)
		},
		func() (*mbr.AnimatedRenderedResource, error) {
			return r.renderer.RenderAnimatedItemNBT(item, options)
		},
	)
}

func (r *Renderer) FileFromSkyBlockItemID(id string) (*RenderedItem, error) {
	return r.RenderSkyBlockItemID(id)
}

func (r *Renderer) FileFromNBT(item any) (*RenderedItem, error) {
	return r.RenderItemNBT(item)
}

func (r *Renderer) PreRenderSkyBlockItemIDs(ctx context.Context, ids []string, options PreRenderOptions) (*PreRenderResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.renderer == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	if _, err := r.requireCacheDir(); err != nil {
		return nil, err
	}

	workers := options.Workers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers < 1 {
		workers = 1
	}

	result := &PreRenderResult{
		Entries: make([]PreRenderedItem, len(ids)),
	}
	indexesByID := make(map[string][]int)
	uniqueIDs := make([]string, 0, len(ids))
	for index, id := range ids {
		trimmed := strings.TrimSpace(id)
		result.Entries[index] = PreRenderedItem{InputID: id}
		if trimmed == "" {
			result.Entries[index].Error = "skyBlockItemID cannot be empty"
			continue
		}
		if _, exists := indexesByID[trimmed]; !exists {
			uniqueIDs = append(uniqueIDs, trimmed)
		}
		indexesByID[trimmed] = append(indexesByID[trimmed], index)
	}

	type renderResult struct {
		inputID string
		item    *RenderedItem
		err     error
		skipped bool
	}

	jobs := make(chan string)
	results := make(chan renderResult)
	workerCount := minInt(workers, maxInt(1, len(uniqueIDs)))
	var wg sync.WaitGroup

	renderOne := func(id string) renderResult {
		if err := ctx.Err(); err != nil {
			return renderResult{inputID: id, err: err}
		}
		item, skipped, err := r.preRenderSkyBlockItemID(id, options.Overwrite)
		return renderResult{inputID: id, item: item, skipped: skipped, err: err}
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				results <- renderOne(id)
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, id := range uniqueIDs {
			if ctx.Err() != nil {
				return
			}
			select {
			case jobs <- id:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	completedByID := make(map[string]renderResult)
	for rendered := range results {
		completedByID[rendered.inputID] = rendered
	}

	cancelErr := ctx.Err()
	for _, id := range uniqueIDs {
		rendered, completed := completedByID[id]
		if !completed {
			rendered = renderResult{inputID: id, err: cancelErr}
			if rendered.err == nil {
				rendered.err = context.Canceled
			}
		}

		for _, index := range indexesByID[id] {
			entry := result.Entries[index]
			if rendered.item != nil {
				entry.Path = rendered.item.Path
				entry.TexturePackID = rendered.item.TexturePackID
			}
			if rendered.err != nil {
				entry.Error = rendered.err.Error()
			}
			entry.Skipped = rendered.skipped
			result.Entries[index] = entry
		}
	}

	for _, entry := range result.Entries {
		if entry.Error != "" {
			result.Failed++
		} else if entry.Skipped {
			result.Skipped++
		} else if strings.TrimSpace(entry.Path) != "" {
			result.Succeeded++
		}
	}

	if cancelErr != nil {
		return result, cancelErr
	}
	return result, nil
}

func (r *Renderer) renderOptions() *mbr.BlockRenderOptions {
	return r.renderOptionsWithPackIDs(r.PackIDs())
}

func (r *Renderer) renderOptionsWithPackIDs(packIDs []string) *mbr.BlockRenderOptions {
	options := mbr.DefaultBlockRenderOptions()
	options.Size = r.Size()
	options.PackIds = append([]string(nil), packIDs...)
	return &options
}

func (r *Renderer) requireCacheDir() (string, error) {
	if r == nil || r.renderer == nil {
		return "", fmt.Errorf("renderer is nil")
	}
	cacheDir := strings.TrimSpace(r.cacheDir)
	if cacheDir == "" {
		return "", fmt.Errorf("cache dir is required")
	}
	return cacheDir, nil
}

func (r *Renderer) cachedSkyBlockItemID(id string, overwrite bool) (*RenderedItem, error) {
	return r.cachedSkyBlockItemIDWithPackIDs(id, r.PackIDs(), overwrite)
}

func (r *Renderer) cachedSkyBlockItemIDWithPackIDs(id string, packIDs []string, overwrite bool) (*RenderedItem, error) {
	if r == nil || r.renderer == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	options := r.renderOptionsWithPackIDs(packIDs)
	return r.cachedRenderedItem(
		overwrite,
		&renderDebugInfo{SkyBlockID: id, MinecraftID: "minecraft:player_head"},
		func() (*mbr.ResourceIdResult, error) {
			return r.renderer.ComputeResourceIdFromSkyBlockItemID(id, options)
		},
		func() (*mbr.AnimatedRenderedResource, error) {
			return r.renderer.RenderAnimatedSkyBlockItemID(id, options)
		},
	)
}

func (r *Renderer) preRenderSkyBlockItemID(id string, overwrite bool) (*RenderedItem, bool, error) {
	return r.preRenderSkyBlockItemIDWithPackIDs(id, r.PackIDs(), overwrite)
}

func (r *Renderer) preRenderSkyBlockItemIDWithPackIDs(id string, packIDs []string, overwrite bool) (*RenderedItem, bool, error) {
	if r == nil || r.renderer == nil {
		return nil, false, fmt.Errorf("renderer is nil")
	}
	options := r.renderOptionsWithPackIDs(packIDs)
	return r.cachedRenderedItemWithSkip(
		overwrite,
		&renderDebugInfo{SkyBlockID: id, MinecraftID: "minecraft:player_head"},
		func() (*mbr.ResourceIdResult, error) {
			return r.renderer.ComputeResourceIdFromSkyBlockItemID(id, options)
		},
		func() (*mbr.AnimatedRenderedResource, error) {
			return r.renderer.RenderAnimatedSkyBlockItemID(id, options)
		},
		r.shouldSkipPreRenderedSkyBlockResource(id),
	)
}

func (r *Renderer) cachedRenderedItem(overwrite bool, debugInfo *renderDebugInfo, compute func() (*mbr.ResourceIdResult, error), render func() (*mbr.AnimatedRenderedResource, error)) (*RenderedItem, error) {
	item, _, err := r.cachedRenderedItemWithSkip(overwrite, debugInfo, compute, render, nil)
	return item, err
}

func (r *Renderer) cachedRenderedItemWithSkip(
	overwrite bool,
	debugInfo *renderDebugInfo,
	compute func() (*mbr.ResourceIdResult, error),
	render func() (*mbr.AnimatedRenderedResource, error),
	shouldSkip func(*mbr.ResourceIdResult) (bool, string),
) (*RenderedItem, bool, error) {
	cacheDir, err := r.requireCacheDir()
	if err != nil {
		return nil, false, err
	}
	resourceID, err := compute()
	if err != nil {
		return nil, false, err
	}
	if resourceID == nil || strings.TrimSpace(resourceID.ResourceId) == "" {
		return nil, false, fmt.Errorf("resource id is empty")
	}
	if shouldSkip != nil {
		if skip, reason := shouldSkip(resourceID); skip {
			if strings.TrimSpace(reason) != "" {
				fmt.Println(reason)
			}
			return nil, true, nil
		}
	}
	if err := r.validateResolvedTextures(resourceID, debugInfo); err != nil {
		return nil, false, err
	}

	key := resourceID.ResourceId
	target := &RenderedItem{
		Path:          renderedDebugWebPPath(cacheDir, resourceID, debugInfo),
		TexturePackID: sourcePackID(resourceID),
	}

	if !overwrite {
		if cached := r.lookupRenderedItem(key); cached != nil {
			return cached, false, nil
		}
		if _, statErr := os.Stat(target.Path); statErr == nil {
			r.rememberRenderedItem(key, target)
			return cloneRenderedItem(target), false, nil
		} else if !os.IsNotExist(statErr) {
			return nil, false, statErr
		}
	}

	inflight, owner := r.beginRender(key)
	if !owner {
		inflight.wg.Wait()
		if inflight.err != nil {
			return nil, false, inflight.err
		}
		return cloneRenderedItem(inflight.item), false, nil
	}
	defer r.finishRender(key, inflight)

	if !overwrite {
		if cached := r.lookupRenderedItem(key); cached != nil {
			inflight.item = cached
			return cloneRenderedItem(cached), false, nil
		}
		if _, statErr := os.Stat(target.Path); statErr == nil {
			r.rememberRenderedItem(key, target)
			inflight.item = target
			return cloneRenderedItem(target), false, nil
		} else if !os.IsNotExist(statErr) {
			inflight.err = statErr
			return nil, false, statErr
		}
	}

	rendered, err := render()
	if err != nil {
		inflight.err = err
		return nil, false, err
	}
	if err := writeRenderedWebP(target.Path, rendered); err != nil {
		inflight.err = err
		return nil, false, err
	}

	r.rememberRenderedItem(key, target)
	inflight.item = target
	return cloneRenderedItem(target), false, nil
}

func (r *Renderer) validateResolvedTextures(resourceID *mbr.ResourceIdResult, debugInfo *renderDebugInfo) error {
	if r == nil || r.renderer == nil || resourceID == nil {
		return nil
	}
	skyBlockID := ""
	minecraftID := ""
	if debugInfo != nil {
		skyBlockID = debugInfo.SkyBlockID
		minecraftID = debugInfo.MinecraftID
	}
	for _, textureID := range resourceID.Textures {
		if r.renderer.TextureIsMissing(textureID) {
			return fmt.Errorf(
				"resolved missing texture %q for skyblock_id=%q minecraft_id=%q model=%s",
				textureID,
				skyBlockID,
				minecraftID,
				debugStringValue(resourceID.Model),
			)
		}
	}
	return nil
}

func (r *Renderer) lookupRenderedItem(key string) *RenderedItem {
	r.renderCacheMu.RLock()
	cached, found := r.renderCache[key]
	r.renderCacheMu.RUnlock()
	if !found {
		return nil
	}
	return &RenderedItem{Path: cached.Path, TexturePackID: cached.TexturePackID}
}

func (r *Renderer) rememberRenderedItem(key string, item *RenderedItem) {
	if item == nil {
		return
	}
	r.renderCacheMu.Lock()
	if r.renderCache == nil {
		r.renderCache = make(map[string]RenderedItem)
	}
	r.renderCache[key] = *item
	r.renderCacheMu.Unlock()
}

func (r *Renderer) beginRender(key string) (*renderInflight, bool) {
	r.inflightMu.Lock()
	defer r.inflightMu.Unlock()
	if r.inflight == nil {
		r.inflight = make(map[string]*renderInflight)
	}
	if existing, found := r.inflight[key]; found {
		return existing, false
	}
	inflight := &renderInflight{}
	inflight.wg.Add(1)
	r.inflight[key] = inflight
	return inflight, true
}

func (r *Renderer) finishRender(key string, inflight *renderInflight) {
	inflight.wg.Done()
	r.inflightMu.Lock()
	if r.inflight[key] == inflight {
		delete(r.inflight, key)
	}
	r.inflightMu.Unlock()
}

func renderedWebPPath(cacheDir string, resourceID string) string {
	return filepath.Join(cacheDir, "rendered", resourceID+".webp")
}

func renderedDebugWebPPath(cacheDir string, resourceID *mbr.ResourceIdResult, debugInfo *renderDebugInfo) string {
	if resourceID == nil {
		return renderedWebPPath(cacheDir, "unknown")
	}

	var parts []string
	if debugInfo != nil {
		if strings.TrimSpace(debugInfo.SkyBlockID) != "" {
			parts = append(parts, "skyblock="+debugInfo.SkyBlockID)
		}
		if strings.TrimSpace(debugInfo.MinecraftID) != "" {
			parts = append(parts, "mc="+debugInfo.MinecraftID)
		}
		if strings.TrimSpace(debugInfo.ItemModel) != "" {
			parts = append(parts, "itemmodel="+debugInfo.ItemModel)
		}
		if strings.TrimSpace(debugInfo.DisplayName) != "" {
			parts = append(parts, "name="+debugInfo.DisplayName)
		}
	}
	if strings.TrimSpace(resourceID.SourcePackId) != "" {
		parts = append(parts, "pack="+resourceID.SourcePackId)
	}
	if resourceID.Model != nil && strings.TrimSpace(*resourceID.Model) != "" {
		parts = append(parts, "model="+*resourceID.Model)
	}
	for i, texture := range resourceID.Textures {
		if i >= 2 {
			break
		}
		if strings.TrimSpace(texture) != "" {
			parts = append(parts, fmt.Sprintf("tex%d=%s", i+1, texture))
		}
	}

	hash := strings.TrimSpace(resourceID.ResourceId)
	if len(hash) > 12 {
		hash = hash[:12]
	}
	parts = append(parts, "hash="+hash)

	filename := sanitizeDebugFilename(strings.Join(parts, "__"))
	if filename == "" {
		filename = sanitizeDebugFilename(resourceID.ResourceId)
	}
	return filepath.Join(cacheDir, "rendered", filename+".webp")
}

func debugInfoFromItemInput(item any) *renderDebugInfo {
	if info := debugInfoFromNbtCompoundInput(item); info != nil {
		return info
	}

	normalized, err := data.NormalizeItemInput(item)
	if err != nil || normalized == nil {
		return nil
	}

	info := &renderDebugInfo{
		SkyBlockID:  normalized.SkyblockID,
		MinecraftID: normalized.ItemID,
		ItemModel:   normalized.ItemModel,
		DisplayName: normalized.DisplayName,
	}
	if info.MinecraftID == "" && normalized.NumericID != nil {
		info.MinecraftID = fmt.Sprintf("numeric_%d", *normalized.NumericID)
	}
	if strings.TrimSpace(info.SkyBlockID) == "" &&
		strings.TrimSpace(info.MinecraftID) == "" &&
		strings.TrimSpace(info.ItemModel) == "" &&
		strings.TrimSpace(info.DisplayName) == "" {
		return nil
	}
	return info
}

func debugInfoFromNbtCompoundInput(item any) *renderDebugInfo {
	var compound *nbt.NbtCompound
	switch typed := item.(type) {
	case *nbt.NbtCompound:
		compound = typed
	case nbt.NbtCompound:
		compound = &typed
	default:
		return nil
	}
	if compound == nil {
		return nil
	}

	info := &renderDebugInfo{}
	if id, ok := nbtCompoundString(compound, "id"); ok {
		info.MinecraftID = id
	}
	if tag, ok := nbtCompoundChild(compound, "tag"); ok {
		if extra, ok := nbtCompoundChild(tag, "ExtraAttributes"); ok {
			if skyblockID, ok := nbtCompoundString(extra, "id"); ok {
				info.SkyBlockID = skyblockID
			}
		}
		if display, ok := nbtCompoundChild(tag, "display"); ok {
			if name, ok := nbtCompoundString(display, "Name"); ok {
				info.DisplayName = name
			}
		}
		if model, ok := nbtCompoundString(tag, "ItemModel"); ok {
			info.ItemModel = model
		}
	}

	if strings.TrimSpace(info.SkyBlockID) == "" &&
		strings.TrimSpace(info.MinecraftID) == "" &&
		strings.TrimSpace(info.ItemModel) == "" &&
		strings.TrimSpace(info.DisplayName) == "" {
		return nil
	}
	return info
}

func nbtCompoundChild(compound *nbt.NbtCompound, key string) (*nbt.NbtCompound, bool) {
	if compound == nil {
		return nil, false
	}
	tag, ok := compound.Get(key)
	if !ok {
		return nil, false
	}
	switch typed := tag.(type) {
	case *nbt.NbtCompound:
		return typed, true
	default:
		return nil, false
	}
}

func nbtCompoundString(compound *nbt.NbtCompound, key string) (string, bool) {
	if compound == nil {
		return "", false
	}
	tag, ok := compound.Get(key)
	if !ok {
		return "", false
	}
	switch typed := tag.(type) {
	case nbt.NbtString:
		return strings.TrimSpace(typed.Value), strings.TrimSpace(typed.Value) != ""
	case *nbt.NbtString:
		if typed == nil {
			return "", false
		}
		return strings.TrimSpace(typed.Value), strings.TrimSpace(typed.Value) != ""
	default:
		return "", false
	}
}

func sanitizeDebugFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(value))
	lastWasUnderscore := false
	for _, r := range value {
		allowed := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' || r == '='
		if allowed {
			builder.WriteRune(r)
			lastWasUnderscore = false
			continue
		}
		if !lastWasUnderscore {
			builder.WriteByte('_')
			lastWasUnderscore = true
		}
	}

	filename := strings.Trim(builder.String(), "._-")
	const maxFilenameLength = 220
	if len(filename) > maxFilenameLength {
		filename = filename[:maxFilenameLength]
		filename = strings.Trim(filename, "._-")
	}
	return filename
}

func sourcePackID(resourceID *mbr.ResourceIdResult) string {
	if resourceID == nil || strings.TrimSpace(resourceID.SourcePackId) == "" {
		return mbr.VanillaPackId
	}
	return resourceID.SourcePackId
}

func (r *Renderer) shouldSkipPreRenderedSkyBlockResource(id string) func(*mbr.ResourceIdResult) (bool, string) {
	expectedPacks := make(map[string]struct{}, len(r.packIDs))
	for _, packID := range r.packIDs {
		trimmed := strings.TrimSpace(packID)
		if trimmed != "" {
			expectedPacks[trimmed] = struct{}{}
		}
	}

	return func(resourceID *mbr.ResourceIdResult) (bool, string) {
		source := sourcePackID(resourceID)
		if _, ok := expectedPacks[source]; !ok {
			return true, fmt.Sprintf(
				"warning: skyblock item %q was skipped during pre-render because no configured custom texture/model resolved; source=%q model=%s textures=%s",
				id,
				source,
				debugStringValue(resourceID.Model),
				strings.Join(resourceID.Textures, ","),
			)
		}

		for _, textureID := range resourceID.Textures {
			if r.renderer.TextureIsMissing(textureID) {
				return true, fmt.Sprintf(
					"warning: skyblock item %q was skipped during pre-render because texture %q could not be resolved; source=%q model=%s textures=%s",
					id,
					textureID,
					source,
					debugStringValue(resourceID.Model),
					strings.Join(resourceID.Textures, ","),
				)
			}
		}

		if len(resourceID.Textures) == 0 {
			return true, fmt.Sprintf(
				"warning: skyblock item %q was skipped during pre-render because it resolved no textures; source=%q model=%s",
				id,
				source,
				debugStringValue(resourceID.Model),
			)
		}

		return false, ""
	}
}

func debugStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func writeRenderedWebP(targetPath string, rendered *mbr.AnimatedRenderedResource) error {
	if rendered == nil {
		return fmt.Errorf("rendered resource is nil")
	}

	frames := rendered.Frames
	if len(frames) == 0 {
		if rendered.Image == nil {
			return fmt.Errorf("rendered image is nil")
		}
		return imagecache.WriteWebPAtomic(targetPath, rendered.Image)
	}

	if len(frames) == 1 {
		if frames[0].Image == nil {
			return fmt.Errorf("rendered frame image is nil")
		}
		return imagecache.WriteWebPAtomic(targetPath, frames[0].Image)
	}

	images := make([]image.Image, 0, len(frames))
	durations := make([]uint, 0, len(frames))
	for _, frame := range frames {
		if frame.Image == nil {
			return fmt.Errorf("rendered frame image is nil")
		}
		duration := frame.DurationMs
		if duration <= 0 {
			duration = 50
		}
		images = append(images, frame.Image)
		durations = append(durations, uint(duration))
	}
	return imagecache.WriteAnimatedWebPAtomic(targetPath, images, durations)
}

func cloneRenderedItem(item *RenderedItem) *RenderedItem {
	if item == nil {
		return nil
	}
	return &RenderedItem{
		Path:          item.Path,
		TexturePackID: item.TexturePackID,
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
