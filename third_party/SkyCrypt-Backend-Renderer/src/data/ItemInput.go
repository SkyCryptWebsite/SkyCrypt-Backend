package data

import (
	"errors"
	"fmt"
	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
	"image/color"
	"math"
	"reflect"
	"strconv"
	"strings"
)

type NormalizedItemInput struct {
	ItemID          string
	NumericID       *int
	Count           int
	Damage          int
	ItemModel       string
	SkyblockID      string
	DisplayName     string
	Lore            []string
	DisplayColor    *color.RGBA
	ExtraAttributes map[string]any
	CustomData      map[string]any
	SkullProfile    map[string]any
	Raw             any
}

func NormalizeItemInput(input any) (*NormalizedItemInput, error) {
	if input == nil {
		return nil, errors.New("item input cannot be nil")
	}

	root, ok := decodedValueToMap(input)
	if !ok {
		return nil, fmt.Errorf("unsupported item input type %T", input)
	}

	normalized := &NormalizedItemInput{
		Count: 1,
		Raw:   input,
	}

	if idValue, ok := lookupValue(root, "id", "ID", "RawId", "raw_id"); ok {
		applyItemIdentifier(normalized, idValue)
	}

	if itemIDValue, ok := lookupValue(
		root,
		"item_id",
		"itemId",
		"itemID",
		"ItemID",
		"minecraft_id",
		"minecraftId",
		"minecraftID",
		"MinecraftID",
		"raw_item_id",
		"rawItemId",
		"RawItemID",
	); ok {
		applyItemIdentifier(normalized, itemIDValue)
	}

	if countValue, ok := lookupValue(root, "Count", "count"); ok {
		if count, ok := toInt(countValue); ok && count > 0 {
			normalized.Count = count
		}
	}

	if damageValue, ok := lookupValue(root, "Damage", "damage"); ok {
		if damage, ok := toInt(damageValue); ok {
			normalized.Damage = damage
		}
	}

	tag, _ := lookupMap(root, "tag", "Tag")
	if tag != nil {
		if itemModel, ok := lookupString(tag, "ItemModel", "item_model", "model"); ok {
			normalized.ItemModel = itemModel
		}

		if display, ok := lookupMap(tag, "display", "Display"); ok {
			if displayName, ok := lookupString(display, "Name", "name"); ok {
				normalized.DisplayName = displayName
			}
			if loreValue, ok := lookupValue(display, "Lore", "lore"); ok {
				normalized.Lore = toStringSlice(loreValue)
			}
			if colorValue, ok := lookupValue(display, "color", "Color"); ok {
				if rgb, ok := toInt(colorValue); ok {
					normalized.DisplayColor = rgbIntToColor(rgb)
				}
			}
		}

		if extraAttributes, ok := lookupMap(tag, "ExtraAttributes", "extraAttributes", "extra_attributes"); ok {
			normalized.ExtraAttributes = extraAttributes
			if skyblockID, ok := lookupString(extraAttributes, "id", "Id", "ID"); ok {
				normalized.SkyblockID = skyblockID
			}
			if customData, ok := lookupMap(extraAttributes, "customData", "custom_data", "CustomData"); ok {
				normalized.CustomData = customData
			}
		}

		if skullProfile := extractSkullProfile(tag); len(skullProfile) > 0 {
			normalized.SkullProfile = skullProfile
		}
	}

	if normalized.ExtraAttributes == nil {
		if extraAttributes, ok := lookupMap(root, "ExtraAttributes", "extraAttributes", "extra_attributes"); ok {
			normalized.ExtraAttributes = extraAttributes
			if normalized.SkyblockID == "" {
				if skyblockID, ok := lookupString(extraAttributes, "id", "Id", "ID"); ok {
					normalized.SkyblockID = skyblockID
				}
			}
		}
	}

	if normalized.CustomData == nil && normalized.ExtraAttributes != nil {
		normalized.CustomData = normalized.ExtraAttributes
	}

	if normalized.ItemModel == "" {
		if itemModel, ok := lookupString(root, "ItemModel", "item_model", "model"); ok {
			normalized.ItemModel = itemModel
		}
	}

	if normalized.SkullProfile == nil {
		if skullProfile := extractSkullProfile(root); len(skullProfile) > 0 {
			normalized.SkullProfile = skullProfile
		}
	}

	return normalized, nil
}

