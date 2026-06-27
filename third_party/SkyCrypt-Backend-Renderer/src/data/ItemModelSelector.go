package data

import (
	"encoding/json"
	"image/color"
	"math"
	"strconv"
	"strings"

	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
)

const maxItemModelSelectorRecursionDepth = 10000

type ItemRenderData struct {
	CustomData               *nbt.NbtCompound
	Profile                  *nbt.NbtCompound
	ItemModel                string
	Layer0Tint               *color.RGBA
	AdditionalLayerTints     map[int]*color.RGBA
	DisableDefaultLayer0Tint bool
	GetLayerTint             func(layerIndex int) (tintColor int, hasTint bool)
	IsDefault                func() bool
}

type ItemModelContext struct {
	ItemData       *ItemRenderData
	DisplayContext string
	ItemName       string
	ItemModel      string
}

type ItemModelSelector interface {
	Resolve(context ItemModelContext) *string
}

type ItemSpecialModelInfo struct {
	Type      string
	BaseModel string
	Texture   string
}

func ResolveAllItemModelSelector(selector ItemModelSelector, context ItemModelContext) []string {
	if selector == nil {
		return nil
	}

	switch typed := selector.(type) {
	case *ItemModelSelectorModel:
		return oneResolvedModel(typed.Resolve(context))
	case *ItemModelSelectorSpecial:
		if nested := ResolveAllItemModelSelector(typed.Nested, context); len(nested) > 0 {
			return nested
		}
		return oneResolvedModel(typed.BaseModel)
	case *ItemModelSelectorComposite:
		var models []string
		seen := make(map[string]struct{})
		for _, child := range typed.Selectors {
			for _, resolved := range ResolveAllItemModelSelector(child, context) {
				if strings.TrimSpace(resolved) == "" {
					continue
				}
				key := strings.ToLower(strings.TrimSpace(resolved))
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				models = append(models, resolved)
			}
		}
		return models
	case *ItemModelSelectorCondition:
		if typed.evaluateCondition(context) {
			return ResolveAllItemModelSelector(typed.OnTrue, context)
		}
		return ResolveAllItemModelSelector(typed.OnFalse, context)
	case *ItemModelSelectorSelect:
		var result []string
		for _, selectCase := range typed.Cases {
			if typed.matches(selectCase.When, context) {
				if resolved := ResolveAllItemModelSelector(selectCase.Selector, context); len(resolved) > 0 {
					result = resolved
					break
				}
			}
		}
		if len(result) == 0 && typed.shouldResolveFirstCaseOnUnsupportedSelector() {
			if len(typed.Cases) > 0 {
				if resolved := ResolveAllItemModelSelector(typed.Cases[0].Selector, context); len(resolved) > 0 {
					result = resolved
				}
			}
		}
		if len(result) == 0 && typed.shouldResolveFirstCaseOnMissingItemModel(context) {
			if len(typed.Cases) > 0 {
				if resolved := ResolveAllItemModelSelector(typed.Cases[0].Selector, context); len(resolved) > 0 {
					result = resolved
				}
			}
		}
		if len(result) == 0 {
			result = ResolveAllItemModelSelector(typed.Fallback, context)
		}
		if len(result) == 0 {
			return oneResolvedModel(typed.Resolve(context))
		}
		return result
	case *ItemModelSelectorRangeDispatch:
		value := typed.getPropertyValue(context)
		var result []string
		if value == nil {
			if typed.shouldResolveFirstEntryOnUnsupportedSelector() && len(typed.Entries) > 0 {
				if resolved := ResolveAllItemModelSelector(typed.Entries[0].Selector, context); len(resolved) > 0 {
					result = resolved
				}
			}
			if len(result) == 0 {
				result = ResolveAllItemModelSelector(typed.Fallback, context)
			}
		} else {
			var matchedEntry *RangeDispatchEntry
			for index := range typed.Entries {
				entry := &typed.Entries[index]
				if *value >= entry.Threshold {
					if matchedEntry == nil || entry.Threshold > matchedEntry.Threshold {
						matchedEntry = entry
					}
				}
			}
			if matchedEntry != nil {
				if resolved := ResolveAllItemModelSelector(matchedEntry.Selector, context); len(resolved) > 0 {
					result = resolved
				}
			}
			if len(result) == 0 {
				result = ResolveAllItemModelSelector(typed.Fallback, context)
			}
		}
		if len(result) == 0 {
			return oneResolvedModel(typed.Resolve(context))
		}
		return result
	case *ItemModelSelectorOptimized:
		return typed.resolveAll(context)
	default:
		return oneResolvedModel(selector.Resolve(context))
	}
}

func ResolveItemModelSelectorSpecialInfo(selector ItemModelSelector, context ItemModelContext) *ItemSpecialModelInfo {
	if selector == nil {
		return nil
	}

	switch typed := selector.(type) {
	case *ItemModelSelectorSpecial:
		if typed.Nested != nil {
			if nested := ResolveItemModelSelectorSpecialInfo(typed.Nested, context); nested != nil {
				return nested
			}
		}
		return typed.Special
	case *ItemModelSelectorComposite:
		for _, child := range typed.Selectors {
			if info := ResolveItemModelSelectorSpecialInfo(child, context); info != nil {
				return info
			}
		}
		return nil
	case *ItemModelSelectorCondition:
		if typed.evaluateCondition(context) {
			return ResolveItemModelSelectorSpecialInfo(typed.OnTrue, context)
		}
		return ResolveItemModelSelectorSpecialInfo(typed.OnFalse, context)
	case *ItemModelSelectorSelect:
		for _, selectCase := range typed.Cases {
			if !typed.matches(selectCase.When, context) || selectCase.Selector == nil {
				continue
			}
			if info := ResolveItemModelSelectorSpecialInfo(selectCase.Selector, context); info != nil {
				return info
			}
			if resolved := selectCase.Selector.Resolve(context); resolved != nil && strings.TrimSpace(*resolved) != "" {
				return nil
			}
		}
		if typed.shouldResolveFirstCaseOnUnsupportedSelector() || typed.shouldResolveFirstCaseOnMissingItemModel(context) {
			if len(typed.Cases) > 0 {
				return ResolveItemModelSelectorSpecialInfo(typed.Cases[0].Selector, context)
			}
		}
		return ResolveItemModelSelectorSpecialInfo(typed.Fallback, context)
	case *ItemModelSelectorRangeDispatch:
		value := typed.getPropertyValue(context)
		if value == nil {
			if typed.shouldResolveFirstEntryOnUnsupportedSelector() && len(typed.Entries) > 0 {
				return ResolveItemModelSelectorSpecialInfo(typed.Entries[0].Selector, context)
			}
			return ResolveItemModelSelectorSpecialInfo(typed.Fallback, context)
		}

		var matchedEntry *RangeDispatchEntry
		for index := range typed.Entries {
			entry := &typed.Entries[index]
			if *value >= entry.Threshold {
				if matchedEntry == nil || entry.Threshold > matchedEntry.Threshold {
					matchedEntry = entry
				}
			}
		}
		if matchedEntry != nil {
			return ResolveItemModelSelectorSpecialInfo(matchedEntry.Selector, context)
		}
		return ResolveItemModelSelectorSpecialInfo(typed.Fallback, context)
	case *ItemModelSelectorOptimized:
		return typed.resolveSpecialInfo(context)
	default:
		return nil
	}
}

