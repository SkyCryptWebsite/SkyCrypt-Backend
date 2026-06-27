package texturepacks

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/assets"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/global"
)

type TexturePackRegistry struct {
	Packs               map[string]RegisteredResourcePack
	RegistrationSources []RegistrationSource
}

func NewTexturePackRegistry() *TexturePackRegistry {
	return &TexturePackRegistry{
		Packs:               make(map[string]RegisteredResourcePack),
		RegistrationSources: []RegistrationSource{},
	}
}

type RegistrationSource struct {
	Path               string
	SearchRecursively  bool
	RegisterSinglePack bool
}

type PackRegistrationFailure struct {
	Directory string
	Reason    string
	Err       error
}

func (texturePackRegistry *TexturePackRegistry) RegisterPack(directory string) (RegisteredResourcePack, error) {
	pack := texturePackRegistry.RegisterPackCore(directory)
	texturePackRegistry.RecordRegistrationSource(RegistrationSource{
		Path:               pack.RootPath,
		SearchRecursively:  false,
		RegisterSinglePack: true,
	})
	return pack, nil
}

func (texturePackRegistry *TexturePackRegistry) RecordRegistrationSource(source RegistrationSource) {
	for _, existing := range texturePackRegistry.RegistrationSources {
		if existing == source {
			return
		}
	}

	texturePackRegistry.RegistrationSources = append(texturePackRegistry.RegistrationSources, source)
}

