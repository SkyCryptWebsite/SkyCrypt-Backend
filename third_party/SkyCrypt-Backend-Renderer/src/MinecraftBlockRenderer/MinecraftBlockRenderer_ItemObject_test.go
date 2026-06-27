package minecraftblockrenderer

import (
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"testing"
)

func TestRenderItemObjectWithResourceIdMatchesDirectRender(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	renderer := CreateFromMinecraftAssets(assetsRoot, nil, nil)
	options := &BlockRenderOptions{Size: 64}

	item := map[string]any{
		"id":    "minecraft:diamond_sword",
		"Count": 1,
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id": "DIAMOND_SWORD",
			},
		},
	}

	fromObject, err := renderer.RenderItemObjectWithResourceId(item, options)
	if err != nil {
		t.Fatal(err)
	}
	direct := renderer.RenderGuiItemWithResourceId("minecraft:diamond_sword", options)
	if direct == nil {
		t.Fatal("direct render returned nil")
	}
	if !imagesAreIdentical(direct.Image, fromObject.Image) {
		t.Fatal("item-object render does not match direct render")
	}
	if fromObject.ResourceId.ResourceId == "" {
		t.Fatal("item-object render returned empty resource id")
	}
}

func TestRenderItemNBTMatchesDirectRender(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	renderer := CreateFromMinecraftAssets(assetsRoot, nil, nil)
	options := &BlockRenderOptions{Size: 64}

	item := nbt.NewNbtCompound(map[string]nbt.NbtTag{
		"id":    nbt.NewNbtString("minecraft:diamond_sword"),
		"Count": nbt.NewNbtByte(1),
	})

	fromNBT, err := renderer.RenderItemNBT(item, options)
	if err != nil {
		t.Fatal(err)
	}
	direct := renderer.RenderGuiItemWithResourceId("minecraft:diamond_sword", options)
	if direct == nil {
		t.Fatal("direct render returned nil")
	}
	if !imagesAreIdentical(direct.Image, fromNBT.Image) {
		t.Fatal("NBT render does not match direct render")
	}
}

func TestComputeResourceIdFromItemObjectIgnoresUnusedCustomData(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	renderer := CreateFromMinecraftAssets(assetsRoot, nil, nil)
	options := &BlockRenderOptions{Size: 64}

	base := map[string]any{
		"id":    "minecraft:diamond_sword",
		"Count": 1,
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id":   "DIAMOND_SWORD",
				"uuid": "one",
			},
		},
	}
	changed := map[string]any{
		"id":    "minecraft:diamond_sword",
		"Count": 1,
		"tag": map[string]any{
			"ExtraAttributes": map[string]any{
				"id":                "DIAMOND_SWORD",
				"uuid":              "two",
				"cultivated_crops":  98761234532,
				"irrelevant_nested": map[string]any{"value": "ignored"},
			},
		},
	}

	first, err := renderer.ComputeResourceIdFromItemObject(base, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := renderer.ComputeResourceIdFromItemObject(changed, options)
	if err != nil {
		t.Fatal(err)
	}
	if first.ResourceId != second.ResourceId {
		t.Fatalf("unused custom data changed resource id:\n%s\n%s", first.ResourceId, second.ResourceId)
	}
}

func TestItemObjectPreservesCustomHeadTextureDataInResourceId(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	renderer := CreateFromMinecraftAssets(assetsRoot, nil, nil)
	options := &BlockRenderOptions{Size: 64}

	item := map[string]any{
		"id": "minecraft:player_head",
		"tag": map[string]any{
			"SkullOwner": map[string]any{
				"Properties": map[string]any{
					"textures": []any{
						map[string]any{
							"Value":     "texture-value-one",
							"Signature": "texture-signature-one",
						},
					},
				},
			},
		},
	}

	result, err := renderer.ComputeResourceIdFromItemObject(item, options)
	if err != nil {
		t.Fatal(err)
	}
	if result.ResourceId == "" {
		t.Fatal("empty resource id")
	}
	changed := map[string]any{
		"id": "minecraft:player_head",
		"tag": map[string]any{
			"SkullOwner": map[string]any{
				"Properties": map[string]any{
					"textures": []any{
						map[string]any{
							"Value":     "texture-value-two",
							"Signature": "texture-signature-two",
						},
					},
				},
			},
		},
	}
	changedResult, err := renderer.ComputeResourceIdFromItemObject(changed, options)
	if err != nil {
		t.Fatal(err)
	}
	if result.ResourceId == changedResult.ResourceId {
		t.Fatal("different skull texture data should produce a different resource id")
	}
}

func TestSkullResolverContextProvidesFullItemData(t *testing.T) {
	assetsRoot := requireFullAssets(t)
	renderer := CreateFromMinecraftAssets(assetsRoot, nil, nil)

	customData := nbt.NewNbtCompound(map[string]nbt.NbtTag{
		"id": nbt.NewNbtString("CUSTOM_HEAD"),
		"stats": nbt.NewNbtCompound(map[string]nbt.NbtTag{
			"damage": nbt.NewNbtInt(42),
		}),
	})
	itemData := &data.ItemRenderData{CustomData: customData}
	var sawContext bool
	options := &BlockRenderOptions{
		Size:     64,
		ItemData: itemData,
		SkullTextureResolver: func(context SkullResolverContext) *string {
			sawContext = true
			if context.ItemId != "player_head" && context.ItemId != "minecraft:player_head" {
				t.Fatalf("context item id = %q", context.ItemId)
			}
			if context.ItemData != itemData {
				t.Fatal("context did not preserve ItemData pointer")
			}
			if context.CustomDataId == nil || *context.CustomDataId != "CUSTOM_HEAD" {
				t.Fatalf("custom data id = %#v", context.CustomDataId)
			}
			if context.CustomData == nil || !context.CustomData.ContainsKey("stats") {
				t.Fatal("nested custom data was not provided")
			}
			texture := "minecraft:item/diamond_sword"
			return &texture
		},
	}

	img := renderer.RenderItem("minecraft:player_head", itemData, options)
	if img == nil || !hasOpaquePixels(img) {
		t.Fatal("skull resolver render did not produce visible pixels")
	}
	if !sawContext {
		t.Fatal("skull resolver was not called")
	}
}