func oneResolvedModel(model *string) []string {
	if model == nil || strings.TrimSpace(*model) == "" {
		return nil
	}
	return []string{*model}
}

type catharsisDataTypeResolver struct{}

func (catharsisDataTypeResolver) SupportsSelectValue(dataType string) bool {
	normalized := normalizeDataType(dataType)
	return normalized == "rarity" || normalized == "modifier"
}

func (catharsisDataTypeResolver) SupportsNumericValue(dataType string) bool {
	switch normalizeDataType(dataType) {
	case "midas_weapon_paid":
		return true
	default:
		return false
	}
}

func (catharsisDataTypeResolver) EvaluateCondition(dataType string, context ItemModelContext) bool {
	_ = context
	switch normalizeDataType(dataType) {
	case "has_skin_fallback":
		return false
	default:
		return false
	}
}

func (catharsisDataTypeResolver) GetSelectValue(dataType string, context ItemModelContext) *string {
	if context.ItemData == nil || context.ItemData.CustomData == nil {
		return nil
	}

	resolver := catharsisDataTypeResolver{}
	if !resolver.SupportsSelectValue(dataType) {
		return nil
	}

	switch normalizeDataType(dataType) {
	case "rarity":
		return normalizeStringValue(getFirstString(context.ItemData.CustomData, "upgradedRarity", "rarity", "tier"))
	case "modifier":
		return normalizeStringValue(getFirstString(context.ItemData.CustomData, "modifier", "reforge", "prefix"))
	default:
		return nil
	}
}

func (catharsisDataTypeResolver) GetNumericValue(dataType string, context ItemModelContext) *float64 {
	resolver := catharsisDataTypeResolver{}
	if !resolver.SupportsNumericValue(dataType) {
		return nil
	}
	if context.ItemData == nil || context.ItemData.CustomData == nil {
		return nil
	}
	switch normalizeDataType(dataType) {
	case "midas_weapon_paid":
		return getFirstNumeric(context.ItemData.CustomData, "winning_bid", "paid", "price", "midas_paid")
	default:
		return nil
	}
}

func normalizeDataType(dataType string) string {
	if strings.TrimSpace(dataType) == "" {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(dataType))
}

func normalizeStringValue(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	v := strings.ToLower(strings.TrimSpace(*value))
	return &v
}

func getFirstString(compound *nbt.NbtCompound, keys ...string) *string {
	if compound == nil {
		return nil
	}

	for _, key := range keys {
		if tag, ok := compound.Get(key); ok {
			if value, okString := nbtTagToString(tag); okString && strings.TrimSpace(value) != "" {
				out := value
				return &out
			}
		}
	}

	return nil
}

func getFirstNumeric(compound *nbt.NbtCompound, keys ...string) *float64 {
	if compound == nil {
		return nil
	}
	for _, key := range keys {
		if tag, ok := compound.Get(key); ok {
			if value, okNumber := nbtTagToFloat64(tag); okNumber {
				return &value
			}
			if text, okString := nbtTagToString(tag); okString {
				if parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil {
					return &parsed
				}
			}
		}
	}
	return nil
}

type ItemModelSelectorModel struct {
	Model     *string
	BaseModel *string
}

func NewItemModelSelectorModel(model string, baseModel string) *ItemModelSelectorModel {
	return &ItemModelSelectorModel{
		Model:     trimToPointer(model),
		BaseModel: trimToPointer(baseModel),
	}
}

func (s *ItemModelSelectorModel) Resolve(context ItemModelContext) *string {
	_ = context
	if s == nil {
		return nil
	}
	if s.Model != nil {
		return s.Model
	}
	return s.BaseModel
}

type ItemModelSelectorSpecial struct {
	BaseModel *string
	Nested    ItemModelSelector
	Special   *ItemSpecialModelInfo
}

func NewItemModelSelectorSpecial(baseModel string, nested ItemModelSelector) *ItemModelSelectorSpecial {
	return NewItemModelSelectorSpecialWithInfo(baseModel, nested, nil)
}

func NewItemModelSelectorSpecialWithInfo(baseModel string, nested ItemModelSelector, special *ItemSpecialModelInfo) *ItemModelSelectorSpecial {
	return &ItemModelSelectorSpecial{
		BaseModel: trimToPointer(baseModel),
		Nested:    nested,
		Special:   special,
	}
}

func (s *ItemModelSelectorSpecial) Resolve(context ItemModelContext) *string {
	if s == nil {
		return nil
	}
	if s.Nested != nil {
		if resolved := s.Nested.Resolve(context); resolved != nil && strings.TrimSpace(*resolved) != "" {
			return resolved
		}
	}
	return s.BaseModel
}

type ItemModelSelectorComposite struct {
	Selectors []ItemModelSelector
}

func (s *ItemModelSelectorComposite) Resolve(context ItemModelContext) *string {
	resolved := ResolveAllItemModelSelector(s, context)
	if len(resolved) == 0 {
		return nil
	}
	return &resolved[0]
}

type ItemModelSelectorCondition struct {
	Property        string
	DataType        *string
	Predicate       *string
	Component       *string
	ValueProperties map[string]string
	ValueLiteral    *string
	OnTrue          ItemModelSelector
	OnFalse         ItemModelSelector
}

func (s *ItemModelSelectorCondition) Resolve(context ItemModelContext) *string {
	if s == nil {
		return nil
	}
	if s.evaluateCondition(context) {
		if s.OnTrue == nil {
			return nil
		}
		return s.OnTrue.Resolve(context)
	}
	if s.OnFalse == nil {
		return nil
	}
	return s.OnFalse.Resolve(context)
}