func (texturePackRegistry *TexturePackRegistry) RegisterPackCore(directory string) RegisteredResourcePack {
	if directory == "" {
		panic("directory cannot be null or whitespace")
	}

	if _, err := os.Stat(directory); os.IsNotExist(err) {
		panic("texture pack directory not found: '" + directory + "'.")
	}

	metaPath := directory + "/meta.json"
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		panic("texture pack meta.json file not found: '" + metaPath + "'.")
	}

	metaDescriptor := texturePackRegistry.LoadMeta(metaPath)
	if metaDescriptor.Id == "" {
		panic("texture pack '" + directory + "' is missing a valid 'id' field in meta.json.")
	}

	catharsisConfigOverrides := texturePackRegistry.NormalizeCatharsisConfigOverrides(metaDescriptor.CatharsisConfig)

	// Detect archive-backed pack: .cats, .cats.zip, or .zip sitting alongside meta.json
	archive := texturePackRegistry.FindPackArchive(directory)

	var provider assets.ResourceProvider
	var namespaceProviders map[string]assets.ResourceProvider
	var namespaceRoots map[string]string
	var assetsPath string
	var sizeBytes int64
	var packFormat *int
	var supportsCit bool
	var catharsisOverlays []string
	var overlayNsProvidersList []OverlayNamespaceProvider
	var catharsisConfigJson string

	if archive != nil {
		var catsFile *assets.CatsFile
		switch archive.Kind {
		case "CatsFile":
			data, err := os.ReadFile(archive.Path)
			if err != nil {
				panic("Failed to read .cats file: " + err.Error())
			}

			catsFile, err = assets.NewCatsFile(data)
			if err != nil {
				panic("Failed to read .cats file: " + err.Error())
			}

			provider = assets.NewCatsResourceProvider(catsFile, archive.Path)
			// Console.WriteLine($"Registered archive-backed texture pack from '{archive.Path}' (kind={archive.Kind}).");
			// fmt.Printf("Registered archive-backed texture pack from '%s' (kind=%s).\n", archive.Path, archive.Kind)
		case "CatsZip":
			zipStream, err := os.Open(archive.Path)
			if err != nil {
				panic("Failed to open .cats.zip file: " + err.Error())
			}
			defer zipStream.Close()

			fileInfo, err := zipStream.Stat()
			if err != nil {
				panic("Failed to stat .cats.zip file: " + err.Error())
			}

			zipArchive, err := zip.NewReader(zipStream, fileInfo.Size())
			if err != nil {
				panic("Failed to read .cats.zip archive: " + err.Error())
			}

			var packEntry *zip.File
			for _, entry := range zipArchive.File {
				if entry.Name == "pack.cats" {
					packEntry = entry
					break
				}
			}
			if packEntry == nil {
				panic(".cats.zip file at '" + archive.Path + "' does not contain pack.cats.")
			}

			entryStream, err := packEntry.Open()
			if err != nil {
				panic("Failed to open pack.cats entry in .cats.zip: " + err.Error())
			}
			defer entryStream.Close()

			memoryStream := new(bytes.Buffer)
			_, err = io.Copy(memoryStream, entryStream)
			if err != nil {
				panic("Failed to read pack.cats from .cats.zip: " + err.Error())
			}

			catsFile, err = assets.NewCatsFile(memoryStream.Bytes())
			if err != nil {
				panic("Failed to parse pack.cats from .cats.zip: " + err.Error())
			}
			provider = assets.NewCatsResourceProvider(catsFile, archive.Path)
			// Console.WriteLine($"Registered archive-backed texture pack from '{archive.Path}' (kind={archive.Kind}).");
			// fmt.Printf("Registered archive-backed texture pack from '%s' (kind=%s).\n", archive.Path, archive.Kind)
			// fmt.Printf("catFile: %v, catProvider: %v\n ", catsFile != nil, true)
		case "Zip":
			provider = assets.NewZipResourceProvider(archive.Path)
			// Console.WriteLine($"Registered archive-backed texture pack from '{archive.Path}' (kind={archive.Kind}).");
			// fmt.Printf("Registered archive-backed texture pack from '%s' (kind=%s).\n", archive.Path, archive.Kind)
		default:
			panic("Unexpected archive kind: " + archive.Kind)
		}

		// Detect if this is a catharsis pack: either a .cats binary, or a ZIP whose
		// pack.mcmeta declares the catharsis:pack/v1 section.
		var mcmetaJsonForCatharsis string
		if provider.FileExists("pack.mcmeta") {
			mcmetaJsonForCatharsis, _ = provider.ReadAllText("pack.mcmeta")
		}

		isCatharsisPack := provider != nil && (archive.Kind == "CatsFile" || (mcmetaJsonForCatharsis != "" && IsCatharsisPackMcmeta(mcmetaJsonForCatharsis)))
		namespaceRoots, namespaceProviders = ResolveNamespaceRootsFromProvider(provider, directory, !isCatharsisPack)

		assetsPath := namespaceRoots["minecraft"]
		if !isCatharsisPack && assetsPath == "" {
			panic("Texture pack archive at '" + archive.Path + "' does not contain an 'assets/minecraft' directory.")
		}

		data, err := os.Stat(archive.Path)
		if err != nil {
			panic("Failed to stat archive file: " + err.Error())
		}

		sizeBytes = data.Size()
		if mcmetaJsonForCatharsis != "" {
			packFormat = ParsePackFormatFromJson(mcmetaJsonForCatharsis)
		}

		if provider.FileExists("config.catharsis.json") {
			catharsisConfigJson, _ = provider.ReadAllText("config.catharsis.json")
		}

		supportsCit = metaDescriptor.SupportsCit || namespaceRoots["cit"] != "" || provider.DirectoryExists("assets/minecraft/optifine/cit")

		// fmt.Printf("isCatharsisPack: %v, packFormat: %v, supportsCit: %v, mcmetaJsonForCatharsis: %v\n", isCatharsisPack, packFormat, supportsCit, mcmetaJsonForCatharsis != "")

		// For catharsis packs, resolve overlay directories from embedded config
		if isCatharsisPack && mcmetaJsonForCatharsis != "" {
			catharsisOverlays = CatharsisPackConfig.ResolveEnabledOverlays(mcmetaJsonForCatharsis, &catharsisConfigJson, catharsisConfigOverrides, false)

			// Console.WriteLine($"Detected catharsis pack with {catharsisOverlays.Count} enabled overlay(s) in '{archive.Path}'.");
			// fmt.Printf("Detected catharsis pack with %d enabled overlay(s) in '%s'.\n", len(catharsisOverlays), archive.Path)

			if len(catharsisOverlays) > 0 {
				var overlayProviders []OverlayNamespaceProvider

				// Detect if the archive uses a prefix for overlay directories.
				// Catharsis packs may store overlays as e.g. "fsr_item_melee" in the
				// archive while pack.mcmeta references them as just "item_melee".
				overlayDirPrefix := DetectCatharsisOverlayPrefix(provider, catharsisOverlays)

				for _, overlayDir := range catharsisOverlays {
					actualDir := overlayDir
					if overlayDirPrefix != nil {
						actualDir = *overlayDirPrefix + overlayDir
					}

					// For .cats archives use CatsResourceProvider; for plain ZIPs use SubPathResourceProvider.
					var overlayProvider assets.ResourceProvider
					if archive.Kind == "CatsFile" {
						overlayProvider = assets.NewCatsResourceProvider(catsFile, archive.Path+"/"+actualDir, actualDir)
					} else {
						overlayProvider = assets.NewSubPathResourceProvider(provider, actualDir)
					}

					if !overlayProvider.DirectoryExists("assets") {
						continue
					}

					_, nsProviders := ResolveNamespaceRootsFromProvider(overlayProvider, directory+"/"+actualDir, false)
					for ns, nsProvider := range nsProviders {
						overlayProviders = append(overlayProviders, OverlayNamespaceProvider{
							Namespace:   ns,
							DisplayPath: directory + "/" + actualDir + "/assets/" + ns,
							Provider:    nsProvider,
						})
					}
				}

				overlayNsProvidersList = overlayProviders
			}
		}

		// For catharsis packs, determine assetsPath from base or overlays
		if isCatharsisPack && assetsPath == "" {
			if assetsPath = namespaceRoots["minecraft"]; assetsPath != "" {
				// Base provider has minecraft namespace
			} else {
				// Check if any overlay provides the minecraft namespace
				var mcOverlay *OverlayNamespaceProvider
				for _, o := range overlayNsProvidersList {
					if o.Namespace == "minecraft" {
						mcOverlay = &o
						break
					}
				}
				if mcOverlay != nil {
					assetsPath = mcOverlay.DisplayPath
				} else {
					panic("Catharsis pack at '" + archive.Path + "' does not provide 'minecraft' namespace in either base assets or enabled overlays.")
				}
			}
		}
	} else {
		// Traditional directory-based pack
		namespaceRoots = ResolveNamespaceRoots(directory)
		assetsPath = namespaceRoots["minecraft"]
		if assetsPath == "" {
			panic("Texture pack at '" + directory + "' does not contain an 'assets/minecraft' directory.")
		}

		packMcMetaPath := directory + "/pack.mcmeta"
		if _, err := os.Stat(packMcMetaPath); err == nil {
			packFormat = ParsePackForm(packMcMetaPath)
		}

		sizeBytes = 0
		for _, path := range namespaceRoots {
			data, err := os.Stat(path)
			if err != nil {
				panic("Failed to stat namespace directory: " + err.Error())
			}

			sizeBytes += data.Size()
		}

		supportsCit = metaDescriptor.SupportsCit || namespaceRoots["cit"] != "" || func() bool {
			citPath := assetsPath + "/optifine/cit"
			if _, err := os.Stat(citPath); err == nil {
				return true
			}
			return false
		}()
	}

	data, err := os.Stat(directory)
	if err != nil {
		panic("Failed to stat pack directory: " + err.Error())
	}

	lastWriteTimeUtc := data.ModTime()

	resourceMeta := ResourcePackMeta{
		Id:          metaDescriptor.Id,
		Name:        metaDescriptor.Name,
		Version:     metaDescriptor.Version,
		Description: metaDescriptor.Description,
		Authors:     metaDescriptor.Authors,
		DownloadUrl: &metaDescriptor.DownloadUrl,
		SupportsCit: supportsCit,
		PackFormat:  packFormat,
	}

	fingerprint := ComputeFingerprint(resourceMeta.Id, resourceMeta.Version, lastWriteTimeUtc, sizeBytes)

	registered := RegisteredResourcePack{
		Id:                        resourceMeta.Id,
		DisplayName:               resourceMeta.Name,
		RootPath:                  directory,
		AssetsPath:                assetsPath,
		NamespaceRoots:            namespaceRoots,
		Meta:                      resourceMeta,
		LastWriteTimeUtc:          lastWriteTimeUtc,
		SizeBytes:                 sizeBytes,
		SupportsCit:               supportsCit,
		Fingerprint:               fingerprint,
		Provider:                  provider,
		NamespaceProviders:        namespaceProviders,
		IsCatharsisPack:           len(catharsisOverlays) > 0,
		CatharsisOverlays:         catharsisOverlays,
		OverlayNamespaceProviders: overlayNsProvidersList,
	}

	if _, exists := texturePackRegistry.Packs[resourceMeta.Id]; exists {
		panic("A texture pack with id '" + resourceMeta.Id + "' has already been registered.")
	}

	texturePackRegistry.Packs[resourceMeta.Id] = registered

	return registered
}

