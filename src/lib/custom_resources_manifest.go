package lib

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/constants"
	"skycrypt/src/models"
	"skycrypt/src/utility"
)

func preRenderSkyBlockItemIDs() []string {
	skyblockItems := constants.ItemsSnapshot()
	seen := map[string]struct{}{}
	itemIDs := make([]string, 0, len(skyblockItems))

	addID := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		itemIDs = append(itemIDs, id)
	}

	for _, item := range skyblockItems {
		addID(item.SkyblockID)
	}

	if appRoot, err := appRootDir(); err == nil {
		if entries, err := os.ReadDir(filepath.Join(appRoot, "NotEnoughUpdates-REPO", "items")); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				name := entry.Name()
				if strings.EqualFold(filepath.Ext(name), ".json") {
					addID(strings.TrimSuffix(name, filepath.Ext(name)))
				}
			}
		}
	}

	notenoughupdates.CACHED_NEU_ITEMS.Range(func(key, value any) bool {
		if keyID, ok := key.(string); ok {
			addID(keyID)
		}

		switch item := value.(type) {
		case models.NEUItem:
			addID(item.NEUId)
			if item.NBT.ExtraAttributes != nil {
				addID(item.NBT.ExtraAttributes.Id)
			}
		case *models.NEUItem:
			if item != nil {
				addID(item.NEUId)
				if item.NBT.ExtraAttributes != nil {
					addID(item.NBT.ExtraAttributes.Id)
				}
			}
		}

		return true
	})

	return itemIDs
}

func renderedResourcePackManifestPath(cacheDir string) string {
	return filepath.Join(cacheDir, "rendered", renderedResourcePackManifestFileName)
}

func buildRenderedResourcePackManifest(resourcePacksPath string, itemIDs []string, renderedIndexCount int, generatedAt time.Time) (renderedResourcePackManifest, error) {
	resourcePackHash, packOrder, packs, err := renderedResourcePackFingerprint(resourcePacksPath)
	if err != nil {
		return renderedResourcePackManifest{}, err
	}
	itemIDHash, itemIDCount := resourcePackItemIDHash(itemIDs)

	if generatedAt.IsZero() {
		generatedAt = time.Now()
	}
	return renderedResourcePackManifest{
		SchemaVersion:      renderedResourcePackManifestSchemaVersion,
		GeneratedAtUnix:    generatedAt.Unix(),
		RendererModule:     renderedResourcePackManifestRendererModule,
		ResourcePackHash:   resourcePackHash,
		ItemIDHash:         itemIDHash,
		ItemIDCount:        itemIDCount,
		RenderedIndexCount: renderedIndexCount,
		PackOrder:          packOrder,
		Packs:              packs,
	}, nil
}

func renderedResourcePackFingerprint(resourcePacksPath string) (string, []string, []renderedResourcePackEntry, error) {
	configs, err := loadResourcePackConfigs(resourcePacksPath)
	if err != nil {
		return "", nil, nil, err
	}
	dirsByPackID, err := resourcePackDirsByCanonicalID(resourcePacksPath)
	if err != nil {
		return "", nil, nil, err
	}

	packOrder := make([]string, 0, len(configs))
	packs := make([]renderedResourcePackEntry, 0, len(configs)+1)
	fingerprintPacks := make([]renderedResourcePackFingerprintEntry, 0, len(configs)+1)
	for _, config := range configs {
		id := strings.TrimSpace(config.Id)
		if id == "" {
			continue
		}

		packOrder = append(packOrder, id)
		packDirName := dirsByPackID[canonicalPackAlias(id)]
		metaHash := ""
		archiveName := ""
		archiveHash := ""
		if packDirName != "" {
			packDir := filepath.Join(resourcePacksPath, packDirName)
			metaHash, err = optionalFileSHA256(filepath.Join(packDir, "meta.json"))
			if err != nil {
				return "", nil, nil, err
			}
			archiveName, archiveHash, err = resourcePackArchivesFingerprint(packDir)
			if err != nil {
				return "", nil, nil, err
			}
		}

		entry := renderedResourcePackEntry{
			ID:          id,
			Name:        strings.TrimSpace(config.Name),
			Version:     strings.TrimSpace(config.Version),
			MetaHash:    metaHash,
			ArchiveHash: archiveHash,
			ArchiveName: archiveName,
		}
		packs = append(packs, entry)
		fingerprintPacks = append(fingerprintPacks, renderedResourcePackFingerprintEntry{
			ID:          entry.ID,
			Name:        entry.Name,
			Version:     entry.Version,
			Priority:    config.Priority,
			MetaHash:    entry.MetaHash,
			ArchiveHash: entry.ArchiveHash,
			ArchiveName: entry.ArchiveName,
		})
	}

	vanillaEntry, vanillaFingerprintEntry, err := vanillaResourcePackFingerprintEntry(resourcePacksPath)
	if err != nil {
		return "", nil, nil, err
	}
	packs = append(packs, vanillaEntry)
	fingerprintPacks = append(fingerprintPacks, vanillaFingerprintEntry)

	payload := struct {
		Packs []renderedResourcePackFingerprintEntry `json:"packs"`
	}{
		Packs: fingerprintPacks,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", nil, nil, err
	}
	sum := sha256.Sum256(payloadBytes)
	return hex.EncodeToString(sum[:]), packOrder, packs, nil
}

