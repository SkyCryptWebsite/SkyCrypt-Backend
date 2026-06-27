package assets

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

type CatsResourceProvider struct {
	_catsFile       *CatsFile
	_prefix         string // e.g. "" or "fsr_item_melee/"
	_rootPath       string
	_fileIndex      map[string]*CatsFileEntry
	_directoryIndex map[string]struct{}
}

// NewCatsResourceProvider creates a provider backed by a .cats archive.
// Optional prefix can be provided as a third variadic argument.
func NewCatsResourceProvider(catsFile *CatsFile, displayPath string, optionalPrefix ...string) *CatsResourceProvider {
	if catsFile == nil {
		panic("catsFile cannot be nil")
	}

	var prefix string
	if len(optionalPrefix) > 0 {
		prefix = optionalPrefix[0]
	}

	normalizedPrefix, err := normalizeSubPath(prefix)
	if err != nil {
		panic(err)
	}
	if normalizedPrefix != "" && !strings.HasSuffix(normalizedPrefix, "/") {
		normalizedPrefix += "/"
	}

	files, dirs := buildIndex(catsFile, normalizedPrefix)

	return &CatsResourceProvider{
		_catsFile:       catsFile,
		_prefix:         normalizedPrefix,
		_rootPath:       displayPath,
		_fileIndex:      files,
		_directoryIndex: dirs,
	}
}

func (c *CatsResourceProvider) RootPath() string {
	return c._rootPath
}

func (c *CatsResourceProvider) FileExists(relativePath string) bool {
	normalized, err := normalizePath(relativePath)
	if err != nil {
		return false
	}
	_, ok := c._fileIndex[normalized]
	return ok
}

func (c *CatsResourceProvider) DirectoryExists(relativePath string) bool {
	if strings.TrimSpace(relativePath) == "" {
		return true
	}

	normalized, err := normalizePath(relativePath)
	if err != nil {
		return false
	}

	normalized = strings.TrimRight(normalized, "/")
	_, ok := c._directoryIndex[normalized]
	return ok
}

func (c *CatsResourceProvider) OpenRead(relativePath string) (io.ReadCloser, error) {
	normalized, err := normalizePath(relativePath)
	if err != nil {
		return nil, err
	}

	entry, ok := c._fileIndex[normalized]
	if !ok {
		return nil, fmt.Errorf("file not found in .cats archive: '%s'", relativePath)
	}

	rs, err := c._catsFile.OpenStream(entry)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(rs), nil
}

func (c *CatsResourceProvider) EnumerateFiles(directory, searchPattern string, recursive bool) ([]string, error) {
	prefix := strings.TrimRight(strings.ReplaceAll(directory, "\\", "/"), "/")
	if prefix == "." {
		prefix = ""
	}

	pattern, err := globToRegex(searchPattern)
	if err != nil {
		return nil, err
	}

	var results []string
	for path := range c._fileIndex {
		if !isWithinDirectory(path, prefix, recursive) {
			continue
		}

		last := strings.LastIndex(path, "/")
		name := path
		if last >= 0 {
			name = path[last+1:]
		}

		if pattern.MatchString(name) {
			results = append(results, path)
		}
	}

	sort.Strings(results)
	return results, nil
}

func (c *CatsResourceProvider) EnumerateDirectories(directory, searchPattern string, recursive bool) ([]string, error) {
	prefix := strings.TrimRight(strings.ReplaceAll(directory, "\\", "/"), "/")
	if prefix == "." {
		prefix = ""
	}

	pattern, err := globToRegex(searchPattern)
	if err != nil {
		return nil, err
	}

	var results []string
	for dir := range c._directoryIndex {
		if !isWithinDirectory(dir, prefix, recursive) {
			continue
		}

		last := strings.LastIndex(dir, "/")
		name := dir
		if last >= 0 {
			name = dir[last+1:]
		}

		if pattern.MatchString(name) {
			results = append(results, dir)
		}
	}

	sort.Strings(results)
	return results, nil
}

