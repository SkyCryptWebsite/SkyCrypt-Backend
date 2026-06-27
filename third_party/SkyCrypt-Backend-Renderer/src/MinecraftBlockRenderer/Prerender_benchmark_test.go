package minecraftblockrenderer

import (
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	texturepacks "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/TexturePacks"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkRenderSkyBlockItemIDColdBatchHPlus(b *testing.B) {
	assetsRoot := benchmarkFullAssets(b)
	packRoot := benchmarkTexturePack(b, "hplus")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		b.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"hplus"})
	options := &BlockRenderOptions{Size: 64, PackIds: []string{"hplus"}}
	ids := []string{"AATROX_BATPHONE", "GEMSTONE_GAUNTLET"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, id := range ids {
			rendered, err := renderer.RenderSkyBlockItemID(id, options)
			if err != nil {
				b.Fatal(err)
			}
			if rendered == nil || rendered.Image == nil {
				b.Fatalf("render returned nil for %s", id)
			}
		}
	}
}

func BenchmarkResolveSkyblockItemModelFromPackProviders(b *testing.B) {
	assetsRoot := benchmarkFullAssets(b)
	packRoot := benchmarkTexturePack(b, "hplus")
	registry := texturepacks.NewTexturePackRegistry()
	if _, err := registry.RegisterPack(packRoot); err != nil {
		b.Fatal(err)
	}
	renderer := CreateFromMinecraftAssets(assetsRoot, registry, []string{"hplus"})
	itemData := &data.ItemRenderData{CustomData: nbt.NewNbtCompound(map[string]nbt.NbtTag{
		"id": nbt.NewNbtString("AATROX_BATPHONE"),
	})}
	if model := renderer.ResolveSkyblockItemModelFromPackProviders("aatrox_batphone", "minecraft:player_head", itemData, "gui"); model == nil {
		b.Fatal("model did not resolve")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model := renderer.ResolveSkyblockItemModelFromPackProviders("aatrox_batphone", "minecraft:player_head", itemData, "gui")
		if model == nil {
			b.Fatal("model did not resolve")
		}
	}
}

func benchmarkRepoRoot(b *testing.B) string {
	b.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		b.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			b.Fatal("could not locate repository root")
		}
		cwd = parent
	}
}

func benchmarkFullAssets(b *testing.B) string {
	b.Helper()
	root := filepath.Join(benchmarkRepoRoot(b), "packs", "assets", "minecraft")
	required := []string{
		filepath.Join(root, "blockstates", "stone.json"),
		filepath.Join(root, "models", "block", "stone.json"),
		filepath.Join(root, "textures", "block", "stone.png"),
		filepath.Join(root, "items", "diamond_sword.json"),
	}
	for _, path := range required {
		if _, err := os.Stat(path); err != nil {
			b.Skipf("full vanilla assets are not available: missing %s", path)
		}
	}
	return root
}

func benchmarkTexturePack(b *testing.B, packName string) string {
	b.Helper()
	root := filepath.Join(benchmarkRepoRoot(b), "texturepacks", packName)
	if _, err := os.Stat(filepath.Join(root, "meta.json")); err != nil {
		b.Skipf("texture pack %q is not available at %s", packName, root)
	}
	return root
}