func (s *ItemModelSelectorCondition) evaluateCondition(context ItemModelContext) bool {
	if strings.EqualFold(s.Property, "catharsis:data_type") {
		return catharsisDataTypeResolver{}.EvaluateCondition(pointerString(s.DataType), context)
	}

	if strings.EqualFold(s.Property, "component") {
		return s.evaluateComponentCondition(context)
	}

	if strings.EqualFold(s.Property, "display_context") {
		if len(s.ValueProperties) > 0 {
			if expected, ok := s.ValueProperties["value"]; ok {
				return strings.EqualFold(expected, context.DisplayContext)
			}
			if expected, ok := s.ValueProperties["equals"]; ok {
				return strings.EqualFold(expected, context.DisplayContext)
			}
		}

		if s.ValueLiteral != nil && strings.TrimSpace(*s.ValueLiteral) != "" {
			return strings.EqualFold(*s.ValueLiteral, context.DisplayContext)
		}

		return false
	}

	if strings.EqualFold(s.Property, "selected") {
		return false
	}

	return false
}

func (s *ItemModelSelectorCondition) evaluateComponentCondition(context ItemModelContext) bool {
	if strings.EqualFold(pointerString(s.Predicate), "custom_data") {
		return s.evaluateCustomData(context)
	}
	return false
}

func (s *ItemModelSelectorCondition) evaluateCustomData(context ItemModelContext) bool {
	if context.ItemData == nil || context.ItemData.CustomData == nil {
		return false
	}

	customData := context.ItemData.CustomData
	if len(s.ValueProperties) > 0 {
		for key, expected := range s.ValueProperties {
			if !TryMatchCustomDataValue(customData, key, expected) {
				return false
			}
		}
		return true
	}

	if s.ValueLiteral != nil && strings.TrimSpace(*s.ValueLiteral) != "" {
		if id := tryGetString(customData, "id"); id != nil && strings.TrimSpace(*id) != "" {
			return strings.EqualFold(*id, *s.ValueLiteral)
		}
	}

	return false
}

func TryMatchCustomDataValue(compound *nbt.NbtCompound, key string, expected string) bool {
	if compound == nil {
		return false
	}

	tag, ok := compound.Get(key)
	if !ok {
		return false
	}

	if isJSONStructure(expected) {
		return tryMatchJSONStructure(tag, expected)
	}

	return matchesPrimitiveValue(tag, expected)
}

func tryGetString(compound *nbt.NbtCompound, key string) *string {
	if compound == nil {
		return nil
	}
	tag, ok := compound.Get(key)
	if !ok {
		return nil
	}
	if value, okString := nbtTagToString(tag); okString && strings.TrimSpace(value) != "" {
		out := value
		return &out
	}
	return nil
}

func matchesPrimitiveValue(tag nbt.NbtTag, expected string) bool {
	if expectedBool, ok := tryParseBoolean(expected); ok {
		if actualBool, okBool := nbtTagToBool(tag); okBool {
			return actualBool == expectedBool
		}
		return false
	}

	if actualString, okString := nbtTagToString(tag); okString {
		return actualString == expected
	}

	if actualNumber, okNumber := nbtTagToFloat64(tag); okNumber {
		if expectedNumber, err := strconv.ParseFloat(expected, 64); err == nil {
			return numericEquals(actualNumber, expectedNumber)
		}
	}

	return false
}

func isJSONStructure(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func tryMatchJSONStructure(tag nbt.NbtTag, jsonText string) bool {
	var expected any
	if err := json.Unmarshal([]byte(jsonText), &expected); err != nil {
		return false
	}
	return matchTagWithJSON(tag, expected)
}

func matchTagWithJSON(tag nbt.NbtTag, expected any) bool {
	switch value := expected.(type) {
	case map[string]any:
		compound, ok := tag.(*nbt.NbtCompound)
		if !ok {
			if compoundValue, okCompound := nbtTagToCompound(tag); okCompound {
				copyCompound := compoundValue
				compound = &copyCompound
			} else {
				return false
			}
		}
		return matchCompound(compound, value)
	case []any:
		return matchArray(tag, value)
	case string:
		if str, ok := nbtTagToString(tag); ok {
			return str == value
		}
		return false
	case float64:
		return matchNumeric(tag, value)
	case bool:
		if actual, ok := nbtTagToBool(tag); ok {
			return actual == value
		}
		return false
	case nil:
		return false
	default:
		return false
	}
}

func nbtTagToCompound(tag nbt.NbtTag) (nbt.NbtCompound, bool) {
	if compound, ok := tag.(*nbt.NbtCompound); ok {
		return *compound, true
	}
	return nbt.NbtCompound{}, false
}

func matchCompound(compound *nbt.NbtCompound, expected map[string]any) bool {
	if compound == nil {
		return false
	}

	for key, expectedValue := range expected {
		child, ok := compound.Get(key)
		if !ok {
			return false
		}
		if !matchTagWithJSON(child, expectedValue) {
			return false
		}
	}

	return true
}

func matchArray(tag nbt.NbtTag, expected []any) bool {
	switch actual := tag.(type) {
	case *nbt.NbtList:
		return matchList(actual, expected)
	case nbt.NbtByteArray:
		values := make([]float64, 0, len(actual.Values))
		for _, v := range actual.Values {
			values = append(values, float64(v))
		}
		return matchPrimitiveArray(values, expected)
	case *nbt.NbtByteArray:
		values := make([]float64, 0, len(actual.Values))
		for _, v := range actual.Values {
			values = append(values, float64(v))
		}
		return matchPrimitiveArray(values, expected)
	case nbt.NbtIntArray:
		values := make([]float64, 0, len(actual.Values))
		for _, v := range actual.Values {
			values = append(values, float64(v))
		}
		return matchPrimitiveArray(values, expected)
	case *nbt.NbtIntArray:
		values := make([]float64, 0, len(actual.Values))
		for _, v := range actual.Values {
			values = append(values, float64(v))
		}
		return matchPrimitiveArray(values, expected)
	case nbt.NbtLongArray:
		values := make([]float64, 0, len(actual.Values))
		for _, v := range actual.Values {
			values = append(values, float64(v))
		}
		return matchPrimitiveArray(values, expected)
	case *nbt.NbtLongArray:
		values := make([]float64, 0, len(actual.Values))
		for _, v := range actual.Values {
			values = append(values, float64(v))
		}
		return matchPrimitiveArray(values, expected)
	default:
		return false
	}
}

func matchList(list *nbt.NbtList, expected []any) bool {
	if list == nil || list.Count() != len(expected) {
		return false
	}
	for index, expectedElement := range expected {
		if !matchTagWithJSON(list.At(index), expectedElement) {
			return false
		}
	}
	return true
}

func matchPrimitiveArray(actual []float64, expected []any) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i, expectedElement := range expected {
		expectedValue, ok := expectedElement.(float64)
		if !ok {
			return false
		}
		if !numericEquals(actual[i], expectedValue) {
			return false
		}
	}
	return true
}

func matchNumeric(tag nbt.NbtTag, expected float64) bool {
	if actual, ok := nbtTagToFloat64(tag); ok {
		return numericEquals(actual, expected)
	}
	return false
}

