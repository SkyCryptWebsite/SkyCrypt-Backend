package minecraftblockrenderer

import (
	"fmt"
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"image"
	"strings"
)

func (renderer *MinecraftBlockRenderer) RenderItemObject(item any, options *BlockRenderOptions) (*image.RGBA, error) {
	normalized, err := data.NormalizeItemInput(item)
	if err != nil {
		return nil, err
	}

	itemName, itemData := renderer.prepareItemObjectRender(normalized)
	if strings.TrimSpace(itemName) == "" {
		return nil, fmt.Errorf("unable to resolve item id from input type %T", item)
	}

	effectiveOptions := mergeItemObjectOptions(options, itemData)
	itemName = renderer.resolvePackedSkyblockItemObjectName(normalized, itemName, effectiveOptions)
	effectiveOptions = renderer.normalizePackedSkyblockItemObjectOptions(normalized, effectiveOptions)
	rendered := renderer.RenderItem(itemName, effectiveOptions.ItemData, effectiveOptions)
	if rendered == nil {
		return nil, fmt.Errorf("failed to render item %s", itemName)
	}
	return rendered, nil
}

func (renderer *MinecraftBlockRenderer) RenderItemObjectWithResourceId(item any, options *BlockRenderOptions) (*RenderedResource, error) {
	normalized, err := data.NormalizeItemInput(item)
	if err != nil {
		return nil, err
	}

	itemName, itemData := renderer.prepareItemObjectRender(normalized)
	if strings.TrimSpace(itemName) == "" {
		return nil, fmt.Errorf("unable to resolve item id from input type %T", item)
	}

	effectiveOptions := mergeItemObjectOptions(options, itemData)
	itemName = renderer.resolvePackedSkyblockItemObjectName(normalized, itemName, effectiveOptions)
	effectiveOptions = renderer.normalizePackedSkyblockItemObjectOptions(normalized, effectiveOptions)
	rendered := renderer.RenderGuiItemWithResourceId(itemName, effectiveOptions)
	if rendered == nil {
		return nil, fmt.Errorf("failed to render item %s", itemName)
	}
	return rendered, nil
}

func (renderer *MinecraftBlockRenderer) RenderAnimatedItemObjectWithResourceId(item any, options *BlockRenderOptions) (*AnimatedRenderedResource, error) {
	normalized, err := data.NormalizeItemInput(item)
	if err != nil {
		return nil, err
	}

	itemName, itemData := renderer.prepareItemObjectRender(normalized)
	if strings.TrimSpace(itemName) == "" {
		return nil, fmt.Errorf("unable to resolve item id from input type %T", item)
	}

	effectiveOptions := mergeItemObjectOptions(options, itemData)
	itemName = renderer.resolvePackedSkyblockItemObjectName(normalized, itemName, effectiveOptions)
	effectiveOptions = renderer.normalizePackedSkyblockItemObjectOptions(normalized, effectiveOptions)
	return renderer.RenderAnimatedGuiItemWithResourceId(itemName, effectiveOptions)
}

func (renderer *MinecraftBlockRenderer) RenderSkyBlockItemID(skyBlockItemID string, options *BlockRenderOptions) (*RenderedResource, error) {
	target, effectiveOptions, err := renderer.prepareSkyBlockItemIDRender(skyBlockItemID, options)
	if err != nil {
		return nil, err
	}
	rendered := renderer.RenderGuiItemWithResourceId(target, effectiveOptions)
	if rendered == nil {
		return nil, fmt.Errorf("failed to render skyblock item %s", skyBlockItemID)
	}
	return rendered, nil
}

func (renderer *MinecraftBlockRenderer) RenderItemNBT(item any, options *BlockRenderOptions) (*RenderedResource, error) {
	return renderer.RenderItemObjectWithResourceId(normalizeNbtRenderInput(item), options)
}

func (renderer *MinecraftBlockRenderer) RenderAnimatedSkyBlockItemID(skyBlockItemID string, options *BlockRenderOptions) (*AnimatedRenderedResource, error) {
	target, effectiveOptions, err := renderer.prepareSkyBlockItemIDRender(skyBlockItemID, options)
	if err != nil {
		return nil, err
	}
	return renderer.RenderAnimatedGuiItemWithResourceId(target, effectiveOptions)
}

