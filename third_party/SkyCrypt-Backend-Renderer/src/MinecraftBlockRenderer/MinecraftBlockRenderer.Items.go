package minecraftblockrenderer

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	imagedraw "image/draw"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src"
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/global"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/imagecache"

	"golang.org/x/image/draw"
)

var BottomAlignedItemSuffixes = []string{
	"_carpet",
	"_trapdoor",
	"_pressure_plate",
	"_weighted_pressure_plate",
}

var InventoryModelSuffixes = map[string]string{
	"_fence":  "_fence_inventory",
	"_wall":   "_wall_inventory",
	"_button": "_button_inventory",
}

var BannerSuffixes = []string{
	"_banner",
}

const inventory3DItemGuiYawCorrection = 90.0

var LegacyDefaultTintLayerOverrides = map[string][]int{
	"wolf_armor_dyed": {1},
}

var DefaultLeatherArmorColor = color.RGBA{R: 0xA0, G: 0x65, B: 0x40, A: 255}

var LegacyDefaultTintOverrides = map[string]color.RGBA{
	"leather_helmet":      DefaultLeatherArmorColor,
	"leather_chestplate":  DefaultLeatherArmorColor,
	"leather_leggings":    DefaultLeatherArmorColor,
	"leather_boots":       DefaultLeatherArmorColor,
	"leather_horse_armor": DefaultLeatherArmorColor,
	"wolf_armor_dyed":     DefaultLeatherArmorColor,
}

var AnimatedDialItems = map[string]struct{}{
	"compass":          {},
	"recovery_compass": {},
	"clock":            {},
}

