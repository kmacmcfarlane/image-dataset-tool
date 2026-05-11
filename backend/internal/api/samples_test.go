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

var _ = Describe("SamplesHandler", func() {
	var (
		tmpDir  string
		handler *api.SamplesHandler
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "samples-handler-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err := store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())

		// Seed project, subject, and samples.
		_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
			VALUES ('p1', 'proj', 'Project', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
			VALUES ('550e8400-e29b-41d4-a716-446655440000', 'p1', 'subj', 'Subject', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
			file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at)
			VALUES ('aaa11111-1111-1111-1111-111111111111', '550e8400-e29b-41d4-a716-446655440000', 'instagram', 'post1', 0,
			'/img1.jpg', 'sha1', 'ph1', 800, 600, '2026-01-01T00:00:00Z', 'pending', 0,
			'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
		Expect(err).NotTo(HaveOccurred())

		sampleStore := store.NewSampleStore(db)
		handler = api.NewSamplesHandler(sampleStore)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("ServeList", func() {
		It("returns 200 with samples array for valid subject_id", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/subjects/550e8400-e29b-41d4-a716-446655440000/samples", nil)
			w := httptest.NewRecorder()

			handler.ServeList(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp api.SampleListResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.Samples).To(HaveLen(1))
			Expect(resp.Samples[0].ID).To(Equal("aaa11111-1111-1111-1111-111111111111"))
			Expect(resp.Samples[0].Status).To(Equal("pending"))
			Expect(resp.Samples[0].ThumbnailPath).To(Equal("/v1/media/aaa11111-1111-1111-1111-111111111111/thumb"))
		})

		It("returns empty array for subject with no samples", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/subjects/00000000-0000-0000-0000-000000000000/samples", nil)
			w := httptest.NewRecorder()

			handler.ServeList(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp api.SampleListResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.Samples).To(BeEmpty())
		})

		It("returns 400 for missing subject_id", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/subjects//samples", nil)
			w := httptest.NewRecorder()

			handler.ServeList(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 400 for invalid subject_id format", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/subjects/../../etc/passwd/samples", nil)
			w := httptest.NewRecorder()

			handler.ServeList(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("ServeUpdateStatus", func() {
		It("updates status and returns updated sample", func() {
			body := strings.NewReader(`{"status":"kept"}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/aaa11111-1111-1111-1111-111111111111/status", body)
			w := httptest.NewRecorder()

			handler.ServeUpdateStatus(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp api.SampleResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.Status).To(Equal("kept"))
			Expect(resp.ID).To(Equal("aaa11111-1111-1111-1111-111111111111"))
		})

		It("returns 404 for non-existent sample", func() {
			body := strings.NewReader(`{"status":"kept"}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/00000000-0000-0000-0000-000000000000/status", body)
			w := httptest.NewRecorder()

			handler.ServeUpdateStatus(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 400 for invalid status value", func() {
			body := strings.NewReader(`{"status":"invalid"}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/aaa11111-1111-1111-1111-111111111111/status", body)
			w := httptest.NewRecorder()

			handler.ServeUpdateStatus(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 400 for missing status", func() {
			body := strings.NewReader(`{}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/aaa11111-1111-1111-1111-111111111111/status", body)
			w := httptest.NewRecorder()

			handler.ServeUpdateStatus(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 400 for invalid sample id", func() {
			body := strings.NewReader(`{"status":"kept"}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/not!valid/status", body)
			w := httptest.NewRecorder()

			handler.ServeUpdateStatus(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("ServeBulkUpdateStatus", func() {
		BeforeEach(func() {
			// Seed additional samples.
			dbPath := filepath.Join(tmpDir, "test.sqlite")
			db, err := store.OpenDB(dbPath)
			Expect(err).NotTo(HaveOccurred())
			defer db.Close()

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at)
				VALUES ('bbb22222-2222-2222-2222-222222222222', '550e8400-e29b-41d4-a716-446655440000', 'instagram', 'post2', 0,
				'/img2.jpg', 'sha2', 'ph2', 800, 600, '2026-01-01T00:00:00Z', 'pending', 0,
				'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates multiple samples atomically", func() {
			body := strings.NewReader(`{"ids":["aaa11111-1111-1111-1111-111111111111","bbb22222-2222-2222-2222-222222222222"],"status":"rejected"}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/bulk-status", body)
			w := httptest.NewRecorder()

			handler.ServeBulkUpdateStatus(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp api.BulkUpdateStatusResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.Updated).To(Equal(int64(2)))
		})

		It("returns 400 for empty ids", func() {
			body := strings.NewReader(`{"ids":[],"status":"kept"}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/bulk-status", body)
			w := httptest.NewRecorder()

			handler.ServeBulkUpdateStatus(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 400 for missing status", func() {
			body := strings.NewReader(`{"ids":["aaa11111-1111-1111-1111-111111111111"]}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/bulk-status", body)
			w := httptest.NewRecorder()

			handler.ServeBulkUpdateStatus(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 400 for invalid id format", func() {
			body := strings.NewReader(`{"ids":["not!valid"],"status":"kept"}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/bulk-status", body)
			w := httptest.NewRecorder()

			handler.ServeBulkUpdateStatus(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})
})