func (texturePackRegistry *TexturePackRegistry) LoadMeta(path string) MetaDescriptor {
	data, err := os.ReadFile(path)
	if err != nil {
		panic("Failed to read texture pack metadata from '" + path + "': " + err.Error())
	}

	var descriptor MetaDescriptor
	err = global.JSON.Unmarshal(data, &descriptor)
	if err != nil {
		panic("Failed to parse texture pack metadata from '" + path + "': " + err.Error())
	}

	return descriptor
}

type MetaDescriptor struct {
	Id              string
	Name            string
	Version         string
	Description     string
	Authors         []string
	DownloadUrl     string
	SupportsCit     bool
	CatharsisConfig map[string]interface{}
}

func (texturePackRegistry *TexturePackRegistry) NormalizeCatharsisConfigOverrides(rawOverrides map[string]interface{}) map[string]string {
	if rawOverrides == nil || len(rawOverrides) == 0 {
		return nil
	}

	normalized := make(map[string]string)
	for id, value := range rawOverrides {
		if id == "" {
			continue
		}

		switch v := value.(type) {
		case string:
			if v != "" {
				normalized[id] = v
			}
		case bool:
			if v {
				normalized[id] = "true"
			} else {
				normalized[id] = "false"
			}
		case float64:
			normalized[id] = strconv.FormatFloat(v, 'f', -1, 64)
		default:
			// Ignore unsupported types
		}
	}

	if len(normalized) == 0 {
		return nil
	}

	return normalized
}