func numericEquals(actual float64, expected float64) bool {
	return math.Abs(actual-expected) < 1e-6
}

func tryParseBoolean(value string) (bool, bool) {
	if strings.EqualFold(value, "true") {
		return true, true
	}
	if strings.EqualFold(value, "false") {
		return false, true
	}
	return false, false
}

type ItemModelSelectorSelectCase struct {
	When     []string
	Selector ItemModelSelector
}

type ItemModelSelectorSelect struct {
	Property  string
	DataType  *string
	Component *string
	Cases     []ItemModelSelectorSelectCase
	Fallback  ItemModelSelector
}

func (s *ItemModelSelectorSelect) Resolve(context ItemModelContext) *string {
	if s == nil {
		return nil
	}

	for _, selectCase := range s.Cases {
		if s.matches(selectCase.When, context) {
			if selectCase.Selector == nil {
				continue
			}
			if resolved := selectCase.Selector.Resolve(context); resolved != nil && strings.TrimSpace(*resolved) != "" {
				return resolved
			}
		}
	}

	if s.shouldResolveFirstCaseOnUnsupportedSelector() {
		if firstResolved := s.resolveFirstCase(context); firstResolved != nil && strings.TrimSpace(*firstResolved) != "" {
			return firstResolved
		}
	}

	if s.shouldResolveFirstCaseOnMissingItemModel(context) {
		if firstResolved := s.resolveFirstCase(context); firstResolved != nil && strings.TrimSpace(*firstResolved) != "" {
			return firstResolved
		}
	}

	if s.Fallback == nil {
		return nil
	}
	return s.Fallback.Resolve(context)
}

func (s *ItemModelSelectorSelect) shouldResolveFirstCaseOnUnsupportedSelector() bool {
	if strings.EqualFold(s.Property, "catharsis:data_type") {
		return !catharsisDataTypeResolver{}.SupportsSelectValue(pointerString(s.DataType))
	}
	return false
}

func (s *ItemModelSelectorSelect) shouldResolveFirstCaseOnMissingItemModel(context ItemModelContext) bool {
	if s == nil || len(s.Cases) == 0 {
		return false
	}
	if !strings.EqualFold(s.Property, "component") || !isItemModelComponent(pointerString(s.Component)) {
		return false
	}
	return strings.TrimSpace(contextItemModel(context)) == ""
}

func (s *ItemModelSelectorSelect) resolveFirstCase(context ItemModelContext) *string {
	if len(s.Cases) == 0 || s.Cases[0].Selector == nil {
		return nil
	}
	return s.Cases[0].Selector.Resolve(context)
}

func (s *ItemModelSelectorSelect) matches(when []string, context ItemModelContext) bool {
	if len(when) == 0 {
		return false
	}

	if strings.EqualFold(s.Property, "catharsis:data_type") {
		value := catharsisDataTypeResolver{}.GetSelectValue(pointerString(s.DataType), context)
		if value == nil || strings.TrimSpace(*value) == "" {
			return false
		}
		for _, candidate := range when {
			if strings.EqualFold(candidate, *value) {
				return true
			}
		}
		return false
	}

	if strings.EqualFold(s.Property, "display_context") {
		for _, value := range when {
			if strings.EqualFold(value, context.DisplayContext) {
				return true
			}
		}
		return false
	}

	if strings.EqualFold(s.Property, "component") {
		for _, value := range when {
			if matchesComponentValue(pointerString(s.Component), value, context) {
				return true
			}
		}
		return false
	}

	return false
}

func matchesComponentValue(component string, value string, context ItemModelContext) bool {
	if strings.TrimSpace(component) == "" && strings.TrimSpace(value) == "" {
		return false
	}

	itemData := context.ItemData

	if isItemModelComponent(component) {
		if strings.TrimSpace(value) == "" {
			return false
		}
		if itemModel := contextItemModel(context); itemModel != "" && itemModelMatches(value, itemModel) {
			return true
		}
		return itemModelMatches(value, context.ItemName)
	}

	if itemData == nil {
		return false
	}

	if strings.EqualFold(value, "minecraft:custom_data") || strings.EqualFold(value, "custom_data") {
		return itemData.CustomData != nil
	}

	if strings.EqualFold(value, "minecraft:profile") || strings.EqualFold(value, "profile") {
		return itemData.Profile != nil
	}

	if strings.EqualFold(value, "minecraft:dyed_color") || strings.EqualFold(value, "dyed_color") {
		return itemData.Layer0Tint != nil || len(itemData.AdditionalLayerTints) > 0 || itemData.DisableDefaultLayer0Tint
	}

	return false
}

func isItemModelComponent(component string) bool {
	return strings.EqualFold(component, "item_model") || strings.EqualFold(component, "minecraft:item_model")
}

func contextItemModel(context ItemModelContext) string {
	if strings.TrimSpace(context.ItemModel) != "" {
		return strings.TrimSpace(context.ItemModel)
	}
	if context.ItemData != nil && strings.TrimSpace(context.ItemData.ItemModel) != "" {
		return strings.TrimSpace(context.ItemData.ItemModel)
	}
	return ""
}

func itemModelMatches(expected string, actual string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	actual = strings.ToLower(strings.TrimSpace(actual))
	if expected == "" || actual == "" {
		return false
	}
	if expected == actual {
		return true
	}
	if !strings.Contains(actual, ":") && expected == "minecraft:"+actual {
		return true
	}
	if !strings.Contains(expected, ":") && actual == "minecraft:"+expected {
		return true
	}
	return false
}

type ItemModelSelectorEmpty struct{}

func (s *ItemModelSelectorEmpty) Resolve(context ItemModelContext) *string {
	_ = context
	return nil
}

type RangeDispatchEntry struct {
	Threshold float64
	Selector  ItemModelSelector
}

type ItemModelSelectorRangeDispatch struct {
	Property  string
	DataType  *string
	Normalize bool
	Entries   []RangeDispatchEntry
	Fallback  ItemModelSelector
}

func (s *ItemModelSelectorRangeDispatch) Resolve(context ItemModelContext) *string {
	if s == nil {
		return nil
	}

	value := s.getPropertyValue(context)
	if value == nil {
		if strings.EqualFold(s.Property, "catharsis:data_type") || s.shouldResolveFirstEntryOnUnsupportedSelector() {
			if firstResolved := s.resolveFirstEntry(context); firstResolved != nil && strings.TrimSpace(*firstResolved) != "" {
				return firstResolved
			}
		}
		if s.Fallback == nil {
			return nil
		}
		return s.Fallback.Resolve(context)
	}

	var matchedEntry *RangeDispatchEntry
	for index := range s.Entries {
		entry := &s.Entries[index]
		if *value >= entry.Threshold {
			if matchedEntry == nil || entry.Threshold > matchedEntry.Threshold {
				matchedEntry = entry
			}
		}
	}

	if matchedEntry != nil && matchedEntry.Selector != nil {
		if resolved := matchedEntry.Selector.Resolve(context); resolved != nil && strings.TrimSpace(*resolved) != "" {
			return resolved
		}
	}

	if s.Fallback == nil {
		return nil
	}
	return s.Fallback.Resolve(context)
}

