package models

type SourceInfo struct {
	Name        string `json:"name"`
	License     string `json:"license"`
	LicenseSPDX string `json:"license_spdx"`
	Repository  string `json:"repository"`
	Source      string `json:"source"`
	Commit      string `json:"commit,omitempty"`
	Notice      string `json:"notice"`
}
