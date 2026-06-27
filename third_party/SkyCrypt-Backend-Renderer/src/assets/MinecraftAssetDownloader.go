package assets

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/global"
	"github.com/DuckySoLucky/SkyCrypt-Backend-Renderer/src/models"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const versionManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

type MinecraftAssetDownloader struct{}

func (downloader *MinecraftAssetDownloader) ResolveAssetsDirectory(assetsPath string) (*string, error) {
	resolvedPath := strings.TrimSpace(assetsPath)
	if resolvedPath == "" {
		resolvedPath = "assets"
	}

	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		if assetsPath == "" {
			return nil, fmt.Errorf("assets path is empty and default 'assets' directory does not exist: %s", resolvedPath)
		}

		return nil, fmt.Errorf("assets directory does not exist: %s", resolvedPath)
	}

	return &resolvedPath, nil
}

func (downloader *MinecraftAssetDownloader) DownloadAndExtractAssets(
	version string,
	outputPath string,
	forceRedownload bool,
) error {
	fmt.Printf("Fetching version manifest from: %s\n", versionManifestURL)

	response, err := global.HTTP_CLIENT.Get(versionManifestURL)
	if err != nil {
		return fmt.Errorf("failed to fetch version manifest: %w", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			fmt.Printf("warning: failed to close output file %s: %v\n", outputPath, closeErr)
		}
	}()

	if response.StatusCode != 200 {
		return fmt.Errorf("unexpected status code %d when fetching version manifest", response.StatusCode)
	}

	var versionManifest models.VersionManifest
	if err := global.JSON.NewDecoder(response.Body).Decode(&versionManifest); err != nil {
		return fmt.Errorf("failed to decode version manifest: %w", err)
	}

	var versionInfo *models.VersionInfo
	for _, v := range versionManifest.Versions {
		if v.ID == version {
			versionInfo = &v
			break
		}
	}

	if versionInfo == nil {
		return fmt.Errorf("version '%s' not found in manifest", version)
	}

	fmt.Printf("Found version '%s' with URL: %s\n", version, versionInfo.URL)

	versionResponse, err := global.HTTP_CLIENT.Get(versionInfo.URL)
	if err != nil {
		return fmt.Errorf("failed to fetch version info: %w", err)
	}
	defer func() {
		if closeErr := versionResponse.Body.Close(); closeErr != nil {
			fmt.Printf("warning: failed to close output file %s: %v\n", outputPath, closeErr)
		}
	}()

	if versionResponse.StatusCode != 200 {
		return fmt.Errorf("unexpected status code %d when fetching version info", versionResponse.StatusCode)
	}

	var versionData models.VersionMetaData
	if err := global.JSON.NewDecoder(versionResponse.Body).Decode(&versionData); err != nil {
		return fmt.Errorf("failed to decode version info: %w", err)
	}

	fmt.Printf("Version '%s' has client download URL: %s\n", version, versionData.Downloads.Client.URL)

	fmt.Printf("Downloading client.jar for version '%s' (%d MB)...\n", version, versionData.Downloads.Client.Size/1024/1024)

	clientPath := fmt.Sprintf("%s/%s_client.jar", outputPath, version)
	if err := downloadFile(versionData.Downloads.Client.URL, clientPath, forceRedownload); err != nil {
		return fmt.Errorf("failed to download client.jar: %w", err)
	}

	fmt.Printf("Verifying download...\n")
	if err := verifyFileHash(clientPath, versionData.Downloads.Client.SHA1); err != nil {
		return fmt.Errorf("file verification failed: %w", err)
	}

	fmt.Printf("Client.jar for version '%s' downloaded and verified successfully at: %s\n", version, clientPath)

	if err := extractAssets(clientPath, outputPath); err != nil {
		return fmt.Errorf("failed to extract assets: %w", err)
	}

	return nil
}

func (downloader *MinecraftAssetDownloader) DownloadAssetsAsProvider(version string, outputPath string, forceRedownload bool) (ResourceProvider, error) {
	if err := downloader.DownloadAndExtractAssets(version, outputPath, forceRedownload); err != nil {
		return nil, err
	}
	assetsRoot := filepath.Join(outputPath, "assets", "minecraft")
	if _, err := os.Stat(assetsRoot); err != nil {
		return nil, err
	}
	return NewDirectoryResourceProvider(assetsRoot)
}

func (downloader *MinecraftAssetDownloader) OpenJarAsProvider(jarPath string) (ResourceProvider, error) {
	if strings.TrimSpace(jarPath) == "" {
		return nil, fmt.Errorf("jarPath cannot be empty")
	}
	provider := NewZipResourceProvider(jarPath)
	if provider.DirectoryExists("assets/minecraft") {
		return NewSubPathResourceProvider(provider, "assets/minecraft"), nil
	}
	return provider, nil
}

func (downloader *MinecraftAssetDownloader) GetAvailableVersions() ([]models.VersionInfo, error) {
	manifest, err := downloader.fetchVersionManifest()
	if err != nil {
		return nil, err
	}
	return manifest.Versions, nil
}

func (downloader *MinecraftAssetDownloader) GetLatestVersion(includeSnapshots bool) (string, error) {
	manifest, err := downloader.fetchVersionManifest()
	if err != nil {
		return "", err
	}
	if includeSnapshots && strings.TrimSpace(manifest.Latest.Snapshot) != "" {
		return manifest.Latest.Snapshot, nil
	}
	if strings.TrimSpace(manifest.Latest.Release) == "" {
		return "", fmt.Errorf("latest release version is missing from manifest")
	}
	return manifest.Latest.Release, nil
}

