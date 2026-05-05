package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	logrus "github.com/sirupsen/logrus"

	settings "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/settings"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/datadir"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// SettingsService implements the settings service interface.
type SettingsService struct {
	secrets *store.SecretsStore
	dataDir string
	keyPath string
	config  map[string]any
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(secrets *store.SecretsStore, dir string, cfg map[string]any) *SettingsService {
	return &SettingsService{
		secrets: secrets,
		dataDir: dir,
		keyPath: datadir.SecretKeyPath(dir),
		config:  cfg,
	}
}

// GetInfo returns system info including data dir path, key status, and config.
func (s *SettingsService) GetInfo(_ context.Context) (*settings.SettingsInfo, error) {
	keyStatus := resolveKeyStatus(s.keyPath)

	cfg := s.config
	if cfg == nil {
		cfg = map[string]any{}
	}

	return &settings.SettingsInfo{
		DataDir:   s.dataDir,
		KeyStatus: keyStatus,
		Config:    cfg,
	}, nil
}

// resolveKeyStatus checks the encryption key file and returns a status string.
func resolveKeyStatus(keyPath string) string {
	_, err := crypto.LoadKey(keyPath)
	if err == nil {
		return "found"
	}
	if errors.Is(err, crypto.ErrKeyMissing) {
		return "missing"
	}
	if errors.Is(err, crypto.ErrKeyPermissions) {
		return "wrong_permissions"
	}
	return "missing"
}

// ListSecrets returns all secret keys (not values).
func (s *SettingsService) ListSecrets(_ context.Context) (*settings.SecretListResult, error) {
	entries, err := s.secrets.List()
	if err != nil {
		logrus.WithError(err).Error("Failed to list secrets")
		return nil, fmt.Errorf("list secrets: %w", err)
	}

	result := &settings.SecretListResult{
		Secrets: make([]*settings.SecretEntry, 0, len(entries)),
	}
	for _, e := range entries {
		result.Secrets = append(result.Secrets, &settings.SecretEntry{
			Key:       e.Key,
			CreatedAt: e.CreatedAt,
			UpdatedAt: e.UpdatedAt,
		})
	}
	return result, nil
}

// SetSecret creates or updates an encrypted secret.
func (s *SettingsService) SetSecret(_ context.Context, p *settings.SetSecretPayload) error {
	if err := s.secrets.Set(p.Key, p.Value); err != nil {
		logrus.WithError(err).WithField("key", p.Key).Error("Failed to set secret")
		return fmt.Errorf("set secret: %w", err)
	}
	logrus.WithField("key", p.Key).Info("Secret upserted")
	return nil
}

// DeleteSecret removes a secret by key.
func (s *SettingsService) DeleteSecret(_ context.Context, p *settings.DeleteSecretPayload) error {
	if err := s.secrets.Delete(p.Key); err != nil {
		logrus.WithError(err).WithField("key", p.Key).Error("Failed to delete secret")
		return fmt.Errorf("delete secret: %w", err)
	}
	logrus.WithField("key", p.Key).Info("Secret deleted")
	return nil
}

// TestProvider tests connectivity to a provider using the stored API key secret.
func (s *SettingsService) TestProvider(_ context.Context, p *settings.TestProviderPayload) (*settings.TestProviderResult, error) {
	log := logrus.WithField("provider", p.Provider)

	// Determine the expected secret key name for the provider.
	secretKey := providerSecretKey(p.Provider)
	if secretKey == "" {
		log.Warn("Unknown provider for connection test")
		return &settings.TestProviderResult{
			Success: false,
			Message: fmt.Sprintf("Unknown provider: %s", p.Provider),
		}, nil
	}

	// Check if an API key secret is stored.
	value, err := s.secrets.Get(secretKey)
	if err != nil {
		if isNotFound(err) {
			return &settings.TestProviderResult{
				Success: false,
				Message: fmt.Sprintf("No API key found for provider %s (secret key: %s)", p.Provider, secretKey),
			}, nil
		}
		return nil, fmt.Errorf("retrieve secret for provider test: %w", err)
	}

	if value == "" {
		return &settings.TestProviderResult{
			Success: false,
			Message: fmt.Sprintf("API key for %s is empty", p.Provider),
		}, nil
	}

	// Perform a lightweight connectivity check using the provider's health/models endpoint.
	msg, ok := testProviderConnectivity(p.Provider, value)
	return &settings.TestProviderResult{
		Success: ok,
		Message: msg,
	}, nil
}

// providerSecretKey returns the secrets table key name for a provider.
func providerSecretKey(provider string) string {
	switch provider {
	case "anthropic":
		return "anthropic_api_key"
	case "openai":
		return "openai_api_key"
	case "xai":
		return "xai_api_key"
	case "gemini":
		return "gemini_api_key"
	default:
		return ""
	}
}

// isNotFound checks whether an error indicates a missing row.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, sql.ErrNoRows)
}

// testProviderConnectivity makes a lightweight HTTP call to verify the provider API key.
// Returns (message, success).
func testProviderConnectivity(provider, apiKey string) (string, bool) {
	var url, authHeader string

	switch provider {
	case "anthropic":
		url = "https://api.anthropic.com/v1/models"
		authHeader = "x-api-key"
	case "openai":
		url = "https://api.openai.com/v1/models"
		authHeader = "Authorization"
		apiKey = "Bearer " + apiKey
	case "xai":
		url = "https://api.x.ai/v1/models"
		authHeader = "Authorization"
		apiKey = "Bearer " + apiKey
	case "gemini":
		url = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", apiKey)
		authHeader = ""
	default:
		return fmt.Sprintf("Unknown provider: %s", provider), false
	}

	req, err := newHTTPRequest(url, authHeader, apiKey)
	if err != nil {
		return fmt.Sprintf("Failed to create request: %v", err), false
	}

	client := defaultHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Connection failed: %v", err), false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 204 {
		return fmt.Sprintf("Connection to %s successful (HTTP %d)", provider, resp.StatusCode), true
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Sprintf("Authentication failed for %s (HTTP %d): invalid API key", provider, resp.StatusCode), false
	}
	return fmt.Sprintf("Provider %s returned HTTP %d", provider, resp.StatusCode), false
}
