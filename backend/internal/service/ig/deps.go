package ig

import "time"

// RequestLogStore persists IG API request logs to SQLite.
type RequestLogStore interface {
	// Insert writes a request log entry.
	Insert(log *RequestLog) error

	// LastIPForAccount returns the public IP used in the most recent request
	// for the given account, or empty string if no prior requests exist.
	LastIPForAccount(accountID string) (string, error)
}

// DiscordNotifier sends alerts/warnings via Discord webhook.
type DiscordNotifier interface {
	// Warn sends a warning-level message (non-blocking).
	Warn(msg string)

	// Alert sends an alert-level message (non-blocking).
	Alert(msg string)
}

// SessionInfo holds the session state for an IG account.
type SessionInfo struct {
	AccountID  string
	Handle     string
	Cookies    string // Raw cookie header string.
	CSRFToken  string // Extracted from cookies for x-csrftoken header.
	ExpiresAt  time.Time
}

// Clock abstracts time for testability.
type Clock interface {
	Now() time.Time
}

// RealClock uses the real system time.
type RealClock struct{}

// Now returns the current UTC time.
func (RealClock) Now() time.Time { return time.Now().UTC() }

// IPResolver resolves the public IP address.
type IPResolver interface {
	// Resolve returns the current public IP address.
	Resolve() (string, error)
}
