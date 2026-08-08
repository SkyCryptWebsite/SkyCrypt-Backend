package models

type StatsInfo map[string]int

type PlayerStat struct {
	ID                      string `json:"id" binding:"required"`
	Name                    string `json:"name" binding:"required"`
	NameLore                string `json:"nameLore" binding:"required"`
	NameShort               string `json:"nameShort" binding:"required"`
	NameTiny                string `json:"nameTiny" binding:"required"`
	Symbol                  string `json:"symbol" binding:"required"`
	Suffix                  string `json:"suffix" binding:"required"`
	Color                   string `json:"color" binding:"required"`
	Category                string `json:"category" binding:"required"`
	Description             string `json:"description" binding:"required"`
	Percent                 bool   `json:"percent,omitempty"`
	Cap                     *int   `json:"cap,omitempty"`
	DisabledOnPrivateIsland bool   `json:"disabledOnPrivateIsland,omitempty"`

	StatsInfo `json:"statsInfo" binding:"required"`
}
type Stats struct {
	Stats []PlayerStat `json:"stats" binding:"required"`
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
