package models

type VersionManifest struct {
	Latest   VersionManifestLatest `json:"latest"`
	Versions []VersionInfo         `json:"versions"`
}

type VersionManifestLatest struct {
	Release  string `json:"release"`
	Snapshot string `json:"snapshot"`
}

type VersionInfo struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

type VersionMetaData struct {
	Id        string                   `json:"id"`
	Type      string                   `json:"type"`
	Downloads VersionMetaDataDownloads `json:"downloads"`
}

type VersionMetaDataDownloads struct {
	Client VersionMetaDataDownloadInfo `json:"client"`
	Server VersionMetaDataDownloadInfo `json:"server"`
}

type VersionMetaDataDownloadInfo struct {
	URL  string `json:"url"`
	Size int64  `json:"size"`
	SHA1 string `json:"sha1"`
}