func (renderer *MinecraftBlockRenderer) RenderAnimatedItemNBT(item any, options *BlockRenderOptions) (*AnimatedRenderedResource, error) {
	return renderer.RenderAnimatedItemObjectWithResourceId(normalizeNbtRenderInput(item), options)
}

func (renderer *MinecraftBlockRenderer) ComputeResourceIdFromSkyBlockItemID(skyBlockItemID string, options *BlockRenderOptions) (*ResourceIdResult, error) {
	target, effectiveOptions, err := renderer.prepareSkyBlockItemIDRender(skyBlockItemID, options)
	if err != nil {
		return nil, err
	}
	rendererForOptions, forwardedOptions := renderer.ResolveRendererForOptions(*effectiveOptions)
	return rendererForOptions.ComputeResourceIdInternal(target, forwardedOptions, nil), nil
}

func (renderer *MinecraftBlockRenderer) ComputeResourceIdFromNBT(item any, options *BlockRenderOptions) (*ResourceIdResult, error) {
	return renderer.ComputeResourceIdFromItemObject(normalizeNbtRenderInput(item), options)
}

func (renderer *MinecraftBlockRenderer) ComputeResourceIdFromItemObject(item any, options *BlockRenderOptions) (*ResourceIdResult, error) {
	normalized, err := data.NormalizeItemInput(item)
	if err != nil {
		return nil, err
	}

	itemName, itemData := renderer.prepareItemObjectRender(normalized)
	if strings.TrimSpace(itemName) == "" {
		return nil, fmt.Errorf("unable to resolve item id from input type %T", item)
	}

	effectiveOptions := mergeItemObjectOptions(options, itemData)
	itemName = renderer.resolvePackedSkyblockItemObjectName(normalized, itemName, effectiveOptions)
	effectiveOptions = renderer.normalizePackedSkyblockItemObjectOptions(normalized, effectiveOptions)
	rendererForOptions, forwardedOptions := renderer.ResolveRendererForOptions(*effectiveOptions)
	return rendererForOptions.ComputeResourceIdInternal(itemName, forwardedOptions, nil), nil
}

func (renderer *MinecraftBlockRenderer) prepareSkyBlockItemIDRender(skyBlockItemID string, options *BlockRenderOptions) (string, *BlockRenderOptions, error) {
	if strings.TrimSpace(skyBlockItemID) == "" {
		return "", nil, fmt.Errorf("skyBlockItemID cannot be empty")
	}

	itemData := &data.ItemRenderData{
		CustomData: nbt.NewNbtCompound(map[string]nbt.NbtTag{
			"id": nbt.NewNbtString(skyBlockItemID),
		}),
	}

	effectiveOptions := mergeItemObjectOptions(options, itemData)
	if strings.TrimSpace(effectiveOptions.CustomTextureFallbackItem) == "" {
		effectiveOptions.CustomTextureFallbackItem = "minecraft:player_head"
	}
	target := "minecraft:player_head"
	if len(effectiveOptions.PackIds) > 0 {
		encodedID := renderer.EncodeFirmamentId(skyBlockItemID)
		if renderer.shouldRenderSkyBlockIDAsVanilla(encodedID, effectiveOptions) {
			target = "minecraft:" + encodedID
			effectiveOptions = renderer.withoutSkyBlockCustomData(effectiveOptions)
		} else {
			target = "firmskyblock:item/" + encodedID
		}
	}
	return target, effectiveOptions, nil
}

func (renderer *MinecraftBlockRenderer) prepareItemObjectRender(normalized *data.NormalizedItemInput) (string, *data.ItemRenderData) {
	itemName := renderer.resolveItemObjectName(normalized)
	itemData := buildItemObjectRenderData(normalized)
	if normalized != nil && normalized.DisplayColor != nil && renderer.ShouldApplyNBTDisplayColor(itemName) {
		itemData = ensureItemRenderData(itemData)
		itemData.Layer0Tint = normalized.DisplayColor
	}
	return itemName, itemData
}