func DecodedMapToNbtCompound(values map[string]any) *nbt.NbtCompound {
	if len(values) == 0 {
		return nil
	}

	items := make(map[string]nbt.NbtTag, len(values))
	for key, value := range values {
		if strings.TrimSpace(key) == "" || value == nil {
			continue
		}
		if tag, ok := decodedValueToNbtTag(value); ok {
			items[key] = tag
		}
	}
	if len(items) == 0 {
		return nil
	}
	return nbt.NewNbtCompound(items)
}

func applyItemIdentifier(normalized *NormalizedItemInput, value any) {
	if normalized == nil {
		return
	}
	if id, ok := toInt(value); ok {
		normalized.NumericID = &id
		return
	}
	if id, ok := toString(value); ok {
		normalized.ItemID = strings.TrimSpace(id)
		if numeric, ok := parseIntString(id); ok {
			normalized.NumericID = &numeric
		}
	}
}

func decodedValueToNbtTag(value any) (nbt.NbtTag, bool) {
	if value == nil {
		return nil, false
	}
	if tag, ok := value.(nbt.NbtTag); ok {
		return tag, true
	}

	switch typed := value.(type) {
	case string:
		return nbt.NewNbtString(typed), true
	case bool:
		if typed {
			return nbt.NewNbtByte(1), true
		}
		return nbt.NewNbtByte(0), true
	case int:
		return nbt.NewNbtInt(int32(typed)), true
	case int8:
		return nbt.NewNbtByte(typed), true
	case int16:
		return nbt.NewNbtShort(typed), true
	case int32:
		return nbt.NewNbtInt(typed), true
	case int64:
		return nbt.NewNbtLong(typed), true
	case uint:
		return nbt.NewNbtLong(int64(typed)), true
	case uint8:
		return nbt.NewNbtByte(int8(typed)), true
	case uint16:
		return nbt.NewNbtInt(int32(typed)), true
	case uint32:
		return nbt.NewNbtLong(int64(typed)), true
	case uint64:
		if typed > math.MaxInt64 {
			return nbt.NewNbtString(strconv.FormatUint(typed, 10)), true
		}
		return nbt.NewNbtLong(int64(typed)), true
	case float32:
		return nbt.NewNbtFloat(typed), true
	case float64:
		if math.Trunc(typed) == typed && typed >= math.MinInt32 && typed <= math.MaxInt32 {
			return nbt.NewNbtInt(int32(typed)), true
		}
		return nbt.NewNbtDouble(typed), true
	case []byte:
		return nbt.NewNbtByteArray(typed), true
	case []int:
		values := make([]int32, 0, len(typed))
		for _, item := range typed {
			values = append(values, int32(item))
		}
		return nbt.NewNbtIntArray(values), true
	case []int32:
		return nbt.NewNbtIntArray(typed), true
	case []int64:
		return nbt.NewNbtLongArray(typed), true
	case map[string]any:
		compound := DecodedMapToNbtCompound(typed)
		return compound, compound != nil
	}

	if mapped, ok := decodedValueToMap(value); ok {
		compound := DecodedMapToNbtCompound(mapped)
		return compound, compound != nil
	}

	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil, false
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		items := make([]nbt.NbtTag, 0, rv.Len())
		elementType := nbt.NbtTagTypeEnd
		for i := 0; i < rv.Len(); i++ {
			tag, ok := decodedValueToNbtTag(rv.Index(i).Interface())
			if !ok {
				continue
			}
			if elementType == nbt.NbtTagTypeEnd {
				elementType = tag.Type()
			}
			items = append(items, tag)
		}
		return nbt.NewNbtList(elementType, items), true
	}

	return nbt.NewNbtString(fmt.Sprint(value)), true
}

func decodedValueToMap(input any) (map[string]any, bool) {
	if input == nil {
		return nil, false
	}
	return reflectValueToMap(reflect.ValueOf(input))
}

func reflectValueToMap(value reflect.Value) (map[string]any, bool) {
	value = unwrapReflectValue(value)
	if !value.IsValid() {
		return nil, false
	}

	switch value.Kind() {
	case reflect.Map:
		result := make(map[string]any, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			key := fmt.Sprint(iter.Key().Interface())
			result[key] = reflectValueToAny(iter.Value())
		}
		return result, true
	case reflect.Struct:
		result := make(map[string]any)
		valueType := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := valueType.Field(i)
			if field.PkgPath != "" {
				continue
			}
			key := fieldTagName(field, "json")
			if key == "" {
				key = fieldTagName(field, "nbt")
			}
			if key == "" {
				key = field.Name
			}
			if key == "-" {
				continue
			}
			result[key] = reflectValueToAny(value.Field(i))
		}
		return result, true
	default:
		return nil, false
	}
}

