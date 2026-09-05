package lib

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	mr "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer"
	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"

	"skycrypt/src/constants"
	"skycrypt/src/utility"
)

func stableTextureKeysFromInput(input ItemTextureInput) []string {
	keys := make([]string, 0, 4)
	if skullKey := skullIdentityFromInput(input); skullKey != "" {
		if skyblockID := strings.TrimSpace(input.SkyBlockID); skyblockID != "" {
			keys = append(keys, "skyblock:"+skyblockID+"|"+skullKey)
		}
		keys = append(keys, skullKey)
		return keys
	}

	itemModel := normalizeMinecraftItemID(input.ItemModel)
	if skyblockID := strings.TrimSpace(input.SkyBlockID); skyblockID != "" {
		if itemModel != "" {
			keys = append(keys, "skyblock:"+skyblockID+"|itemmodel:"+itemModel)
		} else {
			keys = append(keys, "skyblock:"+skyblockID)
		}
		return keys
	}
	if itemModel != "" {
		keys = append(keys, "itemmodel:"+itemModel)
	}
	if id := normalizeMinecraftItemID(input.ID); id != "" {
		keys = append(keys, fmt.Sprintf("mc:%s|damage:%d|color:%d", id, input.Damage, input.DisplayColor))
	}
	if skinHash := skullTextureHashFromOwner(input.SkullOwner); skinHash != "" {
		keys = append(keys, "skull:"+skinHash)
	}
	return keys
}

type textureCacheLookupDetail struct {
	Reason    string
	StableKey string
}

func isGenericPackedSkullTexture(input ItemTextureInput, texture AppliedItemTexture) bool {
	id := normalizeMinecraftItemID(input.ID)
	if id == "" {
		id = normalizeMinecraftItemID(input.ItemModel)
	}
	hasSkullIdentity := skullIdentityFromInput(input) != ""
	if !hasSkullIdentity && !isVanillaSkullItemID(id) {
		return false
	}

	texturePath := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(texture.Texture), "\\", "/"))
	if hasSkullIdentity && (sameTexturePack(texture.TexturePack, "vanilla") || strings.Contains(texturePath, "pack=vanilla")) {
		return true
	}

	return strings.Contains(texturePath, "model=minecraft_item_template_skull") &&
		strings.Contains(texturePath, "tex1=block_soul_sand")
}

func isVanillaSkullItemID(id string) bool {
	switch normalizeMinecraftItemID(id) {
	case "minecraft:player_head",
		"minecraft:zombie_head",
		"minecraft:skeleton_skull",
		"minecraft:wither_skeleton_skull",
		"minecraft:creeper_head",
		"minecraft:dragon_head",
		"minecraft:piglin_head":
		return true
	default:
		return false
	}
}

func cachedTextureForInputDetailed(input ItemTextureInput, textureCtx TextureApplyContext) (AppliedItemTexture, bool, textureCacheLookupDetail) {
	skyblockID := strings.TrimSpace(input.SkyBlockID)
	hasItemModel := normalizeMinecraftItemID(input.ItemModel) != ""
	hasSkullIdentity := skullIdentityFromInput(input) != ""
	if !hasItemModel || hasSkullIdentity {
		if texture, ok := cachedStableSkyBlockTexture(skyblockID, textureCtx); ok && !isGenericPackedSkullTexture(input, texture) {
			return texture, true, textureCacheLookupDetail{
				Reason:    "stable_skyblock_cache",
				StableKey: "skyblock:" + skyblockID,
			}
		}
	}

	for _, stableKey := range stableTextureKeysFromInput(input) {
		if skyblockID != "" && stableKey == "skyblock:"+skyblockID {
			continue
		}
		if texture, ok := cachedTextureForStableKey(stableKey, textureCtx.PackSignature, textureCtx.EnabledPackIDs, textureCtx.EnabledPackSet); ok && !isGenericPackedSkullTexture(input, texture) {
			return texture, true, textureCacheLookupDetail{
				Reason:    "stable_key_cache",
				StableKey: stableKey,
			}
		}
	}
	if skyblockID != "" && !hasSkullIdentity && !hasItemModel {
		if texture, ok := cachedItemTexture(skyblockID, textureCtx); ok && !isGenericPackedSkullTexture(input, texture) {
			return texture, true, textureCacheLookupDetail{
				Reason:    "legacy_skyblock_cache",
				StableKey: skyblockID,
			}
		}
	}
	return AppliedItemTexture{}, false, textureCacheLookupDetail{}
}

