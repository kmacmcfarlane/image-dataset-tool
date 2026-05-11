package store_test

import (
	"database/sql"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("ProjectStore", func() {
	var (
		tmpDir string
		db     *sql.DB
		ps     *store.ProjectStore
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "project-store-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err = store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())

		err = store.Migrate(db)
		Expect(err).NotTo(HaveOccurred())

		ps = store.NewProjectStore(db)
	})

	AfterEach(func() {
		db.Close()
		os.RemoveAll(tmpDir)
	})

	Describe("Create", func() {
		It("inserts a project and sets timestamps", func() {
			p := &model.Project{
				ID:          "test-id-1",
				Slug:        "my-project",
				Name:        "My Project",
				Description: "A test project",
			}
			err := ps.Create(p)
			Expect(err).NotTo(HaveOccurred())
			Expect(p.CreatedAt).NotTo(BeEmpty())
			Expect(p.UpdatedAt).NotTo(BeEmpty())
		})

		It("returns ErrDuplicateSlug on duplicate slug", func() {
			p1 := &model.Project{ID: "id-1", Slug: "dup-slug", Name: "P1"}
			err := ps.Create(p1)
			Expect(err).NotTo(HaveOccurred())

			p2 := &model.Project{ID: "id-2", Slug: "dup-slug", Name: "P2"}
			err = ps.Create(p2)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("already exists")))
		})
	})

	Describe("Get", func() {
		It("retrieves a project by ID with subject count 0", func() {
			p := &model.Project{ID: "get-id", Slug: "get-proj", Name: "Get Test"}
			err := ps.Create(p)
			Expect(err).NotTo(HaveOccurred())

			got, err := ps.Get("get-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.ID).To(Equal("get-id"))
			Expect(got.Slug).To(Equal("get-proj"))
			Expect(got.Name).To(Equal("Get Test"))
			Expect(got.SubjectCount).To(Equal(0))
		})

		It("counts subjects correctly", func() {
			p := &model.Project{ID: "sc-id", Slug: "sc-proj", Name: "Subject Count Test"}
			err := ps.Create(p)
			Expect(err).NotTo(HaveOccurred())

			// Insert subjects directly.
			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('s1', 'sc-id', 'subj-1', 'Subject 1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('s2', 'sc-id', 'subj-2', 'Subject 2', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			got, err := ps.Get("sc-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.SubjectCount).To(Equal(2))
		})

		It("returns ErrNotFound for non-existent ID", func() {
			_, err := ps.Get("nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("List", func() {
		It("returns empty list when no projects exist", func() {
			list, err := ps.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(BeEmpty())
		})

		It("returns projects ordered by created_at DESC", func() {
			p1 := &model.Project{ID: "list-1", Slug: "first", Name: "First"}
			err := ps.Create(p1)
			Expect(err).NotTo(HaveOccurred())

			p2 := &model.Project{ID: "list-2", Slug: "second", Name: "Second"}
			err = ps.Create(p2)
			Expect(err).NotTo(HaveOccurred())

			list, err := ps.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(HaveLen(2))
		})
	})

	Describe("Update", func() {
		It("updates project name", func() {
			p := &model.Project{ID: "upd-id", Slug: "upd-proj", Name: "Original"}
			err := ps.Create(p)
			Expect(err).NotTo(HaveOccurred())

			newName := "Updated Name"
			got, err := ps.Update("upd-id", &newName, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Name).To(Equal("Updated Name"))
		})

		It("updates project description", func() {
			p := &model.Project{ID: "upd-desc-id", Slug: "upd-desc", Name: "Test"}
			err := ps.Create(p)
			Expect(err).NotTo(HaveOccurred())

			newDesc := "New description"
			got, err := ps.Update("upd-desc-id", nil, &newDesc)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Description).To(Equal("New description"))
		})

		It("returns ErrNotFound for non-existent ID", func() {
			name := "Test"
			_, err := ps.Update("nope", &name, nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("Delete", func() {
		It("removes a project", func() {
			p := &model.Project{ID: "del-id", Slug: "del-proj", Name: "Delete Me"}
			err := ps.Create(p)
			Expect(err).NotTo(HaveOccurred())

			err = ps.Delete("del-id")
			Expect(err).NotTo(HaveOccurred())

			_, err = ps.Get("del-id")
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("cascades to subjects", func() {
			p := &model.Project{ID: "cas-id", Slug: "cas-proj", Name: "Cascade"}
			err := ps.Create(p)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('s1', 'cas-id', 'subj', 'Subject', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			err = ps.Delete("cas-id")
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM subjects WHERE project_id = 'cas-id'").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("returns ErrNotFound for non-existent ID", func() {
			err := ps.Delete("nope")
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("SlugExists", func() {
		It("returns false for non-existent slug", func() {
			exists, err := ps.SlugExists("nope")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("returns true for existing slug", func() {
			p := &model.Project{ID: "se-id", Slug: "existing", Name: "Test"}
			err := ps.Create(p)
			Expect(err).NotTo(HaveOccurred())

			exists, err := ps.SlugExists("existing")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})
	})
})
