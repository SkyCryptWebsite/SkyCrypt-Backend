package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"
	"sync"
	"time"

	mr "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer"
	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	"golang.org/x/sync/singleflight"
)

var customResourceRenderer *mr.Renderer

var itemTextureCache = make(map[string]AppliedItemTexture)
var itemTextureCacheMu sync.RWMutex
var renderedSkyBlockIndex = make(map[string]struct{})
var renderedSkyBlockIndexMu sync.RWMutex
var renderedTextureIndexReloadMu sync.Mutex
var renderedTextureIndexCacheDir string
var renderedTextureIndexLastLazyReload time.Time
var renderedTextureIndexLastDirModTime time.Time
var renderedTextureIndexReloadInFlight bool
var customResourcesOnce sync.Once
var customResourcesErr error
var customResourceRendererMu sync.Mutex
var customResourceRendererConfigMu sync.RWMutex
var customResourceRendererConfig customResourceRendererSettings
var resourcePackConfigsOnce sync.Once
var resourcePackConfigs []models.ResourcePackConfig
var resourcePackConfigsErr error

var vanillaAssetsMu sync.RWMutex
var vanillaAssets *vanillaAssetIndex
var resolvedItemTextureCache = make(map[string]AppliedItemTexture)
var resolvedItemTextureCacheMu sync.RWMutex
var itemTextureResolutionGroup singleflight.Group

var renderedTextureIndexLazyReloadInterval = 5 * time.Second
var loadRenderedTextureIndexForRefresh = LoadRenderedTextureIndex

const (
	renderedResourcePackManifestSchemaVersion  = 1
	renderedResourcePackManifestRendererModule = "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer@v0.2.6|custom-model-pack-filter-v2"
	renderedResourcePackManifestFileName       = "resourcepacks-manifest.json"
)

type customResourceRendererSettings struct {
	CacheDir          string
	ResourcePacksPath string
	AssetsPath        string
}

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
	EnabledPackIDs       []string
	EnabledPackSet       map[string]struct{}
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

type renderedResourcePackManifest struct {
	SchemaVersion      int                         `json:"schemaVersion"`
	GeneratedAtUnix    int64                       `json:"generatedAtUnix"`
	RendererModule     string                      `json:"rendererModule"`
	ResourcePackHash   string                      `json:"resourcePackHash"`
	ItemIDHash         string                      `json:"itemIdHash"`
	ItemIDCount        int                         `json:"itemIdCount"`
	RenderedIndexCount int                         `json:"renderedIndexCount"`
	PackOrder          []string                    `json:"packOrder"`
	Packs              []renderedResourcePackEntry `json:"packs"`
}

type renderedResourcePackEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
	MetaHash    string `json:"metaHash,omitempty"`
	ArchiveHash string `json:"archiveHash,omitempty"`
	ArchiveName string `json:"archiveName,omitempty"`
}

type renderedResourcePackFingerprintEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	MetaHash    string `json:"metaHash,omitempty"`
	ArchiveHash string `json:"archiveHash,omitempty"`
	ArchiveName string `json:"archiveName,omitempty"`
}

type skyBlockTextureWarmResult struct {
	OutputPaths map[string]struct{}
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

func isPreforkChildProcess() bool {
	return os.Getenv("FIBER_PREFORK_CHILD") != ""
}

func logCustomResourceRoutine(format string, args ...any) {
	if isPreforkChildProcess() {
		if utility.IsVerboseLogging() || strings.EqualFold(os.Getenv("VERBOSE_LOGGING"), "true") {
			fmt.Printf(format+"\n", args...)
		}
		return
	}
	fmt.Printf(format+"\n", args...)
}

func resolveCustomResourceRendererConfig() (customResourceRendererSettings, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return customResourceRendererSettings{}, fmt.Errorf("failed to get current working directory: %v", err)
	}

	cacheDir := filepath.Join(cwd, "cache")
	resourcePacksPath := filepath.Join(cwd, "assets", "resourcepacks")
	return customResourceRendererSettings{
		CacheDir:          cacheDir,
		ResourcePacksPath: resourcePacksPath,
		AssetsPath:        filepath.Join(resourcePacksPath, "Vanilla", "assets", "minecraft"),
	}, nil
}

func rememberCustomResourceRendererConfig(config customResourceRendererSettings) {
	customResourceRendererConfigMu.Lock()
	customResourceRendererConfig = config
	customResourceRendererConfigMu.Unlock()
}

