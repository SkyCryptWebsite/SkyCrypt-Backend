package models

type Inventory struct {
	Name    string         `json:"name"`
	Texture string         `json:"texture"`
	Items   []StrippedItem `json:"items"`
}