func resourcePackDirsByCanonicalID(resourcePacksPath string) (map[string]string, error) {
	files, err := os.ReadDir(resourcePacksPath)
	if err != nil {
		return nil, err
	}

	dirs := map[string]string{}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		metaPath := filepath.Join(resourcePacksPath, file.Name(), "meta.json")
		metaFile, err := os.Open(metaPath)
		if err != nil {
			continue
		}
		var meta resourcePackMeta
		decodeErr := json.NewDecoder(metaFile).Decode(&meta)
		_ = metaFile.Close()
		if decodeErr != nil {
			continue
		}

		id := strings.TrimSpace(meta.ID)
		if id == "" || strings.EqualFold(id, "vanilla") {
			continue
		}
		dirs[canonicalPackAlias(id)] = file.Name()
	}
	return dirs, nil
}

func vanillaResourcePackFingerprintEntry(resourcePacksPath string) (renderedResourcePackEntry, renderedResourcePackFingerprintEntry, error) {
	vanillaDir := filepath.Join(resourcePacksPath, "Vanilla")
	versionPath := filepath.Join(vanillaDir, "version.txt")
	version := ""
	if versionBytes, err := os.ReadFile(versionPath); err == nil {
		version = strings.TrimSpace(string(versionBytes))
	} else if !os.IsNotExist(err) {
		return renderedResourcePackEntry{}, renderedResourcePackFingerprintEntry{}, err
	}

	versionHash, err := optionalFileSHA256(versionPath)
	if err != nil {
		return renderedResourcePackEntry{}, renderedResourcePackFingerprintEntry{}, err
	}
	archiveName, archiveHash, err := resourcePackArchivesFingerprint(vanillaDir)
	if err != nil {
		return renderedResourcePackEntry{}, renderedResourcePackFingerprintEntry{}, err
	}

	entry := renderedResourcePackEntry{
		ID:          "vanilla",
		Name:        "Vanilla",
		Version:     version,
		MetaHash:    versionHash,
		ArchiveHash: archiveHash,
		ArchiveName: archiveName,
	}
	return entry, renderedResourcePackFingerprintEntry{
		ID:          entry.ID,
		Name:        entry.Name,
		Version:     entry.Version,
		MetaHash:    entry.MetaHash,
		ArchiveHash: entry.ArchiveHash,
		ArchiveName: entry.ArchiveName,
	}, nil
}

func resourcePackArchivesFingerprint(packDir string) (string, string, error) {
	files, err := os.ReadDir(packDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", err
	}

	archiveNames := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() || !resourcePackArchiveCandidate(file.Name()) {
			continue
		}
		archiveNames = append(archiveNames, file.Name())
	}
	sort.Strings(archiveNames)
	if len(archiveNames) == 0 {
		return "", "", nil
	}

	archiveHashes := make([]string, len(archiveNames))
	for i, archiveName := range archiveNames {
		hash, err := optionalFileSHA256(filepath.Join(packDir, archiveName))
		if err != nil {
			return "", "", err
		}
		archiveHashes[i] = hash
	}
	if len(archiveNames) == 1 {
		return archiveNames[0], archiveHashes[0], nil
	}

	combined := sha256.New()
	for i, archiveName := range archiveNames {
		_, _ = combined.Write([]byte(archiveName))
		_, _ = combined.Write([]byte{0})
		_, _ = combined.Write([]byte(archiveHashes[i]))
		_, _ = combined.Write([]byte{'\n'})
	}
	return strings.Join(archiveNames, ","), hex.EncodeToString(combined.Sum(nil)), nil
}