func (c *CatsResourceProvider) Close() error {
	return nil
}

func (c *CatsResourceProvider) GetRelativePath(fullRelativePath string, directoryPrefix string) (string, error) {
	normalized, err := normalizePath(fullRelativePath)
	if err != nil {
		return "", err
	}
	prefix, err := normalizePath(directoryPrefix)
	if err != nil {
		return "", err
	}
	return stripProviderDirectoryPrefix(normalized, prefix)
}

func (c *CatsResourceProvider) ReadAllText(path string) (string, error) {
	rs, err := c.OpenRead(path)
	if err != nil {
		return "", err
	}
	defer rs.Close()

	data, err := io.ReadAll(rs)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// --- Indexing and helpers ---

func buildIndex(catsFile *CatsFile, prefix string) (map[string]*CatsFileEntry, map[string]struct{}) {
	files := make(map[string]*CatsFileEntry)
	dirs := make(map[string]struct{})

	// Determine start directory according to prefix
	var startDir *CatsDirectoryEntry

	if prefix != "" {
		trimmed := strings.TrimRight(prefix, "/")
		entry := catsFile.GetEntry(trimmed)
		if entry == nil {
			return files, dirs
		}
		dir, ok := entry.(*CatsDirectoryEntry)
		if !ok {
			return files, dirs
		}
		startDir = dir
	} else {
		entry := catsFile.GetEntry("")
		if entry == nil {
			return files, dirs
		}
		dir, ok := entry.(*CatsDirectoryEntry)
		if !ok {
			return files, dirs
		}
		startDir = dir
	}

	indexDirectory(startDir, "", files, dirs)
	return files, dirs
}

func indexDirectory(dir *CatsDirectoryEntry, parentPath string, files map[string]*CatsFileEntry, dirs map[string]struct{}) {
	for name, entry := range dir.Children {
		var rawPath string
		if parentPath != "" {
			rawPath = parentPath + "/" + name
		} else {
			rawPath = name
		}
		path, err := normalizePath(rawPath)
		if err != nil {
			continue
		}

		switch e := entry.(type) {
		case *CatsFileEntry:
			files[path] = e
			indexParentDirectories(path, dirs)
		case *CatsDirectoryEntry:
			dirs[path] = struct{}{}
			indexDirectory(e, path, files, dirs)
		}
	}
}

func indexParentDirectories(path string, dirs map[string]struct{}) {
	last := strings.LastIndex(path, "/")
	for last > 0 {
		parent := path[:last]
		if _, exists := dirs[parent]; exists {
			break
		}
		dirs[parent] = struct{}{}
		last = strings.LastIndex(parent, "/")
	}
}

func normalizePath(path string) (string, error) {
	normalized, err := normalizeProviderRelativePath(path)
	if err != nil {
		return "", errors.New(err.Error())
	}
	return normalized, nil
}

func isWithinDirectory(path, prefix string, recursive bool) bool {
	if prefix == "" {
		if recursive {
			return true
		}
		return !strings.Contains(path, "/")
	}

	// prefix match (case-insensitive)
	if len(path) <= len(prefix) {
		return false
	}
	if !strings.EqualFold(path[:len(prefix)], prefix) {
		return false
	}
	if path[len(prefix)] != '/' {
		return false
	}

	if !recursive {
		remaining := path[len(prefix)+1:]
		return !strings.Contains(remaining, "/")
	}

	return true
}

func globToRegex(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		pattern = "*"
	}

	// Escape then replace the glob tokens
	esc := regexp.QuoteMeta(pattern)
	esc = strings.ReplaceAll(esc, "\\*", ".*")
	esc = strings.ReplaceAll(esc, "\\?", ".")
	// Case-insensitive
	full := "(?i)^" + esc + "$"
	return regexp.Compile(full)
}
