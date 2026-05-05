package pipeline_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
)

var _ = Describe("ProviderLimiters", func() {
	It("throttles requests to the configured RPM", func() {
		// 600 RPM = 10 requests per second = ~100ms between requests.
		limiters := pipeline.NewProviderLimiters(map[string]pipeline.RateLimiterConfig{
			"instagram": {RPM: 600},
		})

		ctx := context.Background()
		start := time.Now()

		// First request should be immediate (burst of 1).
		err := limiters.Wait(ctx, "instagram")
		Expect(err).NotTo(HaveOccurred())

		// Second request should be delayed by ~100ms.
		err = limiters.Wait(ctx, "instagram")
		Expect(err).NotTo(HaveOccurred())
		elapsed := time.Since(start)

		// Should take at least 80ms (allowing some tolerance).
		Expect(elapsed).To(BeNumerically(">=", 80*time.Millisecond))
	})

	It("allows unlimited requests for unconfigured providers", func() {
		limiters := pipeline.NewProviderLimiters(map[string]pipeline.RateLimiterConfig{
			"instagram": {RPM: 10},
		})

		ctx := context.Background()
		start := time.Now()

		// Unknown provider should not be rate-limited.
		for i := 0; i < 10; i++ {
			err := limiters.Wait(ctx, "unknown-provider")
			Expect(err).NotTo(HaveOccurred())
		}

		elapsed := time.Since(start)
		Expect(elapsed).To(BeNumerically("<", 1*time.Second))
	})

	It("returns stats for configured providers", func() {
		limiters := pipeline.NewProviderLimiters(map[string]pipeline.RateLimiterConfig{
			"instagram": {RPM: 600},
		})

		rpmLimit, tokens := limiters.Stats("instagram")
		Expect(rpmLimit).To(Equal(600))
		Expect(tokens).To(BeNumerically("<=", 1.0))
		Expect(tokens).To(BeNumerically(">=", 0.0))
	})

	It("respects context cancellation", func() {
		limiters := pipeline.NewProviderLimiters(map[string]pipeline.RateLimiterConfig{
			"slow": {RPM: 1}, // 1 RPM = 1 req per 60 seconds
		})

		ctx := context.Background()
		// Consume the initial burst.
		err := limiters.Wait(ctx, "slow")
		Expect(err).NotTo(HaveOccurred())

		// Cancel context before the next token is available.
		cancelCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err = limiters.Wait(cancelCtx, "slow")
		Expect(err).To(HaveOccurred()) // should fail due to context cancellation
	})
})
