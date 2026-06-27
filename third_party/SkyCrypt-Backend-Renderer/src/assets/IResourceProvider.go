package assets

import (
	"io"
)

type ResourceProvider interface {
	RootPath() string

	FileExists(path string) bool
	DirectoryExists(path string) bool

	OpenRead(path string) (io.ReadCloser, error)

	EnumerateFiles(dir string, pattern string, recursive bool) ([]string, error)
	EnumerateDirectories(dir string, pattern string, recursive bool) ([]string, error)

	ReadAllText(path string) (string, error)
	GetRelativePath(fullRelativePath string, directoryPrefix string) (string, error)

	// Since Go doesn't have "using" blocks, we satisfy IDisposable
	// by including the standard io.Closer interface.
	io.Closer
}