func currentCustomResourceRendererConfig() (customResourceRendererSettings, bool) {
	customResourceRendererConfigMu.RLock()
	config := customResourceRendererConfig
	customResourceRendererConfigMu.RUnlock()
	if strings.TrimSpace(config.CacheDir) == "" || strings.TrimSpace(config.ResourcePacksPath) == "" || strings.TrimSpace(config.AssetsPath) == "" {
		return customResourceRendererSettings{}, false
	}
	return config, true
}

func customResourceRendererConfigForInit() (customResourceRendererSettings, error) {
	if config, ok := currentCustomResourceRendererConfig(); ok {
		return config, nil
	}
	return customResourceRendererSettings{}, fmt.Errorf("[CUSTOM_RESOURCES] renderer config is not initialized")
}

func currentCustomResourceRenderer() *mr.Renderer {
	customResourceRendererMu.Lock()
	renderer := customResourceRenderer
	customResourceRendererMu.Unlock()
	return renderer
}

func PrepareCustomResourceCache() error {
	config, err := resolveCustomResourceRendererConfig()
	if err != nil {
		return err
	}
	rememberCustomResourceRendererConfig(config)

	renderedDir := filepath.Join(config.CacheDir, "rendered")
	timeNow := time.Now()
	loaded, err := reloadRenderedTextureIndex(config.CacheDir)
	if err != nil {
		return fmt.Errorf("[CUSTOM_RESOURCES] Failed to read cache directory: %v", err)
	}
	logCustomResourceRoutine("[CUSTOM_RESOURCES] Loaded %s cached textures from %s in %s", utility.AddCommas(loaded), renderedDir, time.Since(timeNow))
	return nil
}

func StartCustomResources() error {
	customResourcesOnce.Do(func() {
		customResourcesErr = startCustomResources()
	})
	return customResourcesErr
}

func startCustomResources() error {
	timeNow := time.Now()
	config, err := resolveCustomResourceRendererConfig()
	if err != nil {
		return err
	}
	rememberCustomResourceRendererConfig(config)
	if err := normalizeResourcePackJSON(config.ResourcePacksPath); err != nil {
		return fmt.Errorf("[CUSTOM_RESOURCES] Failed to normalize resource-pack JSON: %v", err)
	}

	cacheDir := config.CacheDir
	resourcePacksPath := config.ResourcePacksPath
	assetsPath := config.AssetsPath
	renderedDir := filepath.Join(cacheDir, "rendered")

	preloadRenderer := !isPreforkChildProcess()
	if err := InitRenderer(cacheDir, resourcePacksPath, assetsPath, preloadRenderer); err != nil {
		return err
	}

	logCustomResourceRoutine("[CUSTOM_RESOURCES] Started SkyCrypt renderer instance in %s", time.Since(timeNow))

	_, renderedDirErr := os.Stat(renderedDir)
	if renderedDirErr != nil && !os.IsNotExist(renderedDirErr) {
		return fmt.Errorf("[CUSTOM_RESOURCES] Failed to stat cache directory %s: %v", renderedDir, renderedDirErr)
	}
	renderedDirExists := renderedDirErr == nil

	timeNowv2 := time.Now()
	loaded, err := reloadRenderedTextureIndex(cacheDir)
	if err != nil {
		return fmt.Errorf("[CUSTOM_RESOURCES] Failed to read cache directory: %v", err)
	}
	logCustomResourceRoutine("[CUSTOM_RESOURCES] Loaded %s cached textures from %s in %s", utility.AddCommas(loaded), renderedDir, time.Since(timeNowv2))

	// Render textures only on main thread; generated files are shared in cache/rendered.
	if !isPreforkChildProcess() {
		itemIDs := preRenderSkyBlockItemIDs()
		manifest, err := buildRenderedResourcePackManifest(resourcePacksPath, itemIDs, loaded, time.Now())
		if err != nil {
			return fmt.Errorf("[CUSTOM_RESOURCES] Failed to build resource pack pre-render manifest: %v", err)
		}

		savedManifest, manifestErr := readRenderedResourcePackManifest(cacheDir)
		shouldSkip := false
		reason := ""
		if forceSkyBlockPreRender() {
			reason = "forced by SKYCRYPT_FORCE_RESOURCEPACK_PRERENDER"
		} else if manifestErr != nil {
			reason = fmt.Sprintf("manifest cannot be read: %v", manifestErr)
		} else {
			shouldSkip, reason = shouldSkipSkyBlockPreRender(manifest, savedManifest, loaded)
		}

		if shouldSkip {
			fmt.Printf("[CUSTOM_RESOURCES] Skipping SkyBlock pre-render; resource pack manifest is current (%s)\n", reason)
		} else {
			fmt.Printf("[CUSTOM_RESOURCES] SkyBlock pre-render required; %s\n", reason)
			warmResult, err := warmConfiguredSkyBlockTexturesWithResult(context.Background(), cacheDir, resourcePacksPath, assetsPath, itemIDs, mr.PreRenderOptions{
				ProgressWriter: os.Stdout,
				ShowProgress:   true,
				Overwrite:      true,
			})
			if err != nil {
				return err
			}

			removed, err := cleanupStaleRenderedPackFiles(renderedDir, defaultResourcePackIDs(), warmResult.OutputPaths)
			if err != nil {
				return fmt.Errorf("[CUSTOM_RESOURCES] Failed to clean stale rendered textures: %v", err)
			}
			if removed > 0 {
				fmt.Printf("[CUSTOM_RESOURCES] Removed %s stale rendered textures after resource pack warm\n", utility.AddCommas(removed))
			}

			reloaded, err := reloadRenderedTextureIndex(cacheDir)
			if err != nil {
				return fmt.Errorf("[CUSTOM_RESOURCES] Failed to reload rendered texture index after warm: %v", err)
			}
			manifest.RenderedIndexCount = reloaded
			manifest.GeneratedAtUnix = time.Now().Unix()
			if err := writeRenderedResourcePackManifest(cacheDir, manifest); err != nil {
				return fmt.Errorf("[CUSTOM_RESOURCES] Failed to write resource pack pre-render manifest: %v", err)
			}
			loaded = reloaded
			fmt.Printf("[CUSTOM_RESOURCES] Wrote resource pack pre-render manifest with %s indexed textures\n", utility.AddCommas(loaded))
		}
	} else if !renderedDirExists && os.IsNotExist(renderedDirErr) {
		fmt.Printf("[CUSTOM_RESOURCES] cache/rendered is missing; child process will serve fallbacks until the main process warms textures\n")
	}

	logCustomResourceRoutine("[CUSTOM_RESOURCES] SkyCrypt renderer initialized successfully with %s textures in %s", utility.AddCommas(itemTextureCacheLen()), time.Since(timeNow))

	return nil
}