type PackArchiveInfo struct {
	Kind string
	Path string
}

func (texturePackRegistry *TexturePackRegistry) FindPackArchive(packDirectory string) *PackArchiveInfo {
	// If the directory has an assets/ folder, treat as traditional directory-based pack
	if _, err := os.Stat(packDirectory + "/assets"); err == nil {
		return nil
	}

	// Check for bare pack.cats first
	if _, err := os.Stat(packDirectory + "/pack.cats"); err == nil {
		return &PackArchiveInfo{Kind: "CatsFile", Path: packDirectory + "/pack.cats"}
	}

	// Check for .cats.zip files (catharsis packs wrapped in ZIP for hosting compatibility)
	catsZipFiles, err := os.ReadDir(packDirectory)
	if err == nil {
		for _, entry := range catsZipFiles {
			if !entry.IsDir() && len(entry.Name()) > 9 && entry.Name()[len(entry.Name())-9:] == ".cats.zip" {
				return &PackArchiveInfo{Kind: "CatsZip", Path: packDirectory + "/" + entry.Name()}
			}
		}
	}

	// Check for bare .cats files
	catsFiles, err := os.ReadDir(packDirectory)
	if err == nil {
		for _, entry := range catsFiles {
			if !entry.IsDir() && len(entry.Name()) > 5 && entry.Name()[len(entry.Name())-5:] == ".cats" {
				return &PackArchiveInfo{Kind: "CatsFile", Path: packDirectory + "/" + entry.Name()}
			}
		}
	}

	// Prefer pack.zip specifically, then fall back to the first other .zip found
	if _, err := os.Stat(packDirectory + "/pack.zip"); err == nil {
		return &PackArchiveInfo{Kind: "Zip", Path: packDirectory + "/pack.zip"}
	}

	zipFiles, err := os.ReadDir(packDirectory)
	if err == nil {
		for _, entry := range zipFiles {
			if !entry.IsDir() && len(entry.Name()) > 4 && entry.Name()[len(entry.Name())-4:] == ".zip" {
				return &PackArchiveInfo{Kind: "Zip", Path: packDirectory + "/" + entry.Name()}
			}
		}
	}

	return nil
}

func IsCatharsisPackMcmeta(mcmetaJson string) bool {
	var parsed map[string]interface{}
	err := global.JSON.Unmarshal([]byte(mcmetaJson), &parsed)
	if err != nil {
		return false
	}

	_, exists := parsed["catharsis:pack/v1"]
	return exists
}

