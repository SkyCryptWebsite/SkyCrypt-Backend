package stats

import (
	"bytes"
	"encoding/base64"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"
	"testing"
	"time"

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

func TestProcessItemsUsesItemEndpointForNestedItems(t *testing.T) {
	parent := testItem(1, "PARENT_CACHE")
	parent.ContainsItems = []*skycrypttypes.Item{testItem(1, "NESTED_CACHE")}

	processed := ProcessItems([]*skycrypttypes.Item{parent}, "inventory")
	if len(processed) != 1 {
		t.Fatalf("processed length = %d, want 1", len(processed))
	}
	if processed[0].Texture != testDomain()+"/api/item/PARENT_CACHE" {
		t.Fatalf("parent texture = %q", processed[0].Texture)
	}
	if len(processed[0].ContainsItems) != 1 {
		t.Fatalf("nested length = %d, want 1", len(processed[0].ContainsItems))
	}
	if processed[0].ContainsItems[0].Texture != testDomain()+"/api/item/NESTED_CACHE" {
		t.Fatalf("nested texture = %q", processed[0].ContainsItems[0].Texture)
	}
}

func TestProcessItemsTextureOutputsUseItemEndpointAndPotionSpecialCase(t *testing.T) {
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
	if processed[0].Texture != testDomain()+"/api/item/apple" {
		t.Fatalf("apple texture = %q", processed[0].Texture)
	}
	if processed[1].Texture != testDomain()+"/api/item/leather_helmet" {
		t.Fatalf("leather texture = %q", processed[1].Texture)
	}
	if processed[2].Texture != testDomain()+"/api/item/HEAD_ITEM" {
		t.Fatalf("head texture = %q", processed[2].Texture)
	}
}

func TestProcessItemsUsesItemEndpointForSkyBlockSkull(t *testing.T) {
	want := testDomain() + "/api/item/PROCESS_SKULL_CACHE"
	head := testItem(397, "PROCESS_SKULL_CACHE")
	head.Tag.ItemModel = "minecraft:player_head"
	head.Tag.SkullOwner = &skycrypttypes.SkullOwner{
		ID: "process-skull-id",
		Properties: skycrypttypes.Properties{
			Textures: []skycrypttypes.Texture{{Value: testSkinValue("process-skull-hash")}},
		},
	}

	processed := ProcessItems([]*skycrypttypes.Item{head}, "inventory", []string{"fsr"})
	if len(processed) != 1 {
		t.Fatalf("processed length = %d, want 1", len(processed))
	}
	if processed[0].Texture != want {
		t.Fatalf("skull texture = %q, want %q", processed[0].Texture, want)
	}
}

func TestProcessItemsUsesHeadEndpointForGenericSkull(t *testing.T) {
	want := testDomain() + "/api/head/generic-skull-hash"
	head := testItem(397, "")
	head.Tag.ItemModel = "minecraft:player_head"
	head.Tag.SkullOwner = &skycrypttypes.SkullOwner{
		Properties: skycrypttypes.Properties{
			Textures: []skycrypttypes.Texture{{Value: testSkinValue("generic-skull-hash")}},
		},
	}

	processed := ProcessItems([]*skycrypttypes.Item{head}, "inventory", []string{"fsr"})
	if len(processed) != 1 {
		t.Fatalf("processed length = %d, want 1", len(processed))
	}
	if processed[0].Texture != want {
		t.Fatalf("skull texture = %q, want %q", processed[0].Texture, want)
	}
}

func TestProcessItemsWithStatsCountsNestedItemsAndTextures(t *testing.T) {
	parent := testItem(1, "PARENT_STATS")
	parent.ContainsItems = []*skycrypttypes.Item{testItem(1, "NESTED_STATS")}
	itemStats := NewItemProcessingStats("test", "uuid", "profile", nil)
	processed := ProcessItemsWithNEUCacheAndStats([]*skycrypttypes.Item{parent}, "inventory", map[string]models.NEUItem{}, itemStats)

	if len(processed) != 1 || len(processed[0].ContainsItems) != 1 {
		t.Fatalf("processed nested output lengths = %d/%d, want 1/1", len(processed), len(processed[0].ContainsItems))
	}
	if itemStats.TotalItems != 2 || itemStats.TopLevelItems != 1 || itemStats.NestedItems != 1 || itemStats.ContainerItems != 1 {
		t.Fatalf(
			"item counts total/top/nested/containers = %d/%d/%d/%d, want 2/1/1/1",
			itemStats.TotalItems,
			itemStats.TopLevelItems,
			itemStats.NestedItems,
			itemStats.ContainerItems,
		)
	}
}

func TestProcessItemsWithStatsTracksNEUWikiRouteCache(t *testing.T) {
	items := []*skycrypttypes.Item{
		testItem(1, "WIKI_STATS_MISSING_ITEM"),
		testItem(1, "WIKI_STATS_MISSING_ITEM"),
	}
	itemStats := NewItemProcessingStats("test", "uuid", "profile", nil)
	ProcessItemsWithNEUCacheAndStats(items, "inventory", map[string]models.NEUItem{}, itemStats)

	if itemStats.NEUWikiCacheMisses != 1 || itemStats.NEUWikiCacheHits != 1 {
		t.Fatalf("wiki cache misses/hits = %d/%d, want 1/1", itemStats.NEUWikiCacheMisses, itemStats.NEUWikiCacheHits)
	}
	if itemStats.NEUWikiDuration <= 0 {
		t.Fatal("NEU wiki duration was not recorded")
	}
}

func TestItemProcessingDebugSummaryEnvGate(t *testing.T) {
	itemStats := NewItemProcessingStats("test", "uuid", "profile", nil)
	itemStats.TotalItems = 1

	t.Setenv("ITEM_PROCESSING_DEBUG", "off")
	var disabled bytes.Buffer
	if itemStats.WriteDebugSummary(&disabled, time.Second) {
		t.Fatal("WriteDebugSummary returned true with debug disabled")
	}
	if disabled.Len() != 0 {
		t.Fatalf("disabled debug output = %q, want empty", disabled.String())
	}

	t.Setenv("ITEM_PROCESSING_DEBUG", "summary")
	t.Setenv("ITEM_PROCESSING_DEBUG_SLOW_MS", "0")
	var enabled bytes.Buffer
	if !itemStats.WriteDebugSummary(&enabled, time.Millisecond) {
		t.Fatal("WriteDebugSummary returned false with debug enabled")
	}
	if !strings.Contains(enabled.String(), "[ITEM_PROCESSING_DEBUG]") {
		t.Fatalf("enabled debug output = %q, want item processing tag", enabled.String())
	}
}

func BenchmarkProcessItemsItemEndpoint_1600Items(b *testing.B) {
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
