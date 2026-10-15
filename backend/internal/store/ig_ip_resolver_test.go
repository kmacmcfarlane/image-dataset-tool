package store_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("HTTPIPResolver", func() {
	It("returns the trimmed IP from the response body", func() {
		ts := httptest.NewServer(respondWith(http.StatusOK, []byte(" 203.0.113.7\n")))
		defer ts.Close()

		ip, err := store.NewHTTPIPResolverWithURL(ts.Client(), ts.URL).Resolve()
		Expect(err).NotTo(HaveOccurred())
		Expect(ip).To(Equal("203.0.113.7"))
	})

	It("returns an error on a non-200 response", func() {
		ts := httptest.NewServer(respondWith(http.StatusServiceUnavailable, nil))
		defer ts.Close()

		_, err := store.NewHTTPIPResolverWithURL(ts.Client(), ts.URL).Resolve()
		Expect(err).To(MatchError(ContainSubstring("HTTP 503")))
	})

	It("returns an error when the service is unreachable", func() {
		ts := httptest.NewServer(respondWith(http.StatusOK, nil))
		url := ts.URL
		client := ts.Client()
		ts.Close()

		_, err := store.NewHTTPIPResolverWithURL(client, url).Resolve()
		Expect(err).To(MatchError(ContainSubstring("resolve public IP")))
	})
})
