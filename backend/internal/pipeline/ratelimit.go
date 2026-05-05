package pipeline

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// RateLimiterConfig holds per-provider rate limiting configuration.
type RateLimiterConfig struct {
	// RPM is the maximum requests per minute for this provider.
	RPM int
}

// ProviderLimiters manages per-provider rate limiters.
type ProviderLimiters struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	configs  map[string]RateLimiterConfig
}

// NewProviderLimiters creates a limiter registry from provider configs.
// The configs map is keyed by provider name (e.g. "instagram", "anthropic").
func NewProviderLimiters(configs map[string]RateLimiterConfig) *ProviderLimiters {
	pl := &ProviderLimiters{
		limiters: make(map[string]*rate.Limiter, len(configs)),
		configs:  configs,
	}
	for name, cfg := range configs {
		if cfg.RPM > 0 {
			// rate.Limit converts RPM to requests-per-second.
			rps := rate.Limit(float64(cfg.RPM) / 60.0)
			// Burst of 1: strict throttle, no bursting above the rate.
			pl.limiters[name] = rate.NewLimiter(rps, 1)
		}
	}
	return pl
}

// Wait blocks until the rate limiter for the given provider allows an event,
// or until ctx is cancelled. Returns nil if no limiter is configured for
// the provider (unlimited).
func (pl *ProviderLimiters) Wait(ctx context.Context, provider string) error {
	pl.mu.RLock()
	lim, ok := pl.limiters[provider]
	pl.mu.RUnlock()
	if !ok {
		return nil // no limiter configured — unlimited
	}
	return lim.Wait(ctx)
}

// Stats returns the current rate usage for a provider.
// Returns rpm limit and approximate tokens available.
func (pl *ProviderLimiters) Stats(provider string) (rpmLimit int, tokensAvailable float64) {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	cfg, ok := pl.configs[provider]
	if !ok {
		return 0, 0
	}
	lim, ok := pl.limiters[provider]
	if !ok {
		return cfg.RPM, float64(cfg.RPM)
	}
	return cfg.RPM, lim.Tokens()
}