func (renderer *MinecraftBlockRenderer) resolvePackedSkyblockItemObjectName(normalized *data.NormalizedItemInput, itemName string, options *BlockRenderOptions) string {
	if normalized == nil || options == nil || len(options.PackIds) == 0 {
		return itemName
	}
	if strings.TrimSpace(normalized.SkyblockID) == "" {
		return itemName
	}
	if strings.TrimSpace(options.CustomTextureFallbackItem) == "" {
		options.CustomTextureFallbackItem = renderer.resolvePackedSkyblockFallbackItem(normalized, itemName)
	}
	encodedID := renderer.EncodeFirmamentId(normalized.SkyblockID)
	if renderer.shouldRenderSkyBlockIDAsVanilla(encodedID, options) {
		if strings.TrimSpace(normalized.ItemModel) != "" {
			model := strings.TrimSpace(normalized.ItemModel)
			if mapped, ok := legacyStringItemID(model, normalized.Damage); ok {
				return "minecraft:" + mapped
			}
			return model
		}
		if mapped, ok := resolveLegacyItemName(normalized.ItemID, normalized.NumericID, normalized.Damage); ok {
			return "minecraft:" + mapped
		}
		if strings.TrimSpace(normalized.ItemID) != "" {
			return normalized.ItemID
		}
		return "minecraft:" + encodedID
	}
	return "firmskyblock:item/" + renderer.EncodeFirmamentId(normalized.SkyblockID)
}

func (renderer *MinecraftBlockRenderer) resolvePackedSkyblockFallbackItem(normalized *data.NormalizedItemInput, itemName string) string {
	if normalized != nil {
		if strings.TrimSpace(normalized.ItemModel) != "" {
			model := strings.TrimSpace(normalized.ItemModel)
			if mapped, ok := legacyStringItemID(model, normalized.Damage); ok {
				return "minecraft:" + mapped
			}
			return model
		}
		if mapped, ok := resolveLegacyItemName(normalized.ItemID, normalized.NumericID, normalized.Damage); ok {
			return "minecraft:" + mapped
		}
		if strings.TrimSpace(normalized.ItemID) != "" {
			return normalized.ItemID
		}
	}
	skyblockID := ""
	if normalized != nil {
		skyblockID = strings.TrimSpace(normalized.SkyblockID)
	}
	if strings.TrimSpace(itemName) != "" && !strings.EqualFold(itemName, skyblockID) {
		return itemName
	}
	return "minecraft:player_head"
}

func (renderer *MinecraftBlockRenderer) normalizePackedSkyblockItemObjectOptions(normalized *data.NormalizedItemInput, options *BlockRenderOptions) *BlockRenderOptions {
	if normalized == nil || options == nil || len(options.PackIds) == 0 {
		return options
	}
	if strings.TrimSpace(normalized.SkyblockID) == "" || options.ItemData == nil {
		return options
	}

	if renderer.shouldRenderSkyBlockIDAsVanilla(renderer.EncodeFirmamentId(normalized.SkyblockID), options) {
		return renderer.withoutSkyBlockCustomData(options)
	}

	effectiveOptions := *options
	itemData := *options.ItemData
	if effectiveOptions.CustomTextureFallbackData == nil {
		fallbackData := itemData
		effectiveOptions.CustomTextureFallbackData = &fallbackData
	}
	itemData.Profile = nil
	itemData.Layer0Tint = nil
	itemData.AdditionalLayerTints = nil
	effectiveOptions.ItemData = &itemData
	return &effectiveOptions
}

func (renderer *MinecraftBlockRenderer) shouldRenderSkyBlockIDAsVanilla(encodedID string, options *BlockRenderOptions) bool {
	if renderer == nil || strings.TrimSpace(encodedID) == "" || options == nil || len(options.PackIds) == 0 {
		return false
	}
	if renderer.hasPackedSkyBlockResource(encodedID, options) {
		return false
	}
	return renderer.hasVanillaItem(encodedID)
}