func (downloader *MinecraftAssetDownloader) fetchVersionManifest() (*models.VersionManifest, error) {
	response, err := global.HTTP_CLIENT.Get(versionManifestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version manifest: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code %d when fetching version manifest", response.StatusCode)
	}

	var manifest models.VersionManifest
	if err := global.JSON.NewDecoder(response.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode version manifest: %w", err)
	}
	return &manifest, nil
}

func downloadFile(url string, outputPath string, forceRedownload bool) error {
	if !forceRedownload {
		if _, err := os.Stat(outputPath); err == nil {
			fmt.Printf("File '%s' already exists, skipping download.\n", outputPath)
			return nil
		}
	}

	response, err := global.HTTP_CLIENT.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			fmt.Printf("warning: failed to close output file %s: %v\n", outputPath, closeErr)
		}
	}()

	if response.StatusCode != 200 {
		return fmt.Errorf("unexpected status code %d when downloading file", response.StatusCode)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() {
		if closeErr := outFile.Close(); closeErr != nil {
			fmt.Printf("warning: failed to close output file %s: %v\n", outputPath, closeErr)
		}
	}()

	contentLength := response.ContentLength
	if contentLength <= 0 {
		// fallback to normal copy if content length is unknown
		_, err = io.Copy(outFile, response.Body)
		return err
	}

	var downloaded int64 = 0
	buf := make([]byte, 32*1024)
	lastPercent := -1

	for {
		n, readErr := response.Body.Read(buf)
		if n > 0 {
			if _, writeErr := outFile.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("failed to write file to disk: %w", writeErr)
			}
			downloaded += int64(n)
			percent := int(float64(downloaded) / float64(contentLength) * 100)
			if percent != lastPercent {
				fmt.Printf("\rDownloading... %d%%", percent)
				lastPercent = percent
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("failed to read from response: %w", readErr)
		}
	}
	fmt.Println("\rDownloading... 100%")

	return nil
}

func verifyFileHash(filePath string, expectedHash string) error {
	actualHash, err := computeFileSHA1(filePath)
	if err != nil {
		return fmt.Errorf("failed to compute file hash: %w", err)
	}

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	fmt.Println("File verification successful.")
	return nil
}

func computeFileSHA1(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for hashing: %w", err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("warning: failed to close file %s after hashing: %v\n", filePath, closeErr)
		}
	}()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to hash file content: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func extractAssets(jarPath string, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	archive, err := zip.OpenReader(jarPath)
	if err != nil {
		return fmt.Errorf("failed to open jar as zip: %w", err)
	}
	defer func() {
		if closeErr := archive.Close(); closeErr != nil {
			fmt.Printf("warning: failed to close zip archive %s: %v\n", jarPath, closeErr)
		}
	}()

	assetEntries := make([]*zip.File, 0)
	for _, entry := range archive.File {
		if strings.HasPrefix(strings.ToLower(entry.Name), "assets/minecraft/") {
			assetEntries = append(assetEntries, entry)
		}
	}

	totalEntries := len(assetEntries)
	if totalEntries == 0 {
		return fmt.Errorf("no assets found under assets/minecraft in jar")
	}

	extractedCount := 0
	for _, entry := range assetEntries {
		if entry.FileInfo().IsDir() {
			continue
		}

		relativePath := filepath.Clean(entry.Name)
		destinationPath := filepath.Join(outputDir, relativePath)

		cleanOutput := filepath.Clean(outputDir)
		if !strings.HasPrefix(destinationPath, cleanOutput+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry would escape output directory: %s", entry.Name)
		}

		if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
			return fmt.Errorf("failed to create destination directory for %s: %w", entry.Name, err)
		}

		inFile, err := entry.Open()
		if err != nil {
			return fmt.Errorf("failed to open zip entry %s: %w", entry.Name, err)
		}

		outFile, err := os.Create(destinationPath)
		if err != nil {
			defer func() {
				if closeErr := inFile.Close(); closeErr != nil {
					fmt.Printf("warning: failed to close zip entry %s after failed extraction: %v\n", entry.Name, closeErr)
				}
			}()

			return fmt.Errorf("failed to create extracted file %s: %w", destinationPath, err)
		}

		_, copyErr := io.Copy(outFile, inFile)
		closeOutErr := outFile.Close()
		closeInErr := inFile.Close()

		if copyErr != nil {
			return fmt.Errorf("failed to extract %s: %w", entry.Name, copyErr)
		}
		if closeOutErr != nil {
			return fmt.Errorf("failed to finalize extracted file %s: %w", destinationPath, closeOutErr)
		}
		if closeInErr != nil {
			return fmt.Errorf("failed to close zip entry %s: %w", entry.Name, closeInErr)
		}

		extractedCount++
		if extractedCount%100 == 0 || extractedCount == totalEntries {
			percentage := 70 + int(float64(extractedCount)*25.0/float64(totalEntries))
			fmt.Printf("\rExtracting assets... %d/%d (%d%%)", extractedCount, totalEntries, percentage)
		}
	}

	fmt.Printf("\rExtracting assets... %d/%d (100%%)\n", extractedCount, totalEntries)

	version := strings.TrimSuffix(filepath.Base(jarPath), "_client.jar")
	if version == "" || version == filepath.Base(jarPath) {
		version = "unknown"
	}

	minecraftAssetsPath := filepath.Join(outputDir, "assets", "minecraft")
	fmt.Printf("Successfully extracted %s assets to %s (100%%)\n", version, minecraftAssetsPath)

	return nil
}
