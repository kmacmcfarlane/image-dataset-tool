package ig_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service/ig"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("Sentinel errors", func() {
	DescribeTable("match the store implementation's errors, including when wrapped",
		func(serviceErr, storeErr error) {
			Expect(errors.Is(storeErr, serviceErr)).To(BeTrue())
			Expect(errors.Is(fmt.Errorf("wrapped: %w", storeErr), serviceErr)).To(BeTrue())
		},
		Entry("ErrSessionExpired", ig.ErrSessionExpired, store.ErrIGSessionExpired),
		Entry("ErrThrottled", ig.ErrThrottled, store.ErrIGThrottled),
		Entry("ErrDenied", ig.ErrDenied, store.ErrIGDenied),
		Entry("ErrNoSession", ig.ErrNoSession, store.ErrIGNoSession),
	)

	It("keeps the sentinels distinct", func() {
		all := []error{ig.ErrSessionExpired, ig.ErrThrottled, ig.ErrDenied, ig.ErrNoSession}
		for i, a := range all {
			for j, b := range all {
				Expect(errors.Is(a, b)).To(Equal(i == j))
			}
		}
	})
})
