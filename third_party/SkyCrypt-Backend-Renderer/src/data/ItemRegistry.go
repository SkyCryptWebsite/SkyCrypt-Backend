package data

import (
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/assets"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/global"
	"image/color"
	"os"
	"strings"
)

type ItemInfo struct {
	Name       string
	Model      *string
	Texture    *string
	Selector   ItemModelSelector
	LayerTints map[int]ItemTintInfo
}

type ItemTintInfo struct {
	Kind         ItemTintKind
	DefaultColor *color.RGBA
}

type ItemTintKind string

const (
	ItemTintKindUnspecified ItemTintKind = "unspecified"
	ItemTintKindDye         ItemTintKind = "dye"
	ItemTintKindConstant    ItemTintKind = "constant"
	ItemTintKindUnknown     ItemTintKind = "unknown"
)

type ItemRegistry struct {
	entries map[string]ItemInfo
}

func (registry *ItemRegistry) LoadFromMinecraftAssets(assetsPath string, modelDefinitions map[string]BlockModelDefinition, overlayRoots []string, assetNamespaces *assets.AssetNamespaceRegistry) *ItemRegistry {
	if strings.TrimSpace(assetsPath) == "" {
		panic("assetsPath cannot be null or whitespace")
	}

	if modelDefinitions == nil {
		panic("modelDefinitions cannot be null")
	}

	entries := MinecraftAssetsLoadItemInfosFrom(assetsPath, modelDefinitions, overlayRoots, assetNamespaces)
	for _, entry := range entries {
		if strings.TrimSpace(entry.Name) != "" {
			registry.entries[entry.Name] = entry
		}
	}

	return registry
}

func (registry *ItemRegistry) LoadFromFile(path string) error {
	if path == "" {
		return nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var entries []ItemInfo
	if err := global.JSON.Unmarshal(data, &entries); err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Name != "" {
			registry.entries[entry.Name] = entry
		}
	}

	return nil
}

func (registry *ItemRegistry) GetItemInfo(itemName string) *ItemInfo {
	if itemName == "" {
		return nil
	}

	if registry == nil {
		return nil
	}

	if info, exists := registry.entries[itemName]; exists {
		copy := info
		return &copy
	}

	return nil
}

func (registry *ItemRegistry) TryGetModel(itemName string) (string, bool) {
	if info, exists := registry.entries[itemName]; exists && info.Model != nil && strings.TrimSpace(*info.Model) != "" {
		return *info.Model, true
	}

	return "", false
}

func (registry *ItemRegistry) GetAllItemNames() []string {
	if registry == nil {
		return nil
	}

	names := make([]string, 0, len(registry.entries))
	for name := range registry.entries {
		names = append(names, name)
	}
	return names
}

func NewItemRegistry() *ItemRegistry {
	return &ItemRegistry{
		entries: make(map[string]ItemInfo),
	}
}

var ItemRegistryInstance = &ItemRegistry{
	entries: make(map[string]ItemInfo),
}