func normalizeResourcePackJSON(resourcePacksPath string) error {
	return filepath.WalkDir(resourcePacksPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var decoded any
		if json.Unmarshal(data, &decoded) == nil {
			return nil
		}

		decoder := json.NewDecoder(bytes.NewReader(data))
		var firstValue json.RawMessage
		if err := decoder.Decode(&firstValue); err != nil || !json.Valid(firstValue) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(path, append(firstValue, '\n'), info.Mode())
	})
}

func InitRenderer(cacheDir string, resourcePacksPath string, assetsPath string, preload bool) error {
	config := customResourceRendererSettings{
		CacheDir:          cacheDir,
		ResourcePacksPath: resourcePacksPath,
		AssetsPath:        assetsPath,
	}
	rememberCustomResourceRendererConfig(config)
	return initCustomResourceRenderer(config, preload)
}

func initCustomResourceRenderer(config customResourceRendererSettings, preload bool) error {
	customResourceRendererMu.Lock()
	defer customResourceRendererMu.Unlock()

	renderer, err := newRendererForPackIDs(config.CacheDir, config.ResourcePacksPath, config.AssetsPath, defaultResourcePackIDs(), preload)
	if err != nil {
		return fmt.Errorf("[CUSTOM_RESOURCES] Failed to initialize SkyCrypt renderer: %v", err)
	}
	if renderer == nil {
		return fmt.Errorf("[CUSTOM_RESOURCES] Failed to initialize SkyCrypt renderer: renderer is nil")
	}
	customResourceRenderer = renderer
	return nil
}

func newRendererForPackIDs(cacheDir string, resourcePacksPath string, assetsPath string, packIDs []string, preload bool) (*mr.Renderer, error) {
	return mr.NewRendererWithOptions(mr.Options{
		AssetsRoot:        assetsPath,
		ResourcePacksRoot: resourcePacksPath,
		PackIDs:           packIDsForRenderer(packIDs),
		Preload:           preload,
		CacheDir:          cacheDir,
		VerboseLogging:    false,
	})
}

func ensureCustomResourceRenderer() error {
	if currentCustomResourceRenderer() != nil {
		return nil
	}

	config, err := customResourceRendererConfigForInit()
	if err != nil {
		return err
	}

	customResourceRendererMu.Lock()
	defer customResourceRendererMu.Unlock()
	if customResourceRenderer != nil {
		return nil
	}

	renderer, err := newRendererForPackIDs(config.CacheDir, config.ResourcePacksPath, config.AssetsPath, defaultResourcePackIDs(), false)
	if err != nil {
		return fmt.Errorf("[CUSTOM_RESOURCES] Failed to initialize SkyCrypt renderer: %v", err)
	}
	if renderer == nil {
		return fmt.Errorf("[CUSTOM_RESOURCES] Failed to initialize SkyCrypt renderer: renderer is nil")
	}
	customResourceRenderer = renderer
	return nil
}