func (renderer *MinecraftBlockRenderer) hasPackedSkyBlockResource(encodedID string, options *BlockRenderOptions) bool {
	effective := MergeBlockRenderOptions(options)
	packRenderer, _ := renderer.ResolveRendererForOptions(effective)
	if packRenderer == nil {
		packRenderer = renderer
	}
	if entry := packRenderer.getSkyblockItemDefinition(strings.ToLower(encodedID)); entry.Loaded {
		return true
	}
	if packRenderer._modelResolver != nil {
		if resolved, exists := packRenderer._modelResolver.TryResolve("firmskyblock:item/" + encodedID); exists && resolved != nil {
			return true
		}
	}
	return false
}

func (renderer *MinecraftBlockRenderer) hasVanillaItem(encodedID string) bool {
	if renderer == nil || renderer._itemRegistry == nil || strings.TrimSpace(encodedID) == "" {
		return false
	}
	return renderer._itemRegistry.GetItemInfo(encodedID) != nil ||
		renderer._itemRegistry.GetItemInfo("minecraft:"+encodedID) != nil
}

func (renderer *MinecraftBlockRenderer) withoutSkyBlockCustomData(options *BlockRenderOptions) *BlockRenderOptions {
	if options == nil {
		return nil
	}
	effectiveOptions := *options
	effectiveOptions.CustomTextureFallbackItem = ""
	effectiveOptions.CustomTextureFallbackData = nil
	if options.ItemData != nil {
		itemData := *options.ItemData
		itemData.CustomData = nil
		itemData.Profile = nil
		effectiveOptions.ItemData = &itemData
	}
	return &effectiveOptions
}

func (renderer *MinecraftBlockRenderer) resolveItemObjectName(normalized *data.NormalizedItemInput) string {
	if normalized == nil {
		return ""
	}

	if strings.TrimSpace(normalized.ItemModel) != "" {
		model := strings.TrimSpace(normalized.ItemModel)
		if mapped, ok := legacyStringItemID(model, normalized.Damage); ok {
			return mapped
		}
		if strings.HasPrefix(strings.ToLower(model), "minecraft:") {
			return strings.TrimPrefix(model, "minecraft:")
		}
		return model
	}

	if mapped, ok := resolveLegacyItemName(normalized.ItemID, normalized.NumericID, normalized.Damage); ok {
		return mapped
	}

	if strings.TrimSpace(normalized.ItemID) != "" {
		return normalized.ItemID
	}

	if strings.TrimSpace(normalized.SkyblockID) != "" {
		return normalized.SkyblockID
	}

	return ""
}

func buildItemObjectRenderData(normalized *data.NormalizedItemInput) *data.ItemRenderData {
	if normalized == nil {
		return nil
	}

	itemData := &data.ItemRenderData{}
	itemData.ItemModel = resolveItemObjectSelectorModel(normalized)
	if normalized.ExtraAttributes != nil {
		itemData.CustomData = data.DecodedMapToNbtCompound(normalized.ExtraAttributes)
	}
	if itemData.CustomData == nil && normalized.CustomData != nil {
		itemData.CustomData = data.DecodedMapToNbtCompound(normalized.CustomData)
	}
	if normalized.SkullProfile != nil {
		itemData.Profile = skullProfileMapToNbtCompound(normalized.SkullProfile)
	}

	if itemData.CustomData == nil && itemData.Profile == nil && itemData.Layer0Tint == nil && strings.TrimSpace(itemData.ItemModel) == "" {
		return nil
	}

	return itemData
}

func resolveItemObjectSelectorModel(normalized *data.NormalizedItemInput) string {
	if normalized == nil {
		return ""
	}
	if strings.TrimSpace(normalized.ItemModel) != "" {
		model := strings.TrimSpace(normalized.ItemModel)
		if mapped, ok := legacyStringItemID(model, normalized.Damage); ok {
			return "minecraft:" + mapped
		}
		return model
	}
	if mapped, ok := resolveLegacyItemName(normalized.ItemID, normalized.NumericID, normalized.Damage); ok {
		return "minecraft:" + mapped
	}
	if strings.TrimSpace(normalized.ItemID) != "" {
		return strings.TrimSpace(normalized.ItemID)
	}
	return ""
}

func ensureItemRenderData(itemData *data.ItemRenderData) *data.ItemRenderData {
	if itemData != nil {
		return itemData
	}
	return &data.ItemRenderData{}
}