func (s *ItemModelSelectorRangeDispatch) shouldResolveFirstEntryOnUnsupportedSelector() bool {
	if strings.EqualFold(s.Property, "catharsis:data_type") {
		return !catharsisDataTypeResolver{}.SupportsNumericValue(pointerString(s.DataType))
	}
	return false
}

func (s *ItemModelSelectorRangeDispatch) resolveFirstEntry(context ItemModelContext) *string {
	if len(s.Entries) == 0 || s.Entries[0].Selector == nil {
		return nil
	}
	return s.Entries[0].Selector.Resolve(context)
}

func (s *ItemModelSelectorRangeDispatch) getPropertyValue(context ItemModelContext) *float64 {
	if strings.EqualFold(s.Property, "catharsis:data_type") {
		return catharsisDataTypeResolver{}.GetNumericValue(pointerString(s.DataType), context)
	}

	if strings.EqualFold(s.Property, "count") {
		value := 1.0
		return &value
	}

	return nil
}

type CustomDataCompositeMapping struct {
	ExpectedValues map[string]string
	Model          *string
	Selector       ItemModelSelector
}

type ItemModelSelectorOptimized struct {
	customDataIDToModel    map[string]string
	customDataIDToSelector map[string]ItemModelSelector
	fallbackSelector       ItemModelSelector
	compositeMappings      []CustomDataCompositeMapping
}

func NewItemModelSelectorOptimized(
	customDataIDToModel map[string]string,
	customDataIDToSelector map[string]ItemModelSelector,
	compositeMappings []CustomDataCompositeMapping,
	fallbackSelector ItemModelSelector,
) *ItemModelSelectorOptimized {
	return &ItemModelSelectorOptimized{
		customDataIDToModel:    customDataIDToModel,
		customDataIDToSelector: customDataIDToSelector,
		compositeMappings:      compositeMappings,
		fallbackSelector:       fallbackSelector,
	}
}

func (s *ItemModelSelectorOptimized) Resolve(context ItemModelContext) *string {
	if s == nil {
		return nil
	}

	resolved := s.resolveAll(context)
	if len(resolved) > 0 {
		return &resolved[0]
	}

	return nil
}

func (s *ItemModelSelectorOptimized) resolveAll(context ItemModelContext) []string {
	if s == nil {
		return nil
	}

	if context.ItemData != nil && context.ItemData.CustomData != nil {
		customData := context.ItemData.CustomData
		customDataKey := ""

		if id := tryGetString(customData, "id"); id != nil && strings.TrimSpace(*id) != "" {
			customDataKey = *id
		} else if model := tryGetString(customData, "model"); model != nil && strings.TrimSpace(*model) != "" {
			customDataKey = *model
		}

		if customDataKey != "" {
			if model, ok := s.customDataIDToModel[customDataKey]; ok {
				return oneResolvedModel(&model)
			}

			if selector, ok := s.customDataIDToSelector[customDataKey]; ok && selector != nil {
				return ResolveAllItemModelSelector(selector, context)
			}
		}
	}

	if context.ItemData != nil && context.ItemData.CustomData != nil && len(s.compositeMappings) > 0 {
		compositeCustomData := context.ItemData.CustomData
		for _, mapping := range s.compositeMappings {
			if !matchesComposite(compositeCustomData, mapping.ExpectedValues) {
				continue
			}

			if mapping.Selector != nil {
				return ResolveAllItemModelSelector(mapping.Selector, context)
			}

			if mapping.Model != nil && strings.TrimSpace(*mapping.Model) != "" {
				return oneResolvedModel(mapping.Model)
			}
		}
	}

	if s.fallbackSelector == nil {
		return nil
	}

	return ResolveAllItemModelSelector(s.fallbackSelector, context)
}

func (s *ItemModelSelectorOptimized) CustomDataMappingCount() int {
	if s == nil {
		return 0
	}
	return len(s.customDataIDToModel) + len(s.customDataIDToSelector) + len(s.compositeMappings)
}

func (s *ItemModelSelectorOptimized) resolveSpecialInfo(context ItemModelContext) *ItemSpecialModelInfo {
	if s == nil {
		return nil
	}

	if context.ItemData != nil && context.ItemData.CustomData != nil {
		customData := context.ItemData.CustomData
		customDataKey := ""

		if id := tryGetString(customData, "id"); id != nil && strings.TrimSpace(*id) != "" {
			customDataKey = *id
		} else if model := tryGetString(customData, "model"); model != nil && strings.TrimSpace(*model) != "" {
			customDataKey = *model
		}

		if customDataKey != "" {
			if selector, ok := s.customDataIDToSelector[customDataKey]; ok && selector != nil {
				return ResolveItemModelSelectorSpecialInfo(selector, context)
			}
			if _, ok := s.customDataIDToModel[customDataKey]; ok {
				return nil
			}
		}
	}

	if context.ItemData != nil && context.ItemData.CustomData != nil && len(s.compositeMappings) > 0 {
		compositeCustomData := context.ItemData.CustomData
		for _, mapping := range s.compositeMappings {
			if !matchesComposite(compositeCustomData, mapping.ExpectedValues) {
				continue
			}
			if mapping.Selector != nil {
				return ResolveItemModelSelectorSpecialInfo(mapping.Selector, context)
			}
			return nil
		}
	}

	return ResolveItemModelSelectorSpecialInfo(s.fallbackSelector, context)
}

func matchesComposite(customData *nbt.NbtCompound, expected map[string]string) bool {
	for key, value := range expected {
		if !TryMatchCustomDataValue(customData, key, value) {
			return false
		}
	}
	return true
}

type customDataExtractionResult struct {
	fallbackModel               any
	encounteredUnsupportedValue bool
}

func ParseItemModelSelectorFromJSON(data []byte) (ItemModelSelector, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	return ParseItemModelSelectorFromRoot(root), nil
}