type skyblockItemDefinitionCacheEntry struct {
	Loaded         bool
	Selector       data.ItemModelSelector
	ModelReference string
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ShouldAlignGuiItemToBottom(itemName string) bool {
	if itemName == "" {
		return false
	}

	for _, suffix := range BottomAlignedItemSuffixes {
		if strings.HasSuffix(itemName, suffix) {
			return true
		}
	}

	return itemName == "carpet" || itemName == "trapdoor" || itemName == "pressure_plate"
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) NormalizeItemTextureKey(itemName string) string {
	normalizedItemName := strings.ToLower(itemName)
	if strings.HasPrefix(normalizedItemName, "minecraft") {
		normalizedItemName = strings.TrimPrefix(normalizedItemName, "minecraft:")
	}

	normalizedItemName = strings.ReplaceAll(normalizedItemName, "\\", "/")
	normalizedItemName = strings.Trim(normalizedItemName, "/")

	return normalizedItemName
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) RenderGuiItemInternal(itemName string, options *BlockRenderOptions, capture *ItemRenderCapture) *image.RGBA {
	normalizedItemKey := _minecraftBlockRenderer.NormalizeItemTextureKey(itemName)
	if capture != nil {
		capture.OriginalTarget = strings.TrimSpace(itemName)
		capture.NormalizedItemKey = normalizedItemKey
	}

	alignToBottom := _minecraftBlockRenderer.ShouldAlignGuiItemToBottom(normalizedItemKey)
	var postScale *float64
	finalizeGuiResult := func(img *image.RGBA) *image.RGBA {
		if img == nil {
			return nil
		}
		if capture != nil {
			capture.FinalOptions = options
		}

		if postScale != nil {
			_minecraftBlockRenderer.ApplyCenteredScale(img, *postScale)
		}

		if alignToBottom {
			_minecraftBlockRenderer.AlignImageToBottom(img)
		}

		return img
	}

	itemInfo := _minecraftBlockRenderer._itemRegistry.GetItemInfo(normalizedItemKey)
	if capture != nil {
		capture.ItemInfo = itemInfo
	}

	model, candidates, resolvedModelName, compositeModelNames, specialModel := _minecraftBlockRenderer.ResolveItemModel(normalizedItemKey, itemInfo, *options)
	// Console.WriteLine($"Resolved model for item '{itemName}': '{model?.Name}' with candidates [{string.Join(", ", modelCandidates)}].");
	_ = candidates

	if capture != nil {
		capture.Model = model
		capture.ModelCandidates = candidates
		capture.ResolvedModelName = resolvedModelName
		capture.CompositeModelNames = compositeModelNames
		capture.SpecialModel = specialModel
	}

	if _minecraftBlockRenderer.IsBannerItem(normalizedItemKey) || (resolvedModelName != nil && _minecraftBlockRenderer.IsBannerItem(*resolvedModelName)) {
		options.AdditionalScale *= 0.8
	}

	if len(compositeModelNames) > 1 {
		if compositeRender := _minecraftBlockRenderer.TryRenderCompositeItemModels(normalizedItemKey, itemInfo, compositeModelNames, *options); compositeRender != nil {
			return finalizeGuiResult(compositeRender)
		}
	}

	if options.OverrideGuiTransform == nil && options.UseGuiTransform && model != nil {
		if guiOverride, exists := model.Display["gui"]; exists {
			if _minecraftBlockRenderer.ShouldUseSkyBlockTemplateSkullItemOrientation(model, *options) {
				guiOverride = cloneTransformWithYawOffset(guiOverride, -90)
			}
			options.OverrideGuiTransform = guiOverride
		}
	}

	postScale = _minecraftBlockRenderer.GetPostRenderScale(normalizedItemKey)
	if postScale == nil && resolvedModelName != nil {
		postScale = _minecraftBlockRenderer.GetPostRenderScale(*resolvedModelName)
	}

	shouldPreferHead, preparedOptions := _minecraftBlockRenderer.ShouldPreferPlayerHeadRenderer(normalizedItemKey, model, candidates, *options)
	options = preparedOptions

	if shouldPreferHead {
		rendered := _minecraftBlockRenderer.RenderPlayerHead(itemName, model, candidates, *options)
		if rendered != nil {
			return finalizeGuiResult(rendered)
		}
	}

	// fmt.Printf("\nnormalizedItemKey: %v\nitemInfo: %v\nmodel: %v\noptions: %v\n", normalizedItemKey, itemInfo, model, options)

	if specialRender := _minecraftBlockRenderer.TryRenderSpecialChestItem(specialModel, model, *options); specialRender != nil {
		return finalizeGuiResult(specialRender)
	}

	flatRender := _minecraftBlockRenderer.TryRenderGuiTextureLayers(itemName, itemInfo, model, *options)
	if flatRender != nil {
		return finalizeGuiResult(flatRender)
	}

	bedRender := _minecraftBlockRenderer.TryRenderBedItem(itemName, model, *options)
	if bedRender != nil {
		return finalizeGuiResult(bedRender)
	}

	if !_minecraftBlockRenderer.HasExplicitFlatHeadOverride(model, candidates, *options) {
		headComposite := _minecraftBlockRenderer.RenderPlayerHead(itemName, model, candidates, *options)
		if headComposite != nil {
			return finalizeGuiResult(headComposite)
		}
	}

	if model != nil && _minecraftBlockRenderer.IsBillboardModel(model) {
		billboardTextures := _minecraftBlockRenderer.CollectBillboardTextures(model, nil)
		rendered := _minecraftBlockRenderer.TryRenderFlatItemFromIdentifiers(billboardTextures, model, *options, itemName)
		if rendered != nil {
			return finalizeGuiResult(rendered)
		}
	}

	if model != nil && len(model.Elements) > 0 {
		if !_minecraftBlockRenderer.ModelUsesMissingTexturePlaceholder(model) {
			renderOptions := _minecraftBlockRenderer.applyInventory3DItemGuiOrientation(model, *options)
			rendered := _minecraftBlockRenderer.RenderModel(model, renderOptions, &itemName)
			if rendered != nil {
				return finalizeGuiResult(rendered)
			}
		}
	}

	renderBlockEntityFallback := _minecraftBlockRenderer.TryRenderBlockEntityFallback(itemName, itemInfo, model, candidates, *options)
	if renderBlockEntityFallback != nil {
		return finalizeGuiResult(renderBlockEntityFallback)
	}

	return finalizeGuiResult(_minecraftBlockRenderer.RenderFallbackTexture(itemName, itemInfo, model, *options))

}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) DetermineDisplayContext(options BlockRenderOptions) string {
	if options.UseGuiTransform {
		return "gui"
	}
	return "none"
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ResolveItemModel(itemName string, itemInfo *data.ItemInfo, options BlockRenderOptions) (*data.BlockModelInstance, []string, *string, []string, *data.ItemSpecialModelInfo) {
	displayContext := _minecraftBlockRenderer.DetermineDisplayContext(options)
	var dynamicModel *string
	var dynamicModels []string
	var specialModel *data.ItemSpecialModelInfo
	if itemInfo != nil && itemInfo.Selector != nil {
		selectorContext := data.ItemModelContext{
			ItemData:       options.ItemData,
			DisplayContext: displayContext,
			ItemName:       itemName,
			ItemModel:      itemModelForSelectorContext(options.ItemData),
		}
		dynamicModels = compactModelNames(data.ResolveAllItemModelSelector(itemInfo.Selector, selectorContext))
		specialModel = data.ResolveItemModelSelectorSpecialInfo(itemInfo.Selector, selectorContext)
		if len(dynamicModels) > 0 {
			dynamicModel = &dynamicModels[0]
		}
	}

	firmamentModel := _minecraftBlockRenderer.GetFirmamentModel(options.ItemData)
	skyblockItemModels := _minecraftBlockRenderer.GetSkyblockItemModels(itemName, options.ItemData, displayContext)

	var primaryModel *string
	if itemInfo != nil {
		primaryModel = itemInfo.Model
	}
	var fallbackModel string
	if firmamentModel != nil && strings.TrimSpace(*firmamentModel) != "" {
		fallbackModel = *firmamentModel
	} else if dynamicModel != nil && strings.TrimSpace(*dynamicModel) != "" {
		fallbackModel = *dynamicModel
	} else if primaryModel != nil && strings.TrimSpace(*primaryModel) != "" {
		fallbackModel = *primaryModel
	} else {
		fallbackModel = itemName
	}

	var candidates []string
	var seen = make(map[string]struct{})

	appendCandidates := func(primary *string, includeItemNameFallback bool) {
		if primary == nil || strings.TrimSpace(*primary) == "" {
			return
		}

		if !includeItemNameFallback {
			for _, candidate := range _minecraftBlockRenderer.EnumerateCandidateNames(*primary) {
				if _, exists := seen[candidate]; !exists {
					seen[candidate] = struct{}{}
					candidates = append(candidates, candidate)
				}
			}
			return
		}

		for _, candidate := range _minecraftBlockRenderer.BuildModelCandidates(*primary, itemName) {
			if _, exists := seen[candidate]; !exists {
				seen[candidate] = struct{}{}
				candidates = append(candidates, candidate)
			}
		}
	}

	preferSkyBlockSelectorModel := hasExplicitSelectorItemModel(options.ItemData) && len(skyblockItemModels) > 0
	if preferSkyBlockSelectorModel {
		appendCandidates(&skyblockItemModels[0], false)
		appendCandidates(firmamentModel, false)
	} else {
		appendCandidates(firmamentModel, false)
		if len(skyblockItemModels) > 0 {
			appendCandidates(&skyblockItemModels[0], false)
		}
	}
	for _, modelName := range dynamicModels {
		appendCandidates(&modelName, false)
	}
	appendCandidates(primaryModel, true)
	appendCandidates(&fallbackModel, true)
	appendCandidates(&itemName, true)

	if len(candidates) == 0 {
		candidates = append(candidates, itemName)
	}

	var model *data.BlockModelInstance
	var resolvedModelName *string
	for _, candidate := range candidates {
		resolved, exists := _minecraftBlockRenderer._modelResolver.TryResolve(candidate)
		if exists && resolved != nil {
			model = resolved
			resolvedModelName = &candidate
			normalizedName := _minecraftBlockRenderer.NormalizeModelIdentifier(*resolvedModelName)
			if !strings.EqualFold(model.Name, normalizedName) {
				model = &data.BlockModelInstance{
					Name:        normalizedName,
					ParentChain: model.ParentChain,
					Textures:    model.Textures,
					Display:     model.Display,
					Elements:    model.Elements,
				}
			}
			break
		}
	}

	compositeModelNames := _minecraftBlockRenderer.ResolveCompositeModelNames(resolvedModelName, skyblockItemModels, dynamicModels)
	return model, candidates, resolvedModelName, compositeModelNames, specialModel
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ResolveCompositeModelNames(resolvedModelName *string, skyblockItemModels []string, dynamicModels []string) []string {
	if len(skyblockItemModels) > 1 && _minecraftBlockRenderer.ContainsResolvedModel(skyblockItemModels, resolvedModelName) {
		return skyblockItemModels
	}
	if len(dynamicModels) > 1 && _minecraftBlockRenderer.ContainsResolvedModel(dynamicModels, resolvedModelName) {
		return dynamicModels
	}
	return nil
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ContainsResolvedModel(modelNames []string, resolvedModelName *string) bool {
	if resolvedModelName == nil || strings.TrimSpace(*resolvedModelName) == "" {
		return false
	}
	normalizedResolved := _minecraftBlockRenderer.NormalizeModelIdentifier(*resolvedModelName)
	for _, modelName := range modelNames {
		if strings.EqualFold(_minecraftBlockRenderer.NormalizeModelIdentifier(modelName), normalizedResolved) {
			return true
		}
	}
	return false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryRenderCompositeItemModels(itemName string, itemInfo *data.ItemInfo, modelNames []string, options BlockRenderOptions) *image.RGBA {
	if len(modelNames) <= 1 {
		return nil
	}

	canvas := image.NewRGBA(image.Rect(0, 0, options.Size, options.Size))
	renderedAny := false

	for _, modelName := range modelNames {
		if strings.TrimSpace(modelName) == "" {
			continue
		}

		modelNameCopy := modelName
		model := _minecraftBlockRenderer.ResolveModelOrNull(&modelNameCopy)
		if model == nil {
			continue
		}

		childOptions := options
		if childOptions.OverrideGuiTransform == nil && childOptions.UseGuiTransform {
			if guiOverride := model.GetDisplayTransform("gui"); guiOverride != nil {
				childOptions.OverrideGuiTransform = guiOverride
			}
		}

		modelCandidates := []string{modelName}
		var layer *image.RGBA

		if shouldPreferHead, preparedOptions := _minecraftBlockRenderer.ShouldPreferPlayerHeadRenderer(itemName, model, modelCandidates, childOptions); shouldPreferHead {
			if headRender := _minecraftBlockRenderer.RenderPlayerHead(itemName, model, modelCandidates, *preparedOptions); headRender != nil {
				layer = headRender
			}
		}

		if layer == nil {
			layer = _minecraftBlockRenderer.TryRenderGuiTextureLayers(itemName, itemInfo, model, childOptions)
		}
		if layer == nil {
			layer = _minecraftBlockRenderer.TryRenderBedItem(itemName, model, childOptions)
		}
		if layer == nil && !_minecraftBlockRenderer.HasExplicitFlatHeadOverride(model, modelCandidates, childOptions) {
			layer = _minecraftBlockRenderer.RenderPlayerHead(itemName, model, modelCandidates, childOptions)
		}
		if layer == nil && len(model.Elements) > 0 {
			if !_minecraftBlockRenderer.ModelUsesMissingTexturePlaceholder(model) {
				renderOptions := _minecraftBlockRenderer.applyInventory3DItemGuiOrientation(model, childOptions)
				layer = _minecraftBlockRenderer.RenderModel(model, renderOptions, &itemName)
			}
		}
		if layer == nil {
			layer = _minecraftBlockRenderer.TryRenderBlockEntityFallback(itemName, itemInfo, model, modelCandidates, childOptions)
		}
		if layer == nil {
			layer = _minecraftBlockRenderer.RenderFallbackTexture(itemName, itemInfo, model, childOptions)
		}
		if layer == nil {
			continue
		}

		imagedraw.Draw(canvas, canvas.Bounds(), layer, image.Point{}, imagedraw.Over)
		renderedAny = true
	}

	if !renderedAny {
		return nil
	}

	return canvas
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryRenderSpecialChestItem(special *data.ItemSpecialModelInfo, baseModel *data.BlockModelInstance, options BlockRenderOptions) *image.RGBA {
	if special == nil || !strings.EqualFold(special.Type, "minecraft:chest") {
		return nil
	}

	textureID := specialChestEntityTexture(special.Texture)
	if textureID == "" {
		return nil
	}
	texture := _minecraftBlockRenderer._textureRepository.GetTexture(textureID)
	if texture == nil || _minecraftBlockRenderer._textureRepository.IsMissingTexture(texture) {
		return nil
	}

	textures := _minecraftBlockRenderer.CloneTextureDictionary(baseModel)
	textures["chest"] = textureID
	textures["particle"] = textureID

	displaySource := baseModel
	templateChest := "item/template_chest"
	if template := _minecraftBlockRenderer.ResolveModelOrNull(&templateChest); template != nil {
		displaySource = template
	}
	display := _minecraftBlockRenderer.CloneDisplayDictionary(displaySource)

	renderOptions := options
	if guiTransform, ok := display["gui"]; ok {
		renderOptions.OverrideGuiTransform = guiTransform
	}

	modelName := specialModelResourceMarker(special)
	chestModel := data.BlockModelInstance{
		Name:     modelName,
		Textures: textures,
		Display:  display,
		Elements: specialChestElements(),
	}

	renderOptions = _minecraftBlockRenderer.applySpecialChestItemGuiOrientation(&chestModel, renderOptions)
	rendered := _minecraftBlockRenderer.RenderModel(&chestModel, renderOptions, &modelName)
	if rendered == nil || !hasVisiblePixels(rendered) {
		return nil
	}
	return rendered
}

func specialChestElements() []data.ModelElement {
	return []data.ModelElement{
		chestElement(
			data.Vector3{X: 1, Y: 0, Z: 1},
			data.Vector3{X: 15, Y: 10, Z: 15},
			map[data.BlockFaceDirection]data.ModelFace{
				data.Down:  chestFace(data.Vector4{X: 3.5, Y: 4.75, Z: 7, W: 8.25}, intPtr(180)),
				data.North: chestFace(data.Vector4{X: 10.5, Y: 8.25, Z: 14, W: 10.75}, intPtr(180)),
				data.East:  chestFace(data.Vector4{X: 0, Y: 8.25, Z: 3.5, W: 10.75}, intPtr(180)),
				data.South: chestFace(data.Vector4{X: 3.5, Y: 8.25, Z: 7, W: 10.75}, intPtr(180)),
				data.West:  chestFace(data.Vector4{X: 7, Y: 8.25, Z: 10.5, W: 10.75}, intPtr(180)),
			},
		),
		chestElement(
			data.Vector3{X: 1, Y: 10, Z: 1},
			data.Vector3{X: 15, Y: 14, Z: 15},
			map[data.BlockFaceDirection]data.ModelFace{
				data.Up:    chestFace(data.Vector4{X: 3.5, Y: 4.75, Z: 7, W: 8.25}, nil),
				data.North: chestFace(data.Vector4{X: 10.5, Y: 3.75, Z: 14, W: 4.75}, intPtr(180)),
				data.East:  chestFace(data.Vector4{X: 0, Y: 3.75, Z: 3.5, W: 4.75}, intPtr(180)),
				data.South: chestFace(data.Vector4{X: 3.5, Y: 3.75, Z: 7, W: 4.75}, intPtr(180)),
				data.West:  chestFace(data.Vector4{X: 7, Y: 3.75, Z: 10.5, W: 4.75}, intPtr(180)),
			},
		),
		chestElement(
			data.Vector3{X: 7, Y: 7, Z: 0},
			data.Vector3{X: 9, Y: 11, Z: 1},
			map[data.BlockFaceDirection]data.ModelFace{
				data.Down:  chestFace(data.Vector4{X: 0.25, Y: 0, Z: 0.75, W: 0.25}, intPtr(180)),
				data.Up:    chestFace(data.Vector4{X: 0.75, Y: 0, Z: 1.25, W: 0.25}, intPtr(180)),
				data.North: chestFace(data.Vector4{X: 1, Y: 0.25, Z: 1.5, W: 1.25}, intPtr(180)),
				data.West:  chestFace(data.Vector4{X: 0.75, Y: 0.25, Z: 1, W: 1.25}, intPtr(180)),
				data.East:  chestFace(data.Vector4{X: 0, Y: 0.25, Z: 0.25, W: 1.25}, intPtr(180)),
			},
		),
	}
}

func chestElement(from data.Vector3, to data.Vector3, faces map[data.BlockFaceDirection]data.ModelFace) data.ModelElement {
	return data.ModelElement{
		From:  from,
		To:    to,
		Shade: true,
		Faces: faces,
	}
}

func chestFace(uv data.Vector4, rotation *int) data.ModelFace {
	return data.ModelFace{
		Texture:  "#chest",
		Uv:       &uv,
		Rotation: rotation,
	}
}

func intPtr(value int) *int {
	return &value
}

func specialChestEntityTexture(texture string) string {
	switch strings.ToLower(strings.TrimSpace(texture)) {
	case "minecraft:normal", "normal", "":
		return "minecraft:entity/chest/normal"
	case "minecraft:trapped", "trapped":
		return "minecraft:entity/chest/trapped"
	case "minecraft:ender", "ender":
		return "minecraft:entity/chest/ender"
	case "minecraft:christmas", "christmas":
		return "minecraft:entity/chest/christmas"
	case "minecraft:copper", "copper":
		return "minecraft:entity/chest/copper"
	case "minecraft:copper_exposed", "copper_exposed":
		return "minecraft:entity/chest/copper_exposed"
	case "minecraft:copper_weathered", "copper_weathered":
		return "minecraft:entity/chest/copper_weathered"
	case "minecraft:copper_oxidized", "copper_oxidized":
		return "minecraft:entity/chest/copper_oxidized"
	default:
		return ""
	}
}

func specialModelResourceMarker(special *data.ItemSpecialModelInfo) string {
	if special == nil {
		return ""
	}
	specialType := strings.TrimSpace(special.Type)
	texture := strings.TrimSpace(special.Texture)
	if texture == "" {
		texture = "minecraft:normal"
	}
	return fmt.Sprintf("special:%s:%s:%s", specialType, texture, strings.TrimSpace(special.BaseModel))
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetFirmamentModel(itemData *data.ItemRenderData) *string {
	if itemData == nil || itemData.CustomData == nil {
		return nil
	}

	if itemData.CustomData != nil {
		if tag, ok := itemData.CustomData.Get("id"); ok {
			if rawSkyblockId, ok := nbtStringFromTag(tag); ok && strings.TrimSpace(rawSkyblockId) != "" {
				encodedId := _minecraftBlockRenderer.EncodeFirmamentId(rawSkyblockId)
				firmamentModel := "firmskyblock:item/" + encodedId
				return &firmamentModel
			}
		}
	}

	return nil
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) EncodeFirmamentId(skyblockId string) string {
	lowercaseId := strings.ToLower(skyblockId)
	var builder strings.Builder
	for _, c := range lowercaseId {
		switch c {
		case ':':
			builder.WriteString("___")
		case ';':
			builder.WriteString("__")
		default:
			if _minecraftBlockRenderer.isValidResourceLocationChar(c) {
				builder.WriteRune(c)
			} else {
				builder.WriteString("__" + hex.EncodeToString([]byte(string(c))))
			}
		}
	}

	return builder.String()
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) isValidResourceLocationChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '_' ||
		c == '-' ||
		c == '.' ||
		c == '/'
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetSkyblockItemModels(itemName string, itemData *data.ItemRenderData, displayContext string) []string {
	if itemData == nil || itemData.CustomData == nil {
		return nil
	}

	if tag, ok := itemData.CustomData.Get("id"); ok {
		if rawSkyblockId, ok := nbtStringFromTag(tag); ok && strings.TrimSpace(rawSkyblockId) != "" {
			encodedId := _minecraftBlockRenderer.EncodeFirmamentId(rawSkyblockId)
			info := _minecraftBlockRenderer._itemRegistry.GetItemInfo(encodedId)
			if info != nil {
				var models []string

				if info.Selector != nil {
					selectorContext := data.ItemModelContext{
						ItemData:       itemData,
						DisplayContext: displayContext,
						ItemName:       itemName,
						ItemModel:      itemModelForSelectorContext(itemData),
					}
					models = data.ResolveAllItemModelSelector(info.Selector, selectorContext)
				}

				if len(models) == 0 && info.Model != nil {
					models = append(models, *info.Model)
				}

				filtered := filterNamespacedModels(models)
				if len(filtered) == 0 && info.Model != nil && strings.Contains(*info.Model, ":") {
					return filterNamespacedModels([]string{*info.Model})
				}
				if len(filtered) > 0 {
					return filtered
				}
			}

			if models := _minecraftBlockRenderer.ResolveSkyblockItemModelsFromPackProviders(encodedId, itemName, itemData, displayContext); len(models) > 0 {
				return models
			}
		}
	}

	return nil
}

func filterNamespacedModels(models []string) []string {
	return filterModelNames(models, true)
}

func compactModelNames(models []string) []string {
	return filterModelNames(models, false)
}

func filterModelNames(models []string, requireNamespace bool) []string {
	var filtered []string
	seen := make(map[string]struct{})
	for _, model := range models {
		if strings.TrimSpace(model) == "" || (requireNamespace && !strings.Contains(model, ":")) {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(model))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, model)
	}
	return filtered
}

func nbtStringFromTag(tag nbt.NbtTag) (string, bool) {
	switch value := tag.(type) {
	case nbt.NbtString:
		return value.Value, true
	case *nbt.NbtString:
		return value.Value, true
	default:
		return "", false
	}
}

func itemModelForSelectorContext(itemData *data.ItemRenderData) string {
	if itemData == nil {
		return ""
	}
	return strings.TrimSpace(itemData.ItemModel)
}

func hasExplicitSelectorItemModel(itemData *data.ItemRenderData) bool {
	return strings.TrimSpace(itemModelForSelectorContext(itemData)) != ""
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ResolveSkyblockItemModelFromPackProviders(encodedId string, itemName string, itemData *data.ItemRenderData, displayContext string) *string {
	models := _minecraftBlockRenderer.ResolveSkyblockItemModelsFromPackProviders(encodedId, itemName, itemData, displayContext)
	if len(models) == 0 {
		return nil
	}
	return &models[0]
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ResolveSkyblockItemModelsFromPackProviders(encodedId string, itemName string, itemData *data.ItemRenderData, displayContext string) []string {
	if strings.TrimSpace(encodedId) == "" {
		return nil
	}

	cacheKey := strings.ToLower(encodedId)
	entry := _minecraftBlockRenderer.getSkyblockItemDefinition(cacheKey)
	if !entry.Loaded {
		return nil
	}

	if entry.Selector != nil {
		resolved := data.ResolveAllItemModelSelector(entry.Selector, data.ItemModelContext{
			ItemData:       itemData,
			DisplayContext: displayContext,
			ItemName:       itemName,
			ItemModel:      itemModelForSelectorContext(itemData),
		})
		if len(resolved) > 0 {
			filtered := filterNamespacedModels(resolved)
			if len(filtered) > 0 {
				return filtered
			}
		}
	}

	if strings.TrimSpace(entry.ModelReference) != "" {
		modelReference := entry.ModelReference
		return filterNamespacedModels([]string{modelReference})
	}

	return nil
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) getSkyblockItemDefinition(encodedId string) skyblockItemDefinitionCacheEntry {
	_minecraftBlockRenderer._skyblockItemDefinitionsMu.RLock()
	if _minecraftBlockRenderer._skyblockItemDefinitions != nil {
		if entry, found := _minecraftBlockRenderer._skyblockItemDefinitions[encodedId]; found {
			_minecraftBlockRenderer._skyblockItemDefinitionsMu.RUnlock()
			return entry
		}
	}
	_minecraftBlockRenderer._skyblockItemDefinitionsMu.RUnlock()

	entry := _minecraftBlockRenderer.loadSkyblockItemDefinition(encodedId)

	_minecraftBlockRenderer._skyblockItemDefinitionsMu.Lock()
	if _minecraftBlockRenderer._skyblockItemDefinitions == nil {
		_minecraftBlockRenderer._skyblockItemDefinitions = make(map[string]skyblockItemDefinitionCacheEntry)
	}
	if cached, found := _minecraftBlockRenderer._skyblockItemDefinitions[encodedId]; found {
		_minecraftBlockRenderer._skyblockItemDefinitionsMu.Unlock()
		return cached
	}
	_minecraftBlockRenderer._skyblockItemDefinitions[encodedId] = entry
	_minecraftBlockRenderer._skyblockItemDefinitionsMu.Unlock()

	return entry
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) loadSkyblockItemDefinition(encodedId string) skyblockItemDefinitionCacheEntry {
	direct := "assets/skyblock/items/" + strings.ToLower(encodedId) + ".json"
	firmamentDirect := "assets/firmskyblock/items/" + strings.ToLower(encodedId) + ".json"
	fileName := strings.ToLower(encodedId) + ".json"
	suffix := "/" + direct
	firmamentSuffix := "/" + firmamentDirect

	for packIndex := len(_minecraftBlockRenderer._packContext.Packs) - 1; packIndex >= 0; packIndex-- {
		pack := _minecraftBlockRenderer._packContext.Packs[packIndex]
		if pack.Provider == nil {
			continue
		}

		candidates := []string{}
		if pack.Provider.FileExists(direct) {
			candidates = append(candidates, direct)
		}
		if pack.Provider.FileExists(firmamentDirect) {
			candidates = append(candidates, firmamentDirect)
		}
		if len(candidates) == 0 {
			if files, err := pack.Provider.EnumerateFiles("assets/skyblock/items", fileName, true); err == nil {
				candidates = append(candidates, files...)
			}
			if files, err := pack.Provider.EnumerateFiles("assets/firmskyblock/items", fileName, true); err == nil {
				candidates = append(candidates, files...)
			}
		}
		if len(candidates) == 0 {
			files, err := pack.Provider.EnumerateFiles("", fileName, true)
			if err != nil {
				continue
			}
			for _, file := range files {
				lower := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
				if lower == direct || lower == firmamentDirect || strings.HasSuffix(lower, suffix) || strings.HasSuffix(lower, firmamentSuffix) {
					candidates = append(candidates, file)
				}
			}
		}

		for _, candidate := range candidates {
			jsonContent, err := pack.Provider.ReadAllText(candidate)
			if err != nil {
				continue
			}

			var itemDefinition map[string]interface{}
			if err := global.JSON.Unmarshal([]byte(jsonContent), &itemDefinition); err != nil {
				continue
			}

			selector := data.ParseItemModelSelectorFromRoot(itemDefinition)
			modelReference := data.ResolveModelReferenceFromItemDefinition(itemDefinition)
			if selector == nil && strings.TrimSpace(modelReference) == "" {
				continue
			}

			return skyblockItemDefinitionCacheEntry{
				Loaded:         true,
				Selector:       selector,
				ModelReference: modelReference,
			}
		}
	}

	return skyblockItemDefinitionCacheEntry{}
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) EnumerateCandidateNames(name string) []string {
	if strings.TrimSpace(name) == "" {
		return nil
	}

	var candidates []string
	candidates = append(candidates, name)
	candidates = append(candidates, _minecraftBlockRenderer.GenerateInventoryVariants(name)...)
	return candidates
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GenerateInventoryVariants(name string) []string {
	prefix, baseName := _minecraftBlockRenderer.SplitModelName(name)
	if strings.TrimSpace(baseName) == "" {
		return nil
	}

	var variants []string
	if !strings.HasSuffix(baseName, "_inventory") {
		variants = append(variants, prefix+baseName+"_inventory")

		for suffix, replacement := range InventoryModelSuffixes {
			if strings.HasSuffix(baseName, suffix) {
				replaced := baseName[:len(baseName)-len(suffix)] + replacement
				variants = append(variants, prefix+replaced)
			}
		}
	}

	return variants
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) SplitModelName(name string) (string, string) {
	if strings.TrimSpace(name) == "" {
		return "", ""
	}

	lastSlash := strings.LastIndex(name, "/")
	if lastSlash >= 0 {
		return name[:lastSlash+1], name[lastSlash+1:]
	}

	return "", name
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) BuildModelCandidates(primaryName string, itemName string) []string {
	var seen = make(map[string]struct{})
	var candidates []string

	for _, candidate := range _minecraftBlockRenderer.EnumerateCandidateNames(primaryName) {
		if _, exists := seen[candidate]; !exists {
			seen[candidate] = struct{}{}
			candidates = append(candidates, candidate)
		}
	}

	if !strings.EqualFold(primaryName, itemName) {
		for _, candidate := range _minecraftBlockRenderer.EnumerateCandidateNames(itemName) {
			if _, exists := seen[candidate]; !exists {
				seen[candidate] = struct{}{}
				candidates = append(candidates, candidate)
			}
		}
	}

	return candidates
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) IsBannerItem(normalizedItemKey string) bool {
	if strings.TrimSpace(normalizedItemKey) == "" {
		return false
	}

	if strings.Contains(strings.ToLower(normalizedItemKey), "banner") {
		return true
	}

	for _, suffix := range BannerSuffixes {
		if strings.HasSuffix(strings.ToLower(normalizedItemKey), suffix) {
			return true
		}
	}

	return false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetPostRenderScale(normalizedItemKey string) *float64 {
	if strings.TrimSpace(normalizedItemKey) == "" {
		return nil
	}

	if _minecraftBlockRenderer.IsBedItem(normalizedItemKey) {
		scale := 0.92
		return &scale
	}

	return nil
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) IsBedItem(normalizedItemKey string) bool {
	if strings.TrimSpace(normalizedItemKey) == "" {
		return false
	}

	if strings.EqualFold(normalizedItemKey, "bed") {
		return true
	}

	return strings.HasSuffix(strings.ToLower(normalizedItemKey), "_bed")
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ShouldPreferPlayerHeadRenderer(itemName string, model *data.BlockModelInstance, modelCandidates []string, options BlockRenderOptions) (bool, *BlockRenderOptions) {
	adjustedOptions := &options
	itemData := options.ItemData
	resolver := options.SkullTextureResolver
	hasResolver := resolver != nil

	var hasHeadTextureOverride bool
	if itemData != nil {
		_, hasHeadTextureOverride = _minecraftBlockRenderer.GetHeadTextureOverride(itemData.CustomData)
	}

	if hasResolver && (itemData == nil || itemData.CustomData == nil || !hasHeadTextureOverride) {
		var profile *nbt.NbtCompound
		var customData *nbt.NbtCompound
		if itemData != nil {
			profile = itemData.Profile
			customData = itemData.CustomData
		}

		var customDataId *string
		if customData != nil {
			if idValue, ok := _minecraftBlockRenderer.TryGetString(customData, "id"); ok && strings.TrimSpace(idValue) != "" {
				customDataId = &idValue
			}
		}

		context := SkullResolverContext{
			ItemId:       itemName,
			ItemData:     itemData,
			CustomDataId: customDataId,
			Profile:      profile,
			CustomData:   customData,
		}

		resolvedTextureValue := resolver(context)
		if resolvedTextureValue != nil && strings.TrimSpace(*resolvedTextureValue) != "" {
			resolverCompound := nbt.NewNbtCompound(map[string]nbt.NbtTag{
				"texture": nbt.NbtString{Value: *resolvedTextureValue},
			})

			mergedCustom := _minecraftBlockRenderer.MergeCustomDataCompounds(customData, resolverCompound)

			var updatedItemData *data.ItemRenderData
			if itemData == nil {
				updatedItemData = &data.ItemRenderData{CustomData: mergedCustom}
			} else {
				updatedItemData = &data.ItemRenderData{
					CustomData: mergedCustom,
					Profile:    profile,
					ItemModel:  itemData.ItemModel,
				}
			}

			adjustedOptions = &BlockRenderOptions{
				Size:                      options.Size,
				YawInDegrees:              options.YawInDegrees,
				PitchInDegrees:            options.PitchInDegrees,
				RollInDegrees:             options.RollInDegrees,
				PerspectiveAmount:         options.PerspectiveAmount,
				UseGuiTransform:           options.UseGuiTransform,
				AdditionalScale:           options.AdditionalScale,
				AdditionalTranslation:     options.AdditionalTranslation,
				OverrideGuiTransform:      options.OverrideGuiTransform,
				PackIds:                   options.PackIds,
				ItemData:                  updatedItemData,
				SkullTextureResolver:      options.SkullTextureResolver,
				EnableAntiAliasing:        options.EnableAntiAliasing,
				CustomTextureFallbackItem: options.CustomTextureFallbackItem,
				CustomTextureFallbackData: options.CustomTextureFallbackData,
			}
			itemData = updatedItemData
		}
	}

	if !strings.EqualFold(_minecraftBlockRenderer.NormalizeItemTextureKey(itemName), "player_head") {
		return false, adjustedOptions
	}

	var hasCustomHeadOverride bool
	if itemData != nil && itemData.CustomData != nil {
		_, hasCustomHeadOverride = _minecraftBlockRenderer.GetHeadTextureOverride(itemData.CustomData)
	}
	hasSkinSource := (itemData != nil && itemData.Profile != nil) ||
		hasCustomHeadOverride ||
		hasResolver
	if !hasSkinSource {
		return false, adjustedOptions
	}

	if _minecraftBlockRenderer.HasExplicitFlatHeadOverride(model, modelCandidates, *adjustedOptions) {
		return false, adjustedOptions
	}

	return true, adjustedOptions
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetHeadTextureOverride(customData *nbt.NbtCompound) (string, bool) {
	if customData == nil {
		return "", false
	}

	if value, ok := _minecraftBlockRenderer.TryGetString(customData, "texture"); ok && strings.TrimSpace(value) != "" {
		return value, true
	}

	if value, ok := _minecraftBlockRenderer.TryGetString(customData, "skin"); ok && strings.TrimSpace(value) != "" {
		return value, true
	}

	if value, ok := _minecraftBlockRenderer.TryGetString(customData, "skin_texture"); ok && strings.TrimSpace(value) != "" {
		return value, true
	}

	return "", false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryGetString(compound *nbt.NbtCompound, key string) (string, bool) {
	if compound == nil {
		return "", false
	}

	tag, ok := compound.Get(key)
	if !ok {
		return "", false
	}

	return nbtStringFromTag(tag)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) MergeCustomDataCompounds(base *nbt.NbtCompound, overlay *nbt.NbtCompound) *nbt.NbtCompound {
	if base == nil && overlay == nil {
		return nil
	}

	merged := make(map[string]nbt.NbtTag)
	if base != nil {
		for key, value := range base.Items() {
			merged[key] = value
		}
	}
	if overlay != nil {
		for key, value := range overlay.Items() {
			merged[key] = value
		}
	}

	return nbt.NewNbtCompound(merged)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ModelChainContainsTemplateSkull(model *data.BlockModelInstance) bool {
	if model == nil {
		return false
	}

	if _minecraftBlockRenderer.ContainsTemplateSkullToken(model.Name) {
		return true
	}

	for _, parent := range model.ParentChain {
		if _minecraftBlockRenderer.ContainsTemplateSkullToken(parent) {
			return true
		}
	}

	return false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ContainsTemplateSkullToken(candidate string) bool {
	if strings.TrimSpace(candidate) == "" {
		return false
	}

	return strings.Contains(strings.ToLower(candidate), "template_skull")
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ShouldUseSkyBlockTemplateSkullItemOrientation(model *data.BlockModelInstance, options BlockRenderOptions) bool {
	if model == nil || len(model.Elements) == 0 || options.ItemData == nil || options.ItemData.CustomData == nil {
		return false
	}
	if !_minecraftBlockRenderer.ModelChainContainsTemplateSkull(model) {
		return false
	}
	if _, ok := _minecraftBlockRenderer.TryGetString(options.ItemData.CustomData, "id"); !ok {
		return false
	}
	return true
}

func cloneTransformWithYawOffset(source *data.TransformDefinition, yawOffset float64) *data.TransformDefinition {
	if source == nil {
		return nil
	}

	var rotation []float64
	if source.Rotation != nil {
		rotation = make([]float64, len(*source.Rotation))
		copy(rotation, *source.Rotation)
	}
	for len(rotation) < 3 {
		rotation = append(rotation, 0)
	}
	rotation[1] += yawOffset

	var translation []float64
	if source.Translation != nil {
		translation = make([]float64, len(*source.Translation))
		copy(translation, *source.Translation)
	}

	var scale []float64
	if source.Scale != nil {
		scale = make([]float64, len(*source.Scale))
		copy(scale, *source.Scale)
	}

	return &data.TransformDefinition{
		Rotation:    &rotation,
		Translation: &translation,
		Scale:       &scale,
	}
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) applyInventory3DItemGuiOrientation(model *data.BlockModelInstance, options BlockRenderOptions) BlockRenderOptions {
	return _minecraftBlockRenderer.applyGuiYawCorrection(model, options, inventory3DItemGuiYawCorrection)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) applySpecialChestItemGuiOrientation(model *data.BlockModelInstance, options BlockRenderOptions) BlockRenderOptions {
	return _minecraftBlockRenderer.applyGuiYawCorrection(model, options, -inventory3DItemGuiYawCorrection-90)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) applyGuiYawCorrection(model *data.BlockModelInstance, options BlockRenderOptions, yawCorrection float64) BlockRenderOptions {
	if options.OverrideGuiTransform == nil && !options.UseGuiTransform {
		return options
	}

	source := options.OverrideGuiTransform
	if source == nil && options.UseGuiTransform && model != nil {
		source = model.GetDisplayTransform("gui")
	}
	if source == nil {
		source = DefaultGuiTransform
	}

	options.OverrideGuiTransform = cloneTransformWithYawOffset(source, yawCorrection)
	return options
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) IsPlayerHeadCandidate(itemName string, model *data.BlockModelInstance, modelCandidates []string) bool {
	normalized := _minecraftBlockRenderer.NormalizeItemTextureKey(itemName)
	if !strings.EqualFold(normalized, "player_head") {
		return false
	}

	if _minecraftBlockRenderer.ModelChainContainsTemplateSkull(model) {
		return true
	}

	for _, candidate := range modelCandidates {
		if _minecraftBlockRenderer.ContainsTemplateSkullToken(candidate) {
			return true
		}
	}

	return true
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) RenderPlayerHead(itemName string, model *data.BlockModelInstance, modelCandidates []string, options BlockRenderOptions) *image.RGBA {
	if !_minecraftBlockRenderer.IsPlayerHeadCandidate(itemName, model, modelCandidates) {
		return nil
	}

	skinSource := _minecraftBlockRenderer.TryResolveHeadSkin(itemName, options)
	if skinSource == nil {
		return nil
	}

	var rotation *[]float64
	if options.OverrideGuiTransform != nil {
		rotation = options.OverrideGuiTransform.Rotation
	}
	if rotation == nil && model != nil {
		if displayTransform, ok := model.Display["gui"]; ok {
			rotation = displayTransform.Rotation
		}
	}

	pitch := options.PitchInDegrees
	yaw := options.YawInDegrees
	roll := options.RollInDegrees

	if rotation != nil {
		if len(*rotation) > 0 {
			pitch += float64((*rotation)[0])
		}
		if len(*rotation) > 1 {
			yaw += float64((*rotation)[1])
		}
		if len(*rotation) > 2 {
			roll += float64((*rotation)[2])
		}
	}

	yaw -= 180.0

	headOptions := src.NewRenderOptions(options.Size, float64(yaw), float64(pitch), float64(roll))
	headOptions.PerspectiveAmount = float64(options.PerspectiveAmount)

	rendered := headOptions.RenderHead(headOptions, *skinSource)

	return &rendered
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryResolveHeadSkin(itemName string, options BlockRenderOptions) *image.RGBA {
	itemData := options.ItemData

	if itemData != nil && itemData.CustomData != nil {
		textureId, found := _minecraftBlockRenderer.GetHeadTextureOverride(itemData.CustomData)
		if found {
			skin := _minecraftBlockRenderer._textureRepository.GetTexture(textureId)
			if skin != nil && !_minecraftBlockRenderer._textureRepository.IsMissingTexture(skin) {
				return skin
			}
		}
	}

	if options.SkullTextureResolver != nil {
		var customDataId *string
		var profile *nbt.NbtCompound
		var customData *nbt.NbtCompound

		if itemData != nil {
			profile = itemData.Profile
			customData = itemData.CustomData
			if idValue, ok := _minecraftBlockRenderer.TryGetString(itemData.CustomData, "id"); ok && strings.TrimSpace(idValue) != "" {
				customDataId = &idValue
			}
		}

		context := SkullResolverContext{
			ItemId:       itemName,
			ItemData:     itemData,
			CustomDataId: customDataId,
			Profile:      profile,
			CustomData:   customData,
		}

		resolvedTexture := options.SkullTextureResolver(context)
		if resolvedTexture != nil && strings.TrimSpace(*resolvedTexture) != "" {
			if skin, ok := _minecraftBlockRenderer.TryLoadSkinFromTextureValue(*resolvedTexture); ok {
				return skin
			}
		}
	}

	if itemData != nil && itemData.Profile != nil {
		if skin, ok := _minecraftBlockRenderer.TryGetProfileSkin(itemData.Profile); ok {
			return skin
		}
	}

	if skin, ok := _minecraftBlockRenderer.TryGetDefaultPlayerSkin(); ok {
		return skin
	}

	return nil
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryLoadSkinFromTextureValue(textureValue string) (*image.RGBA, bool) {
	if strings.TrimSpace(textureValue) == "" {
		return nil, false
	}

	// Try to decode as base64 first (typical format from Skyblock repos)
	if decodedUrl, ok := _minecraftBlockRenderer.TryDecodeSkinPayload(textureValue); ok {
		if skin, err := _minecraftBlockRenderer.TryLoadSkinFromURL(decodedUrl); err == nil {
			return &skin, true
		}
	}

	// Try as direct URL
	if skin, err := _minecraftBlockRenderer.TryLoadSkinFromURL(textureValue); err == nil {
		return &skin, true
	}

	return nil, false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryGetProfileSkin(profile *nbt.NbtCompound) (*image.RGBA, bool) {
	if profile == nil {
		return nil, false
	}

	if skinURL, ok := _minecraftBlockRenderer.TryExtractSkinURL(profile); ok {
		if skin, err := _minecraftBlockRenderer.TryLoadSkinFromURL(skinURL); err == nil {
			return &skin, true
		}
	}

	return nil, false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryExtractSkinURL(profile *nbt.NbtCompound) (string, bool) {
	if profile == nil {
		return "", false
	}

	propertiesTag, ok := profile.Get("properties")
	if !ok {
		return "", false
	}

	properties, ok := propertiesTag.(*nbt.NbtList)
	if !ok {
		return "", false
	}

	for _, entry := range properties.Items() {
		propertyCompound, ok := entry.(*nbt.NbtCompound)
		if !ok {
			continue
		}

		name, hasName := _minecraftBlockRenderer.TryGetString(propertyCompound, "name")
		if !hasName || !strings.EqualFold(name, "textures") {
			continue
		}

		encoded, hasEncoded := _minecraftBlockRenderer.TryGetString(propertyCompound, "value")
		if !hasEncoded || strings.TrimSpace(encoded) == "" {
			continue
		}

		if decodedURL, decoded := _minecraftBlockRenderer.TryDecodeSkinPayload(encoded); decoded {
			return decodedURL, true
		}
	}

	return "", false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryDecodeSkinPayload(encodedPayload string) (string, bool) {
	encodedPayload = strings.TrimSpace(encodedPayload)
	if encodedPayload == "" {
		return "", false
	}

	paddingNeeded := (4 - (len(encodedPayload) % 4)) % 4
	padded := encodedPayload + strings.Repeat("=", paddingNeeded)

	payloadBytes, err := base64.StdEncoding.DecodeString(padded)
	if err != nil {
		return "", false
	}

	var payload struct {
		Textures struct {
			Skin struct {
				URL string `json:"url"`
			} `json:"SKIN"`
		} `json:"textures"`
	}

	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", false
	}

	candidate := strings.TrimSpace(payload.Textures.Skin.URL)
	if candidate == "" {
		return "", false
	}

	return candidate, true
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryLoadSkinFromURL(rawURL string) (image.RGBA, error) {
	normalized, ok := _minecraftBlockRenderer.TryNormalizeSkinURL(rawURL)
	if !ok {
		return image.RGBA{}, os.ErrInvalid
	}

	_minecraftBlockRenderer._playerSkinCacheMu.RLock()
	if _minecraftBlockRenderer._playerSkinCache != nil {
		if cached, found := _minecraftBlockRenderer._playerSkinCache[normalized]; found && cached != nil {
			cachedCopy := *cached
			_minecraftBlockRenderer._playerSkinCacheMu.RUnlock()
			return cachedCopy, nil
		}
	}
	_minecraftBlockRenderer._playerSkinCacheMu.RUnlock()

	skin, err := _minecraftBlockRenderer.LoadOrDownloadPlayerSkin(normalized)
	if err != nil {
		_minecraftBlockRenderer._playerSkinCacheMu.Lock()
		if _minecraftBlockRenderer._playerSkinCache != nil {
			delete(_minecraftBlockRenderer._playerSkinCache, normalized)
		}
		_minecraftBlockRenderer._playerSkinCacheMu.Unlock()
		return image.RGBA{}, err
	}

	skinCopy := skin
	_minecraftBlockRenderer._playerSkinCacheMu.Lock()
	if _minecraftBlockRenderer._playerSkinCache == nil {
		_minecraftBlockRenderer._playerSkinCache = make(map[string]*image.RGBA)
	}
	_minecraftBlockRenderer._playerSkinCache[normalized] = &skinCopy
	_minecraftBlockRenderer._playerSkinCacheMu.Unlock()

	return skin, nil
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryNormalizeSkinURL(rawURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil || !parsed.IsAbs() {
		return "", false
	}

	if !strings.EqualFold(parsed.Hostname(), "textures.minecraft.net") {
		return "", false
	}

	parsed.Scheme = "https"
	parsed.Host = strings.ToLower(parsed.Hostname())

	return parsed.String(), true
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) LoadOrDownloadPlayerSkin(normalizedURL string) (image.RGBA, error) {
	if skin, ok := _minecraftBlockRenderer.TryLoadSkinFromDisk(normalizedURL); ok {
		return skin, nil
	}

	return _minecraftBlockRenderer.DownloadPlayerSkin(normalizedURL)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryLoadSkinFromDisk(normalizedURL string) (image.RGBA, bool) {
	path := _minecraftBlockRenderer.GetSkinCachePath(normalizedURL)
	if path == nil {
		return image.RGBA{}, false
	}

	if _, err := os.Stat(*path); err != nil {
		return image.RGBA{}, false
	}

	imageFromDisk, err := imagecache.ReadRGBA(*path)
	if err == nil {
		return *imageFromDisk, true
	}

	_ = os.Remove(*path)
	return image.RGBA{}, false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) DownloadPlayerSkin(normalizedURL string) (image.RGBA, error) {
	response, err := global.HTTP_CLIENT.Get(normalizedURL)
	if err != nil {
		return image.RGBA{}, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return image.RGBA{}, os.ErrInvalid
	}

	decoded, err := data.LoadImageFromStream(response.Body)
	if err != nil {
		return image.RGBA{}, err
	}

	_minecraftBlockRenderer.TryPersistSkin(normalizedURL, decoded)
	return *decoded, nil
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryPersistSkin(normalizedURL string, skin *image.RGBA) {
	if skin == nil {
		return
	}

	path := _minecraftBlockRenderer.GetSkinCachePath(normalizedURL)
	if path == nil {
		return
	}

	_ = imagecache.WriteWebPAtomic(*path, skin)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetSkinCachePath(normalizedURL string) *string {
	if strings.TrimSpace(_minecraftBlockRenderer._playerSkinCacheDirectory) == "" {
		return nil
	}

	fileName := _minecraftBlockRenderer.GetSkinCacheFileName(normalizedURL)
	path := filepath.Join(_minecraftBlockRenderer._playerSkinCacheDirectory, fileName)
	return &path
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetSkinCacheFileName(normalizedURL string) string {
	return imagecache.HashKey("player_skin", normalizedURL) + ".webp"
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryGetDefaultPlayerSkin() (*image.RGBA, bool) {
	candidates := []string{
		"minecraft:entity/player/wide/steve",
		"minecraft:entity/steve",
		"minecraft:entity/player/wide/alex",
		"minecraft:entity/alex",
	}

	for _, candidate := range candidates {
		if skin, ok := _minecraftBlockRenderer._textureRepository.TryGetTexture(candidate); ok {
			return skin, true
		}
	}

	return nil, false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) AlignImageToBottom(img *image.RGBA) {
	bounds := _minecraftBlockRenderer.FindOpaqueBounds(img)
	if bounds.Dy() <= 0 {
		return
	}

	desiredTop := img.Bounds().Max.Y - bounds.Dy()
	deltaY := desiredTop - bounds.Min.Y
	if deltaY == 0 {
		return
	}

	clone := image.NewRGBA(img.Bounds())
	drawImage(clone, img, image.Point{})
	clearImage(img)
	drawImage(img, clone, image.Point{0, deltaY})
}

func drawImage(dst *image.RGBA, src *image.RGBA, offset image.Point) {
	draw.Draw(dst, src.Bounds().Add(offset), src, src.Bounds().Min, draw.Over)
}

func clearImage(img *image.RGBA) {
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.Set(x, y, image.Transparent)
		}
	}
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) FindOpaqueBounds(img *image.RGBA) image.Rectangle {
	minX := img.Bounds().Max.X
	minY := img.Bounds().Max.Y
	maxX := img.Bounds().Min.X - 1
	maxY := img.Bounds().Min.Y - 1

	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}

			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}

	if maxX < minX || maxY < minY {
		return image.Rectangle{}
	}

	return image.Rect(minX, minY, maxX+1, maxY+1)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ApplyCenteredScale(img *image.RGBA, scaleFactor float64) {
	if scaleFactor <= 0 || math.Abs(scaleFactor-1) < 1e-3 {
		return
	}

	targetWidth := int(math.Max(1, math.Round(float64(img.Bounds().Dx())*scaleFactor)))
	targetHeight := int(math.Max(1, math.Round(float64(img.Bounds().Dy())*float64(scaleFactor))))

	resized := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.ApproxBiLinear.Scale(resized, resized.Bounds(), img, img.Bounds(), draw.Over, nil)

	clearImage(img)
	offsetX := (img.Bounds().Dx() - targetWidth) / 2
	offsetY := (img.Bounds().Dy() - targetHeight) / 2
	drawImage(img, resized, image.Point{offsetX, offsetY})
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryRenderGuiTextureLayers(itemName string, itemInfo *data.ItemInfo, model *data.BlockModelInstance, options BlockRenderOptions) *image.RGBA {
	var candidates []string
	seen := make(map[string]struct{})
	isBillboardModel := model != nil && _minecraftBlockRenderer.IsBillboardModel(model)
	hasModelLayer := false

	tryAdd := func(candidate *string, allowNonGuiTexture bool, markModelLayer bool) {
		if candidate == nil || strings.TrimSpace(*candidate) == "" {
			return
		}

		if !allowNonGuiTexture && !_minecraftBlockRenderer.IsGuiTexture(*candidate) {
			return
		}

		if _, exists := seen[*candidate]; !exists {
			seen[*candidate] = struct{}{}
			candidates = append(candidates, *candidate)
			if markModelLayer {
				hasModelLayer = true
			}
		}
	}

	if model != nil {
		var orderedLayers []struct {
			Key   string
			Value string
		}
		for key, value := range model.Textures {
			if strings.HasPrefix(key, "layer") {
				orderedLayers = append(orderedLayers, struct {
					Key   string
					Value string
				}{Key: key, Value: value})
			}
		}
		sort.SliceStable(orderedLayers, func(i, j int) bool {
			return strings.Compare(orderedLayers[i].Key, orderedLayers[j].Key) < 0
		})

		for _, layer := range orderedLayers {
			tryAdd(&layer.Value, isBillboardModel, true)
		}
	}

	if !hasModelLayer && itemInfo != nil && itemInfo.Texture != nil && strings.TrimSpace(*itemInfo.Texture) != "" {
		tryAdd(itemInfo.Texture, isBillboardModel, false)
	}

	if !hasModelLayer {
		normalized := _minecraftBlockRenderer.NormalizeItemTextureKey(itemName)
		if strings.Contains(normalized, ":") {
			tryAdd(&normalized, false, false)
		} else {
			first := fmt.Sprintf("minecraft:item/%s", normalized)
			tryAdd(&first, false, false)
			second := fmt.Sprintf("minecraft:item/%s_overlay", normalized)
			tryAdd(&second, false, false)
			third := fmt.Sprintf("item/%s", normalized)
			tryAdd(&third, false, false)
			fourth := fmt.Sprintf("textures/item/%s", normalized)
			tryAdd(&fourth, false, false)
		}
	}

	if model != nil && len(model.Elements) > 0 {
		return nil
	}

	if len(candidates) == 0 {
		return nil
	}

	rendered := _minecraftBlockRenderer.TryRenderFlatItemFromIdentifiers(candidates, model, options, itemName)
	if rendered == nil {
		return nil
	}

	return rendered
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) IsBillboardModel(model *data.BlockModelInstance) bool {
	if model == nil {
		return false
	}

	if _, exists := model.Textures["cross"]; exists {
		return true
	}

	for _, parent := range model.ParentChain {
		if _minecraftBlockRenderer.ParentIndicatesBillboard(parent) {
			return true
		}
	}

	return _minecraftBlockRenderer.ParentIndicatesBillboard(model.Name)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ParentIndicatesBillboard(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "cross") ||
		strings.Contains(lower, "tinted_cross") ||
		strings.Contains(lower, "seagrass") ||
		strings.Contains(lower, "item/generated") ||
		strings.Contains(lower, "builtin/generated")
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) IsGuiTexture(textureId string) bool {
	if strings.TrimSpace(textureId) == "" {
		return false
	}

	normalized := strings.ReplaceAll(textureId, "\\", "/")
	lower := strings.ToLower(normalized)
	return strings.Contains(lower, "/item/") ||
		strings.Contains(lower, ":item/") ||
		strings.Contains(lower, "/items/") ||
		strings.Contains(lower, ":items/") ||
		strings.Contains(lower, "textures/item/")
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryRenderFlatItemFromIdentifiers(identifiers []string, model *data.BlockModelInstance, options BlockRenderOptions, tintContext string) *image.RGBA {
	resolved := _minecraftBlockRenderer.ResolveTextureIdentifiers(identifiers, model)
	var available []string

	for _, textureId := range resolved {
		texture := _minecraftBlockRenderer._textureRepository.GetTexture(textureId)
		if texture != nil && !_minecraftBlockRenderer._textureRepository.IsMissingTexture(texture) {
			available = append(available, textureId)
		}
	}

	if len(available) == 0 {
		return nil
	}

	// fmt.Printf("\navailable: %v\noptions: %v\ntintContext: %v\n", available, options, tintContext)
	rendered, err := _minecraftBlockRenderer.RenderFlatItem(available, options, tintContext)
	if err != nil {
		return nil
	}

	return &rendered
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ResolveTextureIdentifiers(identifiers []string, model *data.BlockModelInstance) []string {
	var resolved []string
	seen := make(map[string]struct{})

	for _, identifier := range identifiers {
		if strings.TrimSpace(identifier) == "" {
			continue
		}

		textureId := ResolveTexture(identifier, model)
		if strings.TrimSpace(textureId) == "" || strings.EqualFold(textureId, "minecraft:missingno") {
			continue
		}

		canonical := _minecraftBlockRenderer.NormalizeResourceKey(&textureId)
		if strings.TrimSpace(canonical) == "" {
			canonical = textureId
		}

		if _, exists := seen[canonical]; !exists {
			seen[canonical] = struct{}{}
			resolved = append(resolved, textureId)
		}
	}

	return resolved
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) RenderFlatItem(layerTextureIds []string, options BlockRenderOptions, tintContext string) (image.RGBA, error) {
	if len(layerTextureIds) == 0 {
		return image.RGBA{}, fmt.Errorf("no texture layers resolved for item %q", tintContext)
	}

	canvas := image.NewRGBA(image.Rect(0, 0, options.Size, options.Size))
	itemInfo := (*data.ItemInfo)(nil)
	normalizedItemKey := (*string)(nil)

	if strings.TrimSpace(tintContext) != "" {
		itemKey := _minecraftBlockRenderer.NormalizeItemTextureKey(tintContext)
		normalizedItemKey = &itemKey

		if _minecraftBlockRenderer._itemRegistry != nil {
			itemInfo = _minecraftBlockRenderer._itemRegistry.GetItemInfo(tintContext)
			if itemInfo == nil && !strings.EqualFold(*normalizedItemKey, tintContext) {
				itemInfo = _minecraftBlockRenderer._itemRegistry.GetItemInfo(*normalizedItemKey)
			}
		}
	}

	primaryTintLayerIndex := _minecraftBlockRenderer.DeterminePrimaryTintLayerIndex(normalizedItemKey, itemInfo)
	explicitItemData := options.ItemData
	disablePrimaryDefault := explicitItemData != nil && explicitItemData.DisableDefaultLayer0Tint

	for layerIndex, textureId := range layerTextureIds {
		explicitLayerTint := _minecraftBlockRenderer.GetExplicitLayerTint(explicitItemData, layerIndex, primaryTintLayerIndex)
		layerTint := explicitLayerTint
		defaultTintApplied := false
		defaultTint, _ := _minecraftBlockRenderer.TryResolveDefaultLayerTint(normalizedItemKey, itemInfo, layerIndex, layerIndex == primaryTintLayerIndex, disablePrimaryDefault)
		if layerTint == nil && defaultTint != nil {
			layerTint = defaultTint
			defaultTintApplied = true
		}

		if defaultTintApplied && _minecraftBlockRenderer.ShouldBypassDefaultLayerTint(textureId, layerIndex, primaryTintLayerIndex, len(layerTextureIds)) {
			layerTint = nil
			defaultTintApplied = false
		}

		hasExplicitPerLayerTint := explicitItemData != nil && explicitItemData.AdditionalLayerTints != nil && explicitItemData.AdditionalLayerTints[layerIndex] != nil
		hasPrimaryExplicitTint := layerIndex == primaryTintLayerIndex && explicitItemData != nil && explicitItemData.Layer0Tint != nil
		skipContextTint := layerTint != nil || hasExplicitPerLayerTint || hasPrimaryExplicitTint
		// fmt.Printf("ResolveItemLayerTexture: %v %v %v\n", textureId, tintContext, skipContextTint)
		texture := _minecraftBlockRenderer.ResolveItemLayerTexture(textureId, tintContext, skipContextTint)
		if texture == nil || _minecraftBlockRenderer._textureRepository.IsMissingTexture(texture) {
			return image.RGBA{}, fmt.Errorf("texture %q could not be resolved for item %q", textureId, tintContext)
		}

		scale := math.Min(float64(options.Size)/float64(texture.Bounds().Dx()), float64(options.Size)/float64(texture.Bounds().Dy()))
		// fmt.Printf("Scale: %f\n", scale)
		targetWidth := int(math.Max(1, math.Round(float64(texture.Bounds().Dx())*scale)))
		targetHeight := int(math.Max(1, math.Round(float64(texture.Bounds().Dy())*scale)))

		// fmt.Printf("targetWidth, targetHeight: %d %d\n", targetWidth, targetHeight)

		resized := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
		draw.NearestNeighbor.Scale(resized, resized.Bounds(), texture, texture.Bounds(), draw.Over, nil)

		if layerTint != nil {
			_minecraftBlockRenderer.ApplyLayerTint(resized, *layerTint)
		}

		offset := image.Point{(canvas.Bounds().Dx() - targetWidth) / 2, (canvas.Bounds().Dy() - targetHeight) / 2}
		drawImage(canvas, resized, offset)
	}

	return *canvas, nil
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) DeterminePrimaryTintLayerIndex(normalizedItemKey *string, itemInfo *data.ItemInfo) int {
	if itemInfo != nil && len(itemInfo.LayerTints) > 0 {
		var dyeLayers []int
		for index, tint := range itemInfo.LayerTints {
			if tint.Kind == data.ItemTintKindDye {
				dyeLayers = append(dyeLayers, index)
			}
		}

		sort.Ints(dyeLayers)
		if len(dyeLayers) > 0 {
			return dyeLayers[0]
		}

		var tintLayerIndices []int
		for index := range itemInfo.LayerTints {
			tintLayerIndices = append(tintLayerIndices, index)
		}
		sort.Ints(tintLayerIndices)
		return tintLayerIndices[0]
	}

	if normalizedItemKey != nil && strings.TrimSpace(*normalizedItemKey) != "" {
		if overrides, found := LegacyDefaultTintLayerOverrides[*normalizedItemKey]; found && len(overrides) > 0 {
			min := overrides[0]
			for _, val := range overrides {
				if val < min {
					min = val
				}
			}
			return min
		}
	}

	return 0
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryResolveDefaultLayerTint(normalizedItemKey *string, itemInfo *data.ItemInfo, layerIndex int, isPrimaryDyeLayer bool, disablePrimaryDefault bool) (*color.RGBA, bool) {
	if itemInfo != nil && len(itemInfo.LayerTints) > 0 {
		tintInfo := itemInfo.LayerTints[layerIndex]
		switch tintInfo.Kind {
		case data.ItemTintKindDye:
			if !disablePrimaryDefault || !isPrimaryDyeLayer {
				if tintInfo.DefaultColor != nil {
					return tintInfo.DefaultColor, true
				}
			}
		case data.ItemTintKindConstant:
			if tintInfo.DefaultColor != nil {
				return tintInfo.DefaultColor, true
			}
		default:
			if tintInfo.DefaultColor != nil && !(disablePrimaryDefault && isPrimaryDyeLayer) {
				return tintInfo.DefaultColor, true
			}
		}
	}

	if normalizedItemKey == nil || strings.TrimSpace(*normalizedItemKey) == "" {
		return nil, false
	}

	if overrides, found := LegacyDefaultTintLayerOverrides[*normalizedItemKey]; found {
		for _, overrideLayer := range overrides {
			if overrideLayer == layerIndex {
				if !(disablePrimaryDefault && isPrimaryDyeLayer) {
					if overrideColor, colorFound := LegacyDefaultTintOverrides[*normalizedItemKey]; colorFound {
						return &overrideColor, true
					}
				}
				break
			}
		}
	}

	if layerIndex == 0 && LegacyDefaultTintOverrides != nil {
		if legacyColor, found := LegacyDefaultTintOverrides[*normalizedItemKey]; found {
			if !(disablePrimaryDefault && isPrimaryDyeLayer) {
				return &legacyColor, true
			}
		}
	}

	if normalizedItemKey != nil && strings.HasPrefix(strings.ToLower(*normalizedItemKey), "leather_") && layerIndex == 0 && !(disablePrimaryDefault && isPrimaryDyeLayer) {
		return &DefaultLeatherArmorColor, true
	}

	return nil, false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetExplicitLayerTint(explicitItemData *data.ItemRenderData, layerIndex int, primaryTintLayerIndex int) *color.RGBA {
	if explicitItemData == nil {
		return nil
	}

	if explicitItemData.AdditionalLayerTints != nil {
		if explicitTint, found := explicitItemData.AdditionalLayerTints[layerIndex]; found {
			return explicitTint
		}
	}

	if layerIndex == primaryTintLayerIndex && explicitItemData.Layer0Tint != nil {
		return explicitItemData.Layer0Tint
	}

	return nil
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ShouldBypassDefaultLayerTint(textureId string, layerIndex int, primaryTintLayerIndex int, totalLayerCount int) bool {
	if totalLayerCount != 1 {
		return false
	}

	if layerIndex != primaryTintLayerIndex {
		return false
	}

	textureNamespace := _minecraftBlockRenderer.ExtractResourceNamespace(textureId)
	return !strings.EqualFold(textureNamespace, "minecraft")
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ExtractResourceNamespace(textureId string) string {
	if strings.TrimSpace(textureId) == "" {
		return "minecraft"
	}

	normalized := strings.TrimSpace(textureId)
	colonIndex := strings.Index(normalized, ":")
	if colonIndex >= 0 {
		return strings.ToLower(normalized[:colonIndex])
	}

	return "minecraft"
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ResolveItemLayerTexture(textureId string, tintContext string, skipContextTint bool) *image.RGBA {
	if skipContextTint {
		return _minecraftBlockRenderer._textureRepository.GetTexture(textureId)
	}

	constantTint := _minecraftBlockRenderer.TryGetConstantTint(textureId, &tintContext)
	if constantTint != nil {
		return _minecraftBlockRenderer._textureRepository.GetTintedTexture(textureId, *constantTint, ConstantTintStrength, float64(1.0))
	}

	biomeKind := _minecraftBlockRenderer.TryGetBiomeTintKind(textureId, tintContext)
	if biomeKind != nil {
		return _minecraftBlockRenderer.GetBiomeTintedTexture(textureId, *biomeKind)
	}

	if _minecraftBlockRenderer.ShouldApplyItemColorTint(textureId, tintContext) {
		fallbackTint := _minecraftBlockRenderer.GetColorFromBlockName(tintContext)
		if fallbackTint == nil {
			fallbackTint = _minecraftBlockRenderer.GetColorFromBlockName(textureId)
		}
		if fallbackTint != nil {
			return _minecraftBlockRenderer._textureRepository.GetTintedTexture(textureId, *fallbackTint, float64(1.0), float64(1.0))
		}
	}

	return _minecraftBlockRenderer._textureRepository.GetTexture(textureId)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ShouldApplyItemColorTint(textureId string, tintContext string) bool {
	if strings.TrimSpace(tintContext) == "" {
		return false
	}

	normalizedContext := _minecraftBlockRenderer.NormalizeResourceKey(&tintContext)
	if strings.TrimSpace(normalizedContext) == "" {
		return false
	}

	textureKey := _minecraftBlockRenderer.NormalizeResourceKey(&textureId)
	itemInfo := _minecraftBlockRenderer._itemRegistry.GetItemInfo(normalizedContext)
	if itemInfo != nil {
		var model *data.BlockModelInstance

		if itemInfo.Model != nil && strings.TrimSpace(*itemInfo.Model) != "" {
			model = _minecraftBlockRenderer.ResolveModelOrNull(itemInfo.Model)
		}

		if model == nil {
			model = _minecraftBlockRenderer.ResolveModelOrNull(&normalizedContext)
		}

		if model != nil {
			if _minecraftBlockRenderer.ModelChainIndicatesDyeTint(model) {
				return true
			}

			return _minecraftBlockRenderer.ShouldApplyColorByHeuristic(textureKey, normalizedContext)
		}
	}

	return _minecraftBlockRenderer.ShouldApplyColorByHeuristic(textureKey, normalizedContext)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ResolveModelOrNull(name *string) *data.BlockModelInstance {
	if name == nil || strings.TrimSpace(*name) == "" {
		return nil
	}

	model, ok := _minecraftBlockRenderer._modelResolver.TryResolve(*name)
	if !ok || model == nil {
		return nil
	}

	return model
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ModelChainIndicatesDyeTint(model *data.BlockModelInstance) bool {
	if _minecraftBlockRenderer.IsDyeTintTemplate(model.Name) {
		return true
	}

	for _, parent := range model.ParentChain {
		if _minecraftBlockRenderer.IsDyeTintTemplate(parent) {
			return true
		}
	}

	return false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) IsDyeTintTemplate(candidate string) bool {
	if strings.TrimSpace(candidate) == "" {
		return false
	}

	lower := strings.ToLower(candidate)
	return strings.Contains(lower, "template_shulker_box") || strings.Contains(lower, "template_banner")
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) GetColorFromBlockName(blockName string) *color.RGBA {
	if strings.TrimSpace(blockName) == "" {
		return nil
	}

	name := _minecraftBlockRenderer.NormalizeResourceKey(&blockName)
	if strings.HasSuffix(name, "bundle") || strings.Contains(name, "_bundle") {
		return nil
	}

	if constantColor, found := BiomeTints.ConstantColors[name]; found {
		return &constantColor
	}

	for colorName, colorValue := range ColorMap {
		if strings.HasPrefix(name, colorName) {
			rgba := color.RGBA{
				R: colorValue.R,
				G: colorValue.G,
				B: colorValue.B,
				A: colorValue.A,
			}
			return &rgba
		}
	}

	return nil
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ShouldApplyColorByHeuristic(textureKey string, contextKey string) bool {
	if !_minecraftBlockRenderer.ContainsColorToken(contextKey) {
		return false
	}

	if strings.TrimSpace(textureKey) == "" {
		return true
	}

	return !_minecraftBlockRenderer.ContainsColorToken(textureKey)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ContainsColorToken(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}

	for colorName := range ColorMap {
		if _minecraftBlockRenderer.ContainsColorTokenWithName(value, colorName) {
			return true
		}
	}

	return false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ContainsColorTokenWithName(source string, token string) bool {
	index := strings.Index(strings.ToLower(source), strings.ToLower(token))
	for index >= 0 {
		beforeIndex := index - 1
		afterIndex := index + len(token)
		hasLetterBefore := beforeIndex >= 0 && unicode.IsLetter(rune(source[beforeIndex]))
		hasLetterAfter := afterIndex < len(source) && unicode.IsLetter(rune(source[afterIndex]))

		if !hasLetterBefore && !hasLetterAfter {
			return true
		}

		nextSearchStart := index + 1
		if nextSearchStart >= len(source) {
			break
		}
		index = strings.Index(strings.ToLower(source[nextSearchStart:]), strings.ToLower(token))
		if index >= 0 {
			index += nextSearchStart
		}
	}

	return false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ApplyLayerTint(img *image.RGBA, tint color.RGBA) {
	tintVector := struct {
		R, G, B, A float64
	}{
		R: float64(tint.R) / 255.0,
		G: float64(tint.G) / 255.0,
		B: float64(tint.B) / 255.0,
		A: 1.0,
	}

	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			pixel := img.At(x, y).(color.RGBA)
			alpha := float64(pixel.A) / 255.0
			r := math.Min(float64(pixel.R)/255.0*tintVector.R, 1.0)
			g := math.Min(float64(pixel.G)/255.0*tintVector.G, 1.0)
			b := math.Min(float64(pixel.B)/255.0*tintVector.B, 1.0)
			a := alpha * tintVector.A
			img.Set(x, y, color.RGBA{
				R: uint8(r * 255.0),
				G: uint8(g * 255.0),
				B: uint8(b * 255.0),
				A: uint8(a * 255.0),
			})
		}
	}
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryRenderBedItem(itemName string, itemModel *data.BlockModelInstance, options BlockRenderOptions) *image.RGBA {
	normalizedName := _minecraftBlockRenderer.NormalizeItemTextureKey(itemName)
	if !strings.HasSuffix(normalizedName, "_bed") && !strings.EqualFold(normalizedName, "bed") {
		return nil
	}

	colorName := "red"
	if !strings.EqualFold(normalizedName, "bed") {
		colorName = normalizedName[:len(normalizedName)-4]
	}

	bedTextureId := fmt.Sprintf("minecraft:entity/bed/%s", colorName)
	texture := _minecraftBlockRenderer._textureRepository.GetTexture(bedTextureId)
	if texture == nil {
		bedTextureId = "minecraft:entity/bed/red"
		texture = _minecraftBlockRenderer._textureRepository.GetTexture(bedTextureId)
		if texture == nil {
			return nil
		}
	}

	bedHead := "bed/bed_head"
	headModel := _minecraftBlockRenderer.ResolveModelOrNull(&bedHead)
	bedFoot := "bed/bed_foot"
	footModel := _minecraftBlockRenderer.ResolveModelOrNull(&bedFoot)
	if headModel == nil || footModel == nil {
		rendered, err := _minecraftBlockRenderer.RenderFlatItem([]string{bedTextureId}, options, itemName)
		if err != nil {
			return nil
		}
		return &rendered
	}

	var elements []data.ModelElement
	elements = append(elements, _minecraftBlockRenderer.CloneAndTranslateElements(headModel, data.Vector3{X: 0, Y: 0, Z: -16}, false, false)...)
	elements = append(elements, _minecraftBlockRenderer.CloneAndTranslateElements(footModel, data.Vector3{X: 0, Y: 0, Z: 0}, true, true)...)
	if len(elements) == 0 {
		return nil
	}

	textures := _minecraftBlockRenderer.CloneTextureDictionary(itemModel)
	textures["bed"] = bedTextureId
	if _, found := textures["particle"]; !found {
		textures["particle"] = _minecraftBlockRenderer.DetermineBedParticleTexture(colorName, bedTextureId)
	}

	displaySource := itemModel
	if displaySource == nil || len(displaySource.Display) == 0 {
		templateBed := "item/template_bed"
		displaySource = _minecraftBlockRenderer.ResolveModelOrNull(&templateBed)
		if displaySource == nil {
			displaySource = itemModel
		}
	}

	display := _minecraftBlockRenderer.CloneDisplayDictionary(displaySource)
	_minecraftBlockRenderer.AdjustBedGuiTransform(display)

	renderOptions := options
	if adjustedGui, found := display["gui"]; found {
		renderOptions.OverrideGuiTransform = adjustedGui
	} else {
		renderOptions.OverrideGuiTransform = nil
	}

	composite := data.BlockModelInstance{
		Name:     "minecraft:generated/bed_composite",
		Textures: textures,
		Display:  display,
		Elements: elements,
	}

	rendered := _minecraftBlockRenderer.RenderModel(&composite, renderOptions, nil)
	if rendered == nil {
		return nil
	}

	return rendered
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) CloneAndTranslateElements(source *data.BlockModelInstance, translation data.Vector3, flipBottomFaces bool, flipNorthSouthFaces bool) []data.ModelElement {
	var result []data.ModelElement
	for i := 0; i < len(source.Elements); i++ {
		result = append(result, _minecraftBlockRenderer.CloneAndTranslateElement(source.Elements[i], translation, flipBottomFaces, flipNorthSouthFaces))
	}

	return result
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) CloneAndTranslateElement(element data.ModelElement, translation data.Vector3, flipBottomFaces bool, flipNorthSouthFaces bool) data.ModelElement {
	from := data.Vector3{
		X: element.From.X + translation.X,
		Y: element.From.Y + translation.Y,
		Z: element.From.Z + translation.Z,
	}
	to := data.Vector3{
		X: element.To.X + translation.X,
		Y: element.To.Y + translation.Y,
		Z: element.To.Z + translation.Z,
	}

	var rotation *data.ElementRotation
	if element.Rotation != nil {
		rotation = &data.ElementRotation{
			AngleInDegrees: element.Rotation.AngleInDegrees,
			Origin: data.Vector3{
				X: element.Rotation.Origin.X + translation.X,
				Y: element.Rotation.Origin.Y + translation.Y,
				Z: element.Rotation.Origin.Z + translation.Z,
			},
			Axis:    element.Rotation.Axis,
			Rescale: element.Rotation.Rescale,
		}
	}

	faces := make(map[data.BlockFaceDirection]data.ModelFace)
	elementHeight := element.To.Y - element.From.Y
	shouldFlipLargeFaces := elementHeight > 3.01
	for direction, face := range element.Faces {
		if flipBottomFaces && direction == data.Down && shouldFlipLargeFaces {
			var uv *data.Vector4
			if face.Uv != nil {
				raw := *face.Uv
				uv = &data.Vector4{
					X: raw.Z,
					Y: raw.Y,
					Z: raw.X,
					W: raw.W,
				}
			}

			var rotated *int
			if face.Rotation != nil {
				rot := (*face.Rotation + 180) % 360
				rotated = &rot
			}

			faces[direction] = data.ModelFace{
				Texture:   face.Texture,
				Uv:        uv,
				Rotation:  rotated,
				TintIndex: face.TintIndex,
				CullFace:  face.CullFace,
			}
		} else if flipNorthSouthFaces && shouldFlipLargeFaces && (direction == data.North || direction == data.South) {
			var uv *data.Vector4
			if face.Uv != nil {
				raw := *face.Uv
				uv = &data.Vector4{
					X: raw.X,
					Y: raw.W,
					Z: raw.Z,
					W: raw.Y,
				}
			}

			rot := 0
			if face.Rotation != nil {
				rot = (*face.Rotation + 180) % 360
			}
			faces[direction] = data.ModelFace{
				Texture:   face.Texture,
				Uv:        uv,
				Rotation:  &rot,
				TintIndex: face.TintIndex,
				CullFace:  face.CullFace,
			}
		} else {
			faces[direction] = data.ModelFace{
				Texture:   face.Texture,
				Uv:        face.Uv,
				Rotation:  face.Rotation,
				TintIndex: face.TintIndex,
				CullFace:  face.CullFace,
			}
		}
	}

	return data.ModelElement{
		From:     from,
		To:       to,
		Rotation: rotation,
		Faces:    faces,
		Shade:    element.Shade,
	}
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) CloneTextureDictionary(source *data.BlockModelInstance) map[string]string {
	result := make(map[string]string)
	if source == nil || len(source.Textures) == 0 {
		return result
	}

	for key, value := range source.Textures {
		result[key] = value
	}

	return result
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) DetermineBedParticleTexture(colorName string, fallbackTextureId string) string {
	candidates := []string{
		fmt.Sprintf("minecraft:block/%s_wool", colorName),
		fallbackTextureId,
	}

	for _, candidate := range candidates {
		if _minecraftBlockRenderer._textureRepository.GetTexture(candidate) != nil {
			return candidate
		}
	}

	return fallbackTextureId
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) CloneDisplayDictionary(source *data.BlockModelInstance) map[string]*data.TransformDefinition {
	result := make(map[string]*data.TransformDefinition)
	if source == nil || len(source.Display) == 0 {
		return result
	}

	for key, transform := range source.Display {
		var rotation []float64
		if transform.Rotation != nil {
			rotation = make([]float64, len(*transform.Rotation))
			copy(rotation, *transform.Rotation)
		}

		var translation []float64
		if transform.Translation != nil {
			translation = make([]float64, len(*transform.Translation))
			copy(translation, *transform.Translation)
		}

		var scale []float64
		if transform.Scale != nil {
			scale = make([]float64, len(*transform.Scale))
			copy(scale, *transform.Scale)
		}

		result[key] = &data.TransformDefinition{
			Rotation:    &rotation,
			Translation: &translation,
			Scale:       &scale,
		}
	}

	return result
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) AdjustBedGuiTransform(display map[string]*data.TransformDefinition) {
	const rotationAdjustment = 180.0
	const scaleMultiplier = 0.9
	defaultScale := []float64{0.48, 0.48, 0.48}
	translationAdjustment := []float64{-2.5, -2.75, 0}

	gui, found := display["gui"]
	if !found {
		display["gui"] = &data.TransformDefinition{
			Rotation:    &[]float64{30, 160 + rotationAdjustment, 0},
			Translation: &translationAdjustment,
			Scale:       &defaultScale,
		}
		return
	}

	rotationArray := make([]float64, 3)
	if gui.Rotation != nil {
		copy(rotationArray, *gui.Rotation)
	}
	rotationArray[1] = float64(math.Mod(float64(rotationArray[1])+rotationAdjustment, 360))

	var translationArray []float64
	if gui.Translation == nil || len(*gui.Translation) == 0 {
		translationArray = make([]float64, 3)
	} else {
		translationArray = make([]float64, len(*gui.Translation))
		copy(translationArray, *gui.Translation)
	}
	for i := 0; i < 3; i++ {
		translationArray[i] += translationAdjustment[i]
	}

	var scaleArray []float64
	if gui.Scale == nil || len(*gui.Scale) == 0 {
		scaleArray = make([]float64, len(defaultScale))
		copy(scaleArray, defaultScale)
	} else {
		scaleArray = make([]float64, len(*gui.Scale))
		copy(scaleArray, *gui.Scale)
		for i := 0; i < len(scaleArray); i++ {
			scaleArray[i] *= scaleMultiplier
		}
	}

	display["gui"] = &data.TransformDefinition{
		Rotation:    &rotationArray,
		Translation: &translationArray,
		Scale:       &scaleArray,
	}
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) HasExplicitFlatHeadOverride(model *data.BlockModelInstance, modelCandidates []string, options BlockRenderOptions) bool {
	hasNonDefaultCandidate := _minecraftBlockRenderer.HasNonDefaultPlayerHeadModelCandidate(modelCandidates)
	itemData := options.ItemData
	hasProfileData := itemData != nil && itemData.Profile != nil
	var exists bool
	if itemData != nil {
		_, exists = _minecraftBlockRenderer.GetHeadTextureOverride(itemData.CustomData)
	}
	hasCustomTexture := itemData != nil && itemData.CustomData != nil && exists

	if (hasProfileData || hasCustomTexture) &&
		hasNonDefaultCandidate &&
		_minecraftBlockRenderer.ModelChainContainsTemplateSkull(model) {
		if _minecraftBlockRenderer.IsResolvedCustomTemplateSkullModel(model) {
			return true
		}
		return false
	}

	if _minecraftBlockRenderer.ModelChainContainsTemplateSkull(model) {
		return hasNonDefaultCandidate
	}

	if !hasNonDefaultCandidate {
		for _, candidate := range modelCandidates {
			if _minecraftBlockRenderer.ContainsTemplateSkullToken(candidate) {
				return false
			}
		}
	}

	if model != nil {
		return true
	}

	return hasNonDefaultCandidate
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) IsResolvedCustomTemplateSkullModel(model *data.BlockModelInstance) bool {
	if model == nil {
		return false
	}

	if strings.TrimSpace(model.Name) == "" || _minecraftBlockRenderer.ContainsTemplateSkullToken(model.Name) {
		return false
	}

	return !_minecraftBlockRenderer.IsDefaultPlayerHeadModelCandidate(model.Name)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) HasNonDefaultPlayerHeadModelCandidate(modelCandidates []string) bool {
	if len(modelCandidates) == 0 {
		return false
	}

	for _, candidate := range modelCandidates {
		if _minecraftBlockRenderer.IsDefaultPlayerHeadModelCandidate(candidate) {
			continue
		}

		if !_minecraftBlockRenderer.ContainsTemplateSkullToken(candidate) {
			return true
		}
	}

	return false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) IsDefaultPlayerHeadModelCandidate(candidate string) bool {
	if strings.TrimSpace(candidate) == "" {
		return false
	}

	normalized := _minecraftBlockRenderer.NormalizeItemTextureKey(candidate)
	return strings.EqualFold(normalized, "player_head") ||
		strings.EqualFold(normalized, "item/player_head") ||
		strings.EqualFold(normalized, "player_head_inventory") ||
		strings.EqualFold(normalized, "item/player_head_inventory") ||
		strings.EqualFold(normalized, "player_head#inventory") ||
		strings.EqualFold(normalized, "item/player_head#inventory")
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) CollectBillboardTextures(model *data.BlockModelInstance, itemInfo *data.ItemInfo) []string {
	var textures []string
	seen := make(map[string]struct{})

	tryAdd := func(candidate *string) {
		if candidate != nil && strings.TrimSpace(*candidate) != "" {
			if _, exists := seen[*candidate]; !exists {
				seen[*candidate] = struct{}{}
				textures = append(textures, *candidate)
			}
		}
	}

	if model != nil {
		if crossTexture, found := model.Textures["cross"]; found {
			tryAdd(&crossTexture)
		}

		if genericTexture, found := model.Textures["texture"]; found {
			tryAdd(&genericTexture)
		}
	}

	if len(textures) == 0 && itemInfo != nil {
		tryAdd(itemInfo.Texture)
	}

	return textures
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) CollectItemLayerTextures(model *data.BlockModelInstance, itemInfo *data.ItemInfo) []string {
	var layers []string
	seen := make(map[string]struct{})

	tryAdd := func(candidate *string) {
		if candidate == nil || strings.TrimSpace(*candidate) == "" {
			return
		}
		key := strings.ToLower(strings.TrimSpace(*candidate))
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		layers = append(layers, *candidate)
	}

	if model != nil {
		var orderedLayers []struct {
			Key   string
			Value string
		}
		for key, value := range model.Textures {
			if strings.HasPrefix(strings.ToLower(key), "layer") {
				orderedLayers = append(orderedLayers, struct {
					Key   string
					Value string
				}{Key: key, Value: value})
			}
		}
		sort.SliceStable(orderedLayers, func(i, j int) bool {
			return strings.Compare(strings.ToLower(orderedLayers[i].Key), strings.ToLower(orderedLayers[j].Key)) < 0
		})
		for _, layer := range orderedLayers {
			tryAdd(&layer.Value)
		}
	}

	if itemInfo != nil {
		tryAdd(itemInfo.Texture)
	}

	return layers
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryRenderBlockEntityFallback(itemName string, itemInfo *data.ItemInfo, model *data.BlockModelInstance, modelCandidates []string, options BlockRenderOptions) *image.RGBA {
	blockOptions := options
	if options.OverrideGuiTransform != nil && model != nil && len(model.Elements) == 0 {
		if itemGuiTransform, found := model.Display["gui"]; found && itemGuiTransform == options.OverrideGuiTransform {
			blockOptions.OverrideGuiTransform = nil
		}
	}

	for _, candidate := range _minecraftBlockRenderer.EnumerateBlockFallbackNames(itemName, itemInfo, model, modelCandidates) {
		if rendered := _minecraftBlockRenderer.TryRenderBlockItem(candidate, blockOptions); rendered != nil {
			return rendered
		}
	}

	return nil
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) EnumerateBlockFallbackNames(itemName string, itemInfo *data.ItemInfo, model *data.BlockModelInstance, modelCandidates []string) []string {
	var results []string
	seen := make(map[string]struct{})

	tryAdd := func(candidate *string) {
		if candidate != nil && strings.TrimSpace(*candidate) != "" {
			normalized := _minecraftBlockRenderer.NormalizeResourceKey(candidate)
			if _, exists := seen[normalized]; !exists {
				seen[normalized] = struct{}{}
				results = append(results, *candidate)
			}
		}
	}

	for _, candidate := range _minecraftBlockRenderer.NormalizeToBlockCandidates(itemName) {
		tryAdd(&candidate)
	}

	if itemInfo != nil {
		if itemInfo.Model != nil {
			for _, candidate := range _minecraftBlockRenderer.NormalizeToBlockCandidates(*itemInfo.Model) {
				tryAdd(&candidate)
			}
		}
		if itemInfo.Texture != nil {
			for _, candidate := range _minecraftBlockRenderer.NormalizeToBlockCandidates(*itemInfo.Texture) {
				tryAdd(&candidate)
			}
		}
	}

	for _, modelCandidate := range modelCandidates {
		for _, candidate := range _minecraftBlockRenderer.NormalizeToBlockCandidates(modelCandidate) {
			tryAdd(&candidate)
		}
	}

	if model != nil {
		for _, candidate := range _minecraftBlockRenderer.NormalizeToBlockCandidates(model.Name) {
			tryAdd(&candidate)
		}

		for _, parent := range model.ParentChain {
			for _, candidate := range _minecraftBlockRenderer.NormalizeToBlockCandidates(parent) {
				tryAdd(&candidate)
			}
		}

		for _, texture := range model.Textures {
			for _, candidate := range _minecraftBlockRenderer.NormalizeToBlockCandidates(texture) {
				tryAdd(&candidate)
			}
		}
	}

	return results
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) NormalizeToBlockCandidates(value string) []string {
	var candidates []string
	if strings.TrimSpace(value) == "" {
		return candidates
	}

	normalized := strings.TrimSpace(value)
	normalized = strings.ReplaceAll(normalized, "\\", "/")

	if strings.HasPrefix(normalized, "#") {
		return candidates
	}

	if strings.HasPrefix(strings.ToLower(normalized), "minecraft:") {
		normalized = normalized[10:]
	}

	normalized = strings.TrimLeft(normalized, "/")

	if strings.HasPrefix(strings.ToLower(normalized), "textures/") {
		normalized = normalized[9:]
	}

	if strings.HasPrefix(strings.ToLower(normalized), "models/") {
		normalized = normalized[7:]
	}

	if strings.HasPrefix(strings.ToLower(normalized), "block/") {
		normalized = normalized[6:]
	} else if strings.HasPrefix(strings.ToLower(normalized), "blocks/") {
		normalized = normalized[7:]
	} else if strings.HasPrefix(strings.ToLower(normalized), "item/") ||
		strings.HasPrefix(strings.ToLower(normalized), "items/") {
		return candidates
	} else if strings.HasPrefix(strings.ToLower(normalized), "builtin/") {
		return candidates
	}

	normalized = strings.Trim(normalized, "/")
	if strings.TrimSpace(normalized) == "" {
		return candidates
	}

	candidates = append(candidates, normalized)
	return candidates
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryRenderBlockItem(blockName string, options BlockRenderOptions) *image.RGBA {
	modelName := blockName
	if _minecraftBlockRenderer._blockRegistry != nil {
		if mappedModel, found := _minecraftBlockRenderer._blockRegistry.TryGetModel(blockName); found && strings.TrimSpace(mappedModel) != "" {
			modelName = mappedModel
		}
	}
	model, found := _minecraftBlockRenderer._modelResolver.TryResolve(modelName)
	if !found {
		return nil
	}

	options = _minecraftBlockRenderer.applyInventory3DItemGuiOrientation(model, options)
	rendered := _minecraftBlockRenderer.RenderBlock(blockName, options)
	if rendered == nil || !hasVisiblePixels(rendered) {
		return nil
	}

	return rendered
}

func hasVisiblePixels(img *image.RGBA) bool {
	if img == nil {
		return false
	}
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.RGBAAt(x, y).A > 10 {
				return true
			}
		}
	}
	return false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) RenderFallbackTexture(itemName string, itemInfo *data.ItemInfo, model *data.BlockModelInstance, options BlockRenderOptions) *image.RGBA {
	if rendered := _minecraftBlockRenderer.TryRenderFlatItemFromIdentifiers(_minecraftBlockRenderer.CollectItemLayerTextures(model, itemInfo), model, options, itemName); rendered != nil {
		return rendered
	}

	if itemInfo != nil && itemInfo.Texture != nil && strings.TrimSpace(*itemInfo.Texture) != "" {
		if rendered := _minecraftBlockRenderer.TryRenderEmbeddedTexture(*itemInfo.Texture, options, itemName); rendered != nil {
			return rendered
		}
	}

	for _, candidate := range _minecraftBlockRenderer.EnumerateTextureFallbackCandidates(itemName) {
		if rendered := _minecraftBlockRenderer.TryRenderEmbeddedTexture(candidate, options, itemName); rendered != nil {
			return rendered
		}
	}

	return nil
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryRenderEmbeddedTexture(textureId string, options BlockRenderOptions, tintContext string) *image.RGBA {
	texture := _minecraftBlockRenderer._textureRepository.GetTexture(textureId)
	if texture == nil || _minecraftBlockRenderer._textureRepository.IsMissingTexture(texture) {
		return nil
	}

	rendered, err := _minecraftBlockRenderer.RenderFlatItem([]string{textureId}, options, tintContext)
	if err != nil {
		return nil
	}
	return &rendered
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ShouldFallbackMissingCustomTexture(itemName string, options BlockRenderOptions) bool {
	fallbackItem := strings.TrimSpace(options.CustomTextureFallbackItem)
	if fallbackItem == "" {
		return false
	}
	if strings.EqualFold(_minecraftBlockRenderer.NormalizeItemTextureKey(itemName), _minecraftBlockRenderer.NormalizeItemTextureKey(fallbackItem)) {
		return false
	}
	return true
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) ModelUsesMissingTexturePlaceholder(model *data.BlockModelInstance) bool {
	if model == nil {
		return false
	}
	for _, textureId := range _minecraftBlockRenderer.CollectResolvedTextures(model) {
		texture := _minecraftBlockRenderer._textureRepository.GetTexture(textureId)
		if texture != nil && _minecraftBlockRenderer._textureRepository.IsMissingTexture(texture) {
			return true
		}
	}
	return false
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) TryRenderCustomTextureFallback(itemName string, options BlockRenderOptions, capture *ItemRenderCapture) *image.RGBA {
	if !_minecraftBlockRenderer.ShouldFallbackMissingCustomTexture(itemName, options) {
		return nil
	}

	fallbackItem := strings.TrimSpace(options.CustomTextureFallbackItem)
	fallbackOptions := options
	fallbackOptions.CustomTextureFallbackItem = ""
	fallbackOptions.PackIds = nil
	if fallbackOptions.CustomTextureFallbackData != nil {
		fallbackOptions.ItemData = fallbackOptions.CustomTextureFallbackData
		fallbackOptions.CustomTextureFallbackData = nil
	}
	fallbackRenderer := _minecraftBlockRenderer
	if _minecraftBlockRenderer._packRegistry != nil && len(_minecraftBlockRenderer._packContext.PackIds) > 0 {
		fallbackRenderer = _minecraftBlockRenderer.GetRendererForPackStack([]string{})
	}

	fmt.Printf("warning: item %q custom texture was not found; falling back to vanilla item %q\n", itemName, fallbackItem)
	return fallbackRenderer.RenderGuiItemInternal(fallbackItem, &fallbackOptions, capture)
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) EnumerateTextureFallbackCandidates(itemName string) []string {
	var candidates []string
	seen := make(map[string]struct{})
	normalized := _minecraftBlockRenderer.NormalizeItemTextureKey(itemName)
	if strings.Contains(normalized, ":") {
		return []string{normalized}
	}

	for _, candidate := range _minecraftBlockRenderer.EnumerateTextureNameVariants(normalized) {
		if _, exists := seen[candidate]; !exists {
			seen[candidate] = struct{}{}
			candidates = append(candidates, candidate)
		}
	}

	if _, isAnimatedDial := AnimatedDialItems[normalized]; isAnimatedDial {
		for _, candidate := range _minecraftBlockRenderer.EnumerateTextureNameVariants(normalized + "_00") {
			if _, exists := seen[candidate]; !exists {
				seen[candidate] = struct{}{}
				candidates = append(candidates, candidate)
			}
		}
	}

	return candidates
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) EnumerateTextureNameVariants(textureKey string) []string {
	var candidates []string
	if strings.Contains(textureKey, ":") {
		candidates = append(candidates, textureKey)
		return candidates
	}
	candidates = append(candidates, textureKey)
	candidates = append(candidates, fmt.Sprintf("minecraft:item/%s", textureKey))
	candidates = append(candidates, fmt.Sprintf("item/%s", textureKey))
	candidates = append(candidates, fmt.Sprintf("textures/item/%s", textureKey))
	candidates = append(candidates, fmt.Sprintf("minecraft:block/%s", textureKey))
	candidates = append(candidates, fmt.Sprintf("block/%s", textureKey))
	return candidates
}

func (_minecraftBlockRenderer *MinecraftBlockRenderer) RenderGuiItemWithResourceId(itemName string, options *BlockRenderOptions) *RenderedResource {
	effectiveOptions := MergeBlockRenderOptions(options)

	rendered, forwardedOptions := _minecraftBlockRenderer.ResolveRendererForOptions(effectiveOptions)
	capture := ItemRenderCapture{}

	// fmt.Printf("Rendering GUI item: %s with options: %+v %+v\n", itemName, forwardedOptions, capture)
	image := rendered.RenderGuiItemInternal(itemName, &forwardedOptions, &capture)
	if image == nil {
		return nil
	}
	resourceTarget := strings.TrimSpace(itemName)
	if strings.TrimSpace(capture.OriginalTarget) != "" {
		resourceTarget = capture.OriginalTarget
	}
	idOptions := forwardedOptions
	if capture.FinalOptions != nil {
		idOptions = *capture.FinalOptions
	}
	resourceId := rendered.ComputeResourceIdInternal(resourceTarget, idOptions, capture.ToResolution())
	return &RenderedResource{
		Image:      image,
		ResourceId: *resourceId,
	}

}
