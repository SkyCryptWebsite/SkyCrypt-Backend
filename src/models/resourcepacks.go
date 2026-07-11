package models

type ResourcePackConfig struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Author   string `json:"author"`
	Url      string `json:"url"`
	Icon     string `json:"icon"`
	Priority int    `json:"priority"`
}