func reflectValueToAny(value reflect.Value) any {
	value = unwrapReflectValue(value)
	if !value.IsValid() {
		return nil
	}

	switch value.Kind() {
	case reflect.Map:
		if mapped, ok := reflectValueToMap(value); ok {
			return mapped
		}
	case reflect.Struct:
		if mapped, ok := reflectValueToMap(value); ok {
			return mapped
		}
	case reflect.Slice, reflect.Array:
		items := make([]any, 0, value.Len())
		for i := 0; i < value.Len(); i++ {
			items = append(items, reflectValueToAny(value.Index(i)))
		}
		return items
	}

	return value.Interface()
}

func unwrapReflectValue(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

func fieldTagName(field reflect.StructField, tagName string) string {
	tag := field.Tag.Get(tagName)
	if tag == "" {
		return ""
	}
	name := strings.Split(tag, ",")[0]
	return strings.TrimSpace(name)
}

func lookupMap(values map[string]any, keys ...string) (map[string]any, bool) {
	value, ok := lookupValue(values, keys...)
	if !ok {
		return nil, false
	}
	return decodedValueToMap(value)
}

func lookupString(values map[string]any, keys ...string) (string, bool) {
	value, ok := lookupValue(values, keys...)
	if !ok {
		return "", false
	}
	return toString(value)
}

func lookupValue(values map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		for actualKey, value := range values {
			if strings.EqualFold(actualKey, key) {
				return value, true
			}
		}
	}
	return nil, false
}

func toString(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	rv := unwrapReflectValue(reflect.ValueOf(value))
	if rv.IsValid() && rv.CanInterface() {
		value = rv.Interface()
	}
	switch typed := value.(type) {
	case string:
		return typed, strings.TrimSpace(typed) != ""
	case fmt.Stringer:
		text := typed.String()
		return text, strings.TrimSpace(text) != ""
	}
	return fmt.Sprint(value), true
}

func toInt(value any) (int, bool) {
	if value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case uint:
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		return int(typed), true
	case float32:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		return parseIntString(typed)
	}

	rv := unwrapReflectValue(reflect.ValueOf(value))
	if !rv.IsValid() {
		return 0, false
	}
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return int(rv.Float()), true
	case reflect.String:
		return parseIntString(rv.String())
	}
	return 0, false
}

func parseIntString(value string) (int, bool) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func toStringSlice(value any) []string {
	if value == nil {
		return nil
	}
	rv := unwrapReflectValue(reflect.ValueOf(value))
	if !rv.IsValid() {
		return nil
	}
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		if text, ok := toString(value); ok {
			return []string{text}
		}
		return nil
	}

	output := make([]string, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		if text, ok := toString(reflectValueToAny(rv.Index(i))); ok {
			output = append(output, text)
		}
	}
	return output
}

func rgbIntToColor(value int) *color.RGBA {
	return &color.RGBA{
		R: uint8((value >> 16) & 0xFF),
		G: uint8((value >> 8) & 0xFF),
		B: uint8(value & 0xFF),
		A: 0xFF,
	}
}

func extractSkullProfile(values map[string]any) map[string]any {
	skullOwner, ok := lookupMap(values, "SkullOwner", "skullOwner", "skull_owner")
	if !ok {
		return nil
	}

	properties, ok := lookupMap(skullOwner, "Properties", "properties")
	if !ok {
		return nil
	}
	texturesValue, ok := lookupValue(properties, "textures", "Textures")
	if !ok {
		return nil
	}

	textures := toAnySlice(texturesValue)
	if len(textures) == 0 {
		return nil
	}
	texture, ok := decodedValueToMap(textures[0])
	if !ok {
		return nil
	}

	profile := map[string]any{}
	if value, ok := lookupString(texture, "Value", "value"); ok {
		profile["value"] = value
	}
	if signature, ok := lookupString(texture, "Signature", "signature"); ok {
		profile["signature"] = signature
	}
	if name, ok := lookupString(texture, "Name", "name"); ok {
		profile["name"] = name
	}
	if id, ok := lookupString(skullOwner, "Id", "ID", "id"); ok {
		profile["id"] = id
	}
	return profile
}

func toAnySlice(value any) []any {
	rv := unwrapReflectValue(reflect.ValueOf(value))
	if !rv.IsValid() || (rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array) {
		return nil
	}
	output := make([]any, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		output = append(output, reflectValueToAny(rv.Index(i)))
	}
	return output
}
