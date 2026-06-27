package minecraftblockrenderer

import (
	"regexp"
	"strings"

	nbt "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/NBT"
)

type SkullTextureInfo struct {
	Value     string
	Signature *string
}

var skullTextureBodyRegex = regexp.MustCompile(`(?is)textures\s*:\s*\[\s*0\s*:\s*\{(?P<body>.*?)\}\s*\]`)
var skullTextureValueRegex = regexp.MustCompile(`(?is)Value\s*:\s*"(?P<value>(?:\\.|[^"])*)"`)
var skullTextureSignatureRegex = regexp.MustCompile(`(?is)Signature\s*:\s*"(?P<signature>(?:\\.|[^"])*)"`)

func DefaultSkullTextureResolver(context SkullResolverContext) *string {
	if context.Profile != nil {
		if texture := ExtractSkullTextureFromCompound(context.Profile); texture != nil && strings.TrimSpace(texture.Value) != "" {
			return &texture.Value
		}
	}

	if context.CustomData != nil {
		if texture := ExtractSkullTextureFromCompound(context.CustomData); texture != nil && strings.TrimSpace(texture.Value) != "" {
			return &texture.Value
		}
	}

	return nil
}

func ExtractSkullTexture(nbtTag string) *SkullTextureInfo {
	trimmed := strings.TrimSpace(nbtTag)
	if trimmed == "" {
		return nil
	}

	if info := extractSkullTextureFromText(trimmed); info != nil {
		return info
	}

	parser := nbt.NbtParser{}
	document, err := parser.ParseSnbt(trimmed)
	if err != nil {
		return nil
	}

	if texture := extractSkullTextureFromTag(document.Root); texture != nil {
		return texture
	}

	return nil
}

func ExtractSkullTextureFromCompound(compound *nbt.NbtCompound) *SkullTextureInfo {
	return extractSkullTextureFromTag(compound)
}

func extractSkullTextureFromText(text string) *SkullTextureInfo {
	match := skullTextureBodyRegex.FindStringSubmatch(text)
	if match == nil {
		return nil
	}

	body := match[skullTextureBodyRegex.SubexpIndex("body")]
	valueMatch := skullTextureValueRegex.FindStringSubmatch(body)
	if valueMatch == nil {
		return nil
	}

	value := unescapeSkullTextureText(valueMatch[skullTextureValueRegex.SubexpIndex("value")])
	if strings.TrimSpace(value) == "" {
		return nil
	}

	var signature *string
	if signatureMatch := skullTextureSignatureRegex.FindStringSubmatch(body); signatureMatch != nil {
		signatureValue := unescapeSkullTextureText(signatureMatch[skullTextureSignatureRegex.SubexpIndex("signature")])
		if strings.TrimSpace(signatureValue) != "" {
			signature = &signatureValue
		}
	}

	return &SkullTextureInfo{Value: value, Signature: signature}
}

func extractSkullTextureFromTag(tag nbt.NbtTag) *SkullTextureInfo {
	return extractSkullTextureFromTagDepth(tag, 0)
}

func extractSkullTextureFromTagDepth(tag nbt.NbtTag, depth int) *SkullTextureInfo {
	if tag == nil || depth > 8 {
		return nil
	}

	switch typed := tag.(type) {
	case nbt.NbtString:
		if !looksLikeSkullTextureText(typed.Value) {
			return nil
		}
		if texture := ExtractSkullTexture(typed.Value); texture != nil {
			return texture
		}
	case *nbt.NbtString:
		if !looksLikeSkullTextureText(typed.Value) {
			return nil
		}
		if texture := ExtractSkullTexture(typed.Value); texture != nil {
			return texture
		}
	case *nbt.NbtCompound:
		return extractSkullTextureFromCompoundValue(typed, depth)
	case *nbt.NbtList:
		return extractSkullTextureFromList(typed, depth)
	}

	return nil
}

func looksLikeSkullTextureText(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "textures") &&
		(strings.Contains(lower, "value") || strings.Contains(lower, "signature") || strings.Contains(lower, "minecraft.net/texture"))
}

func extractSkullTextureFromCompoundValue(compound *nbt.NbtCompound, depth int) *SkullTextureInfo {
	if compound == nil {
		return nil
	}

	if texture := extractSkullTextureFromProfileCompound(compound, depth); texture != nil {
		return texture
	}

	for _, key := range compound.Keys() {
		value, ok := compound.Get(key)
		if !ok {
			continue
		}

		if texture := extractSkullTextureFromTagDepth(value, depth+1); texture != nil {
			return texture
		}
	}

	return nil
}

func extractSkullTextureFromList(list *nbt.NbtList, depth int) *SkullTextureInfo {
	if list == nil {
		return nil
	}

	for _, item := range list.Items() {
		if texture := extractSkullTextureFromTagDepth(item, depth+1); texture != nil {
			return texture
		}
	}

	return nil
}

func extractSkullTextureFromProfileCompound(compound *nbt.NbtCompound, depth int) *SkullTextureInfo {
	if compound == nil {
		return nil
	}

	if value, ok := tryGetCompoundString(compound, "value"); ok && strings.TrimSpace(value) != "" {
		info := &SkullTextureInfo{Value: value}
		if signature, okSignature := tryGetCompoundString(compound, "signature"); okSignature && strings.TrimSpace(signature) != "" {
			info.Signature = &signature
		}
		return info
	}

	if list := getCompoundList(compound, "properties"); list != nil {
		for _, item := range list.Items() {
			propertyCompound, ok := item.(*nbt.NbtCompound)
			if !ok {
				continue
			}

			name, okName := tryGetCompoundString(propertyCompound, "name")
			if !okName || !strings.EqualFold(strings.TrimSpace(name), "textures") {
				continue
			}

			if value, okValue := tryGetCompoundString(propertyCompound, "value"); okValue && strings.TrimSpace(value) != "" {
				info := &SkullTextureInfo{Value: value}
				if signature, okSignature := tryGetCompoundString(propertyCompound, "signature"); okSignature && strings.TrimSpace(signature) != "" {
					info.Signature = &signature
				}
				return info
			}

			if texture := extractSkullTextureFromTagDepth(propertyCompound, depth+1); texture != nil {
				return texture
			}
		}
	}

	if textures := getCompoundList(compound, "textures"); textures != nil {
		if texture := extractSkullTextureFromList(textures, depth+1); texture != nil {
			return texture
		}
	}

	return nil
}

func tryGetCompoundString(compound *nbt.NbtCompound, key string) (string, bool) {
	if compound == nil {
		return "", false
	}

	tag, ok := compound.Get(key)
	if !ok {
		return "", false
	}

	switch typed := tag.(type) {
	case nbt.NbtString:
		return typed.Value, true
	case *nbt.NbtString:
		return typed.Value, true
	default:
		return "", false
	}
}

func getCompoundList(compound *nbt.NbtCompound, key string) *nbt.NbtList {
	if compound == nil {
		return nil
	}

	tag, ok := compound.Get(key)
	if !ok {
		return nil
	}

	switch typed := tag.(type) {
	case *nbt.NbtList:
		return typed
	default:
		return nil
	}
}

func unescapeSkullTextureText(value string) string {
	if strings.IndexByte(value, '\\') < 0 {
		return value
	}

	value = strings.ReplaceAll(value, `\\`, `\`)
	value = strings.ReplaceAll(value, `\"`, `"`)
	return value
}
