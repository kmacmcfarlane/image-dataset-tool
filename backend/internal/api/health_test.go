package api_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}

var _ = Describe("HealthService", func() {
	var svc *api.HealthService

	BeforeEach(func() {
		svc = api.NewHealthService()
	})

	Describe("Check", func() {
		It("returns ok status", func() {
			result, err := svc.Check(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Status).To(Equal("ok"))
		})
	})
})
