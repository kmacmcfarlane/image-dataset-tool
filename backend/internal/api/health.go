package api

import (
	"context"

	health "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/health"
)

// HealthService implements the health service interface.
type HealthService struct{}

// NewHealthService creates a new health service.
func NewHealthService() *HealthService {
	return &HealthService{}
}

// Check returns the current health status.
func (s *HealthService) Check(_ context.Context) (*health.HealthResult, error) {
	return &health.HealthResult{
		Status: "ok",
	}, nil
}
