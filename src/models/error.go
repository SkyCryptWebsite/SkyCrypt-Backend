package models

type ProcessingError struct {
	Error   string  `json:"error"`
	Status  *string `json:"status,omitempty"`
	Message *string `json:"message,omitempty"`
}

// APIError is returned when a request is rejected before it reaches the endpoint, e.g. for a missing API key.
type APIError struct {
	Error string `json:"error" example:"Invalid API key."`
	Code  string `json:"code" example:"INVALID_API_KEY"`
}
