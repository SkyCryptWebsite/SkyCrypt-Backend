package texturepacks

import (
	"os"
	"path/filepath"
	"testing"
)

func texturePackTestRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			t.Fatal("could not locate repository root")
		}
		cwd = parent
	}
}

func TestRequiredCatharsisPacksRegister(t *testing.T) {
	root := filepath.Join(texturePackTestRepoRoot(t), "texturepacks")
	for _, packDir := range []string{"fsr", "hplus"} {
		t.Run(packDir, func(t *testing.T) {
			path := filepath.Join(root, packDir)
			if _, err := os.Stat(filepath.Join(path, "meta.json")); err != nil {
				t.Fatalf("required texture pack fixture %q is missing: %v", packDir, err)
			}

			registry := NewTexturePackRegistry()
			pack, err := registry.RegisterPack(path)
			if err != nil {
				t.Fatalf("RegisterPack returned error: %v", err)
			}
			if pack.Id == "" {
				t.Fatal("registered pack has empty id")
			}
			if pack.Provider == nil {
				t.Fatal("archive-backed pack did not expose a resource provider")
			}
			if !pack.IsCatharsisPack {
				t.Fatalf("pack %q should be detected as a Catharsis pack", packDir)
			}
			if len(pack.CatharsisOverlays) == 0 {
				t.Fatalf("pack %q did not expose enabled Catharsis overlays", packDir)
			}
			if len(pack.OverlayNamespaceProviders) == 0 {
				t.Fatalf("pack %q did not expose overlay namespace providers", packDir)
			}
			if _, ok := registry.Packs[pack.Id]; !ok {
				t.Fatalf("pack id %q was not recorded in registry", pack.Id)
			}
		})
	}
}

func TestRegisterAllPacksFindsRequiredPacks(t *testing.T) {
	root := filepath.Join(texturePackTestRepoRoot(t), "texturepacks")
	registry := NewTexturePackRegistry()
	packs := registry.RegisterAllPacks(root, false)
	if len(packs) < 2 {
		t.Fatalf("registered packs = %d, want at least 2", len(packs))
	}
	for _, id := range []string{"fsr", "hplus"} {
		if _, ok := registry.Packs[id]; !ok {
			t.Fatalf("RegisterAllPacks did not register %q", id)
		}
	}
	if len(registry.RegistrationSources) == 0 {
		t.Fatal("RegisterAllPacks did not record registration source")
	}
}