func ResolveNamespaceRootsFromProvider(provider assets.ResourceProvider, displayRoot string, requireMinecraft bool) (map[string]string, map[string]assets.ResourceProvider) {
	if !provider.DirectoryExists("assets") {
		panic("Texture pack at '" + displayRoot + "' does not contain an 'assets' directory.")
	}

	namespaces := make(map[string]string)
	nsProviders := make(map[string]assets.ResourceProvider)

	dirs, err := provider.EnumerateDirectories("assets", "*", false)
	if err != nil {
		panic("Failed to enumerate namespaces in texture pack at '" + displayRoot + "': " + err.Error())
	}

	for _, nsDir := range dirs {
		// nsDir is like "assets/minecraft"
		name := nsDir
		if idx := lastIndexOf(nsDir, '/'); idx != -1 {
			name = nsDir[idx+1:]
		}

		if name == "" || namespaces[name] != "" {
			continue
		}

		displayPath := displayRoot + "/assets/" + name
		namespaces[name] = displayPath
		nsProviders[name] = assets.NewSubPathResourceProvider(provider, nsDir)
	}

	if requireMinecraft {
		if _, ok := namespaces["minecraft"]; !ok {
			panic("Texture pack at '" + displayRoot + "' does not contain an 'assets/minecraft' directory.")
		}
	}

	return namespaces, nsProviders
}

func lastIndexOf(s string, sep byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == sep {
			return i
		}
	}
	return -1
}

func ParsePackFormatFromJson(json string) *int {
	var parsed map[string]interface{}
	err := global.JSON.Unmarshal([]byte(json), &parsed)
	if err != nil {
		return nil
	}

	pack, ok := parsed["pack"].(map[string]interface{})
	if !ok {
		return nil
	}

	format, ok := pack["pack_format"].(float64)
	if !ok {
		return nil
	}

	formatInt := int(format)
	return &formatInt
}

func DetectCatharsisOverlayPrefix(provider assets.ResourceProvider, overlayNames []string) *string {
	if len(overlayNames) == 0 {
		return nil
	}

	// Check if the first overlay directory exists directly (no prefix needed)
	if provider.DirectoryExists(overlayNames[0]) {
		return nil
	}

	// Enumerate root-level directories to find a match with a prefix
	testName := overlayNames[0]
	suffix := "_" + testName

	dirs, err := provider.EnumerateDirectories("", "*", false)
	if err != nil {
		return nil
	}

	for _, dir := range dirs {
		dirName := dir
		if idx := lastIndexOf(dir, '/'); idx != -1 {
			dirName = dir[idx+1:]
		}

		if len(dirName) >= len(suffix) && dirName[len(dirName)-len(suffix):] == suffix {
			prefix := dirName[:len(dirName)-len(testName)]
			return &prefix
		}
	}

	return nil
}

func ResolveNamespaceRoots(root string) map[string]string {
	assetsRoot := root + "/assets"
	if _, err := os.Stat(assetsRoot); os.IsNotExist(err) {
		panic("Texture pack at '" + root + "' does not contain an 'assets' directory.")
	}

	namespaces := make(map[string]string)
	entries, err := os.ReadDir(assetsRoot)
	if err != nil {
		panic("Failed to enumerate namespaces in texture pack at '" + root + "': " + err.Error())
	}

	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if name != "" && namespaces[name] == "" {
				namespaces[name] = assetsRoot + "/" + name
			}
		}
	}

	if namespaces["minecraft"] == "" {
		panic("Texture pack at '" + root + "' does not contain an 'assets/minecraft' directory.")
	}

	return namespaces
}

func ParsePackForm(packMcMetaPath string) *int {
	data, err := os.ReadFile(packMcMetaPath)
	if err != nil {
		return nil
	}

	var parsed map[string]interface{}
	err = global.JSON.Unmarshal(data, &parsed)
	if err != nil {
		return nil
	}

	pack, ok := parsed["pack"].(map[string]interface{})
	if !ok {
		return nil
	}

	format, ok := pack["pack_format"].(float64)
	if !ok {
		return nil
	}

	formatInt := int(format)
	return &formatInt
}

