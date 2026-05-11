package store_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("SampleStore", func() {
	var (
		tmpDir      string
		sampleStore *store.SampleStore
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "sample-store-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err := store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())

		// Seed project and subject.
		_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
			VALUES ('p1', 'proj', 'Project', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
			VALUES ('s1', 'p1', 'subj', 'Subject', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
		Expect(err).NotTo(HaveOccurred())

		sampleStore = store.NewSampleStore(db)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("ListBySubject", func() {
		It("returns empty slice for subject with no samples", func() {
			samples, err := sampleStore.ListBySubject("s1")
			Expect(err).NotTo(HaveOccurred())
			Expect(samples).To(BeEmpty())
		})

		It("returns all samples for a subject ordered by created_at DESC", func() {
			db, err := store.OpenDB(filepath.Join(tmpDir, "test.sqlite"))
			Expect(err).NotTo(HaveOccurred())
			defer db.Close()

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at)
				VALUES ('sam1', 's1', 'instagram', 'post1', 0, '/img1.jpg', 'sha1', 'ph1', 800, 600,
				'2026-01-01T00:00:00Z', 'pending', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, duplicate_of, created_at, updated_at)
				VALUES ('sam2', 's1', 'instagram', 'post2', 0, '/img2.jpg', 'sha2', 'ph2', 1024, 768,
				'2026-01-02T00:00:00Z', 'kept', 1, '["sam1"]', '2026-01-02T00:00:00Z', '2026-01-02T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			samples, err := sampleStore.ListBySubject("s1")
			Expect(err).NotTo(HaveOccurred())
			Expect(samples).To(HaveLen(2))

			// Most recent first.
			Expect(samples[0].ID).To(Equal("sam2"))
			Expect(samples[0].Status).To(Equal("kept"))
			Expect(samples[0].IsDuplicate).To(BeTrue())
			Expect(samples[0].DuplicateOf).To(Equal(`["sam1"]`))
			Expect(samples[0].Width).To(Equal(1024))

			Expect(samples[1].ID).To(Equal("sam1"))
			Expect(samples[1].Status).To(Equal("pending"))
			Expect(samples[1].IsDuplicate).To(BeFalse())
			Expect(samples[1].DuplicateOf).To(BeEmpty())
		})

		It("does not return samples from other subjects", func() {
			db, err := store.OpenDB(filepath.Join(tmpDir, "test.sqlite"))
			Expect(err).NotTo(HaveOccurred())
			defer db.Close()

			// Create another subject.
			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('s2', 'p1', 'other', 'Other', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at)
				VALUES ('sam-other', 's2', 'instagram', 'post99', 0, '/other.jpg', 'sha99', 'ph99', 100, 100,
				'2026-01-01T00:00:00Z', 'pending', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			samples, err := sampleStore.ListBySubject("s1")
			Expect(err).NotTo(HaveOccurred())
			Expect(samples).To(BeEmpty())
		})
	})
})