func cachedStableSkyBlockTexture(skyblockID string, textureCtx TextureApplyContext) (AppliedItemTexture, bool) {
	skyblockID = strings.TrimSpace(skyblockID)
	if skyblockID == "" {
		return AppliedItemTexture{}, false
	}
	return cachedTextureForStableKey("skyblock:"+skyblockID, textureCtx.PackSignature, textureCtx.EnabledPackIDs, textureCtx.EnabledPackSet)
}

func setCachedTextureForInput(input ItemTextureInput, textureCtx TextureApplyContext, texture AppliedItemTexture) {
	if texture.Texture == "" {
		return
	}
	for _, stableKey := range stableTextureKeysFromInput(input) {
		setCachedTextureForStableKey(textureCtx.PackSignature, stableKey, texture)
	}
}

func itemTextureInputRenderMap(input ItemTextureInput) map[string]any {
	id := normalizeMinecraftItemID(input.ID)
	if id == "" && input.NumericID > 0 {
		if mapped := constants.GetVanillaItemId(constants.ItemModel{NumericId: input.NumericID, ItemDamage: input.Damage}); mapped != "" {
			id = "minecraft:" + mapped
		}
	}
	if id == "" && input.ItemModel != "" {
		id = normalizeMinecraftItemID(input.ItemModel)
	}

	tag := input.Tag
	if tag == nil {
		tag = map[string]any{"ExtraAttributes": map[string]any{"id": input.SkyBlockID}}
	}

	itemMap := map[string]any{
		"id":          id,
		"tag":         tag,
		"damage":      input.Damage,
		"item_id":     input.NumericID,
		"skyblock_id": input.SkyBlockID,
	}
	if input.ItemModel != "" {
		itemMap["ItemModel"] = input.ItemModel
	}
	if input.Texture != "" {
		itemMap["texture"] = input.Texture
	}
	return itemMap
}

func itemTextureInputFromMap(itemMap map[string]any) ItemTextureInput {
	itemModel := itemModelFromItem(itemMap)
	id := explicitMinecraftItemIDFromItem(itemMap)
	numericID, _ := textureInt(itemMap, "item_id", "itemId", "itemID")
	damage, _ := textureInt(itemMap, "item_damage", "itemDamage", "damage", "Damage")
	displayColor := 0
	if armorColor := textureString(itemMap, "armor_color", "armorColor"); armorColor != "" {
		if parsed, err := strconv.ParseInt(strings.TrimPrefix(armorColor, "#"), 16, 32); err == nil {
			displayColor = int(parsed)
		}
	} else if color, ok := textureInt(itemMap, "display_color", "displayColor"); ok {
		displayColor = color
	} else if colorHex := displayColorFromItem(itemMap); colorHex != "" {
		if parsed, err := strconv.ParseInt(colorHex, 16, 32); err == nil {
			displayColor = int(parsed)
		}
	}
	tag, _ := textureValue(itemMap, "tag", "Tag")

	return ItemTextureInput{
		ID:           id,
		ItemModel:    itemModel,
		SkyBlockID:   skyblockIDFromItem(itemMap),
		NumericID:    numericID,
		Damage:       damage,
		Texture:      textureString(itemMap, "texture", "Texture"),
		DisplayColor: displayColor,
		Tag:          tag,
	}
}

