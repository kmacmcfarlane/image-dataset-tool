package api_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
)

var _ = Describe("Middleware", func() {
	Describe("CORSMiddleware", func() {
		It("sets CORS headers on normal requests", func() {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := api.CORSMiddleware(inner)
			req := httptest.NewRequest("GET", "/health", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			Expect(rec.Header().Get("Access-Control-Allow-Origin")).To(Equal("http://localhost:3000"))
			Expect(rec.Header().Get("Access-Control-Allow-Methods")).To(ContainSubstring("GET"))
		})

		It("responds to OPTIONS preflight with 204", func() {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := api.CORSMiddleware(inner)
			req := httptest.NewRequest("OPTIONS", "/v1/settings/secrets", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNoContent))
		})
	})

	Describe("ErrorMiddleware", func() {
		It("catches panics and returns JSON error", func() {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("test panic")
			})

			handler := api.ErrorMiddleware(inner)
			req := httptest.NewRequest("GET", "/panic", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
			Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(rec.Body.String()).To(ContainSubstring("internal server error"))
		})
	})
})
