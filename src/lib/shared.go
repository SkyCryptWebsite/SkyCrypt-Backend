package lib

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var appRootOnce sync.Once
var appRootPath string
var appRootErr error

var CACHE_DIR = defaultCacheDir()
var CACHE_SUBDIRS = []string{
	filepath.Join(CACHE_DIR, "heads"),
	filepath.Join(CACHE_DIR, "leather"),
	filepath.Join(CACHE_DIR, "potions"),
}

func defaultCacheDir() string {
	root, err := appRootDir()
	if err != nil {
		return "cache"
	}

	return filepath.Join(root, "cache")
}

func appRootDir() (string, error) {
	appRootOnce.Do(func() {
		appRootPath, appRootErr = findAppRoot()
	})

	return appRootPath, appRootErr
}

func findAppRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	for {
		if dirExists(filepath.Join(cwd, "assets", "resourcepacks")) {
			return cwd, nil
		}

		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("failed to find app root from %s", cwd)
		}
		cwd = parent
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func init() {
	if _, err := os.Stat(CACHE_DIR); os.IsNotExist(err) {
		err := os.MkdirAll(CACHE_DIR, 0755)
		if err != nil {
			panic("Failed to create cache directory: " + err.Error())
		}
	}
	for _, dir := range CACHE_SUBDIRS {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				panic("Failed to create subdirectory: " + dir + ": " + err.Error())
			}
		}
	}
}
