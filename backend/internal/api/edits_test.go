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
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/fileformat"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("EditsHandler", func() {
	var (
		tmpDir  string
		handler *api.EditsHandler
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "edits-handler-test-*")
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

		sampleID := "aaa11111-1111-1111-1111-111111111111"

		// Create on-disk structure for the sample manifest.
		samplesDir := filepath.Join(tmpDir, "projects", "proj", "subjects", "subj", "samples")
		Expect(os.MkdirAll(samplesDir, 0755)).To(Succeed())

		filePath := "projects/proj/subjects/subj/samples/" + sampleID + ".jpg"

		_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
			file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at)
			VALUES (?, 's1', 'instagram', 'post1', 0,
			?, 'sha1', 'ph1', 800, 600, '2026-01-01T00:00:00Z', 'pending', 0,
			'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, sampleID, filePath)
		Expect(err).NotTo(HaveOccurred())

		// Write a sample manifest.
		meta := &fileformat.SampleMetadata{
			ID: sampleID,
			Source: fileformat.SampleSource{
				Platform: "instagram", PostID: "post1", SlideIndex: 0,
			},
			File: fileformat.SampleFile{
				Path: sampleID + ".jpg", SHA256: "sha1", PHash: "ph1",
				Width: 800, Height: 600,
			},
			Status:    "pending",
			Captions:  map[string]fileformat.Caption{},
			Edits:     fileformat.SampleEdits{AutoActions: []string{}},
			FetchedAt: "2026-01-01T00:00:00Z",
			CreatedAt: "2026-01-01T00:00:00Z",
			UpdatedAt: "2026-01-01T00:00:00Z",
		}
		Expect(fileformat.WriteSampleMetadata(filepath.Join(samplesDir, sampleID+".json"), meta)).To(Succeed())

		sampleStore := store.NewSampleStore(db)
		editStore := store.NewEditStore(db)
		editService := service.NewEditService(editStore, sampleStore, tmpDir)
		handler = api.NewEditsHandler(editService)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("ServeGetEdits", func() {
		It("returns default edits when none exist", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/samples/aaa11111-1111-1111-1111-111111111111/edits", nil)
			w := httptest.NewRecorder()

			handler.ServeGetEdits(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp api.EditResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.RotationDeg).To(Equal(0.0))
			Expect(resp.CropBox).To(BeNil())
			Expect(resp.AutoActions).To(BeEmpty())
		})

		It("returns 400 for invalid sample id", func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/samples/not!valid/edits", nil)
			w := httptest.NewRecorder()

			handler.ServeGetEdits(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("ServePutEdits", func() {
		It("creates edits and updates manifest", func() {
			body := strings.NewReader(`{"rotation_deg":90,"auto_actions":["remove_borders"]}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/aaa11111-1111-1111-1111-111111111111/edits", body)
			w := httptest.NewRecorder()

			handler.ServePutEdits(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp api.EditResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.RotationDeg).To(Equal(90.0))
			Expect(resp.AutoActions).To(ConsistOf("remove_borders"))

			// Verify manifest was updated.
			sampleID := "aaa11111-1111-1111-1111-111111111111"
			manifestPath := filepath.Join(tmpDir, "projects", "proj", "subjects", "subj", "samples", sampleID+".json")
			meta, err := fileformat.ReadSampleMetadata(manifestPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(meta.Edits.RotationDeg).To(Equal(90.0))
			Expect(meta.Edits.AutoActions).To(ConsistOf("remove_borders"))
		})

		It("creates edits with crop box", func() {
			body := strings.NewReader(`{"rotation_deg":0,"crop_box":{"x":10,"y":20,"w":100,"h":200}}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/aaa11111-1111-1111-1111-111111111111/edits", body)
			w := httptest.NewRecorder()

			handler.ServePutEdits(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp api.EditResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.CropBox).NotTo(BeNil())
			Expect(resp.CropBox.X).To(Equal(10.0))
			Expect(resp.CropBox.W).To(Equal(100.0))
		})

		It("is idempotent (same edits twice produces same result)", func() {
			bodyStr := `{"rotation_deg":90}`

			body1 := strings.NewReader(bodyStr)
			req1 := httptest.NewRequest(http.MethodPut, "/v1/samples/aaa11111-1111-1111-1111-111111111111/edits", body1)
			w1 := httptest.NewRecorder()
			handler.ServePutEdits(w1, req1)
			Expect(w1.Code).To(Equal(http.StatusOK))

			body2 := strings.NewReader(bodyStr)
			req2 := httptest.NewRequest(http.MethodPut, "/v1/samples/aaa11111-1111-1111-1111-111111111111/edits", body2)
			w2 := httptest.NewRecorder()
			handler.ServePutEdits(w2, req2)
			Expect(w2.Code).To(Equal(http.StatusOK))

			var resp1, resp2 api.EditResponse
			Expect(json.Unmarshal(w1.Body.Bytes(), &resp1)).To(Succeed())
			Expect(json.Unmarshal(w2.Body.Bytes(), &resp2)).To(Succeed())
			Expect(resp1.RotationDeg).To(Equal(resp2.RotationDeg))
		})

		It("clearing all edits resets to original", func() {
			// First apply some edits.
			body := strings.NewReader(`{"rotation_deg":90}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/aaa11111-1111-1111-1111-111111111111/edits", body)
			w := httptest.NewRecorder()
			handler.ServePutEdits(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))

			// Now clear all edits (empty edit = reset).
			body2 := strings.NewReader(`{"rotation_deg":0}`)
			req2 := httptest.NewRequest(http.MethodPut, "/v1/samples/aaa11111-1111-1111-1111-111111111111/edits", body2)
			w2 := httptest.NewRecorder()
			handler.ServePutEdits(w2, req2)
			Expect(w2.Code).To(Equal(http.StatusOK))

			var resp api.EditResponse
			Expect(json.Unmarshal(w2.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.RotationDeg).To(Equal(0.0))
			Expect(resp.CropBox).To(BeNil())
		})

		It("returns 404 for non-existent sample", func() {
			body := strings.NewReader(`{"rotation_deg":90}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/00000000-0000-0000-0000-000000000000/edits", body)
			w := httptest.NewRecorder()

			handler.ServePutEdits(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 400 for invalid JSON body", func() {
			body := strings.NewReader(`{invalid}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/aaa11111-1111-1111-1111-111111111111/edits", body)
			w := httptest.NewRecorder()

			handler.ServePutEdits(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("ServeDeleteEdits", func() {
		It("resets edits to original", func() {
			// Apply edits first.
			body := strings.NewReader(`{"rotation_deg":90}`)
			req := httptest.NewRequest(http.MethodPut, "/v1/samples/aaa11111-1111-1111-1111-111111111111/edits", body)
			w := httptest.NewRecorder()
			handler.ServePutEdits(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))

			// Delete edits.
			delReq := httptest.NewRequest(http.MethodDelete, "/v1/samples/aaa11111-1111-1111-1111-111111111111/edits", nil)
			delW := httptest.NewRecorder()
			handler.ServeDeleteEdits(delW, delReq)
			Expect(delW.Code).To(Equal(http.StatusNoContent))

			// Verify GET returns defaults.
			getReq := httptest.NewRequest(http.MethodGet, "/v1/samples/aaa11111-1111-1111-1111-111111111111/edits", nil)
			getW := httptest.NewRecorder()
			handler.ServeGetEdits(getW, getReq)
			Expect(getW.Code).To(Equal(http.StatusOK))

			var resp api.EditResponse
			Expect(json.Unmarshal(getW.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.RotationDeg).To(Equal(0.0))
		})
	})
})