func resourcePackArchiveCandidate(fileName string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(fileName))
	return strings.HasSuffix(lowerName, ".zip") || strings.HasSuffix(lowerName, ".jar")
}

func optionalFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func resourcePackItemIDHash(itemIDs []string) (string, int) {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(itemIDs))
	for _, id := range itemIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	sort.Strings(normalized)

	hash := sha256.New()
	for _, id := range normalized {
		_, _ = hash.Write([]byte(id))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), len(normalized)
}

func readRenderedResourcePackManifest(cacheDir string) (*renderedResourcePackManifest, error) {
	manifestBytes, err := os.ReadFile(renderedResourcePackManifestPath(cacheDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var manifest renderedResourcePackManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func shouldSkipSkyBlockPreRender(current renderedResourcePackManifest, saved *renderedResourcePackManifest, loadedRenderedCount int) (bool, string) {
	if saved == nil {
		return false, "manifest is missing"
	}
	if saved.SchemaVersion != renderedResourcePackManifestSchemaVersion {
		return false, fmt.Sprintf("manifest schema changed from %d to %d", saved.SchemaVersion, renderedResourcePackManifestSchemaVersion)
	}
	if strings.TrimSpace(saved.RendererModule) != renderedResourcePackManifestRendererModule {
		return false, "renderer manifest version changed"
	}
	if strings.TrimSpace(saved.ResourcePackHash) != current.ResourcePackHash {
		return false, "resource pack fingerprint changed"
	}
	if saved.RenderedIndexCount <= 0 {
		return false, "manifest has no completed rendered textures"
	}
	if loadedRenderedCount < saved.RenderedIndexCount {
		return false, fmt.Sprintf("rendered cache has %d indexed textures, manifest expects at least %d", loadedRenderedCount, saved.RenderedIndexCount)
	}
	if strings.TrimSpace(saved.ItemIDHash) != current.ItemIDHash || saved.ItemIDCount != current.ItemIDCount {
		return true, fmt.Sprintf("resource pack fingerprint current with %s indexed textures; SkyBlock item IDs changed and will render lazily", utility.AddCommas(loadedRenderedCount))
	}
	return true, fmt.Sprintf("manifest current with %s indexed textures", utility.AddCommas(loadedRenderedCount))
}

func forceSkyBlockPreRender() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("SKYCRYPT_FORCE_RESOURCEPACK_PRERENDER")))
	return value == "1" || value == "true" || value == "yes"
}

