package service_test

import (
	"archive/zip"
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

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("ZipExportService", func() {
	var (
		db      *sql.DB
		tmpDir  string
		svc     *service.ZipExportService
		cleanup func()
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "zip-export-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err = store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())

		exportStore := store.NewExportStore(db)
		svc = service.NewZipExportService(exportStore, tmpDir)

		cleanup = func() {
			db.Close()
			os.RemoveAll(tmpDir)
		}
	})

	AfterEach(func() {
		cleanup()
	})

	createTestJPEG := func() []byte {
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		img.Set(0, 0, color.White)
		var buf bytes.Buffer
		Expect(jpeg.Encode(&buf, img, nil)).To(Succeed())
		return buf.Bytes()
	}

	seedSubjectWithSamples := func(count int) string {
		now := time.Now().UTC().Format(time.RFC3339)

		_, err := db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
			VALUES ('proj-1', 'my-project', 'My Project', ?, ?)`, now, now)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
			VALUES ('subj-1', 'proj-1', 'my-subject', 'My Subject', ?, ?)`, now, now)
		Expect(err).NotTo(HaveOccurred())

		sampleDir := filepath.Join(tmpDir, "media")
		Expect(os.MkdirAll(sampleDir, 0755)).To(Succeed())
		jpegData := createTestJPEG()

		for i := 0; i < count; i++ {
			sampleID := sampleIDFromIndex(i)
			filename := sampleID + ".jpg"
			Expect(os.WriteFile(filepath.Join(sampleDir, filename), jpegData, 0644)).To(Succeed())

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id,
				slide_index, file_path, sha256, phash, width, height, fetched_at, status, is_duplicate,
				created_at, updated_at)
				VALUES (?, 'subj-1', 'instagram', ?, 0, ?, ?, '0', 2, 2, ?, 'kept', 0, ?, ?)`,
				sampleID, "post-"+sampleID, "media/"+filename, "sha-"+sampleID, now, now, now)
			Expect(err).NotTo(HaveOccurred())
		}

		return "subj-1"
	}

	Describe("StreamZip", func() {
		It("produces a valid zip with image files", func() {
			subjectID := seedSubjectWithSamples(3)

			reader, err := svc.StreamZip(context.Background(), service.ZipExportParams{
				SubjectIDs: []string{subjectID},
				Format:     "kohya-ss",
			})
			Expect(err).NotTo(HaveOccurred())

			data, err := io.ReadAll(reader)
			Expect(err).NotTo(HaveOccurred())
			reader.Close()

			// Parse as zip.
			zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			Expect(err).NotTo(HaveOccurred())

			// Expect 3 files (no captions since no study provided).
			Expect(zr.File).To(HaveLen(3))

			// All files should be under my-project/my-subject/ prefix.
			for _, f := range zr.File {
				Expect(f.Name).To(HavePrefix("my-project/my-subject/"))
				Expect(f.Name).To(HaveSuffix(".jpg"))
			}
		})

		It("includes captions when study slug is provided", func() {
			subjectID := seedSubjectWithSamples(1)

			// Create a caption study and caption.
			now := time.Now().UTC().Format(time.RFC3339)
			_, err := db.Exec(`INSERT INTO caption_studies (id, project_id, name, slug,
				provider, model, prompt_template, params_json, created_at, updated_at)
				VALUES ('study-1', 'proj-1', 'Test Study', 'test-study',
				'test', 'test-model', 'template', '{}', ?, ?)`, now, now)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO captions (id, sample_id, study_id, text, created_at)
				VALUES ('cap-1', ?, 'study-1', 'a test caption', ?)`,
				sampleIDFromIndex(0), now)
			Expect(err).NotTo(HaveOccurred())

			studySlug := "test-study"
			reader, err := svc.StreamZip(context.Background(), service.ZipExportParams{
				SubjectIDs: []string{subjectID},
				StudySlug:  studySlug,
				Format:     "kohya-ss",
			})
			Expect(err).NotTo(HaveOccurred())

			data, err := io.ReadAll(reader)
			Expect(err).NotTo(HaveOccurred())
			reader.Close()

			zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			Expect(err).NotTo(HaveOccurred())

			// 1 image + 1 caption = 2 files.
			Expect(zr.File).To(HaveLen(2))

			var hasJpg, hasTxt bool
			for _, f := range zr.File {
				if filepath.Ext(f.Name) == ".jpg" {
					hasJpg = true
				}
				if filepath.Ext(f.Name) == ".txt" {
					hasTxt = true
					// Read caption content.
					rc, err := f.Open()
					Expect(err).NotTo(HaveOccurred())
					content, err := io.ReadAll(rc)
					Expect(err).NotTo(HaveOccurred())
					rc.Close()
					Expect(string(content)).To(Equal("a test caption"))
				}
			}
			Expect(hasJpg).To(BeTrue())
			Expect(hasTxt).To(BeTrue())
		})

		It("returns error for empty subject_ids", func() {
			_, err := svc.StreamZip(context.Background(), service.ZipExportParams{
				SubjectIDs: []string{},
				Format:     "kohya-ss",
			})
			Expect(err).To(HaveOccurred())
		})

		It("returns error for nonexistent subject", func() {
			_, err := svc.StreamZip(context.Background(), service.ZipExportParams{
				SubjectIDs: []string{"nonexistent"},
				Format:     "kohya-ss",
			})
			Expect(err).To(HaveOccurred())
		})

		It("returns error when no kept samples exist", func() {
			now := time.Now().UTC().Format(time.RFC3339)
			_, err := db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
				VALUES ('proj-empty', 'empty', 'Empty', ?, ?)`, now, now)
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('subj-empty', 'proj-empty', 'empty', 'Empty', ?, ?)`, now, now)
			Expect(err).NotTo(HaveOccurred())

			_, err = svc.StreamZip(context.Background(), service.ZipExportParams{
				SubjectIDs: []string{"subj-empty"},
				Format:     "kohya-ss",
			})
			Expect(err).To(HaveOccurred())
		})

		It("returns error for unsupported format", func() {
			_, err := svc.StreamZip(context.Background(), service.ZipExportParams{
				SubjectIDs: []string{"subj-1"},
				Format:     "unsupported",
			})
			Expect(err).To(HaveOccurred())
		})
	})
})

// sampleIDFromIndex generates a deterministic sample ID from an index.
func sampleIDFromIndex(i int) string {
	return "sample-" + string(rune('a'+i))
}
