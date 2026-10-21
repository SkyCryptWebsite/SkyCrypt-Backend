package models

type ProcessingError struct {
	Error   string  `json:"error"`
	Status  *string `json:"status,omitempty"`
	Message *string `json:"message,omitempty"`
}