func skullIdentityFromInput(input ItemTextureInput) string {
	if input.SkullOwner != nil {
		if id := strings.TrimSpace(input.SkullOwner.ID); id != "" {
			return "skullid:" + id
		}
		if skinHash := skullTextureHashFromOwner(input.SkullOwner); skinHash != "" {
			return "skull:" + skinHash
		}
	}

	switch tag := input.Tag.(type) {
	case *skycrypttypes.Tag:
		if tag == nil || tag.SkullOwner == nil {
			return ""
		}
		if id := strings.TrimSpace(tag.SkullOwner.ID); id != "" {
			return "skullid:" + id
		}
		if skinHash := skullTextureHashFromOwner(tag.SkullOwner); skinHash != "" {
			return "skull:" + skinHash
		}
		return ""
	case skycrypttypes.Tag:
		if tag.SkullOwner == nil {
			return ""
		}
		if id := strings.TrimSpace(tag.SkullOwner.ID); id != "" {
			return "skullid:" + id
		}
		if skinHash := skullTextureHashFromOwner(tag.SkullOwner); skinHash != "" {
			return "skull:" + skinHash
		}
		return ""
	case skycrypttypes.TextureItemExtraAttributes:
		if tag.SkullOwner == nil {
			return ""
		}
		if id := strings.TrimSpace(tag.SkullOwner.ID); id != "" {
			return "skullid:" + id
		}
		if skinHash := skullTextureHashFromOwner(tag.SkullOwner); skinHash != "" {
			return "skull:" + skinHash
		}
		return ""
	case *skycrypttypes.TextureItemExtraAttributes:
		if tag == nil || tag.SkullOwner == nil {
			return ""
		}
		if id := strings.TrimSpace(tag.SkullOwner.ID); id != "" {
			return "skullid:" + id
		}
		if skinHash := skullTextureHashFromOwner(tag.SkullOwner); skinHash != "" {
			return "skull:" + skinHash
		}
		return ""
	}

	if _, ok := input.Tag.(map[string]any); ok {
		if tagID := skullOwnerIDFromTag(input.Tag); tagID != "" {
			return "skullid:" + tagID
		}
		if skinHash := skullTextureHashFromTag(input.Tag); skinHash != "" {
			return "skull:" + skinHash
		}
	}
	return ""
}

func skullTextureHashFromOwner(skullOwner *skycrypttypes.SkullOwner) string {
	if skullOwner == nil || len(skullOwner.Properties.Textures) == 0 {
		return ""
	}
	return utility.GetSkinHash(skullOwner.Properties.Textures[0].Value)
}

func skullFallbackTexture(input ItemTextureInput, textureCtx TextureApplyContext) AppliedItemTexture {
	if texture := headTextureURL(input.Texture); texture.Texture != "" {
		return texture
	}
	if skinHash := skullTextureHashFromOwner(input.SkullOwner); skinHash != "" {
		return AppliedItemTexture{Texture: fmt.Sprintf("%s/api/head/%s", textureCtx.Domain, skinHash)}
	}
	if skinHash := skullTextureHashFromTag(input.Tag); skinHash != "" {
		return AppliedItemTexture{Texture: fmt.Sprintf("%s/api/head/%s", textureCtx.Domain, skinHash)}
	}
	return AppliedItemTexture{}
}

func rendererPackIDsForInput(input ItemTextureInput, enabledPackIDs []string) []string {
	model := normalizeMinecraftItemID(input.ItemModel)
	if !strings.HasPrefix(model, "hypixel_skyblock:item/") {
		return enabledPackIDs
	}

	supported := make([]string, 0, len(enabledPackIDs))
	for _, packID := range enabledPackIDs {
		if canonicalPackAlias(packID) == "aether_pack" {
			continue
		}
		supported = append(supported, packID)
	}
	return supported
}

const textureDecisionSampleLimit = 50

func textureDebugTraceEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("ITEM_PROCESSING_DEBUG")), "trace")
}

func addTextureDecisionSample(stats *TextureApplyStats, sample TextureDecisionSample) {
	if stats == nil {
		return
	}
	if len(stats.Samples) < textureDecisionSampleLimit {
		stats.Samples = append(stats.Samples, sample)
		return
	}

	shortestIndex := 0
	for index := 1; index < len(stats.Samples); index++ {
		if stats.Samples[index].Duration < stats.Samples[shortestIndex].Duration {
			shortestIndex = index
		}
	}
	if sample.Duration > stats.Samples[shortestIndex].Duration {
		stats.Samples[shortestIndex] = sample
	}
}

