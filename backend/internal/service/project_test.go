package service_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

func TestService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Service Suite")
}

var _ = Describe("ProjectService", func() {
	var (
		tmpDir  string
		svc     *service.ProjectService
		ps      *store.ProjectStore
		cleanup func()
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "project-svc-test-*")
		Expect(err).NotTo(HaveOccurred())

		// Create projects subdir.
		err = os.MkdirAll(filepath.Join(tmpDir, "projects"), 0755)
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err := store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())

		err = store.Migrate(db)
		Expect(err).NotTo(HaveOccurred())

		ps = store.NewProjectStore(db)
		svc = service.NewProjectService(ps, tmpDir)

		cleanup = func() {
			db.Close()
			os.RemoveAll(tmpDir)
		}
	})

	AfterEach(func() {
		cleanup()
	})

	Describe("Create", func() {
		It("creates a project with slug, manifest, and DB row", func() {
			p, err := svc.Create("My Test Project", "A description")
			Expect(err).NotTo(HaveOccurred())
			Expect(p.ID).NotTo(BeEmpty())
			Expect(p.Slug).To(Equal("my-test-project"))
			Expect(p.Name).To(Equal("My Test Project"))
			Expect(p.Description).To(Equal("A description"))
			Expect(p.CreatedAt).NotTo(BeEmpty())

			// Verify directory and manifest exist.
			manifestPath := filepath.Join(tmpDir, "projects", "my-test-project", "manifest.json")
			_, err = os.Stat(manifestPath)
			Expect(err).NotTo(HaveOccurred())

			// Verify subjects directory exists.
			subjectsDir := filepath.Join(tmpDir, "projects", "my-test-project", "subjects")
			info, err := os.Stat(subjectsDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})

		It("returns conflict error on duplicate slug", func() {
			_, err := svc.Create("Unique Project", "")
			Expect(err).NotTo(HaveOccurred())

			_, err = svc.Create("Unique Project", "")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("already exists")))
		})

		It("returns error for name that produces empty slug", func() {
			_, err := svc.Create("!!!", "")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("cannot generate slug")))
		})
	})

	Describe("Get", func() {
		It("retrieves a created project", func() {
			created, err := svc.Create("Get Test", "")
			Expect(err).NotTo(HaveOccurred())

			got, err := svc.Get(created.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Name).To(Equal("Get Test"))
			Expect(got.SubjectCount).To(Equal(0))
		})

		It("returns error for non-existent project", func() {
			_, err := svc.Get("nonexistent-id")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("List", func() {
		It("returns all projects", func() {
			_, err := svc.Create("Project A", "")
			Expect(err).NotTo(HaveOccurred())
			_, err = svc.Create("Project B", "")
			Expect(err).NotTo(HaveOccurred())

			list, err := svc.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(HaveLen(2))
		})
	})

	Describe("Update", func() {
		It("updates project name and manifest", func() {
			created, err := svc.Create("Original Name", "Original desc")
			Expect(err).NotTo(HaveOccurred())

			newName := "Updated Name"
			updated, err := svc.Update(created.ID, &newName, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Name).To(Equal("Updated Name"))
			Expect(updated.Slug).To(Equal("original-name")) // slug is immutable
		})

		It("updates description", func() {
			created, err := svc.Create("Desc Test", "")
			Expect(err).NotTo(HaveOccurred())

			newDesc := "New description"
			updated, err := svc.Update(created.ID, nil, &newDesc)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Description).To(Equal("New description"))
		})
	})

	Describe("Delete", func() {
		It("soft deletes (DB only, keeps directory)", func() {
			created, err := svc.Create("Delete Me", "")
			Expect(err).NotTo(HaveOccurred())

			err = svc.Delete(created.ID, false)
			Expect(err).NotTo(HaveOccurred())

			// DB row gone.
			_, err = svc.Get(created.ID)
			Expect(err).To(HaveOccurred())

			// Directory still exists.
			projectDir := filepath.Join(tmpDir, "projects", "delete-me")
			_, err = os.Stat(projectDir)
			Expect(err).NotTo(HaveOccurred())
		})

		It("hard deletes (DB + directory)", func() {
			created, err := svc.Create("Hard Delete", "")
			Expect(err).NotTo(HaveOccurred())

			err = svc.Delete(created.ID, true)
			Expect(err).NotTo(HaveOccurred())

			// DB row gone.
			_, err = svc.Get(created.ID)
			Expect(err).To(HaveOccurred())

			// Directory gone.
			projectDir := filepath.Join(tmpDir, "projects", "hard-delete")
			_, err = os.Stat(projectDir)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		It("returns error for non-existent project", func() {
			err := svc.Delete("nonexistent-id", false)
			Expect(err).To(HaveOccurred())
		})
	})
})