func ParseItemModelSelectorFromRoot(root any) ItemModelSelector {
	obj, ok := root.(map[string]any)
	if !ok {
		return nil
	}

	if modelElement, okModel := obj["model"]; okModel {
		if optimized := tryOptimizeCustomDataSelector(modelElement); optimized != nil {
			return optimized
		}

		selector := parseItemModelSelector(modelElement, 0)
		if selector != nil {
			return selector
		}
	}

	if components, okComponents := obj["components"].(map[string]any); okComponents {
		if componentModel, okComponentModel := components["minecraft:model"]; okComponentModel {
			selector := parseItemModelSelector(componentModel, 0)
			if selector != nil {
				return selector
			}
		}
	}

	if typeProperty, okType := obj["type"].(string); okType && strings.TrimSpace(typeProperty) != "" {
		selector := parseItemModelSelector(obj, 0)
		if selector != nil {
			return selector
		}
	}

	_, hasCases := obj["cases"]
	_, hasOnTrue := obj["on_true"]
	_, hasOnFalse := obj["on_false"]
	if hasCases || hasOnTrue || hasOnFalse {
		selector := parseItemModelSelector(obj, 0)
		if selector != nil {
			return selector
		}
	}

	return nil
}

func tryOptimizeCustomDataSelector(element any) ItemModelSelector {
	if !isDeepCustomDataConditional(element) {
		return nil
	}

	modelMappings := make(map[string]string)
	selectorMappings := make(map[string]ItemModelSelector)
	compositeMappings := make([]CustomDataCompositeMapping, 0)
	result := extractCustomDataMappings(element, modelMappings, selectorMappings, &compositeMappings, 0, 100000)

	if result.encounteredUnsupportedValue || (len(modelMappings) == 0 && len(selectorMappings) == 0) {
		return nil
	}

	var fallbackSelector ItemModelSelector
	if result.fallbackModel != nil {
		if model, ok := result.fallbackModel.(string); ok {
			if strings.TrimSpace(model) != "" {
				fallbackSelector = NewItemModelSelectorModel(model, "")
			}
		} else {
			fallbackSelector = parseItemModelSelector(result.fallbackModel, 0)
		}
	}

	return NewItemModelSelectorOptimized(modelMappings, selectorMappings, compositeMappings, fallbackSelector)
}

func isDeepCustomDataConditional(element any) bool {
	current, ok := element.(map[string]any)
	if !ok {
		return false
	}

	if fallback, hasFallback := current["fallback"]; hasFallback {
		if fallbackObj, okFallbackObj := fallback.(map[string]any); okFallbackObj {
			current = fallbackObj
		}
	}

	customDataCount := 0
	depth := 0

	for i := 0; i < 20; i++ {
		typeValue, typeOk := current["type"].(string)
		if !typeOk || typeValue != "condition" {
			break
		}

		property, propertyOk := current["property"].(string)
		predicate, predicateOk := current["predicate"].(string)
		if propertyOk && predicateOk && property == "component" && predicate == "custom_data" {
			customDataCount++
		}

		depth++
		onFalse, hasOnFalse := current["on_false"].(map[string]any)
		if !hasOnFalse {
			break
		}
		current = onFalse
	}

	_ = depth
	return customDataCount >= 15
}

func extractCustomDataMappings(
	root any,
	modelMappings map[string]string,
	selectorMappings map[string]ItemModelSelector,
	compositeMappings *[]CustomDataCompositeMapping,
	startDepth int,
	maxDepth int,
) customDataExtractionResult {
	startElement := root
	if obj, ok := root.(map[string]any); ok {
		if fallback, hasFallback := obj["fallback"]; hasFallback {
			startElement = fallback
		}
	}

	type queueEntry struct {
		element any
		depth   int
	}

	queue := []queueEntry{{element: startElement, depth: startDepth}}
	var fallbackModel any
	encounteredUnsupportedValue := false

	for len(queue) > 0 {
		entry := queue[0]
		queue = queue[1:]

		if entry.depth > maxDepth {
			continue
		}

		switch current := entry.element.(type) {
		case string:
			fallbackModel = current
			continue
		case map[string]any:
			typeValue, typeOk := current["type"].(string)
			property, propertyOk := current["property"].(string)
			predicate, predicateOk := current["predicate"].(string)

			if typeOk && propertyOk && predicateOk && typeValue == "condition" && property == "component" && predicate == "custom_data" {
				customDataID := ""
				var compositeExpected map[string]string
				supportedKeyFound := false

				if value, hasValue := current["value"]; hasValue {
					switch valueTyped := value.(type) {
					case string:
						customDataID = valueTyped
						supportedKeyFound = strings.TrimSpace(customDataID) != ""
					case map[string]any:
						if id, okID := valueTyped["id"].(string); okID {
							customDataID = id
							supportedKeyFound = true
						} else if model, okModel := valueTyped["model"].(string); okModel {
							customDataID = model
							supportedKeyFound = true
						}

						extracted := make(map[string]string)
						for propertyName, propertyValue := range valueTyped {
							if propertyString, okProp := propertyValue.(string); okProp {
								extracted[propertyName] = propertyString
							}
						}

						if len(extracted) > 0 {
							if strings.TrimSpace(customDataID) != "" {
								delete(extracted, "id")
								delete(extracted, "model")
							}
							if len(extracted) > 0 {
								compositeExpected = extracted
								supportedKeyFound = true
							}
						}
					}
				}

				if !supportedKeyFound {
					encounteredUnsupportedValue = true
				}

				var selector ItemModelSelector
				model := ""

				if onTrue, hasOnTrue := current["on_true"]; hasOnTrue {
					model = extractModelFromElement(onTrue)
					if strings.TrimSpace(model) == "" {
						selector = parseItemModelSelector(onTrue, 0)
					}
				}

				if strings.TrimSpace(customDataID) != "" {
					if strings.TrimSpace(model) != "" {
						modelMappings[customDataID] = model
					} else if selector != nil {
						selectorMappings[customDataID] = selector
					}
				}

				if compositeExpected != nil && (strings.TrimSpace(model) != "" || selector != nil) {
					var modelPtr *string
					if strings.TrimSpace(model) != "" {
						modelCopy := model
						modelPtr = &modelCopy
					}
					*compositeMappings = append(*compositeMappings, CustomDataCompositeMapping{
						ExpectedValues: compositeExpected,
						Model:          modelPtr,
						Selector:       selector,
					})
				}

				if onFalse, hasOnFalse := current["on_false"]; hasOnFalse {
					queue = append(queue, queueEntry{element: onFalse, depth: entry.depth + 1})
				}
			} else {
				fallbackModel = current
			}
		}
	}

	return customDataExtractionResult{
		fallbackModel:               fallbackModel,
		encounteredUnsupportedValue: encounteredUnsupportedValue,
	}
}

func extractModelFromElement(element any) string {
	if modelString, ok := element.(string); ok {
		return modelString
	}

	obj, ok := element.(map[string]any)
	if !ok {
		return ""
	}

	if model, okModel := obj["model"].(string); okModel {
		return model
	}

	if typeValue, okType := obj["type"].(string); okType {
		if typeValue == "model" {
			if model, okModel := obj["model"].(string); okModel {
				return model
			}
		}
	}

	return ""
}

