package texturepacks

import (
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/assets"
	"time"
)

type OverlayNamespaceProvider struct {
	Namespace   string
	DisplayPath string
	Provider    assets.ResourceProvider
}

type RegisteredResourcePack struct {
	Id               string
	DisplayName      string
	RootPath         string
	AssetsPath       string
	NamespaceRoots   map[string]string
	Meta             ResourcePackMeta
	LastWriteTimeUtc time.Time
	SizeBytes        int64
	SupportsCit      bool
	Fingerprint      string

	Provider                  assets.ResourceProvider
	NamespaceProviders        map[string]assets.ResourceProvider
	IsCatharsisPack           bool
	CatharsisOverlays         []string
	OverlayNamespaceProviders []OverlayNamespaceProvider
}

func (pack *RegisteredResourcePack) TryGetNamespacePath(namespace string) (string, bool) {
	if resolved, found := pack.NamespaceRoots[namespace]; found {
		return resolved, true
	}

	return "", false
}

func (pack *RegisteredResourcePack) EnumerateOverlayRootPaths() []string {
	emitted := make(map[string]struct{})

	var result []string

	for _, namespaceName := range []string{"minecraft", "firmskyblock", "cittofirmgenerated", "cit"} {
		if namespacePath, found := pack.NamespaceRoots[namespaceName]; found {
			if _, alreadyEmitted := emitted[namespacePath]; !alreadyEmitted {
				result = append(result, namespacePath)
				emitted[namespacePath] = struct{}{}
			}
		}
	}

	for _, namespacePath := range pack.NamespaceRoots {
		if _, alreadyEmitted := emitted[namespacePath]; !alreadyEmitted {
			result = append(result, namespacePath)
			emitted[namespacePath] = struct{}{}
		}
	}

	return result
}
