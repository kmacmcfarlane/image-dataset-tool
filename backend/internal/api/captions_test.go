package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("CaptionsHandler", func() {
	var (
		tmpDir  string
		handler *api.CaptionsHandler
	)

	const sampleID = "aaa11111-1111-1111-1111-111111111111"

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "captions-handler-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err := store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())

		// Seed project, subject, and sample.
		_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
			VALUES ('p1', 'proj', 'Project', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
			VALUES ('s1', 'p1', 'subj', 'Subject', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, file_path, sha256, phash, width, height, fetched_at, created_at, updated_at)
			VALUES (?, 's1', 'instagram', 'post1', '/path/to/file', 'abc123', 'def456', 100, 100, '2026-01-01', '2026-01-01', '2026-01-01')`, sampleID)
		Expect(err).NotTo(HaveOccurred())

		// Seed a caption study.
		_, err = db.Exec(`INSERT INTO caption_studies (id, project_id, name, slug, provider, model, prompt_template, params_json, created_at, updated_at)
			VALUES ('study1', 'p1', 'GPT Study', 'gpt-study', 'openai', 'gpt-4', 'Describe this image.', '{}', '2026-01-01', '2026-01-01')`)
		Expect(err).NotTo(HaveOccurred())

		captionStore := store.NewCaptionStore(db)
		sampleStore := store.NewSampleStore(db)
		handler = api.NewCaptionsHandler(captionStore, sampleStore)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("ServeListCaptions", func() {
		It("returns empty list for sample with no captions", func() {
			req := httptest.NewRequest("GET", "/v1/samples/"+sampleID+"/captions", nil)
			rr := httptest.NewRecorder()
			handler.ServeListCaptions(rr, req)

			Expect(rr.Code).To(Equal(http.StatusOK))
			var resp api.CaptionListResponse
			Expect(json.NewDecoder(rr.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Captions).To(BeEmpty())
		})

		It("returns 404 for non-existent sample", func() {
			req := httptest.NewRequest("GET", "/v1/samples/00000000-0000-0000-0000-000000000000/captions", nil)
			rr := httptest.NewRecorder()
			handler.ServeListCaptions(rr, req)

			Expect(rr.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 400 for invalid sample ID", func() {
			req := httptest.NewRequest("GET", "/v1/samples/not!valid/captions", nil)
			rr := httptest.NewRecorder()
			handler.ServeListCaptions(rr, req)

			Expect(rr.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("ServeSaveCaption", func() {
		It("saves a caption for a specific study", func() {
			body := `{"text":"A test caption","study_id":"study1"}`
			req := httptest.NewRequest("PUT", "/v1/samples/"+sampleID+"/captions", strings.NewReader(body))
			rr := httptest.NewRecorder()
			handler.ServeSaveCaption(rr, req)

			Expect(rr.Code).To(Equal(http.StatusOK))
			var resp api.CaptionResponse
			Expect(json.NewDecoder(rr.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Text).To(Equal("A test caption"))
			Expect(resp.Study.Name).To(Equal("GPT Study"))
		})

		It("creates manual pseudo-study on first manual edit", func() {
			body := `{"text":"Manual caption","is_manual":true}`
			req := httptest.NewRequest("PUT", "/v1/samples/"+sampleID+"/captions", strings.NewReader(body))
			rr := httptest.NewRecorder()
			handler.ServeSaveCaption(rr, req)

			Expect(rr.Code).To(Equal(http.StatusOK))
			var resp api.CaptionResponse
			Expect(json.NewDecoder(rr.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Text).To(Equal("Manual caption"))
			Expect(resp.Study.Slug).To(Equal("manual"))
			Expect(resp.Study.Provider).To(Equal("manual"))
		})

		It("preserves original study caption when saving manual edit", func() {
			// First, save an auto-generated caption.
			body := `{"text":"Auto-generated caption","study_id":"study1"}`
			req := httptest.NewRequest("PUT", "/v1/samples/"+sampleID+"/captions", strings.NewReader(body))
			rr := httptest.NewRecorder()
			handler.ServeSaveCaption(rr, req)
			Expect(rr.Code).To(Equal(http.StatusOK))

			// Now save a manual edit.
			body = `{"text":"My manual edit","is_manual":true}`
			req = httptest.NewRequest("PUT", "/v1/samples/"+sampleID+"/captions", strings.NewReader(body))
			rr = httptest.NewRecorder()
			handler.ServeSaveCaption(rr, req)
			Expect(rr.Code).To(Equal(http.StatusOK))

			// Verify both captions exist.
			req = httptest.NewRequest("GET", "/v1/samples/"+sampleID+"/captions", nil)
			rr = httptest.NewRecorder()
			handler.ServeListCaptions(rr, req)
			Expect(rr.Code).To(Equal(http.StatusOK))

			var listResp api.CaptionListResponse
			Expect(json.NewDecoder(rr.Body).Decode(&listResp)).To(Succeed())
			Expect(listResp.Captions).To(HaveLen(2))

			// Find original and manual.
			var originalText, manualText string
			for _, c := range listResp.Captions {
				if c.Study.Slug == "gpt-study" {
					originalText = c.Text
				} else if c.Study.Slug == "manual" {
					manualText = c.Text
				}
			}
			Expect(originalText).To(Equal("Auto-generated caption"))
			Expect(manualText).To(Equal("My manual edit"))
		})

		It("returns 404 for non-existent sample", func() {
			body := `{"text":"Will fail","is_manual":true}`
			req := httptest.NewRequest("PUT", "/v1/samples/00000000-0000-0000-0000-000000000000/captions", strings.NewReader(body))
			rr := httptest.NewRecorder()
			handler.ServeSaveCaption(rr, req)

			Expect(rr.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 400 when neither study_id nor is_manual provided", func() {
			body := `{"text":"No study"}`
			req := httptest.NewRequest("PUT", "/v1/samples/"+sampleID+"/captions", strings.NewReader(body))
			rr := httptest.NewRecorder()
			handler.ServeSaveCaption(rr, req)

			Expect(rr.Code).To(Equal(http.StatusBadRequest))
		})
	})
})
