package api

import (
	"net/http"
	"skycrypt/src/forensics"
	"time"
)

var baseTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 20,
	IdleConnTimeout:     90 * time.Second,
	TLSHandshakeTimeout: 5 * time.Second,
	DisableCompression:  false,
	ForceAttemptHTTP2:   true,
}

var HTTPClient = &http.Client{
	Timeout:   10 * time.Second,
	Transport: forensics.InstrumentedRoundTripper(baseTransport),
}
