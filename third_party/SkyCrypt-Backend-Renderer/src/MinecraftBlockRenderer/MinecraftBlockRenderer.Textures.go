package minecraftblockrenderer

import "github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/data"

func ResolveTexture(texture string, model *data.BlockModelInstance) string {
	if texture == "" {
		return "minecraft:missingno"
	}

	if model == nil {
		if len(texture) > 0 && texture[0] == '#' {
			return "minecraft:missingno"
		}
		return texture
	}

	expandTextureReference := func(candidate string, instance *data.BlockModelInstance) string {
		if candidate == "" {
			return ""
		}

		trimmed := candidate
		if len(trimmed) > 0 && trimmed[0] == '#' {
			return trimmed
		}

		if _, ok := instance.Textures[trimmed]; ok {
			return "#" + trimmed
		}

		return trimmed
	}

	visited := make(map[string]struct{})
	current := expandTextureReference(texture, model)

	for len(current) > 0 && current[0] == '#' {
		key := current[1:]
		if _, exists := visited[key]; exists {
			return "minecraft:missingno"
		}
		visited[key] = struct{}{}

		mapped, ok := model.Textures[key]
		if !ok || mapped == "" {
			return "minecraft:missingno"
		}

		current = expandTextureReference(mapped, model)
		if current == "" {
			return "minecraft:missingno"
		}
	}

	return current
}
