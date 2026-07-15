package lib

import "strings"

func NormalizeEnabledPacks(enabledPacks []string) []string {
	if enabledPacks == nil {
		return defaultResourcePackIDs()
	}

	knownPacks := knownResourcePackAliases()
	canonicalIDs := canonicalResourcePackIDs()
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(enabledPacks))
	for _, rawPack := range enabledPacks {
		for _, pack := range strings.Split(rawPack, ",") {
			canonicalPack := canonicalPackAlias(pack)
			if canonicalPack == "" {
				continue
			}
			if _, known := knownPacks[canonicalPack]; !known {
				continue
			}
			if _, duplicate := seen[canonicalPack]; duplicate {
				continue
			}
			seen[canonicalPack] = struct{}{}
			normalized = append(normalized, canonicalIDs[canonicalPack])
		}
	}
	return normalized
}

func canonicalResourcePackIDs() map[string]string {
	canonicalIDs := map[string]string{}
	for _, packID := range defaultResourcePackIDs() {
		canonicalIDs[canonicalPackAlias(packID)] = packID
	}
	return canonicalIDs
}

func enabledPackSet(enabledPackIDs []string) map[string]struct{} {
	enabled := make(map[string]struct{}, len(enabledPackIDs))
	for _, packID := range enabledPackIDs {
		enabled[canonicalPackAlias(packID)] = struct{}{}
	}
	return enabled
}

func packIDsForRenderer(enabledPackIDs []string) []string {
	if enabledPackIDs == nil {
		return nil
	}
	packIDs := make([]string, len(enabledPackIDs))
	copy(packIDs, enabledPackIDs)
	for left, right := 0, len(packIDs)-1; left < right; left, right = left+1, right-1 {
		packIDs[left], packIDs[right] = packIDs[right], packIDs[left]
	}
	return packIDs
}

func knownResourcePackAliases() map[string]struct{} {
	known := map[string]struct{}{}
	for _, packID := range defaultResourcePackIDs() {
		if canonicalPack := canonicalPackAlias(packID); canonicalPack != "" {
			known[canonicalPack] = struct{}{}
		}
	}
	return known
}

func canonicalPackAlias(packID string) string {
	packID = strings.ToLower(strings.TrimSpace(packID))
	packID = strings.ReplaceAll(packID, "-", "_")
	packID = strings.ReplaceAll(packID, " ", "_")
	switch packID {
	case "hypixel_plus", "hplus", "hypixel+":
		return "hplus"
	case "fur_sky_reborn", "fursky_reborn", "fursky", "fsr":
		return "fsr"
	default:
		return packID
	}
}

func texturePackAliases(packID string) []string {
	packID = strings.TrimSpace(packID)
	if packID == "" {
		return nil
	}

	aliases := []string{packID}
	switch canonicalPackAlias(packID) {
	case "hplus":
		aliases = append(aliases, "HYPIXEL_PLUS", "hypixel_plus", "hplus")
	case "fsr":
		aliases = append(aliases, "FSR", "fsr", "FURSKY_REBORN", "fursky_reborn")
	}

	seen := map[string]struct{}{}
	output := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		if _, duplicate := seen[alias]; duplicate {
			continue
		}
		seen[alias] = struct{}{}
		output = append(output, alias)
	}
	return output
}

func sameTexturePack(a string, b string) bool {
	return canonicalPackAlias(a) == canonicalPackAlias(b)
}

func packEnabled(packID string, enabledPacks map[string]struct{}) bool {
	canonicalPack := canonicalPackAlias(packID)
	if canonicalPack == "" {
		return false
	}
	_, enabled := enabledPacks[canonicalPack]
	return enabled
}

func packSignatureFromPackIDs(packIDs []string) string {
	normalized := normalizePackIDs(packIDs)
	if len(normalized) == 0 {
		return "vanilla"
	}
	return orderedPackSignature(normalized)
}

func normalizePackIDs(packIDs []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(packIDs))
	for _, packID := range packIDs {
		packID = strings.TrimSpace(packID)
		if packID == "" {
			continue
		}
		seenKey := canonicalPackAlias(packID)
		if _, duplicate := seen[seenKey]; duplicate {
			continue
		}
		seen[seenKey] = struct{}{}
		normalized = append(normalized, packID)
	}
	return normalized
}

func texturePackEnabled(texturePack string, enabledPacks map[string]struct{}) bool {
	return strings.EqualFold(texturePack, "vanilla") || packEnabled(texturePack, enabledPacks)
}