func (renderer *MinecraftBlockRenderer) ShouldApplyNBTDisplayColor(itemName string) bool {
	normalized := renderer.NormalizeItemTextureKey(itemName)
	if strings.TrimSpace(normalized) == "" {
		return false
	}

	if _, found := LegacyDefaultTintOverrides[normalized]; found {
		return true
	}
	return strings.HasPrefix(strings.ToLower(normalized), "leather_")
}

func mergeItemObjectOptions(options *BlockRenderOptions, itemData *data.ItemRenderData) *BlockRenderOptions {
	effective := MergeBlockRenderOptions(options)
	if itemData != nil {
		effective.ItemData = itemData
	}
	return &effective
}

func normalizeNbtRenderInput(item any) any {
	switch typed := item.(type) {
	case *nbt.NbtCompound:
		return nbtCompoundToMap(typed)
	default:
		return item
	}
}

func nbtCompoundToMap(compound *nbt.NbtCompound) map[string]any {
	if compound == nil {
		return nil
	}
	result := make(map[string]any, compound.Count())
	for key, tag := range compound.Items() {
		result[key] = nbtTagToAny(tag)
	}
	return result
}

func nbtTagToAny(tag nbt.NbtTag) any {
	switch typed := tag.(type) {
	case nbt.NbtString:
		return typed.Value
	case *nbt.NbtString:
		return typed.Value
	case nbt.NbtByte:
		return int(typed.Value)
	case *nbt.NbtByte:
		return int(typed.Value)
	case nbt.NbtShort:
		return int(typed.Value)
	case *nbt.NbtShort:
		return int(typed.Value)
	case nbt.NbtInt:
		return int(typed.Value)
	case *nbt.NbtInt:
		return int(typed.Value)
	case nbt.NbtLong:
		return typed.Value
	case *nbt.NbtLong:
		return typed.Value
	case nbt.NbtFloat:
		return typed.Value
	case *nbt.NbtFloat:
		return typed.Value
	case nbt.NbtDouble:
		return typed.Value
	case *nbt.NbtDouble:
		return typed.Value
	case nbt.NbtByteArray:
		return typed.Values
	case *nbt.NbtByteArray:
		return typed.Values
	case nbt.NbtIntArray:
		return typed.Values
	case *nbt.NbtIntArray:
		return typed.Values
	case nbt.NbtLongArray:
		return typed.Values
	case *nbt.NbtLongArray:
		return typed.Values
	case *nbt.NbtCompound:
		return nbtCompoundToMap(typed)
	case *nbt.NbtList:
		items := make([]any, 0, typed.Count())
		for i := 0; i < typed.Count(); i++ {
			items = append(items, nbtTagToAny(typed.At(i)))
		}
		return items
	default:
		return fmt.Sprint(tag)
	}
}

func skullProfileMapToNbtCompound(profile map[string]any) *nbt.NbtCompound {
	value, hasValue := profileString(profile, "value", "Value")
	if !hasValue || strings.TrimSpace(value) == "" {
		return nil
	}

	propertyEntries := map[string]nbt.NbtTag{
		"name":  nbt.NewNbtString("textures"),
		"value": nbt.NewNbtString(value),
	}
	if signature, ok := profileString(profile, "signature", "Signature"); ok && strings.TrimSpace(signature) != "" {
		propertyEntries["signature"] = nbt.NewNbtString(signature)
	}

	propertyCompound := nbt.NewNbtCompound(propertyEntries)
	profileEntries := map[string]nbt.NbtTag{
		"properties": nbt.NewNbtList(nbt.NbtTagTypeCompound, []nbt.NbtTag{propertyCompound}),
	}

	if id, ok := profileString(profile, "id", "Id", "ID"); ok && strings.TrimSpace(id) != "" {
		profileEntries["id"] = nbt.NewNbtString(id)
	}

	return nbt.NewNbtCompound(profileEntries)
}

func profileString(values map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		for actualKey, value := range values {
			if strings.EqualFold(actualKey, key) {
				text := fmt.Sprint(value)
				return text, strings.TrimSpace(text) != ""
			}
		}
	}
	return "", false
}
