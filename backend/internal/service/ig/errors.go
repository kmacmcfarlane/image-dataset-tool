package ig

import "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"

// Sentinel errors returned by IGClient implementations. They are the same
// values the store implementation returns, so errors.Is works across layers.
var (
	// ErrSessionExpired indicates the session TTL has elapsed.
	ErrSessionExpired = store.ErrIGSessionExpired

	// ErrThrottled indicates a 429 or rate-limit response from Instagram that
	// persisted after backoff.
	ErrThrottled = store.ErrIGThrottled

	// ErrDenied indicates a 401/403/checkpoint/login-redirect response from Instagram.
	ErrDenied = store.ErrIGDenied

	// ErrNoSession indicates no cookies are configured for the account.
	ErrNoSession = store.ErrIGNoSession
)
