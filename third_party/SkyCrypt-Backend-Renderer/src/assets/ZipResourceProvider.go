package assets

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

// ZipResourceProvider reads all entries from a ZIP archive into memory.
// It implements the ResourceProvider interface.
type ZipResourceProvider struct {
	rootPath    string
	entryData   map[string][]byte
	directories map[string]struct{}
}

// NewZipResourceProvider opens a zip file, reads all entries into memory, and closes the archive.
func NewZipResourceProvider(zipFilePath string) *ZipResourceProvider {
	if strings.TrimSpace(zipFilePath) == "" {
		panic("zipFilePath cannot be empty")
	}

	abs, err := filepath.Abs(zipFilePath)
	if err != nil {
		panic(err)
	}

	zr, err := zip.OpenReader(abs)
	if err != nil {
		panic(err)
	}
	defer zr.Close()

	entries, dirs, err := buildIndexFromFiles(zr.File)
	if err != nil {
		panic(err)
	}

	return &ZipResourceProvider{
		rootPath:    abs,
		entryData:   entries,
		directories: dirs,
	}
}

// NewZipResourceProviderFromReader reads all entries from an existing zip.ReadCloser into memory.
// If ownsArchive is true the provided archive will be closed after reading.
func NewZipResourceProviderFromReader(archive *zip.ReadCloser, displayPath string, ownsArchive bool) (*ZipResourceProvider, error) {
	if archive == nil {
		return nil, errors.New("archive cannot be nil")
	}

	entries, dirs, err := buildIndexFromFiles(archive.File)
	if err != nil {
		if ownsArchive {
			archive.Close()
		}
		return nil, err
	}

	if ownsArchive {
		archive.Close()
	}

	return &ZipResourceProvider{
		rootPath:    displayPath,
		entryData:   entries,
		directories: dirs,
	}, nil
}

func (z *ZipResourceProvider) RootPath() string {
	return z.rootPath
}

func (z *ZipResourceProvider) FileExists(relativePath string) bool {
	normalized, err := normalizePath(relativePath)
	if err != nil {
		return false
	}
	_, ok := z.entryData[normalized]
	return ok
}

func (z *ZipResourceProvider) DirectoryExists(relativePath string) bool {
	if strings.TrimSpace(relativePath) == "" {
		return true
	}
	normalized, err := normalizePath(relativePath)
	if err != nil {
		return false
	}
	normalized = strings.TrimRight(normalized, "/")
	_, ok := z.directories[normalized]
	return ok
}

func (z *ZipResourceProvider) OpenRead(relativePath string) (io.ReadCloser, error) {
	normalized, err := normalizePath(relativePath)
	if err != nil {
		return nil, err
	}
	data, ok := z.entryData[normalized]
	if !ok {
		return nil, fmt.Errorf("file not found in ZIP archive: '%s'", relativePath)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (z *ZipResourceProvider) EnumerateFiles(directory, searchPattern string, recursive bool) ([]string, error) {
	prefix := strings.TrimRight(strings.ReplaceAll(directory, "\\", "/"), "/")
	if prefix == "." {
		prefix = ""
	}

	pattern, err := globToRegex(searchPattern)
	if err != nil {
		return nil, err
	}

	var results []string
	for path := range z.entryData {
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

func (z *ZipResourceProvider) EnumerateDirectories(directory, searchPattern string, recursive bool) ([]string, error) {
	prefix := strings.TrimRight(strings.ReplaceAll(directory, "\\", "/"), "/")
	if prefix == "." {
		prefix = ""
	}

	pattern, err := globToRegex(searchPattern)
	if err != nil {
		return nil, err
	}

	var results []string
	for dir := range z.directories {
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

func (z *ZipResourceProvider) Close() error {
	return nil
}

// --- helpers ---

func buildIndexFromFiles(files []*zip.File) (map[string][]byte, map[string]struct{}, error) {
	entries := make(map[string][]byte)
	dirs := make(map[string]struct{})

	for _, f := range files {
		path, err := normalizePath(f.Name)
		if err != nil {
			continue
		}
		if path == "" {
			continue
		}

		if strings.HasSuffix(path, "/") || f.FileInfo().IsDir() {
			dirPath := strings.TrimRight(path, "/")
			dirs[dirPath] = struct{}{}
			indexParentDirectories(dirPath, dirs)
			continue
		}

		data, err := readEntryBytes(f)
		if err != nil {
			return nil, nil, err
		}
		entries[path] = data
		indexParentDirectories(path, dirs)
	}

	return entries, dirs, nil
}

func readEntryBytes(entry *zip.File) ([]byte, error) {
	r, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (z *ZipResourceProvider) GetRelativePath(fullRelativePath string, directoryPrefix string) (string, error) {
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

func (z *ZipResourceProvider) ReadAllText(path string) (string, error) {
	reader, err := z.OpenRead(path)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
