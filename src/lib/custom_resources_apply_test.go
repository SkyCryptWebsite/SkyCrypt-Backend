package lib

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEnsureResourcePackModelIndexBuildsOnceUnderConcurrency(t *testing.T) {
	root := t.TempDir()
	packDir := filepath.Join(root, "pack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "meta.json"), []byte(`{"id":"TEST"}`), 0644); err != nil {
		t.Fatal(err)
	}

	resourcePackModelIndex.RLock()
	previousRoot := resourcePackModelIndex.root
	previousModels := resourcePackModelIndex.models
	resourcePackModelIndex.RUnlock()
	previousBuilder := buildResourcePackModelIndexFunc
	t.Cleanup(func() {
		resourcePackModelIndex.Lock()
		resourcePackModelIndex.root = previousRoot
		resourcePackModelIndex.models = previousModels
		resourcePackModelIndex.Unlock()
		buildResourcePackModelIndexFunc = previousBuilder
	})
	resourcePackModelIndex.Lock()
	resourcePackModelIndex.root = ""
	resourcePackModelIndex.models = nil
	resourcePackModelIndex.Unlock()

	var builds atomic.Int32
	buildResourcePackModelIndexFunc = func(path string) (map[string]map[string]struct{}, error) {
		builds.Add(1)
		time.Sleep(10 * time.Millisecond)
		return buildResourcePackModelIndex(path)
	}

	const callers = 32
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- ensureResourcePackModelIndex(root)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := builds.Load(); got != 1 {
		t.Fatalf("build count = %d, want 1", got)
	}
	resourcePackModelIndex.RLock()
	defer resourcePackModelIndex.RUnlock()
	if resourcePackModelIndex.models["test"] == nil {
		t.Fatalf("index was not fully initialized: %#v", resourcePackModelIndex.models)
	}
}

func TestEnsureResourcePackModelIndexRetriesAfterBuildError(t *testing.T) {
	resourcePackModelIndex.RLock()
	previousRoot := resourcePackModelIndex.root
	previousModels := resourcePackModelIndex.models
	resourcePackModelIndex.RUnlock()
	previousBuilder := buildResourcePackModelIndexFunc
	t.Cleanup(func() {
		resourcePackModelIndex.Lock()
		resourcePackModelIndex.root = previousRoot
		resourcePackModelIndex.models = previousModels
		resourcePackModelIndex.Unlock()
		buildResourcePackModelIndexFunc = previousBuilder
	})
	resourcePackModelIndex.Lock()
	resourcePackModelIndex.root = ""
	resourcePackModelIndex.models = nil
	resourcePackModelIndex.Unlock()

	var builds atomic.Int32
	buildResourcePackModelIndexFunc = func(string) (map[string]map[string]struct{}, error) {
		builds.Add(1)
		return nil, os.ErrNotExist
	}
	done := make(chan error, 1)
	go func() {
		done <- ensureResourcePackModelIndex(filepath.Join(t.TempDir(), "missing"))
	}()
	select {
	case err := <-done:
		if !os.IsNotExist(err) {
			t.Fatalf("error = %v, want os.ErrNotExist", err)
		}
	case <-time.After(time.Second):
		t.Fatal("index build did not return after an error")
	}
	if got := builds.Load(); got != 1 {
		t.Fatalf("build count = %d, want 1", got)
	}
}