func recordTextureDecision(stats *TextureApplyStats, input ItemTextureInput, texture AppliedItemTexture, reason string, stableKey string, duration time.Duration) {
	if stats == nil {
		return
	}

	sample := TextureDecisionSample{
		SkyBlockID:  strings.TrimSpace(input.SkyBlockID),
		MinecraftID: normalizeMinecraftItemID(input.ID),
		ItemModel:   normalizeMinecraftItemID(input.ItemModel),
		Texture:     texture.Texture,
		TexturePack: texture.TexturePack,
		StableKey:   stableKey,
		Reason:      reason,
		Duration:    duration,
	}

	if !strings.HasPrefix(reason, "cache_hit") || duration >= time.Millisecond {
		addTextureDecisionSample(stats, sample)
	}

	if textureDebugTraceEnabled() {
		fmt.Printf(
			"[TEXTURE_DEBUG] reason=%s skyblock_id=%q minecraft_id=%q item_model=%q texture_pack=%q duration=%s texture=%q\n",
			reason,
			sample.SkyBlockID,
			sample.MinecraftID,
			sample.ItemModel,
			sample.TexturePack,
			duration,
			sample.Texture,
		)
	}
}

func recordTextureCacheHit(stats *TextureApplyStats, detail textureCacheLookupDetail) {
	if stats == nil {
		return
	}
	stats.CacheHits++
	switch detail.Reason {
	case "stable_skyblock_cache":
		stats.StableSkyBlockHits++
	case "stable_key_cache":
		stats.StableKeyHits++
	case "legacy_skyblock_cache":
		stats.LegacySkyBlockHits++
	case "raw_map_cache":
		stats.RawMapHits++
	}
}

func recordRuntimeRenderSkip(stats *TextureApplyStats, renderer *mr.Renderer, textureCtx TextureApplyContext, isGenericSkullInput bool) {
	if stats == nil {
		return
	}
	stats.RuntimeRenderSkipped++
	if textureCtx.DisableRuntimeRender {
		stats.RuntimeRenderSkippedDisabled++
		return
	}
	if renderer == nil {
		stats.RuntimeRenderSkippedRendererNil++
		return
	}
	if isGenericSkullInput {
		stats.RuntimeRenderSkippedGenericSkull++
	}
}

func skullOwnerIDFromTag(tag any) string {
	if tag == nil {
		return ""
	}
	tagMap, ok := tag.(map[string]any)
	if !ok {
		normalized, ok := normalizeTextureItem(tag)
		if !ok {
			return ""
		}
		tagMap = normalized
	}
	skullOwner, ok := textureMap(tagMap, "SkullOwner", "skullOwner", "skull_owner")
	if !ok {
		return ""
	}
	return textureString(skullOwner, "Id", "ID", "id")
}

func skullTextureHashFromTag(tag any) string {
	if tag == nil {
		return ""
	}
	tagMap, ok := tag.(map[string]any)
	if !ok {
		normalized, ok := normalizeTextureItem(tag)
		if !ok {
			return ""
		}
		tagMap = normalized
	}
	return skullTextureHashFromValues(tagMap)
}

