package store

import (
	"context"
	"errors"
	"time"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// Sentinel errors returned by HTTPIGClient. The service layer re-exports them
// from internal/service/ig alongside the IGClient interface.
var (
	// ErrIGSessionExpired indicates the session TTL has elapsed.
	ErrIGSessionExpired = errors.New("ig session expired")

	// ErrIGThrottled indicates a 429 (or rate-limit message) from Instagram that persisted
	// after the client's backoff retries were exhausted.
	ErrIGThrottled = errors.New("ig request throttled (429)")

	// ErrIGDenied indicates a 401/403/checkpoint/login-redirect response from Instagram.
	ErrIGDenied = errors.New("ig request denied (auth failure)")

	// ErrIGNoSession indicates no cookies are configured for the account.
	ErrIGNoSession = errors.New("no ig session cookies configured")
)

// IGRequestLogWriter persists IG API request logs. Implemented by IGRequestLogStore.
type IGRequestLogWriter interface {
	// Insert writes a request log entry.
	Insert(log *model.IGRequestLog) error

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

// Clock abstracts time for testability.
type Clock interface {
	Now() time.Time
}

// RealClock uses the real system time.
type RealClock struct{}

// Now returns the current UTC time.
func (RealClock) Now() time.Time { return time.Now().UTC() }

// Sleeper waits for a duration or until the context is done.
type Sleeper interface {
	Sleep(ctx context.Context, d time.Duration) error
}

// RealSleeper sleeps using a timer and honours context cancellation.
type RealSleeper struct{}

// Sleep blocks for d or until ctx is done, returning ctx.Err() in the latter case.
func (RealSleeper) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// IPResolver resolves the public IP address.
type IPResolver interface {
	// Resolve returns the current public IP address.
	Resolve() (string, error)
}
