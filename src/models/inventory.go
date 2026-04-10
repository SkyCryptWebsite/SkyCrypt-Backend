package models

type Inventory struct {
	Name           string         `json:"name"`
	Texture        string         `json:"texture"`
	SeparatorAfter int            `json:"separatorAfter"`
	Items          []StrippedItem `json:"items"`
}
