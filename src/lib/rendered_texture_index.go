package lib

import "strings"

func rememberRenderedSkyBlockID(id string, packID string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	packID = strings.TrimSpace(packID)
	if packID == "" {
		packID = defaultPackSignature()
	}

	renderedSkyBlockIndexMu.Lock()
	renderedSkyBlockIndex[renderedSkyBlockIndexKey(packID, id)] = struct{}{}
	renderedSkyBlockIndexMu.Unlock()
}

func renderedSkyBlockIDKnown(id string, packID string) bool {
	id = strings.TrimSpace(id)
	packID = strings.TrimSpace(packID)
	if id == "" || packID == "" {
		return false
	}

	renderedSkyBlockIndexMu.RLock()
	_, known := renderedSkyBlockIndex[renderedSkyBlockIndexKey(packID, id)]
	renderedSkyBlockIndexMu.RUnlock()
	if known {
		return true
	}

	if strings.Contains(packID, ",") {
		_, known = cachedTextureByKey(textureCacheKey(packID, "skyblock:"+id), nil)
		return known
	}
	for _, alias := range texturePackAliases(packID) {
		texture, cached := cachedTextureByKey(textureCacheKey(alias, "skyblock:"+id), nil)
		if cached && sameTexturePack(texture.TexturePack, packID) {
			return true
		}
	}
	return false
}

func renderedSkyBlockIndexKey(packID string, id string) string {
	if !strings.Contains(packID, ",") {
		packID = canonicalPackAlias(packID)
	}
	return strings.TrimSpace(packID) + "|" + strings.TrimSpace(id)
}