func parseItemModelSelector(element any, depth int) ItemModelSelector {
	if depth > maxItemModelSelectorRecursionDepth {
		return nil
	}

	for {
		if value, ok := element.(string); ok {
			return NewItemModelSelectorModel(value, "")
		}

		obj, ok := element.(map[string]any)
		if !ok {
			return nil
		}

		if _, hasType := obj["type"]; !hasType {
			_, hasCases := obj["cases"]
			_, hasEntries := obj["entries"]
			_, hasModel := obj["model"]
			if !hasCases && !hasEntries && !hasModel {
				if onFalse, hasOnFalse := obj["on_false"]; hasOnFalse {
					element = onFalse
					depth++
					continue
				}
				if onTrue, hasOnTrue := obj["on_true"]; hasOnTrue {
					element = onTrue
					depth++
					continue
				}
			}
		}

		selectorType := determineSelectorType(obj)
		switch selectorType {
		case "model":
			return NewItemModelSelectorModel(getString(obj, "model"), getString(obj, "base"))
		case "special":
			var nested ItemModelSelector
			if nestedElement, okNested := obj["model"]; okNested {
				nested = parseItemModelSelector(nestedElement, depth+1)
			}
			base := getString(obj, "base")
			return NewItemModelSelectorSpecialWithInfo(base, nested, parseItemSpecialModelInfo(base, obj["model"]))
		case "condition":
			return parseCondition(obj, depth+1)
		case "select":
			return parseSelect(obj, depth+1)
		case "range_dispatch":
			return parseRangeDispatch(obj, depth+1)
		case "composite":
			return parseComposite(obj, depth+1)
		case "empty":
			return &ItemModelSelectorEmpty{}
		default:
			return createFallbackSelector(obj)
		}
	}
}

func determineSelectorType(element map[string]any) string {
	if typeProperty, ok := element["type"].(string); ok {
		return normalizeSelectorType(typeProperty)
	}

	if _, ok := element["cases"].([]any); ok {
		return "select"
	}

	if _, ok := element["entries"].([]any); ok {
		return "range_dispatch"
	}

	if _, ok := element["models"].([]any); ok {
		return "composite"
	}

	_, hasOnTrue := element["on_true"]
	_, hasOnFalse := element["on_false"]
	_, hasProperty := element["property"]
	if (hasOnTrue || hasOnFalse) && hasProperty {
		return "condition"
	}

	if modelObj, okModelObj := element["model"].(map[string]any); okModelObj {
		return determineSelectorType(modelObj)
	}

	return "model"
}

func parseCondition(element map[string]any, depth int) ItemModelSelector {
	property := getString(element, "property")
	dataType := trimToPointer(getString(element, "data_type"))
	predicate := trimToPointer(getString(element, "predicate"))
	component := trimToPointer(getString(element, "component"))

	var valueProperties map[string]string
	var valueLiteral *string
	if valueElement, okValue := element["value"]; okValue {
		valueProperties = parseStringMap(valueElement)
		if valueProperties == nil {
			switch typed := valueElement.(type) {
			case string:
				valueLiteral = trimToPointer(typed)
			default:
				if raw, err := json.Marshal(typed); err == nil {
					valueLiteral = trimToPointer(string(raw))
				}
			}
		}
	}

	var onTrue ItemModelSelector
	if onTrueElement, okOnTrue := element["on_true"]; okOnTrue {
		onTrue = parseItemModelSelector(onTrueElement, depth+1)
	}

	var onFalse ItemModelSelector
	if onFalseElement, okOnFalse := element["on_false"]; okOnFalse {
		onFalse = parseItemModelSelector(onFalseElement, depth+1)
	}

	if onTrue == nil && onFalse != nil {
		return onFalse
	}

	return &ItemModelSelectorCondition{
		Property:        property,
		DataType:        dataType,
		Predicate:       predicate,
		Component:       component,
		ValueProperties: valueProperties,
		ValueLiteral:    valueLiteral,
		OnTrue:          onTrue,
		OnFalse:         onFalse,
	}
}

func parseItemSpecialModelInfo(baseModel string, element any) *ItemSpecialModelInfo {
	obj, ok := element.(map[string]any)
	if !ok {
		return nil
	}

	typeValue := getString(obj, "type")
	if strings.TrimSpace(typeValue) == "" {
		return nil
	}

	texture := getString(obj, "texture")
	return &ItemSpecialModelInfo{
		Type:      normalizeNamespacedValue(typeValue),
		BaseModel: strings.TrimSpace(baseModel),
		Texture:   normalizeNamespacedValue(texture),
	}
}

func createFallbackSelector(element map[string]any) ItemModelSelector {
	directModel := getString(element, "model")
	if strings.TrimSpace(directModel) == "" {
		directModel = getString(element, "base")
	}
	if strings.TrimSpace(directModel) == "" {
		return nil
	}
	return NewItemModelSelectorModel(directModel, "")
}

func parseComposite(element map[string]any, depth int) ItemModelSelector {
	modelsArray, ok := element["models"].([]any)
	if !ok {
		return nil
	}

	selectors := make([]ItemModelSelector, 0, len(modelsArray))
	for _, modelElement := range modelsArray {
		parsed := parseItemModelSelector(modelElement, depth)
		if parsed != nil {
			selectors = append(selectors, parsed)
		}
	}

	if len(selectors) == 0 {
		return nil
	}
	return &ItemModelSelectorComposite{Selectors: selectors}
}

func parseSelect(element map[string]any, depth int) ItemModelSelector {
	property := getString(element, "property")
	dataType := trimToPointer(getString(element, "data_type"))
	component := trimToPointer(getString(element, "component"))
	cases := make([]ItemModelSelectorSelectCase, 0)

	if casesElement, okCases := element["cases"].([]any); okCases {
		for _, caseElement := range casesElement {
			caseObject, okCaseObject := caseElement.(map[string]any)
			if !okCaseObject {
				continue
			}

			whenValues := parseWhen(caseObject["when"])
			var selector ItemModelSelector
			if modelElement, okModelElement := caseObject["model"]; okModelElement {
				selector = parseItemModelSelector(modelElement, depth+1)
			}
			cases = append(cases, ItemModelSelectorSelectCase{
				When:     whenValues,
				Selector: selector,
			})
		}
	}

	var fallback ItemModelSelector
	if fallbackElement, okFallback := element["fallback"]; okFallback {
		fallback = parseItemModelSelector(fallbackElement, depth+1)
	}

	return &ItemModelSelectorSelect{
		Property:  property,
		DataType:  dataType,
		Component: component,
		Cases:     cases,
		Fallback:  fallback,
	}
}

