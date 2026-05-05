package api

import (
	"net/http"
	"time"
)

// defaultHTTPClient returns an HTTP client with a sensible timeout for provider tests.
func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// newHTTPRequest constructs a GET request with optional auth header.
func newHTTPRequest(url, authHeader, authValue string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if authHeader != "" {
		req.Header.Set(authHeader, authValue)
	}
	return req, nil
}
