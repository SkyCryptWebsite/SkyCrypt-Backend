package models

// PlayerResolve keeps username first, the order these responses had when they were marshaled from a sorted map.
type PlayerResolve struct {
	Username string `json:"username"`
	UUID     string `json:"uuid"`
}
