package assets

import (
	"fmt"
	"path"
	"strings"
)

func normalizeProviderRelativePath(relativePath string) (string, error) {
	if strings.TrimSpace(relativePath) == "" {
		return "", nil
	}

	normalized := strings.ReplaceAll(relativePath, "\\", "/")
	normalized = strings.TrimSpace(normalized)
	normalized = strings.TrimLeft(normalized, "/")
	cleaned := path.Clean(normalized)
	if cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path traversal is not allowed: %s", relativePath)
	}

	return cleaned, nil
}

func stripProviderDirectoryPrefix(fullRelativePath string, directoryPrefix string) (string, error) {
	full, err := normalizeProviderRelativePath(fullRelativePath)
	if err != nil {
		return "", err
	}
	prefix, err := normalizeProviderRelativePath(directoryPrefix)
	if err != nil {
		return "", err
	}
	if prefix == "" {
		return full, nil
	}
	if strings.EqualFold(full, prefix) {
		return "", nil
	}
	if len(full) > len(prefix) && strings.EqualFold(full[:len(prefix)], prefix) && full[len(prefix)] == '/' {
		return full[len(prefix)+1:], nil
	}
	return "", fmt.Errorf("path '%s' does not start with directory prefix '%s'", fullRelativePath, directoryPrefix)
}