func ComputeFingerprint(id, version string, lastWriteTimeUtc time.Time, sizeBytes int64) string {
	var b strings.Builder
	b.WriteString(id)
	b.WriteByte('|')
	b.WriteString(version)
	b.WriteByte('|')
	b.WriteString(lastWriteTimeUtc.UTC().Format(time.RFC3339Nano))
	b.WriteByte('|')
	b.WriteString(strconv.FormatInt(sizeBytes, 10))
	return ComputeSha256(b.String())
}

func ComputeSha256(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func (texturePackRegistry *TexturePackRegistry) BuildPackStack(packIds []string) *TexturePackStack {
	if packIds == nil {
		panic("packIds cannot be null")
	}

	if len(packIds) == 0 {
		return &TexturePackStack{
			Packs:        []RegisteredResourcePack{},
			OverlayRoots: []PackOverlayRoot{},
			Fingerprint:  "vanilla",
		}
	}

	var ordered []RegisteredResourcePack
	for _, packId := range packIds {
		pack, exists := texturePackRegistry.Packs[packId]
		if !exists {
			panic("Unknown texture pack id '" + packId + "'.")
		}

		ordered = append(ordered, pack)
	}

	var overlayRoots []PackOverlayRoot
	for _, pack := range ordered {
		for _, overlayPath := range pack.EnumerateOverlayRootPaths() {
			overlayRoots = append(overlayRoots, PackOverlayRoot{
				Path:   overlayPath,
				PackId: pack.Id,
			})
		}
	}

	var fingerprintInput strings.Builder
	for i, pack := range ordered {
		if i > 0 {
			fingerprintInput.WriteByte('|')
		}
		fingerprintInput.WriteString(pack.Id)
		fingerprintInput.WriteByte(':')
		fingerprintInput.WriteString(pack.Fingerprint)

	}

	stackFingerprint := ComputeSha256("packstack:" + fingerprintInput.String())
	return &TexturePackStack{
		Packs:        ordered,
		OverlayRoots: overlayRoots,
		Fingerprint:  stackFingerprint,
	}
}

func (texturePackRegistry *TexturePackRegistry) RegisterAllPacks(rootDirectory string, searchRecursively bool) []RegisteredResourcePack {
	results, normalizedRoot := texturePackRegistry.RegisterAllPacksCore(rootDirectory, searchRecursively)
	if rootDirectory != "" && normalizedRoot != nil {
		texturePackRegistry.RecordRegistrationSource(RegistrationSource{
			Path:               *normalizedRoot,
			SearchRecursively:  searchRecursively,
			RegisterSinglePack: false,
		})
	}

	return results
}

func (texturePackRegistry *TexturePackRegistry) RegisterAllPacksCore(rootDirectory string, searchRecursively bool) ([]RegisteredResourcePack, *string) {
	var normalizedRoot *string

	if rootDirectory == "" {
		return []RegisteredResourcePack{}, normalizedRoot
	}

	fullRoot, err := filepath.Abs(rootDirectory)
	if err != nil {
		fmt.Printf("Failed to resolve absolute path for '%s': %s\n", rootDirectory, err.Error())

		return []RegisteredResourcePack{}, normalizedRoot
	}

	if _, err := os.Stat(fullRoot); os.IsNotExist(err) {
		return []RegisteredResourcePack{}, normalizedRoot
	}

	normalizedRoot = &fullRoot

	var results []RegisteredResourcePack
	if _, err := os.Stat(filepath.Join(fullRoot, "meta.json")); err == nil {
		pack := texturePackRegistry.RegisterPackCore(fullRoot)
		results = append(results, pack)
	}

	var candidates []string
	err = filepath.Walk(fullRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Failed to access path '%s': %s\n", path, err.Error())
			return nil
		}

		if info.IsDir() && path != fullRoot {
			if _, err := os.Stat(filepath.Join(path, "meta.json")); err == nil {
				candidates = append(candidates, path)
			}
			if !searchRecursively {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Failed to enumerate directories under '%s': %s\n", fullRoot, err.Error())
		return results, normalizedRoot
	}

	for _, candidate := range candidates {
		pack := texturePackRegistry.RegisterPackCore(candidate)
		if pack.Id != "" {
			results = append(results, pack)
		}
	}

	return results, normalizedRoot
}
