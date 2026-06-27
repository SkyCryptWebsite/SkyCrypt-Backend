package data

import (
	"encoding/json"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/assets"
	"os"
	"strings"
)

type BlockRegistry struct {
	_entries map[string]BlockInfo
}

type BlockInfo struct {
	Name       string
	BlockState *string
	Model      *string
	Texture    *string
}

func (_blockRegistry *BlockRegistry) LoadFromFile(path string) *BlockRegistry {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		panic("Block registry file not found: " + path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		panic("Failed to read block registry file: " + err.Error())
	}

	var entries []BlockInfo
	if err := json.Unmarshal(data, &entries); err != nil {
		panic("Failed to parse block registry data from '" + path + "'.")
	}

	registry := &BlockRegistry{
		_entries: make(map[string]BlockInfo),
	}

	for _, entry := range entries {
		if strings.TrimSpace(entry.Name) == "" {
			continue
		}
		registry._entries[entry.Name] = entry
	}

	return registry
}

func (_blockRegistry *BlockRegistry) LoadFromMinecraftAssets(assetsRoot string, modelDefinitions map[string]BlockModelDefinition, overlayRoots []string, assetNamespaces *assets.AssetNamespaceRegistry) *BlockRegistry {
	if strings.TrimSpace(assetsRoot) == "" {
		panic("assetsRoot cannot be null or whitespace")
	}

	if modelDefinitions == nil {
		panic("modelDefinitions cannot be null")
	}

	// fmt.Printf("Loading block infos from Minecraft assets...\n")
	entries := MinecraftAssetLoaderLoadBlockInfos(assetsRoot, modelDefinitions, overlayRoots, assetNamespaces)
	// fmt.Printf("Balls: %+v\n", entries)

	registry := &BlockRegistry{
		_entries: make(map[string]BlockInfo),
	}

	for _, entry := range entries {
		if strings.TrimSpace(entry.Name) == "" {
			continue
		}

		registry._entries[entry.Name] = entry
		// fmt.Printf("Model: %+v\n", entry)
	}

	return registry
}

// public IReadOnlyList<string> GetAllBlockNames() => _entries.Keys.ToList();
func (_blockRegistry *BlockRegistry) TryGetModel(blockName string) (string, bool) {
	if info, exists := _blockRegistry._entries[blockName]; exists && strings.TrimSpace(*info.Model) != "" {
		return *info.Model, true
	}

	return "", false
}

func (_blockRegistry *BlockRegistry) GetAllBlockNames() []string {
	names := make([]string, 0, len(_blockRegistry._entries))
	for name := range _blockRegistry._entries {
		names = append(names, name)
	}
	return names
}

func NewBlockRegistry() *BlockRegistry {
	return &BlockRegistry{
		_entries: make(map[string]BlockInfo),
	}
}

var BlockRegistryInstance = NewBlockRegistry()
