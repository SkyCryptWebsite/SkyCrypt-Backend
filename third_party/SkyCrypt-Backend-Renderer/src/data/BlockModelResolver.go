package data

import (
	"fmt"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/assets"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/global"
	"os"
	"sync"
)

type BlockModelResolver struct {
	Definitions map[string]BlockModelDefinition
	_cache      map[string]BlockModelInstance
	_cacheMu    sync.RWMutex
}

func (resolver *BlockModelResolver) LoadFromFile(path string) error {
	if path == "" {
		return nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("model definition file not found: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read model definition file: %w", err)
	}

	var definitions map[string]BlockModelDefinition
	if err := global.JSON.Unmarshal(data, &definitions); err != nil {
		return fmt.Errorf("failed to parse block model definitions from '%s': %w", path, err)
	}

	resolver.Definitions = make(map[string]BlockModelDefinition)
	for key, def := range definitions {
		resolver.Definitions[key] = def
	}

	resolver._cache = make(map[string]BlockModelInstance)

	return nil
}

func (resolver *BlockModelResolver) LoadFromMinecraftAssets(assetsPath string, overlayRoots *[]string, assetNamespaces *assets.AssetNamespaceRegistry) *BlockModelResolver {
	if assetsPath == "" {
		panic("assetsPath cannot be empty")
	}

	definitions := MinecraftAssetLoaderInstance.LoadModelDefinitions(assetsPath, overlayRoots, assetNamespaces)
	if definitions == nil {
		panic(fmt.Sprintf("Failed to load model definitions from Minecraft assets at '%s'.", assetsPath))
	}

	for key, def := range definitions {
		resolver.Definitions[key] = def
	}

	resolver._cache = make(map[string]BlockModelInstance)

	return resolver
}

func (resolver *BlockModelResolver) Resolve(model string) *BlockModelInstance {
	if model == "" {
		return nil
	}

	normalizedName := resolver.NormalizeName(model)

	resolver._cacheMu.RLock()
	if resolver._cache != nil {
		if instance, exists := resolver._cache[normalizedName]; exists {
			resolver._cacheMu.RUnlock()
			return &instance
		}
	}
	resolver._cacheMu.RUnlock()

	instance := resolver.ResolveInternal(normalizedName, make(map[string]struct{}))

	resolver._cacheMu.Lock()
	if resolver._cache == nil {
		resolver._cache = make(map[string]BlockModelInstance)
	} else {
		if cached, exists := resolver._cache[normalizedName]; exists {
			resolver._cacheMu.Unlock()
			return &cached
		}
	}
	resolver._cache[normalizedName] = instance
	resolver._cacheMu.Unlock()

	return &instance
}

func (resolver *BlockModelResolver) TryResolve(model string) (*BlockModelInstance, bool) {
	if resolver == nil || model == "" {
		return nil, false
	}
	normalizedName := resolver.NormalizeName(model)
	if _, exists := resolver.Definitions[normalizedName]; !exists {
		return nil, false
	}
	return resolver.Resolve(model), true
}

func (resolver *BlockModelResolver) ResolveInternal(model string, stack map[string]struct{}) BlockModelInstance {
	if _, exists := stack[model]; exists {
		panic(fmt.Sprintf("Detected circular model inheritance involving '%s'.", model))
	}

	definition, exists := resolver.Definitions[model]
	if !exists {
		panic(fmt.Sprintf("Model '%s' was not found in the loaded definitions.", model))
	}

	stack[model] = struct{}{}

	var parentChain []string
	textures := make(map[string]string)
	display := make(map[string]*TransformDefinition)
	var elements []ModelElement

	if definition.Parent != nil && *definition.Parent != "" {
		parent := resolver.ResolveInternal(resolver.NormalizeName(*definition.Parent), stack)
		parentChain = append(parentChain, parent.ParentChain...)
		parentChain = append(parentChain, parent.Name)

		for key, value := range parent.Textures {
			textures[key] = value
		}

		for key, value := range parent.Display {
			display[key] = value
		}

		if len(parent.Elements) > 0 {
			for _, parentElement := range parent.Elements {
				elements = append(elements, resolver.CloneElement(parentElement))
			}
		}
	}

	for key, value := range definition.Textures {
		textures[key] = value
	}

	for key, value := range definition.Display {
		temp := resolver.CloneTransform(value)
		display[key] = &temp
	}

	if len(definition.Elements) > 0 {
		elements = nil
		for _, elementDef := range definition.Elements {
			if convertedElement := resolver.ConvertElement(elementDef); convertedElement != nil {
				elements = append(elements, *convertedElement)
			}
		}
	}

	delete(stack, model)

	return BlockModelInstance{
		Name:        model,
		ParentChain: parentChain,
		Textures:    textures,
		Display:     display,
		Elements:    elements,
	}
}

