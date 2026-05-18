package kohya_test

import (
	"context"
	"database/sql"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/export/kohya"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

func TestKohya(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kohya Writer Suite")
}

// createTestJPEG writes a solid-color JPEG of the given dimensions to path.
func createTestJPEG(path string, w, h int) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 120, G: 80, B: 200, A: 255})
		}
	}
	f, err := os.Create(path)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()
	Expect(jpeg.Encode(f, img, &jpeg.Options{Quality: 90})).To(Succeed())
}

// setupDB opens an in-memory SQLite DB, applies migrations, and returns it.
func setupDB() *sql.DB {
	tmpDir := GinkgoT().TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")
	db, err := store.OpenDB(dbPath)
	Expect(err).NotTo(HaveOccurred())
	Expect(store.Migrate(db)).To(Succeed())
	return db
}

// seedFixtures inserts the minimum fixture rows needed for export tests.
// Returns (projectID, subjectID, studyID).
func seedFixtures(db *sql.DB) (string, string, string) {
	ts := "2026-01-01T00:00:00Z"

	_, err := db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
		VALUES ('proj1', 'my-project', 'My Project', ?, ?)`, ts, ts)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
		VALUES ('subj1', 'proj1', 'my-subject', 'My Subject', ?, ?)`, ts, ts)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO caption_studies (id, project_id, name, slug, provider, model, prompt_template, created_at, updated_at)
		VALUES ('study1', 'proj1', 'My Study', 'my-study', 'anthropic', 'claude', 'Describe this image', ?, ?)`, ts, ts)
	Expect(err).NotTo(HaveOccurred())

	return "proj1", "subj1", "study1"
}

// insertSample inserts a sample row and creates the image file on disk.
func insertSample(db *sql.DB, dataDir, id, status string, w, h int) {
	ts := "2026-01-01T00:00:00Z"
	relPath := id + ".jpg"
	createTestJPEG(filepath.Join(dataDir, relPath), w, h)

	_, err := db.Exec(`INSERT INTO samples
		(id, subject_id, source_platform, source_post_id, slide_index,
		 file_path, sha256, phash, width, height, fetched_at, status, created_at, updated_at)
		VALUES (?, 'subj1', 'instagram', ?, 0, ?, 'sha', 'ph', ?, ?, ?, ?, ?, ?)`,
		id, id, relPath, w, h, ts, status, ts, ts)
	Expect(err).NotTo(HaveOccurred())
}

// insertCaption inserts a caption for a sample+study pair.
func insertCaption(db *sql.DB, sampleID, studyID, text string) {
	ts := "2026-01-01T00:00:00Z"
	_, err := db.Exec(`INSERT INTO captions (id, sample_id, study_id, text, created_at)
		VALUES (?, ?, ?, ?, ?)`, sampleID+"-cap", sampleID, studyID, text, ts)
	Expect(err).NotTo(HaveOccurred())
}

// insertEdit inserts an edit row for a sample.
func insertEdit(db *sql.DB, sampleID string, rotDeg float64, cropJSON *string) {
	ts := "2026-01-01T00:00:00Z"
	_, err := db.Exec(`INSERT INTO edits (sample_id, rotation_deg, crop_box_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`, sampleID, rotDeg, cropJSON, ts, ts)
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("KohyaWriter", func() {
	var (
		ctx       context.Context
		db        *sql.DB
		dataDir   string
		outputDir string
		writer    *kohya.Writer
	)

	BeforeEach(func() {
		ctx = context.Background()
		db = setupDB()
		dataDir = GinkgoT().TempDir()
		outputDir = GinkgoT().TempDir()
		writer = kohya.NewWriter(db, dataDir)
		seedFixtures(db)
	})

	AfterEach(func() {
		db.Close()
	})

	Describe("Write", func() {
		Context("basic export: kept samples with captions", func() {
			It("writes .jpg and .txt for each kept sample with caption", func() {
				// AC: Output: <sample-id>.jpg + <sample-id>.txt pairs
				// AC: Caption .txt from selected study
				insertSample(db, dataDir, "s1", "kept", 100, 100)
				insertSample(db, dataDir, "s2", "kept", 200, 150)
				insertCaption(db, "s1", "study1", "a cat sitting on a mat")
				insertCaption(db, "s2", "study1", "a dog running in the park")

				opts := model.ExportOptions{
					ProjectSlug: "my-project",
					SubjectSlug: "my-subject",
					StudySlug:   "my-study",
					OutputDir:   outputDir,
				}
				result, err := writer.Write(ctx, opts)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.SampleCount).To(Equal(2))
				Expect(result.CaptionCount).To(Equal(2))
				Expect(result.SkippedCount).To(Equal(0))
				Expect(result.Errors).To(BeEmpty())

				// Verify file pairs exist.
				for _, id := range []string{"s1", "s2"} {
					Expect(filepath.Join(outputDir, id+".jpg")).To(BeAnExistingFile())
					Expect(filepath.Join(outputDir, id+".txt")).To(BeAnExistingFile())
				}

				// Verify caption content.
				s1txt, err := os.ReadFile(filepath.Join(outputDir, "s1.txt"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(s1txt)).To(Equal("a cat sitting on a mat"))
			})
		})

		Context("missing captions: .txt omitted when study has no caption for sample", func() {
			It("omits .txt for samples without captions in selected study", func() {
				// AC: Caption .txt from selected study; omit if missing
				insertSample(db, dataDir, "s1", "kept", 100, 100)
				insertSample(db, dataDir, "s2", "kept", 100, 100)
				// Only s1 has a caption; s2 does not.
				insertCaption(db, "s1", "study1", "only s1 has caption")

				opts := model.ExportOptions{
					ProjectSlug: "my-project",
					SubjectSlug: "my-subject",
					StudySlug:   "my-study",
					OutputDir:   outputDir,
				}
				result, err := writer.Write(ctx, opts)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.SampleCount).To(Equal(2))
				Expect(result.CaptionCount).To(Equal(1))
				Expect(result.SkippedCount).To(Equal(0))

				// s1: both files present.
				Expect(filepath.Join(outputDir, "s1.jpg")).To(BeAnExistingFile())
				Expect(filepath.Join(outputDir, "s1.txt")).To(BeAnExistingFile())

				// s2: only image, no caption.
				Expect(filepath.Join(outputDir, "s2.jpg")).To(BeAnExistingFile())
				Expect(filepath.Join(outputDir, "s2.txt")).NotTo(BeAnExistingFile())
			})
		})

		Context("no study slug: no .txt files written", func() {
			It("exports only images when StudySlug is empty", func() {
				insertSample(db, dataDir, "s1", "kept", 100, 100)
				insertCaption(db, "s1", "study1", "a caption")

				opts := model.ExportOptions{
					ProjectSlug: "my-project",
					SubjectSlug: "my-subject",
					StudySlug:   "", // no study
					OutputDir:   outputDir,
				}
				result, err := writer.Write(ctx, opts)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.SampleCount).To(Equal(1))
				Expect(result.CaptionCount).To(Equal(0))

				Expect(filepath.Join(outputDir, "s1.jpg")).To(BeAnExistingFile())
				Expect(filepath.Join(outputDir, "s1.txt")).NotTo(BeAnExistingFile())
			})
		})

		Context("non-kept samples are excluded", func() {
			It("exports only kept samples; pending and rejected are skipped", func() {
				// AC: Exports only status='kept' samples
				insertSample(db, dataDir, "kept1", "kept", 100, 100)
				insertSample(db, dataDir, "pend1", "pending", 100, 100)
				insertSample(db, dataDir, "rej1", "rejected", 100, 100)

				opts := model.ExportOptions{
					ProjectSlug: "my-project",
					SubjectSlug: "my-subject",
					StudySlug:   "my-study",
					OutputDir:   outputDir,
				}
				result, err := writer.Write(ctx, opts)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.SampleCount).To(Equal(1))

				Expect(filepath.Join(outputDir, "kept1.jpg")).To(BeAnExistingFile())
				Expect(filepath.Join(outputDir, "pend1.jpg")).NotTo(BeAnExistingFile())
				Expect(filepath.Join(outputDir, "rej1.jpg")).NotTo(BeAnExistingFile())
			})
		})

		Context("edit-materialized images", func() {
			It("applies 90-degree rotation: output dimensions are swapped", func() {
				// AC: Images are edit-materialized (rotation/crop applied)
				// AC: Test with edit-materialized images: verify dimensions match expected
				insertSample(db, dataDir, "s1", "kept", 200, 100) // 200 wide × 100 tall
				insertEdit(db, "s1", 90, nil)                     // rotate 90 → 100 wide × 200 tall

				opts := model.ExportOptions{
					ProjectSlug: "my-project",
					SubjectSlug: "my-subject",
					StudySlug:   "",
					OutputDir:   outputDir,
				}
				result, err := writer.Write(ctx, opts)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.SampleCount).To(Equal(1))
				Expect(result.Errors).To(BeEmpty())

				// Decode the written JPEG and verify dimensions.
				f, err := os.Open(filepath.Join(outputDir, "s1.jpg"))
				Expect(err).NotTo(HaveOccurred())
				defer f.Close()
				img, _, err := image.Decode(f)
				Expect(err).NotTo(HaveOccurred())
				bounds := img.Bounds()
				Expect(bounds.Dx()).To(Equal(100))
				Expect(bounds.Dy()).To(Equal(200))
			})

			It("applies crop: output dimensions match crop box", func() {
				// AC: Images are edit-materialized (rotation/crop applied)
				insertSample(db, dataDir, "s1", "kept", 200, 150) // 200×150 source
				cropJSON := `{"X":10,"Y":20,"W":80,"H":60}`
				insertEdit(db, "s1", 0, &cropJSON) // crop to 80×60

				opts := model.ExportOptions{
					ProjectSlug: "my-project",
					SubjectSlug: "my-subject",
					StudySlug:   "",
					OutputDir:   outputDir,
				}
				result, err := writer.Write(ctx, opts)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.SampleCount).To(Equal(1))

				f, err := os.Open(filepath.Join(outputDir, "s1.jpg"))
				Expect(err).NotTo(HaveOccurred())
				defer f.Close()
				img, _, err := image.Decode(f)
				Expect(err).NotTo(HaveOccurred())
				bounds := img.Bounds()
				Expect(bounds.Dx()).To(Equal(80))
				Expect(bounds.Dy()).To(Equal(60))
			})
		})

		Context("empty subject: no kept samples", func() {
			It("returns zero counts and creates the output dir", func() {
				insertSample(db, dataDir, "s1", "pending", 100, 100)

				opts := model.ExportOptions{
					ProjectSlug: "my-project",
					SubjectSlug: "my-subject",
					StudySlug:   "my-study",
					OutputDir:   outputDir,
				}
				result, err := writer.Write(ctx, opts)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.SampleCount).To(Equal(0))
				Expect(result.CaptionCount).To(Equal(0))
				_, statErr := os.Stat(outputDir)
				Expect(statErr).NotTo(HaveOccurred())
			})
		})

		Context("error: subject not found", func() {
			It("returns error when subject slug does not exist", func() {
				opts := model.ExportOptions{
					ProjectSlug: "my-project",
					SubjectSlug: "nonexistent",
					StudySlug:   "",
					OutputDir:   outputDir,
				}
				_, err := writer.Write(ctx, opts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("nonexistent"))
			})
		})

		Context("error: study not found", func() {
			It("returns error when study slug does not exist in project", func() {
				insertSample(db, dataDir, "s1", "kept", 100, 100)

				opts := model.ExportOptions{
					ProjectSlug: "my-project",
					SubjectSlug: "my-subject",
					StudySlug:   "nonexistent-study",
					OutputDir:   outputDir,
				}
				_, err := writer.Write(ctx, opts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("nonexistent-study"))
			})
		})
	})
})