func ApplyTextureInput(input ItemTextureInput, textureCtx TextureApplyContext) AppliedItemTexture {
	timeNow := time.Now()
	textureCtx = normalizeTextureApplyContext(textureCtx)
	stats := textureCtx.Stats
	if stats != nil {
		stats.Total++
	}

	finish := func(reason string, stableKey string, texture AppliedItemTexture) AppliedItemTexture {
		duration := time.Since(timeNow)
		if stats != nil {
			stats.TotalDuration += duration
		}
		recordTextureDecision(stats, input, texture, reason, stableKey, duration)
		return texture
	}

	cacheStart := time.Now()
	cachedTexture, ok, cacheDetail := cachedTextureForInputDetailed(input, textureCtx)
	cacheDuration := time.Since(cacheStart)
	if stats != nil {
		stats.CacheDuration += cacheDuration
	}
	if ok {
		recordTextureCacheHit(stats, cacheDetail)
		return finish("cache_hit:"+cacheDetail.Reason, cacheDetail.StableKey, cachedTexture)
	}
	if stats != nil {
		stats.CacheMisses++
	}
	fallbackStart := time.Now()
	finishFallback := func(reason string, stableKey string, texture AppliedItemTexture) AppliedItemTexture {
		if stats != nil {
			stats.FallbackDuration += time.Since(fallbackStart)
		}
		return finish(reason, stableKey, texture)
	}

	skullFallback := skullFallbackTexture(input, textureCtx)
	id := normalizeMinecraftItemID(input.ID)
	if id == "" && input.ItemModel != "" {
		id = normalizeMinecraftItemID(input.ItemModel)
	}
	isGenericSkullInput := skullFallback.Texture != "" && strings.TrimSpace(input.SkyBlockID) == "" && id == "minecraft:player_head"

	itemMap := map[string]any(nil)
	var renderErr error
	renderer := currentCustomResourceRenderer()
	canRuntimeRender := !isGenericSkullInput && !textureCtx.DisableRuntimeRender
	if canRuntimeRender && renderer == nil {
		if err := ensureCustomResourceRenderer(); err == nil {
			renderer = currentCustomResourceRenderer()
		}
	}
	canRuntimeRender = canRuntimeRender && renderer != nil
	if !canRuntimeRender {
		recordRuntimeRenderSkip(stats, renderer, textureCtx, isGenericSkullInput)
	}
	if canRuntimeRender {
		itemMap = itemTextureInputRenderMap(input)
		renderStart := time.Now()
		renderPackIDs := rendererPackIDsForInput(input, textureCtx.EnabledPackIDs)
		customTexture, err := renderer.RenderItemNBTWithPackIDs(itemMap, packIDsForRenderer(renderPackIDs))
		renderDuration := time.Since(renderStart)
		if stats != nil {
			stats.RenderAttempts++
			stats.RenderDuration += renderDuration
		}
		if err != nil {
			if stats != nil {
				stats.RenderErrors++
			}
			renderErr = err
		}
		if customTexture != nil && customTexture.Path != "" && texturePackEnabled(customTexture.TexturePackID, textureCtx.EnabledPackSet) {
			outputTexture := AppliedItemTexture{
				Texture:     publicCacheTextureURL(customTexture.Path),
				TexturePack: customTexture.TexturePackID,
			}
			if !isGenericPackedSkullTexture(input, outputTexture) {
				if stats != nil {
					stats.RenderHits++
				}
				if len(textureCtx.EnabledPackIDs) > 0 {
					setCachedTextureForInput(input, textureCtx, outputTexture)
				}
				return finishFallback("runtime_render_hit", "", outputTexture)
			}
		}

		if renderErr != nil && id != "" && vanillaItemResourceExists(id) {
			if vanillaItem := vanillaRenderItem(itemMap, id); vanillaItem != nil {
				renderStart := time.Now()
				customTexture, err := renderer.RenderItemNBTWithPackIDs(vanillaItem, packIDsForRenderer(renderPackIDs))
				renderDuration := time.Since(renderStart)
				if stats != nil {
					stats.RenderAttempts++
					stats.RenderDuration += renderDuration
				}
				if err != nil {
					if stats != nil {
						stats.RenderErrors++
					}
					renderErr = err
				}
				if customTexture != nil && customTexture.Path != "" && texturePackEnabled(customTexture.TexturePackID, textureCtx.EnabledPackSet) {
					outputTexture := AppliedItemTexture{
						Texture:     publicCacheTextureURL(customTexture.Path),
						TexturePack: customTexture.TexturePackID,
					}
					if !isGenericPackedSkullTexture(input, outputTexture) {
						if stats != nil {
							stats.RenderHits++
						}
						if len(textureCtx.EnabledPackIDs) > 0 {
							setCachedTextureForInput(input, textureCtx, outputTexture)
						}
						return finishFallback("runtime_vanilla_render_hit", "", outputTexture)
					}
				}
			}
		}
	}

	if skullFallback.Texture != "" {
		if stats != nil {
			stats.SkullFallbacks++
			if strings.Contains(skullFallback.Texture, "/api/head/") {
				stats.HeadFallbacks++
			}
		}
		return finishFallback("skull_head_fallback", "", skullFallback)
	}

	if input.NumericID >= 298 && input.NumericID <= 301 {
		armorType := constants.ARMOR_TYPES[input.NumericID-298]
		armorColor := ""
		if input.DisplayColor != 0 {
			armorColor = fmt.Sprintf("%06X", input.DisplayColor)
		}
		if armorColor == "" && input.SkyBlockID != "" {
			if item, ok := constants.GetItem(input.SkyBlockID); ok && item.Color != "" {
				defaultHexColor := item.Color
				armorColor = defaultHexColor
			}
		}
		if armorColor == "" {
			armorColor = "A06540"
		}
		texture := AppliedItemTexture{Texture: fmt.Sprintf("%s/api/leather/%s/%s", textureCtx.Domain, armorType, armorColor)}
		if stats != nil {
			stats.LeatherFallbacks++
		}
		return finishFallback("leather_fallback", "", texture)
	}

	if input.NumericID > 0 && shouldUseLegacyNumericFallback(id) {
		textureID := constants.GetVanillaItemId(constants.ItemModel{
			NumericId:  input.NumericID,
			ItemDamage: input.Damage,
		})
		if textureID != "" && "minecraft:"+textureID != id {
			if stats != nil {
				stats.NumericFallbacks++
			}
			input.ID = "minecraft:" + textureID
			input.NumericID = 0
			texture := ApplyTextureInput(input, textureCtx)
			return finishFallback("numeric_fallback", "", texture)
		}
	}

	if vanillaTexture := vanillaTextureURL(id); vanillaTexture.Texture != "" {
		if stats != nil {
			stats.VanillaFallbacks++
			stats.VanillaTextureFallbacks++
		}
		return finishFallback("vanilla_texture_fallback", "", vanillaTexture)
	}
	if vanillaTexture := vanillaSpecialHeadTextureURL(id); vanillaTexture.Texture != "" {
		if stats != nil {
			stats.VanillaFallbacks++
			stats.VanillaModelFallbacks++
		}
		return finishFallback("vanilla_special_head_fallback", "", vanillaTexture)
	}
	if vanillaTexture := vanillaModelTextureURL(id); vanillaTexture.Texture != "" {
		if stats != nil {
			stats.VanillaFallbacks++
			stats.VanillaModelFallbacks++
		}
		return finishFallback("vanilla_model_fallback", "", vanillaTexture)
	}

	if renderErr != nil {
		fmt.Printf("[CUSTOM_RESOURCES] RenderItemNBT failed id=%q skyblock_id=%q error=%v\n", id, input.SkyBlockID, renderErr)
	}

	fmt.Printf("[CUSTOM_RESOURCES] No texture found for item id=%q skyblock_id=%q item_id=%d damage=%d\n", id, input.SkyBlockID, input.NumericID, input.Damage)
	texture := AppliedItemTexture{Texture: barrierTextureURL()}
	if stats != nil {
		stats.BarrierFallbacks++
	}
	return finishFallback("barrier_fallback", "", texture)
}

