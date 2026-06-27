package nbt

type NbtExtensions struct{}

func (e *NbtExtensions) GetString(tag NbtCompound, key string) *string {
	value, ok := tag.Get(key)
	if !ok {
		return nil
	}

	strValue, ok := value.(*NbtString)
	if !ok {
		return nil
	}

	return &strValue.Value
}

func (e *NbtExtensions) GetByte(tag NbtCompound, key string) *int8 {
	value, ok := tag.Get(key)
	if !ok {
		return nil
	}

	byteValue, ok := value.(*NbtByte)
	if !ok {
		return nil
	}

	return &byteValue.Value
}

func (e *NbtExtensions) GetShort(tag NbtCompound, key string) *int16 {
	value, ok := tag.Get(key)
	if !ok {
		return nil
	}

	shortValue, ok := value.(*NbtShort)
	if !ok {
		return nil
	}

	return &shortValue.Value
}

func (e *NbtExtensions) GetInt(tag NbtCompound, key string) *int32 {
	value, ok := tag.Get(key)
	if !ok {
		return nil
	}

	intValue, ok := value.(*NbtInt)
	if !ok {
		return nil
	}

	return &intValue.Value
}

func (e *NbtExtensions) GetLong(tag NbtCompound, key string) *int64 {
	value, ok := tag.Get(key)
	if !ok {
		return nil
	}

	longValue, ok := value.(*NbtLong)
	if !ok {
		return nil
	}

	return &longValue.Value
}

func (e *NbtExtensions) GetFloat(tag NbtCompound, key string) *float32 {
	value, ok := tag.Get(key)
	if !ok {
		return nil
	}

	floatValue, ok := value.(*NbtFloat)
	if !ok {
		return nil
	}

	return &floatValue.Value
}

func (e *NbtExtensions) GetDouble(tag NbtCompound, key string) *float64 {
	value, ok := tag.Get(key)
	if !ok {
		return nil
	}

	doubleValue, ok := value.(*NbtDouble)
	if !ok {
		return nil
	}

	return &doubleValue.Value
}

func (e *NbtExtensions) GetCompound(tag NbtCompound, key string) *NbtCompound {
	value, ok := tag.Get(key)
	if !ok {
		return nil
	}

	compoundValue, ok := value.(*NbtCompound)
	if !ok {
		return nil
	}

	return compoundValue
}

func (e *NbtExtensions) GetList(tag NbtCompound, key string) *NbtList {
	value, ok := tag.Get(key)
	if !ok {
		return nil
	}

	listValue, ok := value.(*NbtList)
	if !ok {
		return nil
	}

	return listValue
}

func (e *NbtExtensions) GetByteArray(tag NbtCompound, key string) *[]byte {
	value, ok := tag.Get(key)
	if !ok {
		return nil
	}

	byteArrayValue, ok := value.(*NbtByteArray)
	if !ok {
		return nil
	}

	return &byteArrayValue.Values
}

func (e *NbtExtensions) GetIntArray(tag NbtCompound, key string) *[]int32 {
	value, ok := tag.Get(key)
	if !ok {
		return nil
	}

	intArrayValue, ok := value.(*NbtIntArray)
	if !ok {
		return nil
	}

	return &intArrayValue.Values
}

func (e *NbtExtensions) GetLongArray(tag NbtCompound, key string) *[]int64 {
	value, ok := tag.Get(key)
	if !ok {
		return nil
	}

	longArrayValue, ok := value.(*NbtLongArray)
	if !ok {
		return nil
	}

	return &longArrayValue.Values
}

func (e *NbtExtensions) WithProfileComponent(compound NbtCompound, profileValue string, signature string) NbtCompound {
	propertyEntries := map[string]NbtTag{
		"name":      NewNbtString("textures"),
		"value":     NewNbtString(profileValue),
		"signature": NewNbtString(signature),
	}

	propertyCompound := NewNbtCompound(propertyEntries)

	propertiesList := NewNbtList(NbtTagTypeCompound, []NbtTag{propertyCompound})

	profileCompound := NewNbtCompound(map[string]NbtTag{
		"properties": propertiesList,
	})

	components := NbtCompound{}
	otherRootEntries := make(map[string]NbtTag)

	if componentsTag, ok := compound.Get("components"); ok {
		if existingComponents, ok := componentsTag.(*NbtCompound); ok {
			components = *existingComponents
			// Components exist, add profile to them
			components.items["minecraft:profile"] = profileCompound
			for key, value := range compound.items {
				if key != "components" {
					otherRootEntries[key] = value
				}
			}
		} else {
			// "components" exists but is not a compound, treat as if it doesn't exist
			components = NbtCompound{
				items: map[string]NbtTag{
					"minecraft:profile": profileCompound,
				},
			}
			otherRootEntries = compound.items
		}
	}

	return NbtCompound{
		items: otherRootEntries,
	}

}
