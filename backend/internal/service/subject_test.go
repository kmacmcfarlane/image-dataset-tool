package service_test

import (
	"database/sql"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("SubjectService", func() {
	var (
		tmpDir     string
		db         *sql.DB
		svc        *service.SubjectService
		projectSvc *service.ProjectService
		accountSvc *service.AccountService
		cleanup    func()
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "subject-svc-test-*")
		Expect(err).NotTo(HaveOccurred())

		err = os.MkdirAll(filepath.Join(tmpDir, "projects"), 0755)
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		var err2 error
		db, err2 = store.OpenDB(dbPath)
		Expect(err2).NotTo(HaveOccurred())

		err2 = store.Migrate(db)
		Expect(err2).NotTo(HaveOccurred())

		ps := store.NewProjectStore(db)
		ss := store.NewSubjectStore(db)
		as := store.NewAccountStore(db, make([]byte, 32))
		projectSvc = service.NewProjectService(ps, tmpDir)
		svc = service.NewSubjectService(ss, ps, tmpDir)
		accountSvc = service.NewAccountService(as)

		cleanup = func() {
			db.Close()
			os.RemoveAll(tmpDir)
		}
	})

	AfterEach(func() {
		cleanup()
	})

	Describe("Create", func() {
		It("creates a subject with slug, manifest, samples dir, and DB row", func() {
			project, err := projectSvc.Create("Test Project", "")
			Expect(err).NotTo(HaveOccurred())

			sub, err := svc.Create(project.ID, "My Subject")
			Expect(err).NotTo(HaveOccurred())
			Expect(sub.ID).NotTo(BeEmpty())
			Expect(sub.Slug).To(Equal("my-subject"))
			Expect(sub.Name).To(Equal("My Subject"))
			Expect(sub.ProjectID).To(Equal(project.ID))
			Expect(sub.CreatedAt).NotTo(BeEmpty())

			// Verify directory structure.
			manifestPath := filepath.Join(tmpDir, "projects", "test-project", "subjects", "my-subject", "manifest.json")
			_, err = os.Stat(manifestPath)
			Expect(err).NotTo(HaveOccurred())

			samplesDir := filepath.Join(tmpDir, "projects", "test-project", "subjects", "my-subject", "samples")
			info, err := os.Stat(samplesDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})

		It("returns error for non-existent project", func() {
			_, err := svc.Create("nonexistent", "Subject")
			Expect(err).To(HaveOccurred())
		})

		It("returns conflict error on duplicate slug within project", func() {
			project, err := projectSvc.Create("Dup Project", "")
			Expect(err).NotTo(HaveOccurred())

			_, err = svc.Create(project.ID, "Same Name")
			Expect(err).NotTo(HaveOccurred())

			_, err = svc.Create(project.ID, "Same Name")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("already exists")))
		})

		It("returns error for name that produces empty slug", func() {
			project, err := projectSvc.Create("Slug Project", "")
			Expect(err).NotTo(HaveOccurred())

			_, err = svc.Create(project.ID, "!!!")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("cannot generate slug")))
		})
	})

	Describe("Get", func() {
		It("retrieves a created subject", func() {
			project, err := projectSvc.Create("Get Project", "")
			Expect(err).NotTo(HaveOccurred())

			created, err := svc.Create(project.ID, "Get Subject")
			Expect(err).NotTo(HaveOccurred())

			got, err := svc.Get(project.ID, created.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Name).To(Equal("Get Subject"))
			Expect(got.SampleStats.Total).To(Equal(0))
		})

		It("returns error for non-existent subject", func() {
			project, err := projectSvc.Create("Get Project 2", "")
			Expect(err).NotTo(HaveOccurred())

			_, err = svc.Get(project.ID, "nonexistent-id")
			Expect(err).To(HaveOccurred())
		})

		It("returns error when subject belongs to different project", func() {
			p1, err := projectSvc.Create("Project One", "")
			Expect(err).NotTo(HaveOccurred())
			p2, err := projectSvc.Create("Project Two", "")
			Expect(err).NotTo(HaveOccurred())

			sub, err := svc.Create(p1.ID, "Cross Subject")
			Expect(err).NotTo(HaveOccurred())

			// Try to get it with wrong project ID.
			_, err = svc.Get(p2.ID, sub.ID)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("List", func() {
		It("returns all subjects in a project", func() {
			project, err := projectSvc.Create("List Project", "")
			Expect(err).NotTo(HaveOccurred())

			_, err = svc.Create(project.ID, "Subject A")
			Expect(err).NotTo(HaveOccurred())
			_, err = svc.Create(project.ID, "Subject B")
			Expect(err).NotTo(HaveOccurred())

			list, err := svc.List(project.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(HaveLen(2))
		})

		It("returns error for non-existent project", func() {
			_, err := svc.List("nonexistent")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Update", func() {
		It("updates subject name and renames directory", func() {
			project, err := projectSvc.Create("Update Project", "")
			Expect(err).NotTo(HaveOccurred())

			created, err := svc.Create(project.ID, "Original Name")
			Expect(err).NotTo(HaveOccurred())

			newName := "Updated Name"
			updated, err := svc.Update(project.ID, created.ID, &newName)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Name).To(Equal("Updated Name"))
			Expect(updated.Slug).To(Equal("updated-name"))

			// Old directory should not exist.
			oldDir := filepath.Join(tmpDir, "projects", "update-project", "subjects", "original-name")
			_, err = os.Stat(oldDir)
			Expect(os.IsNotExist(err)).To(BeTrue())

			// New directory should exist.
			newDir := filepath.Join(tmpDir, "projects", "update-project", "subjects", "updated-name")
			_, err = os.Stat(newDir)
			Expect(err).NotTo(HaveOccurred())
		})

		It("does nothing when name is nil", func() {
			project, err := projectSvc.Create("NoOp Project", "")
			Expect(err).NotTo(HaveOccurred())

			created, err := svc.Create(project.ID, "No Change")
			Expect(err).NotTo(HaveOccurred())

			got, err := svc.Update(project.ID, created.ID, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Name).To(Equal("No Change"))
		})
	})

	Describe("Delete", func() {
		It("soft deletes (DB only, keeps directory)", func() {
			project, err := projectSvc.Create("Soft Del Project", "")
			Expect(err).NotTo(HaveOccurred())

			created, err := svc.Create(project.ID, "Soft Delete")
			Expect(err).NotTo(HaveOccurred())

			err = svc.Delete(project.ID, created.ID, false)
			Expect(err).NotTo(HaveOccurred())

			// DB row gone.
			_, err = svc.Get(project.ID, created.ID)
			Expect(err).To(HaveOccurred())

			// Directory still exists.
			subjectDir := filepath.Join(tmpDir, "projects", "soft-del-project", "subjects", "soft-delete")
			_, err = os.Stat(subjectDir)
			Expect(err).NotTo(HaveOccurred())
		})

		It("hard deletes (DB + directory)", func() {
			project, err := projectSvc.Create("Hard Del Project", "")
			Expect(err).NotTo(HaveOccurred())

			created, err := svc.Create(project.ID, "Hard Delete")
			Expect(err).NotTo(HaveOccurred())

			err = svc.Delete(project.ID, created.ID, true)
			Expect(err).NotTo(HaveOccurred())

			// DB row gone.
			_, err = svc.Get(project.ID, created.ID)
			Expect(err).To(HaveOccurred())

			// Directory gone.
			subjectDir := filepath.Join(tmpDir, "projects", "hard-del-project", "subjects", "hard-delete")
			_, err = os.Stat(subjectDir)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		It("returns error for non-existent subject", func() {
			project, err := projectSvc.Create("Del NF Project", "")
			Expect(err).NotTo(HaveOccurred())

			err = svc.Delete(project.ID, "nonexistent-id", false)
			Expect(err).To(HaveOccurred())
		})

		It("returns error when subject belongs to different project", func() {
			p1, err := projectSvc.Create("Del P1", "")
			Expect(err).NotTo(HaveOccurred())
			p2, err := projectSvc.Create("Del P2", "")
			Expect(err).NotTo(HaveOccurred())

			sub, err := svc.Create(p1.ID, "Cross Delete")
			Expect(err).NotTo(HaveOccurred())

			err = svc.Delete(p2.ID, sub.ID, false)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("LinkAccount / UnlinkAccount / ListLinkedAccounts", func() {
		It("links and lists accounts", func() {
			project, err := projectSvc.Create("Link Project", "")
			Expect(err).NotTo(HaveOccurred())
			sub, err := svc.Create(project.ID, "Link Subject")
			Expect(err).NotTo(HaveOccurred())
			account, err := accountSvc.Create("instagram", "linkuser")
			Expect(err).NotTo(HaveOccurred())

			err = svc.LinkAccount(project.ID, sub.ID, account.ID)
			Expect(err).NotTo(HaveOccurred())

			linked, err := svc.ListLinkedAccounts(project.ID, sub.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(linked).To(HaveLen(1))
			Expect(linked[0].Handle).To(Equal("linkuser"))
		})

		It("returns conflict error when linking same account twice", func() {
			project, err := projectSvc.Create("Dup Link Project", "")
			Expect(err).NotTo(HaveOccurred())
			sub, err := svc.Create(project.ID, "Dup Link Subject")
			Expect(err).NotTo(HaveOccurred())
			account, err := accountSvc.Create("instagram", "duplinkuser")
			Expect(err).NotTo(HaveOccurred())

			Expect(svc.LinkAccount(project.ID, sub.ID, account.ID)).To(Succeed())
			err = svc.LinkAccount(project.ID, sub.ID, account.ID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("already linked")))
		})

		It("allows linking same account to multiple subjects", func() {
			project, err := projectSvc.Create("Multi Link Project", "")
			Expect(err).NotTo(HaveOccurred())
			s1, err := svc.Create(project.ID, "Multi Link Sub1")
			Expect(err).NotTo(HaveOccurred())
			s2, err := svc.Create(project.ID, "Multi Link Sub2")
			Expect(err).NotTo(HaveOccurred())
			account, err := accountSvc.Create("instagram", "multiuser")
			Expect(err).NotTo(HaveOccurred())

			Expect(svc.LinkAccount(project.ID, s1.ID, account.ID)).To(Succeed())
			Expect(svc.LinkAccount(project.ID, s2.ID, account.ID)).To(Succeed())

			l1, err := svc.ListLinkedAccounts(project.ID, s1.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(l1).To(HaveLen(1))

			l2, err := svc.ListLinkedAccounts(project.ID, s2.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(l2).To(HaveLen(1))
		})

		It("unlinks account without deleting the account itself", func() {
			project, err := projectSvc.Create("Unlink Project", "")
			Expect(err).NotTo(HaveOccurred())
			sub, err := svc.Create(project.ID, "Unlink Subject")
			Expect(err).NotTo(HaveOccurred())
			account, err := accountSvc.Create("instagram", "unlinkuser")
			Expect(err).NotTo(HaveOccurred())

			Expect(svc.LinkAccount(project.ID, sub.ID, account.ID)).To(Succeed())
			Expect(svc.UnlinkAccount(project.ID, sub.ID, account.ID)).To(Succeed())

			// Account still exists.
			got, err := accountSvc.Get(account.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.ID).To(Equal(account.ID))

			// Link is gone.
			linked, err := svc.ListLinkedAccounts(project.ID, sub.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(linked).To(BeEmpty())
		})

		It("returns not found when linking non-existent account", func() {
			project, err := projectSvc.Create("NF Link Project", "")
			Expect(err).NotTo(HaveOccurred())
			sub, err := svc.Create(project.ID, "NF Link Subject")
			Expect(err).NotTo(HaveOccurred())

			err = svc.LinkAccount(project.ID, sub.ID, "nonexistent-account-id")
			Expect(err).To(HaveOccurred())
		})

		It("returns not found when unlinking a non-existent link", func() {
			project, err := projectSvc.Create("NF Unlink Project", "")
			Expect(err).NotTo(HaveOccurred())
			sub, err := svc.Create(project.ID, "NF Unlink Subject")
			Expect(err).NotTo(HaveOccurred())
			account, err := accountSvc.Create("instagram", "nfunlinkuser")
			Expect(err).NotTo(HaveOccurred())

			err = svc.UnlinkAccount(project.ID, sub.ID, account.ID)
			Expect(err).To(HaveOccurred())
		})
	})
})