func ApplyTexture(item any, enabledPacksParam ...[]string) AppliedItemTexture {
	timeNow := time.Now()
	textureCtx := NewTextureApplyContext(enabledPacksParam...)
	stats := textureCtx.Stats
	if itemMap, ok := item.(map[string]any); ok {
		input := itemTextureInputFromMap(itemMap)
		if skullIdentityFromInput(input) == "" {
			if cachedTexture, ok := cachedTextureFromRawMap(itemMap, textureCtx); ok {
				if stats != nil {
					stats.Total++
					recordTextureCacheHit(stats, textureCacheLookupDetail{Reason: "raw_map_cache"})
					duration := time.Since(timeNow)
					stats.TotalDuration += duration
					stats.CacheDuration += duration
					recordTextureDecision(stats, input, cachedTexture, "cache_hit:raw_map_cache", "", duration)
				}
				return cachedTexture
			}
		}
		cacheStart := time.Now()
		cachedTexture, ok, cacheDetail := cachedTextureForInputDetailed(input, textureCtx)
		cacheDuration := time.Since(cacheStart)
		if stats != nil {
			stats.CacheDuration += cacheDuration
		}
		if ok {
			if stats != nil {
				stats.Total++
				recordTextureCacheHit(stats, cacheDetail)
				duration := time.Since(timeNow)
				stats.TotalDuration += duration
				recordTextureDecision(stats, input, cachedTexture, "cache_hit:"+cacheDetail.Reason, cacheDetail.StableKey, duration)
			}
			return cachedTexture
		}
		return ApplyTextureInput(input, textureCtx)
	}

	itemMap, ok := normalizeTextureItem(item)
	if !ok {
		return AppliedItemTexture{Texture: barrierTextureURL()}
	}
	return ApplyTextureInput(itemTextureInputFromMap(itemMap), textureCtx)
}
