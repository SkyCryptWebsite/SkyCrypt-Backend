package api

import (
	"net/http"
	"time"
)

var HTTP_CLIENT = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	},
}
