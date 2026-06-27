package nbt

import "strings"

type NbtTagType byte

const (
	NbtTagTypeEnd NbtTagType = iota
	NbtTagTypeByte
	NbtTagTypeShort
	NbtTagTypeInt
	NbtTagTypeLong
	NbtTagTypeFloat
	NbtTagTypeDouble
	NbtTagTypeByteArray
	NbtTagTypeString
	NbtTagTypeList
	NbtTagTypeCompound
	NbtTagTypeIntArray
	NbtTagTypeLongArray
)

type NbtTag interface {
	Type() NbtTagType
}

type NbtByte struct {
	Value int8
}

func NewNbtByte(value int8) NbtByte {
	return NbtByte{Value: value}
}

func (n NbtByte) Type() NbtTagType {
	return NbtTagTypeByte
}

type NbtShort struct {
	Value int16
}

func NewNbtShort(value int16) NbtShort {
	return NbtShort{Value: value}
}

func (n NbtShort) Type() NbtTagType {
	return NbtTagTypeShort
}

type NbtInt struct {
	Value int32
}

func NewNbtInt(value int32) NbtInt {
	return NbtInt{Value: value}
}

func (n NbtInt) Type() NbtTagType {
	return NbtTagTypeInt
}

type NbtLong struct {
	Value int64
}

func NewNbtLong(value int64) NbtLong {
	return NbtLong{Value: value}
}

func (n NbtLong) Type() NbtTagType {
	return NbtTagTypeLong
}

type NbtFloat struct {
	Value float32
}

func NewNbtFloat(value float32) NbtFloat {
	return NbtFloat{Value: value}
}

func (n NbtFloat) Type() NbtTagType {
	return NbtTagTypeFloat
}

type NbtDouble struct {
	Value float64
}

func NewNbtDouble(value float64) NbtDouble {
	return NbtDouble{Value: value}
}

func (n NbtDouble) Type() NbtTagType {
	return NbtTagTypeDouble
}

type NbtString struct {
	Value string
}

func NewNbtString(value string) NbtString {
	return NbtString{Value: value}
}

func (n NbtString) Type() NbtTagType {
	return NbtTagTypeString
}

type NbtByteArray struct {
	Values []byte
}

func NewNbtByteArray(values []byte) NbtByteArray {
	return NbtByteArray{Values: values}
}

func (n NbtByteArray) Type() NbtTagType {
	return NbtTagTypeByteArray
}

type NbtIntArray struct {
	Values []int32
}

func NewNbtIntArray(values []int32) NbtIntArray {
	return NbtIntArray{Values: values}
}

func (n NbtIntArray) Type() NbtTagType {
	return NbtTagTypeIntArray
}

type NbtLongArray struct {
	Values []int64
}

func NewNbtLongArray(values []int64) NbtLongArray {
	return NbtLongArray{Values: values}
}

func (n NbtLongArray) Type() NbtTagType {
	return NbtTagTypeLongArray
}

type NbtList struct {
	ElementType NbtTagType
	items       []NbtTag
}

func NewNbtList(elementType NbtTagType, items []NbtTag) *NbtList {
	copyItems := append([]NbtTag(nil), items...)
	return &NbtList{
		ElementType: elementType,
		items:       copyItems,
	}
}

func (n *NbtList) Type() NbtTagType {
	return NbtTagTypeList
}

func (n *NbtList) At(index int) NbtTag {
	return n.items[index]
}

func (n *NbtList) Count() int {
	return len(n.items)
}

func (n *NbtList) Items() []NbtTag {
	return append([]NbtTag(nil), n.items...)
}

type NbtCompound struct {
	items map[string]NbtTag
}

func NewNbtCompound(values map[string]NbtTag) *NbtCompound {
	items := make(map[string]NbtTag)
	for key, value := range values {
		normalized := normalizeCompoundKey(key)
		if normalized == "" {
			continue
		}
		items[normalized] = value
	}

	return &NbtCompound{items: items}
}

func (n *NbtCompound) Type() NbtTagType {
	return NbtTagTypeCompound
}

func (n *NbtCompound) Get(key string) (NbtTag, bool) {
	value, ok := n.items[normalizeCompoundKey(key)]
	return value, ok
}

func (n *NbtCompound) Set(key string, value NbtTag) {
	normalized := normalizeCompoundKey(key)
	if normalized == "" {
		return
	}
	n.items[normalized] = value
}

func (n *NbtCompound) ContainsKey(key string) bool {
	_, ok := n.Get(key)
	return ok
}

func (n *NbtCompound) Keys() []string {
	keys := make([]string, 0, len(n.items))
	for key := range n.items {
		keys = append(keys, key)
	}
	return keys
}

func (n *NbtCompound) Values() []NbtTag {
	values := make([]NbtTag, 0, len(n.items))
	for _, value := range n.items {
		values = append(values, value)
	}
	return values
}

func (n *NbtCompound) Count() int {
	return len(n.items)
}

func (n *NbtCompound) Items() map[string]NbtTag {
	copyItems := make(map[string]NbtTag, len(n.items))
	for key, value := range n.items {
		copyItems[key] = value
	}
	return copyItems
}

type NbtDocument struct {
	Root NbtTag
}

func NewNbtDocument(root NbtTag) *NbtDocument {
	return &NbtDocument{Root: root}
}

func (n *NbtDocument) RootCompound() (*NbtCompound, bool) {
	compound, ok := n.Root.(*NbtCompound)
	return compound, ok
}

func normalizeCompoundKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}
