package store

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const ipifyURL = "https://api.ipify.org"

// HTTPIPResolver resolves the public IP via https://api.ipify.org.
type HTTPIPResolver struct {
	client *http.Client
	url    string
}

// NewHTTPIPResolver creates an IPResolver that uses api.ipify.org.
func NewHTTPIPResolver() *HTTPIPResolver {
	return &HTTPIPResolver{
		client: &http.Client{Timeout: 10 * time.Second},
		url:    ipifyURL,
	}
}

// NewHTTPIPResolverWithURL creates an HTTPIPResolver that queries resolverURL
// with the given client (used by tests to point at an httptest.Server).
func NewHTTPIPResolverWithURL(client *http.Client, resolverURL string) *HTTPIPResolver {
	return &HTTPIPResolver{client: client, url: resolverURL}
}

// Resolve returns the current public IP address.
func (r *HTTPIPResolver) Resolve() (string, error) {
	resp, err := r.client.Get(r.url)
	if err != nil {
		return "", fmt.Errorf("resolve public IP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve public IP: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read IP response: %w", err)
	}

	return strings.TrimSpace(string(body)), nil
}
