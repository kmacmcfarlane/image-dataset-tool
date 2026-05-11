package store_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("EditStore", func() {
	var (
		tmpDir    string
		editStore *store.EditStore
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "edit-store-test-*")
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

		_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
			file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at)
			VALUES ('sam1', 's1', 'instagram', 'post1', 0,
			'/img1.jpg', 'sha1', 'ph1', 800, 600, '2026-01-01T00:00:00Z', 'pending', 0,
			'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
		Expect(err).NotTo(HaveOccurred())

		editStore = store.NewEditStore(db)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("GetBySampleID", func() {
		It("returns nil when no edits exist", func() {
			edit, err := editStore.GetBySampleID("sam1")
			Expect(err).NotTo(HaveOccurred())
			Expect(edit).To(BeNil())
		})
	})

	Describe("Upsert", func() {
		It("creates an edit row for a sample", func() {
			edit := &model.Edit{
				SampleID:        "sam1",
				RotationDeg:     90.0,
				AutoActionsJSON: `["remove_borders"]`,
			}

			result, err := editStore.Upsert(edit)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.SampleID).To(Equal("sam1"))
			Expect(result.RotationDeg).To(Equal(90.0))
			Expect(result.AutoActionsJSON).To(Equal(`["remove_borders"]`))
			Expect(result.CropBox).To(BeNil())
			Expect(result.CreatedAt).NotTo(BeEmpty())
			Expect(result.UpdatedAt).NotTo(BeEmpty())
		})

		It("creates an edit with crop box", func() {
			edit := &model.Edit{
				SampleID: "sam1",
				CropBox: &model.CropBox{
					X: 10, Y: 20, W: 100, H: 200,
				},
			}

			result, err := editStore.Upsert(edit)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.CropBox).NotTo(BeNil())
			Expect(result.CropBox.X).To(Equal(10.0))
			Expect(result.CropBox.Y).To(Equal(20.0))
			Expect(result.CropBox.W).To(Equal(100.0))
			Expect(result.CropBox.H).To(Equal(200.0))
		})

		It("updates existing edit (idempotent)", func() {
			// Create initial.
			_, err := editStore.Upsert(&model.Edit{
				SampleID:    "sam1",
				RotationDeg: 90.0,
			})
			Expect(err).NotTo(HaveOccurred())

			// Update.
			result, err := editStore.Upsert(&model.Edit{
				SampleID:    "sam1",
				RotationDeg: 180.0,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RotationDeg).To(Equal(180.0))
		})

		It("returns error for non-existent sample (FK violation)", func() {
			edit := &model.Edit{
				SampleID:    "nonexistent",
				RotationDeg: 90.0,
			}
			_, err := editStore.Upsert(edit)
			Expect(err).To(MatchError(store.ErrNotFound))
		})
	})

	Describe("DeleteBySampleID", func() {
		It("deletes existing edit", func() {
			_, err := editStore.Upsert(&model.Edit{
				SampleID:    "sam1",
				RotationDeg: 90.0,
			})
			Expect(err).NotTo(HaveOccurred())

			err = editStore.DeleteBySampleID("sam1")
			Expect(err).NotTo(HaveOccurred())

			// Verify deleted.
			edit, err := editStore.GetBySampleID("sam1")
			Expect(err).NotTo(HaveOccurred())
			Expect(edit).To(BeNil())
		})

		It("succeeds even when no edit exists (no-op)", func() {
			err := editStore.DeleteBySampleID("sam1")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
