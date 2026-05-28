package service

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// SupportedPlatforms is the list of allowed platform values.
var SupportedPlatforms = []string{"instagram"}

// PlatformRequiredCookieKeys maps each platform to its required cookie key names.
var PlatformRequiredCookieKeys = map[string][]string{
	"instagram": {"sessionid", "csrftoken", "ds_user_id"},
}

// isSupportedPlatform checks if a platform is in the allowed list.
func isSupportedPlatform(platform string) bool {
	return slices.Contains(SupportedPlatforms, platform)
}

// parseCookieKeys extracts cookie key names from a cookie string.
func parseCookieKeys(cookies string) map[string]bool {
	keys := make(map[string]bool)
	for pair := range strings.SplitSeq(cookies, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eqIdx := strings.Index(pair, "=")
		if eqIdx > 0 {
			keys[strings.TrimSpace(pair[:eqIdx])] = true
		}
	}
	return keys
}

// validateCookieKeys checks that required cookie keys for a platform are present.
// Returns a list of missing key names, or nil if all required keys are present.
func validateCookieKeys(platform, cookies string) []string {
	required, ok := PlatformRequiredCookieKeys[platform]
	if !ok {
		return nil
	}
	present := parseCookieKeys(cookies)
	var missing []string
	for _, key := range required {
		if !present[key] {
			missing = append(missing, key)
		}
	}
	return missing
}

// AccountRepository defines the persistence interface used by AccountService.
type AccountRepository interface {
	Create(a *model.Account) error
	Get(id string) (*model.Account, error)
	GetDetail(id string) (*model.AccountDetail, error)
	List() ([]*model.Account, error)
	UpdateCookies(id string, cookies []byte) error
	Delete(id string) error
	HandleExists(platform, handle string) (bool, error)
}

// AccountService handles account business logic.
type AccountService struct {
	store AccountRepository
}

// NewAccountService creates a new AccountService.
func NewAccountService(store AccountRepository) *AccountService {
	return &AccountService{store: store}
}

// Create creates a new account.
func (s *AccountService) Create(platform, handle string) (*model.Account, error) {
	platform = strings.TrimSpace(strings.ToLower(platform))
	handle = strings.TrimSpace(handle)

	if platform == "" {
		return nil, fmt.Errorf("platform is required: %w", ErrBadRequest)
	}
	if !isSupportedPlatform(platform) {
		return nil, fmt.Errorf("unsupported platform %q; supported: %s: %w",
			platform, strings.Join(SupportedPlatforms, ", "), ErrBadRequest)
	}
	if handle == "" {
		return nil, fmt.Errorf("handle is required: %w", ErrBadRequest)
	}

	// Check uniqueness.
	exists, err := s.store.HandleExists(platform, handle)
	if err != nil {
		return nil, fmt.Errorf("check handle: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("account (%s, %s) already exists: %w", platform, handle, ErrConflict)
	}

	id := uuid.New().String()
	a := &model.Account{
		ID:       id,
		Platform: platform,
		Handle:   handle,
	}

	if err := s.store.Create(a); err != nil {
		return nil, fmt.Errorf("store account: %w", err)
	}

	slog.Info("Account created", "id", a.ID, "platform", a.Platform, "handle", a.Handle)

	return a, nil
}

// Get retrieves an account by ID. Never returns cookies.
func (s *AccountService) Get(id string) (*model.Account, error) {
	a, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("account %q: %w", id, ErrNotFound)
		}
		return nil, err
	}
	return a, nil
}

// GetDetail retrieves an account by ID with linked subjects.
func (s *AccountService) GetDetail(id string) (*model.AccountDetail, error) {
	a, err := s.store.GetDetail(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("account %q: %w", id, ErrNotFound)
		}
		return nil, err
	}
	return a, nil
}

// List returns all accounts. Never returns cookies.
func (s *AccountService) List() ([]*model.Account, error) {
	return s.store.List()
}

// UpdateCookies encrypts and stores cookies for an account.
// Cookies content is never logged.
func (s *AccountService) UpdateCookies(id string, cookies string) error {
	acct, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("account %q: %w", id, ErrNotFound)
		}
		return err
	}

	if missing := validateCookieKeys(acct.Platform, cookies); len(missing) > 0 {
		return fmt.Errorf("missing required cookie keys for %s: %s: %w",
			acct.Platform, strings.Join(missing, ", "), ErrBadRequest)
	}

	if err := s.store.UpdateCookies(id, []byte(cookies)); err != nil {
		return fmt.Errorf("update cookies: %w", err)
	}

	slog.Info("Account cookies updated", "id", id)
	return nil
}

// Delete removes an account and all subject_accounts links (via CASCADE).
func (s *AccountService) Delete(id string) error {
	if _, err := s.store.Get(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("account %q: %w", id, ErrNotFound)
		}
		return err
	}

	if err := s.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("account %q: %w", id, ErrNotFound)
		}
		return err
	}

	slog.Info("Account deleted", "id", id)
	return nil
}
