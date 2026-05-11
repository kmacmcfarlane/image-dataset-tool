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

var _ = Describe("SubjectStore", func() {
	var (
		tmpDir string
		db     *sql.DB
		ss     *store.SubjectStore
	)

	// Helper: create a project in DB so FK constraints are satisfied.
	createProject := func(id, slug string) {
		_, err := db.Exec(
			`INSERT INTO projects (id, slug, name, description, created_at, updated_at)
			 VALUES (?, ?, 'Test', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
			id, slug)
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "subject-store-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err = store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())

		err = store.Migrate(db)
		Expect(err).NotTo(HaveOccurred())

		ss = store.NewSubjectStore(db)
	})

	AfterEach(func() {
		db.Close()
		os.RemoveAll(tmpDir)
	})

	Describe("Create", func() {
		It("inserts a subject and sets timestamps", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{
				ID:        "s1",
				ProjectID: "p1",
				Slug:      "my-subject",
				Name:      "My Subject",
			}
			err := ss.Create(sub)
			Expect(err).NotTo(HaveOccurred())
			Expect(sub.CreatedAt).NotTo(BeEmpty())
			Expect(sub.UpdatedAt).NotTo(BeEmpty())
		})

		It("returns ErrDuplicateSlug on duplicate slug within project", func() {
			createProject("p1", "proj-1")
			s1 := &model.Subject{ID: "s1", ProjectID: "p1", Slug: "dup", Name: "S1"}
			err := ss.Create(s1)
			Expect(err).NotTo(HaveOccurred())

			s2 := &model.Subject{ID: "s2", ProjectID: "p1", Slug: "dup", Name: "S2"}
			err = ss.Create(s2)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("already exists")))
		})

		It("allows same slug in different projects", func() {
			createProject("p1", "proj-1")
			createProject("p2", "proj-2")

			s1 := &model.Subject{ID: "s1", ProjectID: "p1", Slug: "same-slug", Name: "S1"}
			err := ss.Create(s1)
			Expect(err).NotTo(HaveOccurred())

			s2 := &model.Subject{ID: "s2", ProjectID: "p2", Slug: "same-slug", Name: "S2"}
			err = ss.Create(s2)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Get", func() {
		It("retrieves a subject with empty sample stats and linked accounts", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "g1", ProjectID: "p1", Slug: "get-sub", Name: "Get Test"}
			err := ss.Create(sub)
			Expect(err).NotTo(HaveOccurred())

			got, err := ss.Get("g1")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.ID).To(Equal("g1"))
			Expect(got.ProjectID).To(Equal("p1"))
			Expect(got.Slug).To(Equal("get-sub"))
			Expect(got.Name).To(Equal("Get Test"))
			Expect(got.SampleStats.Total).To(Equal(0))
			Expect(got.LinkedAccounts).To(BeEmpty())
		})

		It("returns ErrNotFound for non-existent ID", func() {
			_, err := ss.Get("nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("returns sample stats by status", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "stats1", ProjectID: "p1", Slug: "stats-sub", Name: "Stats"}
			err := ss.Create(sub)
			Expect(err).NotTo(HaveOccurred())

			// Insert samples with different statuses.
			for i, status := range []string{"pending", "pending", "kept", "rejected"} {
				_, err := db.Exec(`INSERT INTO samples
					(id, subject_id, source_platform, source_post_id, slide_index, file_path, sha256, phash, width, height, fetched_at, status, created_at, updated_at)
					VALUES (?, 'stats1', 'ig', ?, 0, '/tmp/f', 'abc', 'def', 100, 100, '2026-01-01', ?, '2026-01-01', '2026-01-01')`,
					"sample-"+string(rune('a'+i)), "post-"+string(rune('a'+i)), status)
				Expect(err).NotTo(HaveOccurred())
			}

			got, err := ss.Get("stats1")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.SampleStats.Pending).To(Equal(2))
			Expect(got.SampleStats.Kept).To(Equal(1))
			Expect(got.SampleStats.Rejected).To(Equal(1))
			Expect(got.SampleStats.Total).To(Equal(4))
		})

		It("returns linked accounts", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "la1", ProjectID: "p1", Slug: "linked-sub", Name: "Linked"}
			err := ss.Create(sub)
			Expect(err).NotTo(HaveOccurred())

			// Insert account and link.
			_, err = db.Exec(`INSERT INTO accounts (id, platform, handle, created_at, updated_at)
				VALUES ('a1', 'instagram', 'testuser', '2026-01-01', '2026-01-01')`)
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Exec(`INSERT INTO subject_accounts (subject_id, account_id, created_at)
				VALUES ('la1', 'a1', '2026-01-01')`)
			Expect(err).NotTo(HaveOccurred())

			got, err := ss.Get("la1")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.LinkedAccounts).To(HaveLen(1))
			Expect(got.LinkedAccounts[0].Handle).To(Equal("testuser"))
			Expect(got.LinkedAccounts[0].Platform).To(Equal("instagram"))
		})
	})

	Describe("List", func() {
		It("returns empty list when no subjects exist", func() {
			createProject("p1", "proj-1")
			list, err := ss.List("p1")
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(BeEmpty())
		})

		It("returns subjects with sample counts", func() {
			createProject("p1", "proj-1")
			s1 := &model.Subject{ID: "ls1", ProjectID: "p1", Slug: "sub-a", Name: "A"}
			err := ss.Create(s1)
			Expect(err).NotTo(HaveOccurred())

			s2 := &model.Subject{ID: "ls2", ProjectID: "p1", Slug: "sub-b", Name: "B"}
			err = ss.Create(s2)
			Expect(err).NotTo(HaveOccurred())

			// Add a sample to first subject.
			_, err = db.Exec(`INSERT INTO samples
				(id, subject_id, source_platform, source_post_id, slide_index, file_path, sha256, phash, width, height, fetched_at, status, created_at, updated_at)
				VALUES ('smp1', 'ls1', 'ig', 'post1', 0, '/tmp/f', 'abc', 'def', 100, 100, '2026-01-01', 'pending', '2026-01-01', '2026-01-01')`)
			Expect(err).NotTo(HaveOccurred())

			list, err := ss.List("p1")
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(HaveLen(2))
			// Find subject with sample.
			for _, sub := range list {
				if sub.ID == "ls1" {
					Expect(sub.SampleStats.Total).To(Equal(1))
				} else {
					Expect(sub.SampleStats.Total).To(Equal(0))
				}
			}
		})

		It("only returns subjects for the specified project", func() {
			createProject("p1", "proj-1")
			createProject("p2", "proj-2")
			s1 := &model.Subject{ID: "iso1", ProjectID: "p1", Slug: "sub-1", Name: "S1"}
			err := ss.Create(s1)
			Expect(err).NotTo(HaveOccurred())

			s2 := &model.Subject{ID: "iso2", ProjectID: "p2", Slug: "sub-2", Name: "S2"}
			err = ss.Create(s2)
			Expect(err).NotTo(HaveOccurred())

			list, err := ss.List("p1")
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(HaveLen(1))
			Expect(list[0].ID).To(Equal("iso1"))
		})
	})

	Describe("Update", func() {
		It("updates name and slug", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "u1", ProjectID: "p1", Slug: "old-slug", Name: "Old"}
			err := ss.Create(sub)
			Expect(err).NotTo(HaveOccurred())

			got, err := ss.Update("u1", "New Name", "new-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Name).To(Equal("New Name"))
			Expect(got.Slug).To(Equal("new-name"))
		})

		It("returns ErrNotFound for non-existent ID", func() {
			_, err := ss.Update("nope", "Name", "slug")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("returns ErrDuplicateSlug on conflict", func() {
			createProject("p1", "proj-1")
			s1 := &model.Subject{ID: "uc1", ProjectID: "p1", Slug: "slug-a", Name: "A"}
			err := ss.Create(s1)
			Expect(err).NotTo(HaveOccurred())

			s2 := &model.Subject{ID: "uc2", ProjectID: "p1", Slug: "slug-b", Name: "B"}
			err = ss.Create(s2)
			Expect(err).NotTo(HaveOccurred())

			_, err = ss.Update("uc2", "A", "slug-a")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("already exists")))
		})
	})

	Describe("Delete", func() {
		It("removes a subject", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "d1", ProjectID: "p1", Slug: "del-sub", Name: "Delete"}
			err := ss.Create(sub)
			Expect(err).NotTo(HaveOccurred())

			err = ss.Delete("d1")
			Expect(err).NotTo(HaveOccurred())

			_, err = ss.Get("d1")
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("cascades to samples", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "cd1", ProjectID: "p1", Slug: "cas-sub", Name: "Cascade"}
			err := ss.Create(sub)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO samples
				(id, subject_id, source_platform, source_post_id, slide_index, file_path, sha256, phash, width, height, fetched_at, status, created_at, updated_at)
				VALUES ('csmp1', 'cd1', 'ig', 'post1', 0, '/tmp/f', 'abc', 'def', 100, 100, '2026-01-01', 'pending', '2026-01-01', '2026-01-01')`)
			Expect(err).NotTo(HaveOccurred())

			err = ss.Delete("cd1")
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM samples WHERE subject_id = 'cd1'").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("cascades to edits and captions through samples", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "ce1", ProjectID: "p1", Slug: "cas-edit", Name: "CascadeEdit"}
			err := ss.Create(sub)
			Expect(err).NotTo(HaveOccurred())

			// Insert sample.
			_, err = db.Exec(`INSERT INTO samples
				(id, subject_id, source_platform, source_post_id, slide_index, file_path, sha256, phash, width, height, fetched_at, status, created_at, updated_at)
				VALUES ('esmp1', 'ce1', 'ig', 'post1', 0, '/tmp/f', 'abc', 'def', 100, 100, '2026-01-01', 'pending', '2026-01-01', '2026-01-01')`)
			Expect(err).NotTo(HaveOccurred())

			// Insert edit.
			_, err = db.Exec(`INSERT INTO edits (sample_id, created_at, updated_at) VALUES ('esmp1', '2026-01-01', '2026-01-01')`)
			Expect(err).NotTo(HaveOccurred())

			// Insert caption study and caption.
			_, err = db.Exec(`INSERT INTO caption_studies (id, project_id, name, slug, provider, model, prompt_template, created_at, updated_at)
				VALUES ('cs1', 'p1', 'Study', 'study', 'claude', 'claude-3', 'prompt', '2026-01-01', '2026-01-01')`)
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Exec(`INSERT INTO captions (id, sample_id, study_id, text, created_at)
				VALUES ('cap1', 'esmp1', 'cs1', 'A caption', '2026-01-01')`)
			Expect(err).NotTo(HaveOccurred())

			err = ss.Delete("ce1")
			Expect(err).NotTo(HaveOccurred())

			// Verify cascade.
			var editCount, captionCount int
			err = db.QueryRow("SELECT COUNT(*) FROM edits WHERE sample_id = 'esmp1'").Scan(&editCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(editCount).To(Equal(0))

			err = db.QueryRow("SELECT COUNT(*) FROM captions WHERE sample_id = 'esmp1'").Scan(&captionCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(captionCount).To(Equal(0))
		})

		It("returns ErrNotFound for non-existent ID", func() {
			err := ss.Delete("nope")
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("LinkAccount / UnlinkAccount / ListLinkedAccounts", func() {
		createAccount := func(id, handle string) {
			_, err := db.Exec(`INSERT INTO accounts (id, platform, handle, created_at, updated_at)
				VALUES (?, 'instagram', ?, '2026-01-01', '2026-01-01')`, id, handle)
			Expect(err).NotTo(HaveOccurred())
		}

		It("links an account and lists it", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "link-s1", ProjectID: "p1", Slug: "lnk-sub", Name: "LinkSub"}
			Expect(ss.Create(sub)).To(Succeed())
			createAccount("link-a1", "user1")

			Expect(ss.LinkAccount("link-s1", "link-a1")).To(Succeed())

			accounts, err := ss.ListLinkedAccounts("link-s1")
			Expect(err).NotTo(HaveOccurred())
			Expect(accounts).To(HaveLen(1))
			Expect(accounts[0].ID).To(Equal("link-a1"))
			Expect(accounts[0].Handle).To(Equal("user1"))
		})

		It("returns ErrDuplicateLink when linking same account twice", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "link-s2", ProjectID: "p1", Slug: "lnk-sub2", Name: "LinkSub2"}
			Expect(ss.Create(sub)).To(Succeed())
			createAccount("link-a2", "user2")

			Expect(ss.LinkAccount("link-s2", "link-a2")).To(Succeed())

			err := ss.LinkAccount("link-s2", "link-a2")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("already linked")))
		})

		It("returns ErrNotFound when linking non-existent account", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "link-s3", ProjectID: "p1", Slug: "lnk-sub3", Name: "LinkSub3"}
			Expect(ss.Create(sub)).To(Succeed())

			err := ss.LinkAccount("link-s3", "nonexistent-account")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("allows linking same account to multiple subjects", func() {
			createProject("p1", "proj-1")
			s1 := &model.Subject{ID: "multi-s1", ProjectID: "p1", Slug: "multi-sub1", Name: "Multi1"}
			s2 := &model.Subject{ID: "multi-s2", ProjectID: "p1", Slug: "multi-sub2", Name: "Multi2"}
			Expect(ss.Create(s1)).To(Succeed())
			Expect(ss.Create(s2)).To(Succeed())
			createAccount("multi-a1", "shareduser")

			Expect(ss.LinkAccount("multi-s1", "multi-a1")).To(Succeed())
			Expect(ss.LinkAccount("multi-s2", "multi-a1")).To(Succeed())

			a1, err := ss.ListLinkedAccounts("multi-s1")
			Expect(err).NotTo(HaveOccurred())
			Expect(a1).To(HaveLen(1))

			a2, err := ss.ListLinkedAccounts("multi-s2")
			Expect(err).NotTo(HaveOccurred())
			Expect(a2).To(HaveLen(1))
		})

		It("unlinks an account without deleting it", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "unlink-s1", ProjectID: "p1", Slug: "unlnk-sub", Name: "UnlinkSub"}
			Expect(ss.Create(sub)).To(Succeed())
			createAccount("unlink-a1", "unlink-user")

			Expect(ss.LinkAccount("unlink-s1", "unlink-a1")).To(Succeed())
			Expect(ss.UnlinkAccount("unlink-s1", "unlink-a1")).To(Succeed())

			// Account still exists.
			var count int
			err := db.QueryRow("SELECT COUNT(*) FROM accounts WHERE id = 'unlink-a1'").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// But the link is gone.
			accounts, err := ss.ListLinkedAccounts("unlink-s1")
			Expect(err).NotTo(HaveOccurred())
			Expect(accounts).To(BeEmpty())
		})

		It("returns ErrNotFound when unlinking a non-existent link", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "unlink-s2", ProjectID: "p1", Slug: "unlnk-sub2", Name: "UnlinkSub2"}
			Expect(ss.Create(sub)).To(Succeed())
			createAccount("unlink-a2", "unlink-user2")

			err := ss.UnlinkAccount("unlink-s2", "unlink-a2")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("SlugExistsInProject", func() {
		It("returns false for non-existent slug", func() {
			createProject("p1", "proj-1")
			exists, err := ss.SlugExistsInProject("p1", "nope")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("returns true for existing slug in same project", func() {
			createProject("p1", "proj-1")
			sub := &model.Subject{ID: "se1", ProjectID: "p1", Slug: "existing", Name: "Test"}
			err := ss.Create(sub)
			Expect(err).NotTo(HaveOccurred())

			exists, err := ss.SlugExistsInProject("p1", "existing")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("returns false for same slug in different project", func() {
			createProject("p1", "proj-1")
			createProject("p2", "proj-2")
			sub := &model.Subject{ID: "se2", ProjectID: "p1", Slug: "cross", Name: "Test"}
			err := ss.Create(sub)
			Expect(err).NotTo(HaveOccurred())

			exists, err := ss.SlugExistsInProject("p2", "cross")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})
})
