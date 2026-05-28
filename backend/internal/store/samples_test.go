package store_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
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

	Describe("GetByID", func() {
		BeforeEach(func() {
			db, err := store.OpenDB(filepath.Join(tmpDir, "test.sqlite"))
			Expect(err).NotTo(HaveOccurred())
			defer db.Close()

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at)
				VALUES ('sam1', 's1', 'instagram', 'post1', 0, '/img1.jpg', 'sha1', 'ph1', 800, 600,
				'2026-01-01T00:00:00Z', 'pending', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the sample for a valid ID", func() {
			sample, err := sampleStore.GetByID("sam1")
			Expect(err).NotTo(HaveOccurred())
			Expect(sample.ID).To(Equal("sam1"))
			Expect(sample.Status).To(Equal("pending"))
		})

		It("returns ErrNotFound for non-existent ID", func() {
			_, err := sampleStore.GetByID("nonexistent")
			Expect(err).To(MatchError(store.ErrNotFound))
		})
	})

	Describe("UpdateStatus", func() {
		BeforeEach(func() {
			db, err := store.OpenDB(filepath.Join(tmpDir, "test.sqlite"))
			Expect(err).NotTo(HaveOccurred())
			defer db.Close()

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at)
				VALUES ('sam1', 's1', 'instagram', 'post1', 0, '/img1.jpg', 'sha1', 'ph1', 800, 600,
				'2026-01-01T00:00:00Z', 'pending', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("valid status transitions",
			func(newStatus string) {
				updated, err := sampleStore.UpdateStatus("sam1", newStatus)
				Expect(err).NotTo(HaveOccurred())
				Expect(updated.Status).To(Equal(newStatus))
				Expect(updated.UpdatedAt).NotTo(Equal("2026-01-01T00:00:00Z"))
			},
			Entry("pending to kept", "kept"),
			Entry("pending to rejected", "rejected"),
			Entry("pending to pending (noop)", "pending"),
		)

		It("returns ErrNotFound for non-existent sample", func() {
			_, err := sampleStore.UpdateStatus("nonexistent", "kept")
			Expect(err).To(MatchError(store.ErrNotFound))
		})

		It("rejects invalid status", func() {
			_, err := sampleStore.UpdateStatus("sam1", "invalid")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid status"))
		})

		It("is reversible between states", func() {
			// pending -> kept
			updated, err := sampleStore.UpdateStatus("sam1", "kept")
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Status).To(Equal("kept"))

			// kept -> rejected
			updated, err = sampleStore.UpdateStatus("sam1", "rejected")
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Status).To(Equal("rejected"))

			// rejected -> pending
			updated, err = sampleStore.UpdateStatus("sam1", "pending")
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Status).To(Equal("pending"))
		})
	})

	Describe("BulkUpdateStatus", func() {
		BeforeEach(func() {
			db, err := store.OpenDB(filepath.Join(tmpDir, "test.sqlite"))
			Expect(err).NotTo(HaveOccurred())
			defer db.Close()

			for _, id := range []string{"sam1", "sam2", "sam3"} {
				_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
					file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at)
					VALUES (?, 's1', 'instagram', ?, 0, '/img.jpg', 'sha', 'ph', 800, 600,
					'2026-01-01T00:00:00Z', 'pending', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
					id, "post-"+id)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("updates all specified samples atomically", func() {
			affected, err := sampleStore.BulkUpdateStatus([]string{"sam1", "sam2", "sam3"}, "kept")
			Expect(err).NotTo(HaveOccurred())
			Expect(affected).To(Equal(int64(3)))

			// Verify each.
			for _, id := range []string{"sam1", "sam2", "sam3"} {
				s, err := sampleStore.GetByID(id)
				Expect(err).NotTo(HaveOccurred())
				Expect(s.Status).To(Equal("kept"))
			}
		})

		It("skips non-existent IDs without error", func() {
			affected, err := sampleStore.BulkUpdateStatus([]string{"sam1", "nonexistent"}, "rejected")
			Expect(err).NotTo(HaveOccurred())
			Expect(affected).To(Equal(int64(1)))
		})

		It("returns 0 for empty IDs slice", func() {
			affected, err := sampleStore.BulkUpdateStatus([]string{}, "kept")
			Expect(err).NotTo(HaveOccurred())
			Expect(affected).To(Equal(int64(0)))
		})

		It("rejects invalid status", func() {
			_, err := sampleStore.BulkUpdateStatus([]string{"sam1"}, "invalid")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid status"))
		})
	})

	Describe("InsertOrIgnore", func() {
		It("inserts a new sample and returns true", func() {
			sample := &model.Sample{
				ID: "sam-new", SubjectID: "s1", SourcePlatform: "instagram",
				SourcePostID: "post-new", SlideIndex: 0, FilePath: "/new.jpg",
				SHA256: "abc123", PHash: "def456", Width: 800, Height: 600,
				FetchedAt: "2026-01-01T00:00:00Z", Status: "pending",
			}
			inserted, err := sampleStore.InsertOrIgnore(sample)
			Expect(err).NotTo(HaveOccurred())
			Expect(inserted).To(BeTrue())

			got, err := sampleStore.GetByID("sam-new")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.SHA256).To(Equal("abc123"))
		})

		It("returns false on duplicate (subject_id, source_post_id, slide_index)", func() {
			sample := &model.Sample{
				ID: "sam-1", SubjectID: "s1", SourcePlatform: "instagram",
				SourcePostID: "post-dup", SlideIndex: 0, FilePath: "/first.jpg",
				SHA256: "first", PHash: "ph1", Width: 800, Height: 600,
				FetchedAt: "2026-01-01T00:00:00Z", Status: "pending",
			}
			inserted, err := sampleStore.InsertOrIgnore(sample)
			Expect(err).NotTo(HaveOccurred())
			Expect(inserted).To(BeTrue())

			sample2 := &model.Sample{
				ID: "sam-2", SubjectID: "s1", SourcePlatform: "instagram",
				SourcePostID: "post-dup", SlideIndex: 0, FilePath: "/second.jpg",
				SHA256: "second", PHash: "ph2", Width: 800, Height: 600,
				FetchedAt: "2026-01-01T00:00:00Z", Status: "pending",
			}
			inserted, err = sampleStore.InsertOrIgnore(sample2)
			Expect(err).NotTo(HaveOccurred())
			Expect(inserted).To(BeFalse())
		})

		It("handles duplicate_of JSON array", func() {
			sample := &model.Sample{
				ID: "sam-d", SubjectID: "s1", SourcePlatform: "instagram",
				SourcePostID: "post-d", SlideIndex: 0, FilePath: "/d.jpg",
				SHA256: "sha-d", PHash: "ph-d", Width: 800, Height: 600,
				FetchedAt: "2026-01-01T00:00:00Z", Status: "pending",
				IsDuplicate: true, DuplicateOf: `["sam-orig"]`,
			}
			inserted, err := sampleStore.InsertOrIgnore(sample)
			Expect(err).NotTo(HaveOccurred())
			Expect(inserted).To(BeTrue())

			got, err := sampleStore.GetByID("sam-d")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.IsDuplicate).To(BeTrue())
			Expect(got.DuplicateOf).To(Equal(`["sam-orig"]`))
		})
	})

	Describe("FindDuplicatesBySHA256", func() {
		BeforeEach(func() {
			db, err := store.OpenDB(filepath.Join(tmpDir, "test.sqlite"))
			Expect(err).NotTo(HaveOccurred())
			defer db.Close()

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at)
				VALUES ('sam1', 's1', 'instagram', 'post1', 0, '/img1.jpg', 'aabbcc', 'ph1', 800, 600,
				'2026-01-01T00:00:00Z', 'pending', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds samples with matching sha256", func() {
			ids, err := sampleStore.FindDuplicatesBySHA256("s1", "aabbcc")
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(ConsistOf("sam1"))
		})

		It("returns empty for non-matching sha256", func() {
			ids, err := sampleStore.FindDuplicatesBySHA256("s1", "zzzzz")
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(BeEmpty())
		})

		It("does not match across subjects", func() {
			ids, err := sampleStore.FindDuplicatesBySHA256("other-subject", "aabbcc")
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(BeEmpty())
		})
	})

	Describe("FindDuplicatesByPHash", func() {
		BeforeEach(func() {
			db, err := store.OpenDB(filepath.Join(tmpDir, "test.sqlite"))
			Expect(err).NotTo(HaveOccurred())
			defer db.Close()

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at)
				VALUES ('sam1', 's1', 'instagram', 'post1', 0, '/img1.jpg', 'sha1', '0000000000000000', 800, 600,
				'2026-01-01T00:00:00Z', 'pending', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds samples within Hamming distance threshold", func() {
			ids, err := sampleStore.FindDuplicatesByPHash("s1", "0000000000000001", 6)
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(ConsistOf("sam1"))
		})

		It("excludes samples beyond threshold", func() {
			ids, err := sampleStore.FindDuplicatesByPHash("s1", "ffffffffffffffff", 6)
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(BeEmpty())
		})
	})
})
