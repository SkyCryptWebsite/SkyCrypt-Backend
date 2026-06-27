package assets

import (
	"io"
	"strings"
)

type ResourceProviderExtensions struct{}

func (r *ResourceProviderExtensions) ReadAllText(provider ResourceProvider, relativePath string) (string, error) {
	stream, err := provider.OpenRead(relativePath)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (r *ResourceProviderExtensions) GetRelativePath(fullRelativePath, directoryPrefix string) string {
	prefix := strings.TrimRight(directoryPrefix, "/")
	if len(prefix) == 0 {
		return fullRelativePath
	}

	if len(fullRelativePath) > len(prefix) &&
		fullRelativePath[len(prefix)] == '/' &&
		strings.EqualFold(fullRelativePath[:len(prefix)], prefix) {
		return fullRelativePath[len(prefix)+1:]
	}

	return fullRelativePath
}

var ResourceProviderExtensionsInstance = &ResourceProviderExtensions{}
