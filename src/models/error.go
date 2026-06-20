package models

type ProcessingError struct {
	Error   string  `json:"error"`
	Status  *string `json:"status,omitempty"`
	Message *string `json:"message,omitempty"`
}

type EmojisResponse struct {
	FetchedAt int64                    `json:"fetched_at"`
	Emojis    []map[string]interface{} `json:"emojis"`
}
