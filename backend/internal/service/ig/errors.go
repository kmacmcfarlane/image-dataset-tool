package ig

import "errors"

// Sentinel errors for the IG client.
var (
	// ErrSessionExpired indicates the session TTL has elapsed.
	ErrSessionExpired = errors.New("ig session expired")

	// ErrThrottled indicates a 429 response from Instagram.
	ErrThrottled = errors.New("ig request throttled (429)")

	// ErrDenied indicates a 401/403/checkpoint response from Instagram.
	ErrDenied = errors.New("ig request denied (auth failure)")

	// ErrNoSession indicates no cookies are configured for the account.
	ErrNoSession = errors.New("no ig session cookies configured")
)
