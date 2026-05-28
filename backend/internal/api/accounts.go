package api

import (
	"context"
	"errors"
	"fmt"

	"log/slog"

	accounts "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/accounts"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
)

// AccountsService implements the Goa accounts service interface.
type AccountsService struct {
	svc *service.AccountService
}

// NewAccountsService creates a new AccountsService.
func NewAccountsService(svc *service.AccountService) *AccountsService {
	return &AccountsService{svc: svc}
}

// List returns all accounts. Never returns cookies.
func (s *AccountsService) List(_ context.Context) (*accounts.AccountListResult, error) {
	list, err := s.svc.List()
	if err != nil {
		slog.Error("Failed to list accounts", "error", err)
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	result := &accounts.AccountListResult{
		Accounts: make([]*accounts.AccountResult, 0, len(list)),
	}
	for _, a := range list {
		result.Accounts = append(result.Accounts, mapAccountToResult(a))
	}
	return result, nil
}

// Get returns an account by ID with linked subjects. Never returns cookies.
func (s *AccountsService) Get(_ context.Context, p *accounts.GetPayload) (*accounts.AccountDetailResult, error) {
	detail, err := s.svc.GetDetail(p.ID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, accounts.MakeNotFound(fmt.Errorf("account %q not found", p.ID))
		}
		return nil, fmt.Errorf("get account: %w", err)
	}

	linkedSubjects := make([]*accounts.AccountLinkedSubjectResult, 0, len(detail.LinkedSubjects))
	for _, sub := range detail.LinkedSubjects {
		linkedSubjects = append(linkedSubjects, &accounts.AccountLinkedSubjectResult{
			ID:        sub.ID,
			ProjectID: sub.ProjectID,
			Name:      sub.Name,
		})
	}

	return &accounts.AccountDetailResult{
		ID:             detail.ID,
		Platform:       detail.Platform,
		Handle:         detail.Handle,
		HasCookies:     detail.HasCookies,
		LastLoginAt:    detail.LastLoginAt,
		LinkedSubjects: linkedSubjects,
		CreatedAt:      detail.CreatedAt,
		UpdatedAt:      detail.UpdatedAt,
	}, nil
}

// Create creates a new account.
func (s *AccountsService) Create(_ context.Context, payload *accounts.CreateAccountPayload) (*accounts.AccountResult, error) {
	a, err := s.svc.Create(payload.Platform, payload.Handle)
	if err != nil {
		if errors.Is(err, service.ErrConflict) {
			return nil, accounts.MakeConflict(fmt.Errorf("an account with this platform and handle already exists"))
		}
		if errors.Is(err, service.ErrBadRequest) {
			return nil, accounts.MakeBadRequest(err)
		}
		return nil, fmt.Errorf("create account: %w", err)
	}

	return mapAccountToResult(a), nil
}

// UpdateCookies encrypts and stores cookies for an account.
func (s *AccountsService) UpdateCookies(_ context.Context, payload *accounts.UpdateCookiesPayload) error {
	err := s.svc.UpdateCookies(payload.ID, payload.Cookies)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return accounts.MakeNotFound(fmt.Errorf("account %q not found", payload.ID))
		}
		if errors.Is(err, service.ErrBadRequest) {
			return accounts.MakeBadRequest(err)
		}
		return fmt.Errorf("update cookies: %w", err)
	}
	return nil
}

// Delete removes an account.
func (s *AccountsService) Delete(_ context.Context, payload *accounts.DeletePayload) error {
	err := s.svc.Delete(payload.ID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return accounts.MakeNotFound(fmt.Errorf("account %q not found", payload.ID))
		}
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}

// mapAccountToResult converts a model.Account to Goa AccountResult.
func mapAccountToResult(a *model.Account) *accounts.AccountResult {
	return &accounts.AccountResult{
		ID:          a.ID,
		Platform:    a.Platform,
		Handle:      a.Handle,
		HasCookies:  a.HasCookies,
		LastLoginAt: a.LastLoginAt,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}