func writeRenderedResourcePackManifest(cacheDir string, manifest renderedResourcePackManifest) error {
	manifestPath := renderedResourcePackManifestPath(cacheDir)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		return err
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestBytes = append(manifestBytes, '\n')

	tmpPath := manifestPath + ".tmp"
	if err := os.WriteFile(tmpPath, manifestBytes, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, manifestPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func cleanupStaleRenderedPackFiles(renderedDir string, packIDs []string, keepPaths map[string]struct{}) (int, error) {
	files, err := os.ReadDir(renderedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	targetPacks := map[string]struct{}{}
	for _, packID := range packIDs {
		canonicalPack := canonicalPackAlias(packID)
		if canonicalPack == "" || canonicalPack == "vanilla" {
			continue
		}
		targetPacks[canonicalPack] = struct{}{}
	}
	if len(targetPacks) == 0 {
		return 0, nil
	}

	removed := 0
	for _, file := range files {
		if file.IsDir() || !strings.EqualFold(filepath.Ext(file.Name()), ".webp") {
			continue
		}
		packID := packIDFromRenderedFilename(file.Name())
		canonicalPack := canonicalPackAlias(packID)
		if canonicalPack == "" || canonicalPack == "vanilla" {
			continue
		}
		if _, ok := targetPacks[canonicalPack]; !ok {
			continue
		}

		targetPath := filepath.Join(renderedDir, file.Name())
		if _, keep := keepPaths[renderedOutputPathKey(targetPath)]; keep {
			continue
		}
		if err := os.Remove(targetPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func renderedOutputPathKey(path string) string {
	return filepath.Clean(strings.TrimSpace(path))
}

func packIDFromRenderedFilename(fileName string) string {
	for _, part := range strings.Split(fileName, "__") {
		if !strings.HasPrefix(part, "pack=") {
			continue
		}
		packID := strings.TrimSpace(strings.TrimPrefix(part, "pack="))
		if ext := filepath.Ext(packID); ext != "" {
			packID = strings.TrimSuffix(packID, ext)
		}
		return packID
	}
	return ""
}

func resetRenderedTextureIndex() {
	itemTextureCacheMu.Lock()
	itemTextureCache = make(map[string]AppliedItemTexture)
	itemTextureCacheMu.Unlock()
	clearResolvedItemTextureCache()

	renderedSkyBlockIndexMu.Lock()
	renderedSkyBlockIndex = make(map[string]struct{})
	renderedSkyBlockIndexMu.Unlock()
}

func reloadRenderedTextureIndex(cacheDir string) (int, error) {
	resetRenderedTextureIndex()
	loaded, err := LoadRenderedTextureIndex(cacheDir)
	if err != nil {
		return 0, err
	}
	recordRenderedTextureIndexDirectoryState(cacheDir)
	return loaded, nil
}

func LoadRenderedTextureIndex(cacheDir string) (int, error) {
	if strings.TrimSpace(cacheDir) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return 0, fmt.Errorf("failed to get current working directory: %v", err)
		}
		cacheDir = filepath.Join(cwd, "cache")
	}
	rememberRenderedTextureIndexCacheDir(cacheDir)
	clearPackSignatureTextureCache()
	clearResolvedItemTextureCache()

	renderedDir := filepath.Join(cacheDir, "rendered")
	files, err := os.ReadDir(renderedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	knownPacks := knownResourcePackAliases()
	loaded := 0
	for _, file := range files {
		if file.IsDir() || !strings.EqualFold(filepath.Ext(file.Name()), ".webp") {
			continue
		}

		fileName := file.Name()
		parts := strings.Split(fileName, "__")
		itemId := ""
		minecraftID := ""
		inputItemModel := ""
		resolvedModel := ""
		texturePack := ""
		packSegments := 0
		modelConflict := false
		for _, part := range parts {
			if strings.HasPrefix(part, "skyblock=") {
				itemId = strings.TrimPrefix(part, "skyblock=")
			} else if strings.HasPrefix(part, "mc=") {
				minecraftID = debugFilenameIdentifier(strings.TrimPrefix(part, "mc="))
			} else if strings.HasPrefix(part, "itemmodel=") {
				modelValue := debugFilenameIdentifier(strings.TrimPrefix(part, "itemmodel="))
				if inputItemModel != "" && modelValue != "" && inputItemModel != modelValue {
					modelConflict = true
				}
				if inputItemModel == "" {
					inputItemModel = modelValue
				}
			} else if strings.HasPrefix(part, "model=") {
				modelValue := debugFilenameIdentifier(strings.TrimPrefix(part, "model="))
				if resolvedModel != "" && modelValue != "" && resolvedModel != modelValue {
					modelConflict = true
				}
				if resolvedModel == "" {
					resolvedModel = modelValue
				}
			} else if strings.HasPrefix(part, "pack=") {
				packSegments++
				texturePack = strings.TrimSpace(strings.TrimPrefix(part, "pack="))
			}
		}
		if modelConflict || packSegments != 1 || !validRenderedTexturePackID(texturePack, knownPacks) {
			continue
		}
		itemModel := inputItemModel
		if itemModel == "" {
			itemModel = resolvedModel
		}

		texture := AppliedItemTexture{
			Texture:     publicCacheTextureURL(filepath.Join(renderedDir, fileName)),
			TexturePack: texturePack,
		}
		if texture.Texture == "" || isStaleVanillaChestParticleRender(texture.Texture) {
			continue
		}
		if itemId != "" && texturePack != "" {
			setCachedTextureForStableKey(texturePack, "skyblock:"+itemId, texture)
			if itemModel != "" {
				setCachedTextureForStableKey(texturePack, "skyblock:"+itemId+"|itemmodel:"+normalizeMinecraftItemID(itemModel), texture)
			}
			loaded++
		}
		if texturePack != "" {
			if itemModel != "" {
				setCachedTextureForStableKey(texturePack, "itemmodel:"+normalizeMinecraftItemID(itemModel), texture)
			}
			if minecraftID != "" {
				setCachedTextureForStableKey(texturePack, fmt.Sprintf("mc:%s|damage:0|color:0", normalizeMinecraftItemID(minecraftID)), texture)
			}
		}
	}

	return loaded, nil
}

func validRenderedTexturePackID(packID string, knownPacks map[string]struct{}) bool {
	packID = strings.TrimSpace(packID)
	if packID == "" {
		return false
	}
	if strings.EqualFold(packID, "vanilla") {
		return true
	}
	_, known := knownPacks[canonicalPackAlias(packID)]
	return known
}

func rememberRenderedTextureIndexCacheDir(cacheDir string) {
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return
	}
	renderedTextureIndexReloadMu.Lock()
	renderedTextureIndexCacheDir = cacheDir
	renderedTextureIndexReloadMu.Unlock()
}

func recordRenderedTextureIndexDirectoryState(cacheDir string) {
	cacheDir = strings.TrimSpace(cacheDir)
	modTime := time.Time{}
	if cacheDir != "" {
		if info, err := os.Stat(filepath.Join(cacheDir, "rendered")); err == nil {
			modTime = info.ModTime()
		}
	}

	renderedTextureIndexReloadMu.Lock()
	if cacheDir != "" {
		renderedTextureIndexCacheDir = cacheDir
	}
	renderedTextureIndexLastDirModTime = modTime
	renderedTextureIndexLastLazyReload = time.Now()
	renderedTextureIndexReloadInFlight = false
	renderedTextureIndexReloadMu.Unlock()
}

func scheduleRenderedTextureIndexRefresh() {
	now := time.Now()
	renderedTextureIndexReloadMu.Lock()
	if renderedTextureIndexReloadInFlight ||
		(!renderedTextureIndexLastLazyReload.IsZero() && now.Sub(renderedTextureIndexLastLazyReload) < renderedTextureIndexLazyReloadInterval) {
		renderedTextureIndexReloadMu.Unlock()
		return
	}
	cacheDir := renderedTextureIndexCacheDir
	if strings.TrimSpace(cacheDir) == "" {
		renderedTextureIndexReloadMu.Unlock()
		return
	}
	renderedTextureIndexLastLazyReload = now
	renderedTextureIndexReloadInFlight = true
	renderedTextureIndexReloadMu.Unlock()

	go func() {
		defer func() {
			if recovered := recover(); recovered != nil && (utility.IsVerboseLogging() || strings.EqualFold(os.Getenv("VERBOSE_LOGGING"), "true")) {
				logCustomResourceRoutine("[CUSTOM_RESOURCES] Rendered texture index refresh panicked: %v", recovered)
			}
			renderedTextureIndexReloadMu.Lock()
			renderedTextureIndexReloadInFlight = false
			renderedTextureIndexReloadMu.Unlock()
		}()

		renderedDir := filepath.Join(cacheDir, "rendered")
		info, err := os.Stat(renderedDir)
		if err != nil || !info.IsDir() {
			return
		}
		renderedTextureIndexReloadMu.Lock()
		lastModTime := renderedTextureIndexLastDirModTime
		renderedTextureIndexReloadMu.Unlock()
		if !info.ModTime().After(lastModTime) {
			return
		}

		if _, err := loadRenderedTextureIndexForRefresh(cacheDir); err != nil {
			if utility.IsVerboseLogging() || strings.EqualFold(os.Getenv("VERBOSE_LOGGING"), "true") {
				logCustomResourceRoutine("[CUSTOM_RESOURCES] Failed to refresh rendered texture index: %v", err)
			}
			return
		}

		modTime := info.ModTime()
		if refreshedInfo, err := os.Stat(renderedDir); err == nil {
			modTime = refreshedInfo.ModTime()
		}
		renderedTextureIndexReloadMu.Lock()
		renderedTextureIndexLastDirModTime = modTime
		renderedTextureIndexReloadMu.Unlock()
	}()
}