func (resolver *BlockModelResolver) CloneElement(element ModelElement) ModelElement {
	faces := make(map[BlockFaceDirection]ModelFace, len(element.Faces))
	for key, face := range element.Faces {
		faces[key] = ModelFace{
			Texture:   face.Texture,
			Uv:        face.Uv,
			Rotation:  face.Rotation,
			TintIndex: face.TintIndex,
			CullFace:  face.CullFace,
		}
	}

	var rotation *ElementRotation
	if element.Rotation != nil {
		clonedRotation := ElementRotation{
			AngleInDegrees: element.Rotation.AngleInDegrees,
			Origin:         element.Rotation.Origin,
			Axis:           element.Rotation.Axis,
			Rescale:        element.Rotation.Rescale,
		}
		rotation = &clonedRotation
	}

	return ModelElement{
		From:     element.From,
		To:       element.To,
		Rotation: rotation,
		Faces:    faces,
		Shade:    element.Shade,
	}
}

func (resolver *BlockModelResolver) CloneTransform(source TransformDefinition) TransformDefinition {
	var rotation []float64
	if source.Rotation != nil {
		rotation = make([]float64, len(*source.Rotation))
		copy(rotation, *source.Rotation)
	}

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

	return TransformDefinition{
		Rotation:    &rotation,
		Translation: &translation,
		Scale:       &scale,
	}
}

func (resolver *BlockModelResolver) ConvertElement(definition ElementDefinition) *ModelElement {
	if len(definition.From) != 3 || len(definition.To) != 3 {
		return nil
	}

	fromVec := Vector3{definition.From[0], definition.From[1], definition.From[2]}
	toVec := Vector3{definition.To[0], definition.To[1], definition.To[2]}

	var rotation *ElementRotation
	if definition.Rotation != nil && len(definition.Rotation.Origin) == 3 {
		originVec := Vector3{definition.Rotation.Origin[0], definition.Rotation.Origin[1], definition.Rotation.Origin[2]}
		rotation = &ElementRotation{
			AngleInDegrees: definition.Rotation.Angle,
			Origin:         originVec,
			Axis:           definition.Rotation.Axis,
			Rescale:        definition.Rotation.Rescale != nil && *definition.Rotation.Rescale,
		}
	}

	faces := make(map[BlockFaceDirection]ModelFace)
	for key, faceDef := range definition.Faces {
		direction, err := ParseFaceDirection(key)
		if err != nil {
			continue
		}

		var uvVector *Vector4
		if len(faceDef.Uv) == 4 {
			uvVector = &Vector4{faceDef.Uv[0], faceDef.Uv[1], faceDef.Uv[2], faceDef.Uv[3]}
		}

		faces[direction] = ModelFace{
			Texture:   faceDef.Texture,
			Uv:        uvVector,
			Rotation:  faceDef.Rotation,
			TintIndex: faceDef.TintIndex,
			CullFace:  faceDef.CullFace,
		}
	}

	shade := true
	if definition.Shade != nil {
		shade = *definition.Shade
	}

	return &ModelElement{
		From:     fromVec,
		To:       toVec,
		Rotation: rotation,
		Faces:    faces,
		Shade:    shade,
	}
}

func ParseFaceDirection(key string) (BlockFaceDirection, error) {
	switch key {
	case "north", "North", "NORTH":
		return North, nil
	case "south", "South", "SOUTH":
		return South, nil
	case "east", "East", "EAST":
		return East, nil
	case "west", "West", "WEST":
		return West, nil
	case "up", "Up", "UP":
		return Up, nil
	case "down", "Down", "DOWN":
		return Down, nil
	default:
		return 0, fmt.Errorf("invalid face direction: %s", key)
	}
}

func (resolver *BlockModelResolver) NormalizeName(name string) string {
	normalized := name

	if len(normalized) >= 10 && normalized[:10] == "minecraft:" {
		normalized = normalized[10:]
	}

	if len(normalized) >= 6 && normalized[:6] == "block/" {
		normalized = normalized[6:]
	} else if len(normalized) >= 7 && normalized[:7] == "blocks/" {
		normalized = normalized[7:]
	}

	return normalized
}

func NewBlockModelResolver(definitions map[string]BlockModelDefinition) *BlockModelResolver {
	return &BlockModelResolver{
		Definitions: definitions,
		_cache:      make(map[string]BlockModelInstance),
	}
}

var BlockModelResolverInstance = NewBlockModelResolver(make(map[string]BlockModelDefinition))
