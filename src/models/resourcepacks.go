package models

type ResourcePackConfig struct {
	Id          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Author      string   `json:"author"`
	Authors     []string `json:"authors,omitempty"`
	Url         string   `json:"url"`
	DownloadURL string   `json:"downloadUrl,omitempty"`
	Icon        string   `json:"icon"`
	Priority    int      `json:"priority"`
	Disabled    bool     `json:"disabled,omitempty"`
}