func parseRangeDispatch(element map[string]any, depth int) ItemModelSelector {
	property := getString(element, "property")
	dataType := trimToPointer(getString(element, "data_type"))
	normalize := false
	if normalizeElement, okNormalize := element["normalize"].(bool); okNormalize {
		normalize = normalizeElement
	}

	entries := make([]RangeDispatchEntry, 0)
	if entriesElement, okEntries := element["entries"].([]any); okEntries {
		for _, entryElement := range entriesElement {
			entryObject, okEntryObject := entryElement.(map[string]any)
			if !okEntryObject {
				continue
			}

			threshold, okThreshold := toFloat64(entryObject["threshold"])
			if !okThreshold {
				continue
			}

			var selector ItemModelSelector
			if modelElement, okModel := entryObject["model"]; okModel {
				selector = parseItemModelSelector(modelElement, depth+1)
			}

			entries = append(entries, RangeDispatchEntry{
				Threshold: threshold,
				Selector:  selector,
			})
		}
	}

	var fallback ItemModelSelector
	if fallbackElement, okFallback := element["fallback"]; okFallback {
		fallback = parseItemModelSelector(fallbackElement, depth+1)
	}

	return &ItemModelSelectorRangeDispatch{
		Property:  property,
		DataType:  dataType,
		Normalize: normalize,
		Entries:   entries,
		Fallback:  fallback,
	}
}

func parseStringMap(element any) map[string]string {
	obj, ok := element.(map[string]any)
	if !ok {
		return nil
	}

	mapValues := make(map[string]string)
	for key, value := range obj {
		if strings.TrimSpace(key) == "" {
			continue
		}

		asString := ""
		switch typed := value.(type) {
		case string:
			asString = typed
		case float64:
			if math.Mod(typed, 1) == 0 {
				asString = strconv.FormatInt(int64(typed), 10)
			} else {
				asString = strconv.FormatFloat(typed, 'f', -1, 64)
			}
		case bool:
			if typed {
				asString = "true"
			} else {
				asString = "false"
			}
		case nil:
			asString = "null"
		default:
			if raw, err := json.Marshal(typed); err == nil {
				asString = string(raw)
			}
		}

		if strings.TrimSpace(asString) != "" {
			mapValues[key] = asString
		}
	}

	if len(mapValues) == 0 {
		return nil
	}

	return mapValues
}

func parseWhen(element any) []string {
	switch typed := element.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return []string{}
		}
		return []string{typed}
	case []any:
		values := make([]string, 0)
		for _, entry := range typed {
			if str, ok := entry.(string); ok && strings.TrimSpace(str) != "" {
				values = append(values, str)
			}
		}
		return values
	default:
		return []string{}
	}
}

func getString(element map[string]any, propertyName string) string {
	if property, ok := element[propertyName].(string); ok {
		return property
	}
	return ""
}

func normalizeSelectorType(value string) string {
	if strings.TrimSpace(value) == "" {
		return "model"
	}

	typeValue := strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(typeValue), "minecraft:") {
		typeValue = typeValue[10:]
	}

	return strings.ToLower(typeValue)
}

func normalizeNamespacedValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, ":") {
		return strings.ToLower(trimmed)
	}
	return "minecraft:" + strings.ToLower(trimmed)
}

func trimToPointer(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	copyValue := trimmed
	return &copyValue
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func toFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func nbtTagToString(tag nbt.NbtTag) (string, bool) {
	switch typed := tag.(type) {
	case nbt.NbtString:
		return typed.Value, true
	case *nbt.NbtString:
		return typed.Value, true
	case nbt.NbtInt:
		return strconv.FormatInt(int64(typed.Value), 10), true
	case *nbt.NbtInt:
		return strconv.FormatInt(int64(typed.Value), 10), true
	case nbt.NbtLong:
		return strconv.FormatInt(typed.Value, 10), true
	case *nbt.NbtLong:
		return strconv.FormatInt(typed.Value, 10), true
	case nbt.NbtShort:
		return strconv.FormatInt(int64(typed.Value), 10), true
	case *nbt.NbtShort:
		return strconv.FormatInt(int64(typed.Value), 10), true
	case nbt.NbtByte:
		return strconv.FormatInt(int64(typed.Value), 10), true
	case *nbt.NbtByte:
		return strconv.FormatInt(int64(typed.Value), 10), true
	case nbt.NbtFloat:
		return strconv.FormatFloat(float64(typed.Value), 'f', -1, 64), true
	case *nbt.NbtFloat:
		return strconv.FormatFloat(float64(typed.Value), 'f', -1, 64), true
	case nbt.NbtDouble:
		return strconv.FormatFloat(typed.Value, 'f', -1, 64), true
	case *nbt.NbtDouble:
		return strconv.FormatFloat(typed.Value, 'f', -1, 64), true
	default:
		return "", false
	}
}

func nbtTagToFloat64(tag nbt.NbtTag) (float64, bool) {
	switch typed := tag.(type) {
	case nbt.NbtByte:
		return float64(typed.Value), true
	case *nbt.NbtByte:
		return float64(typed.Value), true
	case nbt.NbtShort:
		return float64(typed.Value), true
	case *nbt.NbtShort:
		return float64(typed.Value), true
	case nbt.NbtInt:
		return float64(typed.Value), true
	case *nbt.NbtInt:
		return float64(typed.Value), true
	case nbt.NbtLong:
		return float64(typed.Value), true
	case *nbt.NbtLong:
		return float64(typed.Value), true
	case nbt.NbtFloat:
		return float64(typed.Value), true
	case *nbt.NbtFloat:
		return float64(typed.Value), true
	case nbt.NbtDouble:
		return typed.Value, true
	case *nbt.NbtDouble:
		return typed.Value, true
	default:
		return 0, false
	}
}

func nbtTagToBool(tag nbt.NbtTag) (bool, bool) {
	switch typed := tag.(type) {
	case nbt.NbtByte:
		return typed.Value != 0, true
	case *nbt.NbtByte:
		return typed.Value != 0, true
	case nbt.NbtShort:
		return typed.Value != 0, true
	case *nbt.NbtShort:
		return typed.Value != 0, true
	case nbt.NbtInt:
		return typed.Value != 0, true
	case *nbt.NbtInt:
		return typed.Value != 0, true
	case nbt.NbtLong:
		return typed.Value != 0, true
	case *nbt.NbtLong:
		return typed.Value != 0, true
	case nbt.NbtString:
		parsed, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(typed.Value)))
		return parsed, err == nil
	case *nbt.NbtString:
		parsed, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(typed.Value)))
		return parsed, err == nil
	default:
		return false, false
	}
}
