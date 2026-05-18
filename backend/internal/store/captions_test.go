package store_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("CaptionStore", func() {
	var (
		db         *sql.DB
		capStore   *store.CaptionStore
	)

	BeforeEach(func() {
		var err error
		db, err = sql.Open("sqlite", ":memory:?_pragma=foreign_keys(ON)")
		Expect(err).NotTo(HaveOccurred())

		err = store.Migrate(db)
		Expect(err).NotTo(HaveOccurred())

		capStore = store.NewCaptionStore(db)

		// Seed project, subject, sample, and caption study.
		_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
			VALUES ('proj1', 'test', 'Test Project', '2026-01-01', '2026-01-01')`)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
			VALUES ('sub1', 'proj1', 'test-sub', 'Test Subject', '2026-01-01', '2026-01-01')`)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, file_path, sha256, phash, width, height, fetched_at, created_at, updated_at)
			VALUES ('samp1', 'sub1', 'instagram', 'post1', '/path/to/file', 'abc123', 'def456', 100, 100, '2026-01-01', '2026-01-01', '2026-01-01')`)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`INSERT INTO caption_studies (id, project_id, name, slug, provider, model, prompt_template, params_json, created_at, updated_at)
			VALUES ('study1', 'proj1', 'GPT Study', 'gpt-study', 'openai', 'gpt-4', 'Describe this image.', '{}', '2026-01-01', '2026-01-01')`)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if db != nil {
			db.Close()
		}
	})

	Describe("UpsertCaption", func() {
		It("creates a new caption", func() {
			cap := &model.Caption{
				SampleID: "samp1",
				StudyID:  "study1",
				Text:     "A test caption",
			}

			result, err := capStore.UpsertCaption(cap)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Text).To(Equal("A test caption"))
			Expect(result.SampleID).To(Equal("samp1"))
			Expect(result.StudyID).To(Equal("study1"))
			Expect(result.ID).NotTo(BeEmpty())
		})

		It("updates existing caption on same sample+study", func() {
			cap := &model.Caption{
				SampleID: "samp1",
				StudyID:  "study1",
				Text:     "First version",
			}
			_, err := capStore.UpsertCaption(cap)
			Expect(err).NotTo(HaveOccurred())

			cap2 := &model.Caption{
				SampleID: "samp1",
				StudyID:  "study1",
				Text:     "Updated version",
			}
			result, err := capStore.UpsertCaption(cap2)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Text).To(Equal("Updated version"))
		})

		It("returns ErrNotFound for non-existent sample", func() {
			cap := &model.Caption{
				SampleID: "nonexistent",
				StudyID:  "study1",
				Text:     "Will fail",
			}
			_, err := capStore.UpsertCaption(cap)
			Expect(err).To(MatchError(store.ErrNotFound))
		})
	})

	Describe("ListBySampleID", func() {
		It("returns empty list for sample with no captions", func() {
			results, err := capStore.ListBySampleID("samp1")
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(BeEmpty())
		})

		It("returns captions with study metadata", func() {
			cap := &model.Caption{
				SampleID: "samp1",
				StudyID:  "study1",
				Text:     "A caption",
			}
			_, err := capStore.UpsertCaption(cap)
			Expect(err).NotTo(HaveOccurred())

			results, err := capStore.ListBySampleID("samp1")
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].Caption.Text).To(Equal("A caption"))
			Expect(results[0].Study.Name).To(Equal("GPT Study"))
			Expect(results[0].Study.Provider).To(Equal("openai"))
		})
	})

	Describe("GetOrCreateManualStudy", func() {
		It("creates a manual study if none exists", func() {
			study, err := capStore.GetOrCreateManualStudy("proj1")
			Expect(err).NotTo(HaveOccurred())
			Expect(study).NotTo(BeNil())
			Expect(study.Slug).To(Equal("manual"))
			Expect(study.Provider).To(Equal("manual"))
			Expect(study.ProjectID).To(Equal("proj1"))
		})

		It("returns existing manual study on second call", func() {
			study1, err := capStore.GetOrCreateManualStudy("proj1")
			Expect(err).NotTo(HaveOccurred())

			study2, err := capStore.GetOrCreateManualStudy("proj1")
			Expect(err).NotTo(HaveOccurred())

			Expect(study1.ID).To(Equal(study2.ID))
		})
	})

	Describe("GetBySampleAndStudy", func() {
		It("returns nil for non-existent caption", func() {
			result, err := capStore.GetBySampleAndStudy("samp1", "study1")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("returns caption when it exists", func() {
			cap := &model.Caption{
				SampleID: "samp1",
				StudyID:  "study1",
				Text:     "Found it",
			}
			_, err := capStore.UpsertCaption(cap)
			Expect(err).NotTo(HaveOccurred())

			result, err := capStore.GetBySampleAndStudy("samp1", "study1")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Text).To(Equal("Found it"))
		})
	})
})
