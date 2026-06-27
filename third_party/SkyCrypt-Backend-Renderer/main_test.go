package renderer

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	mbr "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/MinecraftBlockRenderer"
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	texturepacks "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/TexturePacks"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/imagecache"
)

func TestNewRendererRequiresPackIDs(t *testing.T) {
	_, err := NewRenderer(Options{
		AssetsRoot:        "assets",
		ResourcePacksRoot: "resourcepacks",
		CacheDir:          t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected missing pack IDs to return an error")
	}
}

func TestNewRendererRequiresCacheDir(t *testing.T) {
	_, err := NewRenderer(Options{
		AssetsRoot:        "assets",
		ResourcePacksRoot: "resourcepacks",
		PackIDs:           []string{"testpack"},
	})
	if err == nil || !strings.Contains(err.Error(), "cache dir is required") {
		t.Fatalf("expected cache dir error, got %v", err)
	}
}

func TestNewRendererPurgesOldCacheVersion(t *testing.T) {
	assetsRoot := createRootMinimalAssets(t)
	packRoot := createRootSkyblockPack(t, "testpack")
	cacheDir := t.TempDir()
	stalePath := filepath.Join(cacheDir, "rendered", "stale.webp")
	if err := os.MkdirAll(filepath.Dir(stalePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stalePath, []byte("old-bad-cache"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := NewRenderer(Options{
		AssetsRoot:        assetsRoot,
		ResourcePacksRoot: filepath.Dir(packRoot),
		PackIDs:           []string{"testpack"},
		CacheDir:          cacheDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("old cache file was not purged, err=%v", err)
	}
}

func TestRendererPackIDsReturnsCopy(t *testing.T) {
	source := []string{"fsr", "hplus"}
	renderer := &Renderer{packIDs: append([]string(nil), source...)}

	packIDs := renderer.PackIDs()
	packIDs[0] = "changed"

	if got := renderer.PackIDs()[0]; got != "fsr" {
		t.Fatalf("PackIDs exposed internal slice, got %q", got)
	}
}

func TestRenderSkyBlockItemIDWritesWebPCache(t *testing.T) {
	renderer := newTestRenderer(t)

	item, err := renderer.RenderSkyBlockItemID("TEST_ITEM")
	if err != nil {
		t.Fatal(err)
	}
	assertRenderedItem(t, renderer, item, "testpack")
}

func TestRenderItemNBTWritesWebPCache(t *testing.T) {
	renderer := newTestRenderer(t)

	expectedResource, err := renderer.MinecraftRenderer().ComputeResourceIdFromSkyBlockItemID("TEST_ITEM", renderer.renderOptions())
	if err != nil {
		t.Fatal(err)
	}

	input := map[string]any{
		"id": "minecraft:player_head",
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id": "TEST_ITEM",
			},
		},
	}
	expectedPath := renderedDebugWebPPath(renderer.cacheDir, expectedResource, debugInfoFromItemInput(input))

	item, err := renderer.RenderItemNBT(input)
	if err != nil {
		t.Fatal(err)
	}
	if item.Path != expectedPath {
		t.Fatalf("RenderItemNBT path = %q, want SkyBlock ID path %q", item.Path, expectedPath)
	}
	assertRenderedItem(t, renderer, item, sourcePackID(expectedResource))
}

func TestRenderItemNBTCompoundWritesSkyBlockPackCache(t *testing.T) {
	renderer := newTestRenderer(t)

	expectedResource, err := renderer.MinecraftRenderer().ComputeResourceIdFromSkyBlockItemID("TEST_ITEM", renderer.renderOptions())
	if err != nil {
		t.Fatal(err)
	}

	rootID := nbt.NewNbtString("minecraft:player_head")
	extraID := nbt.NewNbtString("TEST_ITEM")
	input := nbt.NewNbtCompound(map[string]nbt.NbtTag{
		"id": &rootID,
		"tag": nbt.NewNbtCompound(map[string]nbt.NbtTag{
			"ExtraAttributes": nbt.NewNbtCompound(map[string]nbt.NbtTag{
				"id": &extraID,
			}),
		}),
	})
	item, err := renderer.RenderItemNBT(input)
	if err != nil {
		t.Fatal(err)
	}
	if item.Path != renderedDebugWebPPath(renderer.cacheDir, expectedResource, debugInfoFromItemInput(input)) {
		t.Fatalf("RenderItemNBT path = %q, want SkyBlock ID path", item.Path)
	}
	assertRenderedItem(t, renderer, item, sourcePackID(expectedResource))
	assertCachedWebPHasNonBlackOpaqueColor(t, item.Path)
}

func TestRenderItemNBTSkyBlockPackIgnoresVanillaDisplayTint(t *testing.T) {
	renderer := newTestRenderer(t)

	expectedResource, err := renderer.MinecraftRenderer().ComputeResourceIdFromSkyBlockItemID("TEST_ITEM", renderer.renderOptions())
	if err != nil {
		t.Fatal(err)
	}

	input := map[string]any{
		"id": "minecraft:player_head",
		"tag": map[string]any{
			"display": map[string]any{
				"color": 0,
			},
			"ExtraAttributes": map[string]any{
				"id": "TEST_ITEM",
			},
		},
	}
	item, err := renderer.RenderItemNBT(input)
	if err != nil {
		t.Fatal(err)
	}
	if item.Path != renderedDebugWebPPath(renderer.cacheDir, expectedResource, debugInfoFromItemInput(input)) {
		t.Fatalf("RenderItemNBT path = %q, want SkyBlock ID path", item.Path)
	}
	assertRenderedItem(t, renderer, item, sourcePackID(expectedResource))
	assertCachedWebPHasNonBlackOpaqueColor(t, item.Path)
}

func TestRenderItemNBTVanillaIgnoresDisplayTintOnNonTintableItem(t *testing.T) {
	renderer := newTestRenderer(t)

	item, err := renderer.RenderItemNBT(map[string]any{
		"id": "minecraft:diamond_sword",
		"tag": map[string]any{
			"display": map[string]any{
				"color": 0,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderedItem(t, renderer, item, mbr.VanillaPackId)
	assertCachedWebPHasColor(t, item.Path, color.NRGBA{R: 40, G: 180, B: 220, A: 255})
}

func TestRenderItemNBTVanillaBrewingStandIgnoresDisplayTint(t *testing.T) {
	renderer := newTestRenderer(t)

	item, err := renderer.RenderItemNBT(map[string]any{
		"id": "minecraft:brewing_stand",
		"tag": map[string]any{
			"display": map[string]any{
				"color": 0,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderedItem(t, renderer, item, mbr.VanillaPackId)
	assertCachedWebPHasColor(t, item.Path, color.NRGBA{R: 155, G: 94, B: 38, A: 255})
}

func TestRenderItemNBTWebPKeepsVanillaTextureColors(t *testing.T) {
	renderer := newTestRenderer(t)

	item, err := renderer.RenderItemNBT(map[string]any{
		"id": "minecraft:diamond_sword",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderedItem(t, renderer, item, mbr.VanillaPackId)
	assertCachedWebPHasColor(t, item.Path, color.NRGBA{R: 40, G: 180, B: 220, A: 255})
}

func TestRenderItemNBTActualVanillaWebPVisibleToExternalDecoders(t *testing.T) {
	if _, err := exec.LookPath("magick"); err != nil {
		t.Skip("magick not available")
	}
	assetsRoot := filepath.Join("packs", "assets", "minecraft")
	if _, err := os.Stat(filepath.Join(assetsRoot, "textures", "item", "diamond_sword.png")); err != nil {
		t.Skip("local vanilla assets not available")
	}

	resourcePacksRoot := t.TempDir()
	packRoot := filepath.Join(resourcePacksRoot, "testpack")
	writeRootJSON(t, packRoot, "meta.json", `{"id":"testpack","name":"Test Pack","version":"test"}`)
	writeRootJSON(t, packRoot, "pack.mcmeta", `{"pack":{"pack_format":99,"description":"test"}}`)
	if err := os.MkdirAll(filepath.Join(packRoot, "assets", "minecraft"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packRoot, "pack.png"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	renderer, err := NewRenderer(Options{
		AssetsRoot:        assetsRoot,
		ResourcePacksRoot: resourcePacksRoot,
		PackIDs:           []string{"testpack"},
		CacheDir:          t.TempDir(),
		Size:              128,
	})
	if err != nil {
		t.Fatal(err)
	}
	item, err := renderer.RenderItemNBT(map[string]any{"id": "minecraft:diamond_sword"})
	if err != nil {
		t.Fatal(err)
	}

	decodedPath := filepath.Join(t.TempDir(), "decoded.png")
	if out, err := exec.Command("magick", item.Path, decodedPath).CombinedOutput(); err != nil {
		t.Fatalf("magick decode failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertPNGHasNonBlackOpaqueColor(t, decodedPath)
}

func TestRenderItemNBTActualVanillaBrewingStandHasColor(t *testing.T) {
	assetsRoot := filepath.Join("packs", "assets", "minecraft")
	if _, err := os.Stat(filepath.Join(assetsRoot, "items", "brewing_stand.json")); err != nil {
		t.Skip("local vanilla assets not available")
	}

	resourcePacksRoot := t.TempDir()
	packRoot := filepath.Join(resourcePacksRoot, "testpack")
	writeRootJSON(t, packRoot, "meta.json", `{"id":"testpack","name":"Test Pack","version":"test"}`)
	writeRootJSON(t, packRoot, "pack.mcmeta", `{"pack":{"pack_format":99,"description":"test"}}`)
	if err := os.MkdirAll(filepath.Join(packRoot, "assets", "minecraft"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packRoot, "pack.png"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	renderer, err := NewRenderer(Options{
		AssetsRoot:        assetsRoot,
		ResourcePacksRoot: resourcePacksRoot,
		PackIDs:           []string{"testpack"},
		CacheDir:          t.TempDir(),
		Size:              128,
	})
	if err != nil {
		t.Fatal(err)
	}
	item, err := renderer.RenderItemNBT(map[string]any{
		"id": "minecraft:brewing_stand",
		"tag": map[string]any{
			"display": map[string]any{
				"color": 0,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertCachedWebPHasNonBlackOpaqueColor(t, item.Path)
}

func TestRenderItemNBTRealPackWebPMatchesPrerender(t *testing.T) {
	assetsRoot := filepath.Join("packs", "assets", "minecraft")
	if _, err := os.Stat(filepath.Join(assetsRoot, "items", "player_head.json")); err != nil {
		t.Skip("local vanilla assets not available")
	}
	resourcePacksRoot := "texturepacks"
	if _, err := os.Stat(filepath.Join(resourcePacksRoot, "fsr", "meta.json")); err != nil {
		t.Skip("local fsr texture pack not available")
	}

	newRealRenderer := func(t testing.TB) *Renderer {
		t.Helper()
		renderer, err := NewRenderer(Options{
			AssetsRoot:        assetsRoot,
			ResourcePacksRoot: resourcePacksRoot,
			PackIDs:           []string{"fsr"},
			CacheDir:          t.TempDir(),
			Size:              96,
		})
		if err != nil {
			t.Fatal(err)
		}
		return renderer
	}

	const skyblockID = "CROWN_OF_AVARICE"
	prerenderer := newRealRenderer(t)
	prerendered, err := prerenderer.PreRenderSkyBlockItemIDs(context.Background(), []string{skyblockID}, PreRenderOptions{
		Overwrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if prerendered.Succeeded != 1 || prerendered.Entries[0].Error != "" {
		t.Fatalf("unexpected prerender result: %#v", prerendered)
	}
	prerenderPath := prerendered.Entries[0].Path
	assertCachedWebPHasNonBlackOpaqueColor(t, prerenderPath)

	nbtRenderer := newRealRenderer(t)
	renderedNBT, err := nbtRenderer.RenderItemNBT(map[string]any{
		"id":    "minecraft:player_head",
		"Count": 1,
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id": skyblockID,
			},
			"SkullOwner": map[string]any{
				"Properties": map[string]any{
					"textures": []any{
						map[string]any{
							"Value": "dummy-test-texture-value",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(renderedNBT.Path) != filepath.Base(prerenderPath) {
		t.Fatalf("RenderItemNBT cache key = %q, want prerender key %q", filepath.Base(renderedNBT.Path), filepath.Base(prerenderPath))
	}
	if renderedNBT.TexturePackID != prerendered.Entries[0].TexturePackID {
		t.Fatalf("RenderItemNBT texture pack = %q, want prerender pack %q", renderedNBT.TexturePackID, prerendered.Entries[0].TexturePackID)
	}
	assertCachedWebPHasNonBlackOpaqueColor(t, renderedNBT.Path)
}

func TestRenderSkyBlockItemIDReusesExistingWebPCache(t *testing.T) {
	renderer := newTestRenderer(t)

	first, err := renderer.RenderSkyBlockItemID("TEST_ITEM")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first.Path, []byte("sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}

	second, err := renderer.RenderSkyBlockItemID("TEST_ITEM")
	if err != nil {
		t.Fatal(err)
	}
	if second.Path != first.Path || second.TexturePackID != first.TexturePackID {
		t.Fatalf("cache hit changed result: first=%#v second=%#v", first, second)
	}
	data, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "sentinel" {
		t.Fatal("existing webp cache file was overwritten")
	}
}

func TestRenderSkyBlockItemIDMemoryCacheDoesNotStatFile(t *testing.T) {
	renderer := newTestRenderer(t)

	first, err := renderer.RenderSkyBlockItemID("TEST_ITEM")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(first.Path); err != nil {
		t.Fatal(err)
	}

	second, err := renderer.RenderSkyBlockItemID("TEST_ITEM")
	if err != nil {
		t.Fatal(err)
	}
	if second.Path != first.Path || second.TexturePackID != first.TexturePackID {
		t.Fatalf("memory cache changed result: first=%#v second=%#v", first, second)
	}
}

func TestRenderSkyBlockItemIDConcurrentCallsShareResult(t *testing.T) {
	renderer := newTestRenderer(t)
	const calls = 8
	start := make(chan struct{})
	results := make([]*RenderedItem, calls)
	errs := make([]error, calls)
	var wg sync.WaitGroup

	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			results[index], errs[index] = renderer.RenderSkyBlockItemID("TEST_ITEM")
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d failed: %v", i, err)
		}
	}
	for i := 1; i < calls; i++ {
		if results[i].Path != results[0].Path || results[i].TexturePackID != results[0].TexturePackID {
			t.Fatalf("concurrent call %d returned different result: %#v != %#v", i, results[i], results[0])
		}
	}
	assertWebPFile(t, results[0].Path)
}

func TestRenderItemNBTWritesAnimatedWebP(t *testing.T) {
	renderer := newTestRenderer(t)

	item, err := renderer.RenderItemNBT(map[string]any{
		"id": "clock",
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Path == "" || item.TexturePackID == "" {
		t.Fatalf("unexpected rendered item: %#v", item)
	}
	assertWebPFile(t, item.Path)
	data, err := os.ReadFile(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ANIM") || !strings.Contains(string(data), "ANMF") {
		t.Fatal("animated webp does not contain ANIM/ANMF chunks")
	}
}

func TestPlayerSkinCacheUsesConfiguredWebPDirectory(t *testing.T) {
	renderer := newTestRenderer(t)
	skin := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			skin.SetRGBA(x, y, color.RGBA{R: 30, G: 80, B: 120, A: 255})
		}
	}

	normalizedURL := "https://textures.minecraft.net/texture/test-skin"
	renderer.MinecraftRenderer().TryPersistSkin(normalizedURL, skin)
	path := renderer.MinecraftRenderer().GetSkinCachePath(normalizedURL)
	if path == nil {
		t.Fatal("skin cache path is nil")
	}
	if filepath.Ext(*path) != ".webp" {
		t.Fatalf("skin cache extension = %q, want .webp", filepath.Ext(*path))
	}
	expectedPrefix := filepath.Join(renderer.cacheDir, "player_skins") + string(os.PathSeparator)
	if !strings.HasPrefix(*path, expectedPrefix) {
		t.Fatalf("skin cache path %q is not under %q", *path, expectedPrefix)
	}
	assertWebPFile(t, *path)
	if _, ok := renderer.MinecraftRenderer().TryLoadSkinFromDisk(normalizedURL); !ok {
		t.Fatal("persisted webp skin was not readable")
	}
}

func TestAnimatedTextureFramesUseDerivedCache(t *testing.T) {
	renderer := newTestRenderer(t)

	if _, err := renderer.RenderItemNBT(map[string]any{"id": "clock"}); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(renderer.cacheDir, "derived", "animations", "*", "frame-000.webp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected animated texture frame cache files")
	}
	assertWebPFile(t, matches[0])
}

func TestPreRenderSkyBlockItemIDsWritesWebPCache(t *testing.T) {
	renderer := newTestRenderer(t)

	result, err := renderer.PreRenderSkyBlockItemIDs(context.Background(), []string{"TEST_ITEM"}, PreRenderOptions{
		Workers: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Succeeded != 1 || result.Failed != 0 {
		t.Fatalf("result counts = %d succeeded, %d failed", result.Succeeded, result.Failed)
	}
	entry := result.Entries[0]
	if entry.InputID != "TEST_ITEM" || entry.TexturePackID != "testpack" || entry.Path == "" || entry.Error != "" {
		t.Fatalf("unexpected entry: %#v", entry)
	}
	assertRenderedPath(t, renderer, entry.Path)
	assertWebPFile(t, entry.Path)
}

func TestPreRenderSkyBlockItemIDsSkipsMissingCustomTexture(t *testing.T) {
	assetsRoot := createRootMinimalAssets(t)
	packRoot := createRootSkyblockPack(t, "testpack")
	writeRootJSON(t, packRoot, "assets/skyblock/items/broken_texture.json", `{"model":{"type":"model","model":"firmskyblock:item/broken_texture"}}`)
	writeRootJSON(t, packRoot, "assets/firmskyblock/models/item/broken_texture.json", `{"parent":"builtin/generated","textures":{"layer0":"firmskyblock:item/does_not_exist"}}`)

	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(t.TempDir(), "cache")
	blockRenderer := mbr.CreateFromMinecraftAssets(assetsRoot, registry, []string{"testpack"})
	blockRenderer.SetCacheDirectory(cacheDir)
	renderer := &Renderer{
		renderer:    blockRenderer,
		packIDs:     []string{"testpack"},
		size:        32,
		cacheDir:    cacheDir,
		renderCache: make(map[string]RenderedItem),
		inflight:    make(map[string]*renderInflight),
	}

	expectedResource, err := renderer.MinecraftRenderer().ComputeResourceIdFromSkyBlockItemID("BROKEN_TEXTURE", renderer.renderOptions())
	if err != nil {
		t.Fatal(err)
	}
	expectedPath := renderedDebugWebPPath(renderer.cacheDir, expectedResource, &renderDebugInfo{
		SkyBlockID:  "BROKEN_TEXTURE",
		MinecraftID: "minecraft:player_head",
	})

	result, err := renderer.PreRenderSkyBlockItemIDs(context.Background(), []string{"BROKEN_TEXTURE"}, PreRenderOptions{
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Succeeded != 0 || result.Failed != 0 || result.Skipped != 1 {
		t.Fatalf("result counts = %d succeeded, %d failed, %d skipped", result.Succeeded, result.Failed, result.Skipped)
	}
	entry := result.Entries[0]
	if !entry.Skipped || entry.Path != "" || entry.TexturePackID != "" || entry.Error != "" {
		t.Fatalf("unexpected skipped entry: %#v", entry)
	}
	if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
		t.Fatalf("missing custom texture was written to disk at %q, stat err=%v", expectedPath, err)
	}

	rendered, err := renderer.RenderSkyBlockItemID("BROKEN_TEXTURE")
	if err == nil {
		t.Fatalf("expected direct render to fail for missing custom texture, got %+v", rendered)
	}
	if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
		t.Fatalf("direct render wrote missing custom texture to disk at %q, stat err=%v", expectedPath, err)
	}
}

func TestPreRenderSkyBlockItemIDsSkipsExistingUnlessOverwrite(t *testing.T) {
	renderer := newTestRenderer(t)

	first, err := renderer.PreRenderSkyBlockItemIDs(context.Background(), []string{"TEST_ITEM"}, PreRenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	path := first.Entries[0].Path
	if err := os.WriteFile(path, []byte("sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}

	skipped, err := renderer.PreRenderSkyBlockItemIDs(context.Background(), []string{"TEST_ITEM"}, PreRenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if skipped.Entries[0].Path != path {
		t.Fatalf("path changed on skip: %q != %q", skipped.Entries[0].Path, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "sentinel" {
		t.Fatal("existing cache file was overwritten with Overwrite=false")
	}

	overwritten, err := renderer.PreRenderSkyBlockItemIDs(context.Background(), []string{"TEST_ITEM"}, PreRenderOptions{
		Overwrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertWebPFile(t, overwritten.Entries[0].Path)
}

func TestPreRenderSkyBlockItemIDsPreservesDuplicates(t *testing.T) {
	renderer := newTestRenderer(t)

	result, err := renderer.PreRenderSkyBlockItemIDs(context.Background(), []string{"TEST_ITEM", "TEST_ITEM"}, PreRenderOptions{
		Workers: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 || result.Succeeded != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].Path == "" || result.Entries[0].Path != result.Entries[1].Path {
		t.Fatalf("duplicate entries did not share cache path: %#v", result.Entries)
	}
	if result.Entries[0].TexturePackID != result.Entries[1].TexturePackID {
		t.Fatalf("duplicate entries did not share texture pack id: %#v", result.Entries)
	}
}

func TestPreRenderSkyBlockItemIDsReportsEntryErrors(t *testing.T) {
	renderer := newTestRenderer(t)

	result, err := renderer.PreRenderSkyBlockItemIDs(context.Background(), []string{"TEST_ITEM", " "}, PreRenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Succeeded != 1 || result.Failed != 1 {
		t.Fatalf("result counts = %d succeeded, %d failed", result.Succeeded, result.Failed)
	}
	if result.Entries[1].Error == "" {
		t.Fatalf("blank id did not report an error: %#v", result.Entries[1])
	}
}

func TestPreRenderSkyBlockItemIDsHonorsCanceledContext(t *testing.T) {
	renderer := newTestRenderer(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := renderer.PreRenderSkyBlockItemIDs(ctx, []string{"TEST_ITEM"}, PreRenderOptions{})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if result == nil || result.Failed != 1 || result.Entries[0].Error == "" {
		t.Fatalf("canceled prerender did not return partial failure result: %#v", result)
	}
}

func newTestRenderer(t testing.TB) *Renderer {
	t.Helper()

	assetsRoot := createRootMinimalAssets(t)
	packRoot := createRootSkyblockPack(t, "testpack")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(t.TempDir(), "cache")
	blockRenderer := mbr.CreateFromMinecraftAssets(assetsRoot, registry, []string{"testpack"})
	blockRenderer.SetCacheDirectory(cacheDir)
	return &Renderer{
		renderer:    blockRenderer,
		packIDs:     []string{"testpack"},
		size:        32,
		cacheDir:    cacheDir,
		renderCache: make(map[string]RenderedItem),
		inflight:    make(map[string]*renderInflight),
	}
}

func createRootMinimalAssets(t testing.TB) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "assets", "minecraft")
	writeRootJSON(t, root, "blockstates/stone.json", `{"variants":{"":{"model":"minecraft:block/stone"}}}`)
	writeRootJSON(t, root, "models/block/stone.json", `{
		"textures":{"all":"minecraft:block/stone"},
		"elements":[{"from":[0,0,0],"to":[16,16,16],"faces":{
			"north":{"texture":"#all"},"south":{"texture":"#all"},"east":{"texture":"#all"},
			"west":{"texture":"#all"},"up":{"texture":"#all"},"down":{"texture":"#all"}
		}}]
	}`)
	writeRootJSON(t, root, "items/player_head.json", `{"model":{"model":"minecraft:item/player_head"}}`)
	writeRootJSON(t, root, "items/diamond_sword.json", `{"model":{"model":"minecraft:item/diamond_sword"}}`)
	writeRootJSON(t, root, "items/brewing_stand.json", `{"model":{"model":"minecraft:item/brewing_stand"}}`)
	writeRootJSON(t, root, "items/clock.json", `{"model":{"model":"minecraft:item/clock"}}`)
	writeRootJSON(t, root, "models/item/player_head.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/player_head"}}`)
	writeRootJSON(t, root, "models/item/diamond_sword.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/diamond_sword"}}`)
	writeRootJSON(t, root, "models/item/brewing_stand.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/brewing_stand"}}`)
	writeRootJSON(t, root, "models/item/clock.json", `{"parent":"builtin/generated","textures":{"layer0":"minecraft:item/clock"}}`)
	writeRootPNG(t, filepath.Join(root, "textures", "block", "stone.png"), 16, 16, color.RGBA{R: 180, G: 30, B: 30, A: 255})
	writeRootPNG(t, filepath.Join(root, "textures", "item", "player_head.png"), 16, 16, color.RGBA{R: 90, G: 90, B: 90, A: 255})
	writeRootPNG(t, filepath.Join(root, "textures", "item", "diamond_sword.png"), 16, 16, color.RGBA{R: 40, G: 180, B: 220, A: 255})
	writeRootPNG(t, filepath.Join(root, "textures", "item", "brewing_stand.png"), 16, 16, color.RGBA{R: 155, G: 94, B: 38, A: 255})
	writeRootAnimatedPNG(t, filepath.Join(root, "textures", "item", "clock.png"))
	writeRootJSON(t, root, "textures/item/clock.png.mcmeta", `{"animation":{"frametime":1,"width":16,"height":16}}`)
	return root
}

func createRootSkyblockPack(t testing.TB, id string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), id)
	writeRootJSON(t, root, "meta.json", `{"id":"`+id+`","name":"Test Pack","version":"test"}`)
	writeRootJSON(t, root, "pack.mcmeta", `{"pack":{"pack_format":99,"description":"test"}}`)
	writeRootJSON(t, root, "assets/minecraft/items/player_head.json", `{"model":{"model":"minecraft:item/player_head"}}`)
	writeRootJSON(t, root, "assets/skyblock/items/test_item.json", `{"model":{"type":"model","model":"firmskyblock:item/test_item"}}`)
	writeRootJSON(t, root, "assets/skyblock/items/animated_item.json", `{"model":{"type":"model","model":"firmskyblock:item/animated_item"}}`)
	writeRootJSON(t, root, "assets/firmskyblock/models/item/test_item.json", `{"parent":"builtin/generated","textures":{"layer0":"firmskyblock:item/test_item"}}`)
	writeRootJSON(t, root, "assets/firmskyblock/models/item/animated_item.json", `{"parent":"builtin/generated","textures":{"layer0":"firmskyblock:item/animated_item"}}`)
	writeRootPNG(t, filepath.Join(root, "assets", "firmskyblock", "textures", "item", "test_item.png"), 16, 16, color.RGBA{R: 40, G: 180, B: 220, A: 255})
	writeRootAnimatedPNG(t, filepath.Join(root, "assets", "firmskyblock", "textures", "item", "animated_item.png"))
	writeRootJSON(t, root, "assets/firmskyblock/textures/item/animated_item.png.mcmeta", `{"animation":{"frametime":1,"width":16,"height":16}}`)
	if err := os.WriteFile(filepath.Join(root, "pack.png"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeRootJSON(t testing.TB, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRootPNG(t testing.TB, path string, width int, height int, c color.RGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func writeRootAnimatedPNG(t testing.TB, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 16, 32))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 220, G: 40, B: 40, A: 255})
			img.SetRGBA(x, y+16, color.RGBA{R: 40, G: 220, B: 80, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func assertRenderedItem(t testing.TB, renderer *Renderer, item *RenderedItem, texturePackID string) {
	t.Helper()
	if item == nil || item.Path == "" || item.TexturePackID != texturePackID {
		t.Fatalf("unexpected rendered item: %#v", item)
	}
	assertRenderedPath(t, renderer, item.Path)
	assertWebPFile(t, item.Path)
}

func assertRenderedPath(t testing.TB, renderer *Renderer, path string) {
	t.Helper()
	if filepath.Ext(path) != ".webp" {
		t.Fatalf("cache extension = %q, want .webp", filepath.Ext(path))
	}
	expectedPrefix := filepath.Join(renderer.cacheDir, "rendered") + string(os.PathSeparator)
	if !strings.HasPrefix(path, expectedPrefix) {
		t.Fatalf("render path %q is not under %q", path, expectedPrefix)
	}
}

func assertWebPFile(t testing.TB, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		t.Fatalf("file is not a webp: %s", path)
	}
}

func assertCachedWebPHasColor(t testing.TB, path string, want color.NRGBA) {
	t.Helper()
	img, err := imagecache.ReadRGBA(path)
	if err != nil {
		t.Fatal(err)
	}

	tolerance := 2
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if closeColor(got, want, tolerance) {
				return
			}
		}
	}
	t.Fatalf("cached webp %s does not contain expected color near %#v; sample=%s", path, want, sampleOpaqueColors(img, 5))
}

func assertCachedWebPHasNonBlackOpaqueColor(t testing.TB, path string) {
	t.Helper()
	img, err := imagecache.ReadRGBA(path)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if got.A > 0 && (got.R > 4 || got.G > 4 || got.B > 4) {
				return
			}
		}
	}
	t.Fatalf("cached webp %s has no non-black opaque color; sample=%s", path, sampleOpaqueColors(img, 10))
}

func closeColor(got color.NRGBA, want color.NRGBA, tolerance int) bool {
	return channelDelta(got.R, want.R) <= tolerance &&
		channelDelta(got.G, want.G) <= tolerance &&
		channelDelta(got.B, want.B) <= tolerance &&
		channelDelta(got.A, want.A) <= tolerance
}

func channelDelta(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

func sampleOpaqueColors(img image.Image, limit int) string {
	colors := make([]string, 0, limit)
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if got.A == 0 {
				continue
			}
			colors = append(colors, fmt.Sprintf("(%d,%d,%d,%d)", got.R, got.G, got.B, got.A))
			if len(colors) >= limit {
				return strings.Join(colors, ",")
			}
		}
	}
	return strings.Join(colors, ",")
}

func assertPNGHasNonBlackOpaqueColor(t testing.TB, path string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if got.A > 0 && (got.R > 4 || got.G > 4 || got.B > 4) {
				return
			}
		}
	}
	t.Fatalf("decoded image %s has no non-black opaque color; sample=%s", path, sampleOpaqueColors(img, 10))
}
