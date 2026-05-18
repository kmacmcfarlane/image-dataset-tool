package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
	genexports "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/exports"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("ExportsService", func() {
	var (
		db      *sql.DB
		tmpDir  string
		natsSrv *natsutil.Server
		svc     *api.ExportsService
		cleanup func()
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "exports-svc-test-*")
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
		exportStore := store.NewExportStore(db)
		zipExportSvc := service.NewZipExportService(exportStore, tmpDir)
		svc = api.NewExportsService(exportSvc, zipExportSvc)

		cleanup = func() {
			natsSrv.Shutdown()
			db.Close()
			os.RemoveAll(tmpDir)
		}
	})

	AfterEach(func() {
		cleanup()
	})

	// createTestJPEG generates a valid 1x1 JPEG image.
	createTestJPEG := func() []byte {
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, color.White)
		var buf bytes.Buffer
		Expect(jpeg.Encode(&buf, img, nil)).To(Succeed())
		return buf.Bytes()
	}

	seedData := func() string {
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
			VALUES ('proj-1', 'test-project', 'Test Project', ?, ?)`, now, now)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
			VALUES ('subj-1', 'proj-1', 'test-subject', 'Test Subject', ?, ?)`, now, now)
		Expect(err).NotTo(HaveOccurred())

		// Create a valid JPEG file for the sample.
		sampleDir := filepath.Join(tmpDir, "media")
		Expect(os.MkdirAll(sampleDir, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(sampleDir, "test.jpg"), createTestJPEG(), 0644)).To(Succeed())

		_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id,
			slide_index, file_path, sha256, phash, width, height, fetched_at, status, is_duplicate,
			created_at, updated_at)
			VALUES ('sample-1', 'subj-1', 'instagram', 'post-1', 0, 'media/test.jpg', 'abc', '0', 1, 1, ?, 'kept', 0, ?, ?)`,
			now, now, now)
		Expect(err).NotTo(HaveOccurred())

		return "subj-1"
	}

	Describe("Create", func() {
		It("returns result with job details on valid request", func() {
			subjectID := seedData()

			result, err := svc.Create(context.Background(), &genexports.CreateExportPayload{
				SubjectIds: []string{subjectID},
				Format:     "kohya-ss",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.JobID).NotTo(BeEmpty())
			Expect(result.Status).To(Equal("running"))
			Expect(result.TotalItems).To(Equal(1))
			Expect(result.OutputPath).To(ContainSubstring("exports/test-project/test-subject"))
		})

		It("returns error when subject_ids is empty", func() {
			_, err := svc.Create(context.Background(), &genexports.CreateExportPayload{
				SubjectIds: []string{},
				Format:     "kohya-ss",
			})

			Expect(err).To(HaveOccurred())
		})

		It("returns error when subject not found", func() {
			_, err := svc.Create(context.Background(), &genexports.CreateExportPayload{
				SubjectIds: []string{"nonexistent"},
				Format:     "kohya-ss",
			})

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("DownloadZip", func() {
		It("returns a zip stream for valid subjects with kept samples", func() {
			subjectID := seedData()

			result, body, err := svc.DownloadZip(context.Background(), &genexports.DownloadZipPayload{
				SubjectIds: []string{subjectID},
				Format:     "kohya-ss",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.ContentType).To(Equal("application/zip"))
			Expect(result.ContentDisposition).To(ContainSubstring("export.zip"))

			// Read the body to completion.
			data, err := io.ReadAll(body)
			Expect(err).NotTo(HaveOccurred())
			Expect(body.Close()).To(Succeed())

			// Verify it starts with the PK zip magic bytes.
			Expect(len(data)).To(BeNumerically(">", 4))
			Expect(data[0:2]).To(Equal([]byte("PK")))
		})

		It("returns error when subject_ids is empty", func() {
			_, _, err := svc.DownloadZip(context.Background(), &genexports.DownloadZipPayload{
				SubjectIds: []string{},
				Format:     "kohya-ss",
			})

			Expect(err).To(HaveOccurred())
		})

		It("returns error for nonexistent subject", func() {
			_, _, err := svc.DownloadZip(context.Background(), &genexports.DownloadZipPayload{
				SubjectIds: []string{"nonexistent"},
				Format:     "kohya-ss",
			})

			Expect(err).To(HaveOccurred())
		})

		It("returns error when no kept samples exist", func() {
			now := time.Now().UTC().Format(time.RFC3339)
			_, err := db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
				VALUES ('proj-2', 'empty-project', 'Empty Project', ?, ?)`, now, now)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('subj-2', 'proj-2', 'empty-subject', 'Empty Subject', ?, ?)`, now, now)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = svc.DownloadZip(context.Background(), &genexports.DownloadZipPayload{
				SubjectIds: []string{"subj-2"},
				Format:     "kohya-ss",
			})

			Expect(err).To(HaveOccurred())
		})
	})
})
