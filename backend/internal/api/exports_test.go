package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("ExportsHandler", func() {
	var (
		db        *sql.DB
		tmpDir    string
		natsSrv   *natsutil.Server
		handler   *api.ExportsHandler
		cleanup   func()
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "exports-handler-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err = store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())

		natsDir := filepath.Join(tmpDir, "nats")
		natsCfg := natsutil.DefaultConfig(natsDir)
		natsSrv, err = natsutil.New(natsCfg)
		Expect(err).NotTo(HaveOccurred())

		jobStore := store.NewJobStore(db)
		exportSvc := service.NewExportService(db, natsSrv.JS(), jobStore, tmpDir)
		handler = api.NewExportsHandler(exportSvc)

		cleanup = func() {
			natsSrv.Shutdown()
			db.Close()
			os.RemoveAll(tmpDir)
		}
	})

	AfterEach(func() {
		cleanup()
	})

	seedData := func() string {
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
			VALUES ('proj-1', 'test-project', 'Test Project', ?, ?)`, now, now)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
			VALUES ('subj-1', 'proj-1', 'test-subject', 'Test Subject', ?, ?)`, now, now)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id,
			slide_index, file_path, sha256, phash, width, height, fetched_at, status, is_duplicate,
			created_at, updated_at)
			VALUES ('sample-1', 'subj-1', 'instagram', 'post-1', 0, 'test.jpg', 'abc', '0', 100, 100, ?, 'kept', 0, ?, ?)`,
			now, now, now)
		Expect(err).NotTo(HaveOccurred())

		return "subj-1"
	}

	It("returns 202 Accepted with job details on valid request", func() {
		subjectID := seedData()

		body, _ := json.Marshal(api.CreateExportRequest{
			SubjectIDs: []string{subjectID},
			Format:     "kohya-ss",
		})

		req := httptest.NewRequest(http.MethodPost, "/v1/exports", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeCreate(w, req)

		Expect(w.Code).To(Equal(http.StatusAccepted))

		var resp api.CreateExportResponse
		Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
		Expect(resp.JobID).NotTo(BeEmpty())
		Expect(resp.Status).To(Equal("running"))
		Expect(resp.TotalItems).To(Equal(1))
		Expect(resp.OutputPath).To(ContainSubstring("exports/test-project/test-subject"))
	})

	It("returns 400 when subject_ids is empty", func() {
		body, _ := json.Marshal(api.CreateExportRequest{
			Format: "kohya-ss",
		})

		req := httptest.NewRequest(http.MethodPost, "/v1/exports", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeCreate(w, req)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("returns 400 for invalid JSON", func() {
		req := httptest.NewRequest(http.MethodPost, "/v1/exports", bytes.NewReader([]byte("not json")))
		w := httptest.NewRecorder()

		handler.ServeCreate(w, req)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("returns 404 when subject not found", func() {
		body, _ := json.Marshal(api.CreateExportRequest{
			SubjectIDs: []string{"nonexistent"},
			Format:     "kohya-ss",
		})

		req := httptest.NewRequest(http.MethodPost, "/v1/exports", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeCreate(w, req)
		Expect(w.Code).To(Equal(http.StatusNotFound))
	})

	// Suppress unused import warning.
	_ = context.Background
})
