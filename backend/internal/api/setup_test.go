package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
)

var _ = Describe("SetupHandler", func() {
	var (
		tmpDir  string
		handler *api.SetupHandler
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "setup-test-*")
		Expect(err).NotTo(HaveOccurred())
		handler = api.NewSetupHandler(tmpDir)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("ServeStatus", func() {
		Context("when the encryption key is missing", func() {
			It("returns needs_setup=true and key_status=missing", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
				w := httptest.NewRecorder()

				handler.ServeStatus(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				var resp api.SetupStatusResponse
				Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
				Expect(resp.NeedsSetup).To(BeTrue())
				Expect(resp.KeyStatus).To(Equal("missing"))
				Expect(resp.DataDirWritable).To(BeTrue())
				Expect(resp.DataDir).To(Equal(tmpDir))
			})
		})

		Context("when the encryption key is present", func() {
			BeforeEach(func() {
				keyPath := tmpDir + "/secret.key"
				key := make([]byte, crypto.KeySize)
				for i := range key {
					key[i] = byte(i + 1)
				}
				Expect(os.WriteFile(keyPath, key, 0600)).To(Succeed())
			})

			It("returns needs_setup=false and key_status=found", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
				w := httptest.NewRecorder()

				handler.ServeStatus(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				var resp api.SetupStatusResponse
				Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
				Expect(resp.NeedsSetup).To(BeFalse())
				Expect(resp.KeyStatus).To(Equal("found"))
				Expect(resp.DataDirWritable).To(BeTrue())
			})
		})

		Context("when the data directory is not writable", func() {
			It("returns needs_setup=true and data_dir_writable=false", func() {
				// Use a non-existent directory to simulate not writable.
				h := api.NewSetupHandler("/nonexistent/path/that/does/not/exist")
				req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
				w := httptest.NewRecorder()

				h.ServeStatus(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				var resp api.SetupStatusResponse
				Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
				Expect(resp.NeedsSetup).To(BeTrue())
				Expect(resp.DataDirWritable).To(BeFalse())
			})
		})
	})

	Describe("ServeGenerateKey", func() {
		Context("when the key already exists", func() {
			BeforeEach(func() {
				keyPath := tmpDir + "/secret.key"
				key := make([]byte, crypto.KeySize)
				Expect(os.WriteFile(keyPath, key, 0600)).To(Succeed())
			})

			It("returns 400 with ok=false", func() {
				req := httptest.NewRequest(http.MethodPost, "/api/setup/generate-key", nil)
				w := httptest.NewRecorder()

				handler.ServeGenerateKey(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
				var resp api.GenerateKeyResponse
				Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
				Expect(resp.OK).To(BeFalse())
			})
		})

		Context("when no key exists", func() {
			It("returns 200 with ok=true and creates the key file", func() {
				req := httptest.NewRequest(http.MethodPost, "/api/setup/generate-key", nil)
				w := httptest.NewRecorder()

				handler.ServeGenerateKey(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				var resp api.GenerateKeyResponse
				Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
				Expect(resp.OK).To(BeTrue())

				// Verify the key file was created with correct permissions and size.
				keyPath := tmpDir + "/secret.key"
				info, err := os.Stat(keyPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
				content, err := os.ReadFile(keyPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(content).To(HaveLen(crypto.KeySize))
			})
		})
	})
})
