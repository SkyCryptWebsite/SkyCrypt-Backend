package stats

import (
	"encoding/base64"
	"skycrypt/src/lib"
	"skycrypt/src/utility"
	"testing"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
)

func testSkinValue(hash string) string {
	payload := `{"textures":{"SKIN":{"url":"http://textures.minecraft.net/texture/` + hash + `"}}}`
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

func testDomain() string {
	return utility.GetDomain()
}

func testItem(id int, skyblockID string) *skycrypttypes.Item {
	damage := 0
	return &skycrypttypes.Item{
		ID:     &id,
		Damage: &damage,
		Tag: &skycrypttypes.Tag{
			Display: skycrypttypes.Display{
				Name: "Test Item",
				Lore: []string{"§f§lCOMMON"},
			},
			ExtraAttributes: &skycrypttypes.ExtraAttributes{Id: skyblockID},
		},
	}
}

func TestProcessItemsUsesHotTextureCacheForNestedItems(t *testing.T) {
	previousRenderer := lib.SkyCryptRender
	previousCache := lib.ITEM_TEXTURE_CACHE
	lib.SkyCryptRender = nil
	lib.ITEM_TEXTURE_CACHE = map[string]lib.AppliedItemTexture{
		"PARENT_CACHE": {Texture: testDomain() + "/cache/rendered/skyblock=PARENT_CACHE.webp", TexturePack: "hplus"},
		"NESTED_CACHE": {Texture: testDomain() + "/cache/rendered/skyblock=NESTED_CACHE.webp", TexturePack: "hplus"},
	}
	t.Cleanup(func() {
		lib.SkyCryptRender = previousRenderer
		lib.ITEM_TEXTURE_CACHE = previousCache
	})

	parent := testItem(1, "PARENT_CACHE")
	parent.ContainsItems = []*skycrypttypes.Item{testItem(1, "NESTED_CACHE")}

	processed := ProcessItems([]*skycrypttypes.Item{parent}, "inventory")
	if len(processed) != 1 {
		t.Fatalf("processed length = %d, want 1", len(processed))
	}
	if processed[0].Texture != testDomain()+"/cache/rendered/skyblock=PARENT_CACHE.webp" {
		t.Fatalf("parent texture = %q", processed[0].Texture)
	}
	if len(processed[0].ContainsItems) != 1 {
		t.Fatalf("nested length = %d, want 1", len(processed[0].ContainsItems))
	}
	if processed[0].ContainsItems[0].Texture != testDomain()+"/cache/rendered/skyblock=NESTED_CACHE.webp" {
		t.Fatalf("nested texture = %q", processed[0].ContainsItems[0].Texture)
	}
}

func TestProcessItemsTextureOutputsForVanillaLeatherAndSkull(t *testing.T) {
	previousRenderer := lib.SkyCryptRender
	previousCache := lib.ITEM_TEXTURE_CACHE
	lib.SkyCryptRender = nil
	lib.ITEM_TEXTURE_CACHE = map[string]lib.AppliedItemTexture{}
	t.Cleanup(func() {
		lib.SkyCryptRender = previousRenderer
		lib.ITEM_TEXTURE_CACHE = previousCache
	})

	apple := testItem(260, "")
	leatherID := 298
	leather := testItem(leatherID, "")
	leather.Tag.Display.Color = 0x112233
	head := testItem(397, "HEAD_ITEM")
	head.Tag.SkullOwner = &skycrypttypes.SkullOwner{
		Properties: skycrypttypes.Properties{
			Textures: []skycrypttypes.Texture{{Value: testSkinValue("head-texture-hash")}},
		},
	}

	processed := ProcessItems([]*skycrypttypes.Item{apple, leather, head}, "inventory")
	if len(processed) != 3 {
		t.Fatalf("processed length = %d, want 3", len(processed))
	}
	if processed[0].Texture != testDomain()+"/assets/resourcepacks/Vanilla/assets/minecraft/textures/item/apple.png" {
		t.Fatalf("apple texture = %q", processed[0].Texture)
	}
	if processed[1].Texture != testDomain()+"/api/leather/helmet/112233" {
		t.Fatalf("leather texture = %q", processed[1].Texture)
	}
	if processed[2].Texture != testDomain()+"/api/head/head-texture-hash" {
		t.Fatalf("head texture = %q", processed[2].Texture)
	}
}

func BenchmarkProcessItemsHotTextureCache_1600Items(b *testing.B) {
	previousRenderer := lib.SkyCryptRender
	previousCache := lib.ITEM_TEXTURE_CACHE
	lib.SkyCryptRender = nil
	lib.ITEM_TEXTURE_CACHE = map[string]lib.AppliedItemTexture{
		"BENCH_PROCESS": {Texture: testDomain() + "/cache/rendered/skyblock=BENCH_PROCESS.webp", TexturePack: "hplus"},
	}
	defer func() {
		lib.SkyCryptRender = previousRenderer
		lib.ITEM_TEXTURE_CACHE = previousCache
	}()

	items := make([]*skycrypttypes.Item, 1600)
	for i := range items {
		items[i] = testItem(1, "BENCH_PROCESS")
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		processed := ProcessItems(items, "inventory")
		if len(processed) != len(items) {
			b.Fatalf("processed length = %d, want %d", len(processed), len(items))
		}
	}
}