func WarmCustomResourceRendererAsync(delay time.Duration) {
	go func() {
		if delay > 0 {
			time.Sleep(delay)
		}

		alreadyInitialized := currentCustomResourceRenderer() != nil
		timeNow := time.Now()
		if err := ensureCustomResourceRenderer(); err != nil {
			fmt.Printf("[CUSTOM_RESOURCES] Failed to warm SkyCrypt renderer: %v\n", err)
			return
		}
		if !alreadyInitialized {
			logCustomResourceRoutine("[CUSTOM_RESOURCES] SkyCrypt renderer initialized successfully in %s", time.Since(timeNow))
		}
	}()
}

func warmConfiguredSkyBlockTexturesWithResult(ctx context.Context, cacheDir string, resourcePacksPath string, assetsPath string, itemIDs []string, options mr.PreRenderOptions) (skyBlockTextureWarmResult, error) {
	result := skyBlockTextureWarmResult{OutputPaths: map[string]struct{}{}}
	for _, packID := range defaultResourcePackIDs() {
		renderer := customResourceRenderer
		if len(rendererPackIDs(renderer)) != 1 || rendererPackIDs(renderer)[0] != packID {
			var err error
			renderer, err = newRendererForPackIDs(cacheDir, resourcePacksPath, assetsPath, []string{packID}, false)
			if err != nil {
				return result, fmt.Errorf("[CUSTOM_RESOURCES] Failed to initialize %s renderer: %v", packID, err)
			}
		}

		packResult, err := warmMissingSkyBlockTexturesWithRendererResult(ctx, renderer, []string{packID}, itemIDs, options)
		if err != nil {
			return result, err
		}
		for path := range packResult.OutputPaths {
			result.OutputPaths[path] = struct{}{}
		}
	}
	return result, nil
}

func warmMissingSkyBlockTexturesWithRendererResult(ctx context.Context, renderer *mr.Renderer, packIDs []string, itemIDs []string, options mr.PreRenderOptions) (skyBlockTextureWarmResult, error) {
	result := skyBlockTextureWarmResult{OutputPaths: map[string]struct{}{}}
	if ctx == nil {
		ctx = context.Background()
	}
	if renderer == nil {
		return result, fmt.Errorf("[CUSTOM_RESOURCES] renderer is not initialized")
	}
	packSignature := packSignatureFromPackIDs(packIDs)
	indexPackID := packSignature
	if len(packIDs) == 1 {
		indexPackID = strings.TrimSpace(packIDs[0])
	}

	missing := skyBlockItemIDsToPreRender(itemIDs, indexPackID, options.Overwrite)

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
		return result, nil
	}

	output, err := renderer.PreRenderSkyBlockItemIDs(ctx, missing, options)
	if err != nil {
		return result, fmt.Errorf("[CUSTOM_RESOURCES] Failed to pre-render SkyBlock items for packs %s: %v", packSignature, err)
	}

	for _, item := range output.Entries {
		if item.Error == "" && !item.Skipped {
			if strings.TrimSpace(item.Path) != "" {
				result.OutputPaths[renderedOutputPathKey(item.Path)] = struct{}{}
			}
			setCachedTextureForStableKey(packSignature, "skyblock:"+item.InputID, AppliedItemTexture{
				Texture:     publicCacheTextureURL(item.Path),
				TexturePack: item.TexturePackID,
			})
		}
	}

	fmt.Printf("[CUSTOM_RESOURCES] Pre-rendered %d SkyBlock items for packs %s, skipped %d, failed %d in %s\n", output.Succeeded, packSignature, output.Skipped, output.Failed, time.Since(timeNow))
	return result, nil
}

func skyBlockItemIDsToPreRender(itemIDs []string, indexPackID string, overwrite bool) []string {
	missing := make([]string, 0, len(itemIDs))
	for _, id := range itemIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !overwrite && renderedSkyBlockIDKnown(id, indexPackID) {
			continue
		}
		missing = append(missing, id)
	}
	return missing
}

func rendererPackIDs(renderer *mr.Renderer) []string {
	if renderer == nil {
		return nil
	}
	return packIDsForRenderer(normalizePackIDs(renderer.PackIDs()))
}
