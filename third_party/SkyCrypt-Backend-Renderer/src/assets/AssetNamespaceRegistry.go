package assets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AssetNamespaceRegistry struct {
	_roots            []*AssetNamespaceRoot
	_rootsByNamespace map[string][]*AssetNamespaceRoot
	_deduplicationSet map[string]struct{}
}

func (r *AssetNamespaceRegistry) AddNamespace(namespace string, path string, sourceId string, isVanilla bool) {
	if strings.TrimSpace(namespace) == "" {
		namespace = "minecraft"
	}

	if strings.TrimSpace(path) == "" {
		return
	}

	fullPath, err := filepath.Abs(path)
	if err != nil {
		return
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return
	}

	provider, err := NewDirectoryResourceProvider(fullPath)
	if err != nil {
		fmt.Printf("Error creating resource provider for path '%s': %v\n", fullPath, err)
		return
	}

	r.AddNamespaceWithProvider(namespace, fullPath, sourceId, isVanilla, provider)
}

func (r *AssetNamespaceRegistry) AddNamespaceWithProvider(namespace string, displayPath string, sourceId string, isVanilla bool, provider ResourceProvider) {
	if strings.TrimSpace(namespace) == "" {
		namespace = "minecraft"
	}

	if strings.TrimSpace(displayPath) == "" {
		return
	}

	identity := fmt.Sprintf("%s|%s", strings.ToLower(namespace), strings.ToLower(displayPath))
	if _, exists := r._deduplicationSet[identity]; exists {
		return
	}

	r._deduplicationSet[identity] = struct{}{}

	root := AssetNamespaceRoot{
		Namespace: namespace,
		Path:      displayPath,
		SourceId:  sourceId,
		IsVanilla: isVanilla,
		Provider:  &provider,
	}

	// fmt.Printf("AddNamespaceWithProvider | Adding namespace '%s' with path '%s' (source: %s, isVanilla: %t)\n", namespace, displayPath, sourceId, isVanilla)

	r._roots = append(r._roots, &root)
	r._rootsByNamespace[namespace] = append(r._rootsByNamespace[namespace], &root)
}

func (_assetNamespaceRegistry *AssetNamespaceRegistry) GetRoots(namespace string, sourceId ...string) []*AssetNamespaceRoot {
	if strings.TrimSpace(namespace) == "" {
		namespace = "minecraft"
	}

	roots, exists := _assetNamespaceRegistry._rootsByNamespace[namespace]
	if !exists {
		return []*AssetNamespaceRoot{}
	}

	if len(sourceId) == 0 {
		return roots
	}

	filter := sourceId[0]
	var results []*AssetNamespaceRoot
	for _, root := range roots {
		if strings.EqualFold(root.SourceId, filter) {
			results = append(results, root)
		}
	}

	return results
}

func (r *AssetNamespaceRegistry) ResolveRoots(namespace string, fallbackToMinecraft bool) []*AssetNamespaceRoot {
	roots := r.GetRoots(namespace)
	if len(roots) == 0 && fallbackToMinecraft && !strings.EqualFold(namespace, "minecraft") {
		// fmt.Printf("Namespace '%s' not found. Falling back to 'minecraft' namespace.\n", namespace)
		return r.GetRoots("minecraft")
	}
	// fmt.Printf("Resolved %d roots for namespace '%s':\n", len(roots), namespace)

	return roots
}

func (r *AssetNamespaceRegistry) EnumerateCandidatePaths(namespace string, relativePath string, preferOverrides bool) []string {
	roots := r.ResolveRoots(namespace, true)
	if len(roots) == 0 {
		return []string{}
	}

	var paths []string
	if preferOverrides {
		for i := len(roots) - 1; i >= 0; i-- {
			paths = append(paths, filepath.Join(roots[i].Path, relativePath))
			// fmt.Printf("Added path: %s\n", paths[len(paths)-1])
		}

		return paths
	}

	for _, root := range roots {
		paths = append(paths, filepath.Join(root.Path, relativePath))
		// fmt.Printf("Added path: %s\n", paths[len(paths)-1])
	}

	return paths
}

func (_assetNamespaceRegistry *AssetNamespaceRegistry) GetSources() []string {
	sources := make([]string, 0)
	seen := make(map[string]struct{})
	for _, root := range _assetNamespaceRegistry._roots {
		if _, exists := seen[root.SourceId]; !exists {
			seen[root.SourceId] = struct{}{}
			sources = append(sources, root.SourceId)
		}
	}

	return sources
}

func (_assetNamespaceRegistry *AssetNamespaceRegistry) Roots() []*AssetNamespaceRoot {

	// for _, root := range _assetNamespaceRegistry._roots {
	// 	if root.Provider == nil {
	// 		// fmt.Printf("Warning: AssetNamespaceRoot with namespace '%s' and path '%s' has a nil provider.\n", root.Namespace, root.Path)
	// 	} else {
	// 		// fmt.Printf("AssetNamespaceRoot: Namespace='%s', Path='%s', SourceId='%s', IsVanilla=%t\n", root.Namespace, root.Path, root.SourceId, root.IsVanilla)
	// 	}
	// }

	return _assetNamespaceRegistry._roots
}

func (r *AssetNamespaceRegistry) GetRelativePath(fullRelativePath string, directoryPrefix string) (string, error) {
	if !strings.HasPrefix(fullRelativePath, directoryPrefix) {
		return "", fmt.Errorf("fullRelativePath '%s' does not start with directoryPrefix '%s'", fullRelativePath, directoryPrefix)
	}

	relativePath := strings.TrimPrefix(fullRelativePath, directoryPrefix)
	relativePath = strings.TrimLeft(relativePath, string(filepath.Separator))

	return relativePath, nil
}

type AssetNamespaceRoot struct {
	Namespace string
	Path      string
	SourceId  string
	IsVanilla bool
	Provider  *ResourceProvider
}

func NewAssetNamespaceRegistry() *AssetNamespaceRegistry {
	return &AssetNamespaceRegistry{
		_roots:            []*AssetNamespaceRoot{},
		_rootsByNamespace: make(map[string][]*AssetNamespaceRoot),
		_deduplicationSet: make(map[string]struct{}),
	}
}
