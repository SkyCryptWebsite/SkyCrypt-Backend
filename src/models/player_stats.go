package models

type StatsInfo map[string]int

type PlayerStat struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	NameLore                string `json:"nameLore"`
	NameShort               string `json:"nameShort"`
	NameTiny                string `json:"nameTiny"`
	Symbol                  string `json:"symbol"`
	Suffix                  string `json:"suffix"`
	Color                   string `json:"color"`
	Category                string `json:"category"`
	Description             string `json:"description"`
	Percent                 bool   `json:"percent,omitempty"`
	Cap                     *int   `json:"cap,omitempty"`
	DisabledOnPrivateIsland bool   `json:"disabledOnPrivateIsland,omitempty"`

	StatsInfo `json:"statsInfo,omitempty"`
}
type Stats struct {
	Stats []PlayerStat `json:"stats"`
}

type StatData struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	NameLore                string `json:"nameLore"`
	NameShort               string `json:"nameShort"`
	NameTiny                string `json:"nameTiny"`
	Symbol                  string `json:"symbol"`
	Suffix                  string `json:"suffix"`
	Color                   string `json:"color"`
	Category                string `json:"category"`
	Description             string `json:"description"`
	Percent                 bool   `json:"percent,omitempty"`
	Cap                     *int   `json:"cap,omitempty"`
	DisabledOnPrivateIsland bool   `json:"disabledOnPrivateIsland,omitempty"`
}
