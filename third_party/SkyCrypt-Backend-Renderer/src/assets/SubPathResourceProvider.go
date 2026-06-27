package assets

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type SubPathResourceProvider struct {
	inner     ResourceProvider
	prefix    string
	OwnsInner bool
}

func NewSubPathResourceProvider(inner ResourceProvider, prefix string) *SubPathResourceProvider {
	if inner == nil {
		panic("inner cannot be nil")
	}

	normalizedPrefix, err := normalizeSubPath(prefix)
	if err != nil {
		panic(err)
	}
	if normalizedPrefix != "" && !strings.HasSuffix(normalizedPrefix, "/") {
		normalizedPrefix += "/"
	}

	return &SubPathResourceProvider{
		inner:  inner,
		prefix: normalizedPrefix,
	}
}

func (s *SubPathResourceProvider) RootPath() string {
	if s.prefix == "" {
		return s.inner.RootPath()
	}

	return strings.TrimRight(s.inner.RootPath(), "/\\") + "/" + strings.TrimRight(s.prefix, "/")
}

func (s *SubPathResourceProvider) FileExists(path string) bool {
	normalized, err := normalizeSubPath(path)
	if err != nil {
		return false
	}
	return s.inner.FileExists(s.prefix + normalized)
}

func (s *SubPathResourceProvider) DirectoryExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return s.inner.DirectoryExists(strings.TrimRight(s.prefix, "/"))
	}

	normalized, err := normalizeSubPath(path)
	if err != nil {
		return false
	}
	return s.inner.DirectoryExists(s.prefix + normalized)
}

func (s *SubPathResourceProvider) OpenRead(path string) (io.ReadCloser, error) {
	normalized, err := normalizeSubPath(path)
	if err != nil {
		return nil, err
	}
	return s.inner.OpenRead(s.prefix + normalized)
}

func (s *SubPathResourceProvider) EnumerateFiles(dir string, pattern string, recursive bool) ([]string, error) {
	innerDir := strings.TrimRight(s.prefix, "/")
	if strings.TrimSpace(dir) != "" {
		normalized, err := normalizeSubPath(dir)
		if err != nil {
			return nil, err
		}
		innerDir = s.prefix + normalized
	}

	paths, err := s.inner.EnumerateFiles(innerDir, pattern, recursive)
	if err != nil {
		return nil, err
	}

	stripped := s.stripPrefixedPaths(paths)
	sort.Strings(stripped)
	return stripped, nil
}

func (s *SubPathResourceProvider) EnumerateDirectories(dir string, pattern string, recursive bool) ([]string, error) {
	innerDir := strings.TrimRight(s.prefix, "/")
	if strings.TrimSpace(dir) != "" {
		normalized, err := normalizeSubPath(dir)
		if err != nil {
			return nil, err
		}
		innerDir = s.prefix + normalized
	}

	paths, err := s.inner.EnumerateDirectories(innerDir, pattern, recursive)
	if err != nil {
		return nil, err
	}

	stripped := s.stripPrefixedPaths(paths)
	sort.Strings(stripped)
	return stripped, nil
}

func (s *SubPathResourceProvider) Close() error {
	if s.OwnsInner {
		return s.inner.Close()
	}

	return nil
}

func (s *SubPathResourceProvider) stripPrefixedPaths(paths []string) []string {
	if s.prefix == "" {
		return paths
	}

	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if stripped, ok := s.stripPrefix(path); ok {
			result = append(result, stripped)
		}
	}

	return result
}

func (s *SubPathResourceProvider) stripPrefix(path string) (string, bool) {
	if s.prefix == "" {
		return path, true
	}

	if strings.HasPrefix(strings.ToLower(path), strings.ToLower(s.prefix)) {
		return path[len(s.prefix):], true
	}

	return "", false
}

func normalizeSubPath(path string) (string, error) {
	return normalizeProviderRelativePath(path)
}

func (s *SubPathResourceProvider) ReadAllText(path string) (string, error) {
	normalized, err := normalizeSubPath(path)
	if err != nil {
		return "", err
	}
	return s.inner.ReadAllText(s.prefix + normalized)
}

func (s *SubPathResourceProvider) GetRelativePath(fullRelativePath string, directoryPrefix string) (string, error) {
	full, err := normalizeSubPath(fullRelativePath)
	if err != nil {
		return "", err
	}
	prefix, err := normalizeSubPath(directoryPrefix)
	if err != nil {
		return "", err
	}
	stripped, err := stripProviderDirectoryPrefix(full, prefix)
	if err != nil {
		return "", fmt.Errorf("subpath relative path: %w", err)
	}
	return stripped, nil
}
