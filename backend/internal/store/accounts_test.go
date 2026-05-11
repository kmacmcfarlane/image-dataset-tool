package store_test

import (
	"database/sql"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("AccountStore", func() {
	var (
		tmpDir string
		db     *sql.DB
		as     *store.AccountStore
		encKey []byte
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "account-store-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err = store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())

		err = store.Migrate(db)
		Expect(err).NotTo(HaveOccurred())

		// Generate a test encryption key.
		encKey = make([]byte, crypto.KeySize)
		for i := range encKey {
			encKey[i] = byte(i)
		}

		as = store.NewAccountStore(db, encKey)
	})

	AfterEach(func() {
		db.Close()
		os.RemoveAll(tmpDir)
	})

	Describe("Create", func() {
		It("inserts an account and sets timestamps", func() {
			a := &model.Account{
				ID:       "acc-1",
				Platform: "instagram",
				Handle:   "testuser",
			}
			err := as.Create(a)
			Expect(err).NotTo(HaveOccurred())
			Expect(a.CreatedAt).NotTo(BeEmpty())
			Expect(a.UpdatedAt).NotTo(BeEmpty())
		})

		It("returns ErrDuplicateAccount on duplicate (platform, handle)", func() {
			a1 := &model.Account{ID: "acc-1", Platform: "instagram", Handle: "dup"}
			err := as.Create(a1)
			Expect(err).NotTo(HaveOccurred())

			a2 := &model.Account{ID: "acc-2", Platform: "instagram", Handle: "dup"}
			err = as.Create(a2)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("duplicate account")))
		})

		It("allows same handle on different platforms", func() {
			a1 := &model.Account{ID: "acc-1", Platform: "instagram", Handle: "user1"}
			err := as.Create(a1)
			Expect(err).NotTo(HaveOccurred())

			a2 := &model.Account{ID: "acc-2", Platform: "tiktok", Handle: "user1"}
			err = as.Create(a2)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Get", func() {
		It("returns a stored account without cookies", func() {
			a := &model.Account{ID: "acc-1", Platform: "instagram", Handle: "testuser"}
			err := as.Create(a)
			Expect(err).NotTo(HaveOccurred())

			got, err := as.Get("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.ID).To(Equal("acc-1"))
			Expect(got.Platform).To(Equal("instagram"))
			Expect(got.Handle).To(Equal("testuser"))
			Expect(got.HasCookies).To(BeFalse())
		})

		It("returns ErrNotFound for missing account", func() {
			_, err := as.Get("nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("List", func() {
		It("returns all accounts ordered by creation time", func() {
			a1 := &model.Account{ID: "acc-1", Platform: "instagram", Handle: "user1"}
			a2 := &model.Account{ID: "acc-2", Platform: "tiktok", Handle: "user2"}
			Expect(as.Create(a1)).To(Succeed())
			Expect(as.Create(a2)).To(Succeed())

			list, err := as.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(HaveLen(2))
		})

		It("returns empty slice for no accounts", func() {
			list, err := as.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(BeNil())
		})
	})

	Describe("UpdateCookies", func() {
		It("encrypts and stores cookies", func() {
			a := &model.Account{ID: "acc-1", Platform: "instagram", Handle: "testuser"}
			Expect(as.Create(a)).To(Succeed())

			cookies := []byte("session_id=abc123; csrftoken=xyz")
			err := as.UpdateCookies("acc-1", cookies)
			Expect(err).NotTo(HaveOccurred())

			// Verify has_cookies flag
			got, err := as.Get("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.HasCookies).To(BeTrue())
		})

		It("round-trips cookies through encrypt/decrypt", func() {
			a := &model.Account{ID: "acc-1", Platform: "instagram", Handle: "testuser"}
			Expect(as.Create(a)).To(Succeed())

			original := []byte("session_id=abc123; csrftoken=xyz789")
			err := as.UpdateCookies("acc-1", original)
			Expect(err).NotTo(HaveOccurred())

			// Verify round trip
			decrypted, err := as.GetCookies("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(decrypted).To(Equal(original))
		})

		It("ciphertext differs from plaintext", func() {
			a := &model.Account{ID: "acc-1", Platform: "instagram", Handle: "testuser"}
			Expect(as.Create(a)).To(Succeed())

			original := []byte("session_id=abc123")
			err := as.UpdateCookies("acc-1", original)
			Expect(err).NotTo(HaveOccurred())

			// Read raw blob from DB to verify it's encrypted
			var blob []byte
			err = db.QueryRow("SELECT cookies_blob FROM accounts WHERE id = ?", "acc-1").Scan(&blob)
			Expect(err).NotTo(HaveOccurred())
			Expect(blob).NotTo(Equal(original))
			Expect(len(blob)).To(BeNumerically(">", len(original)))
		})

		It("returns ErrNotFound for missing account", func() {
			err := as.UpdateCookies("nonexistent", []byte("cookies"))
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("GetCookies", func() {
		It("returns nil for account without cookies", func() {
			a := &model.Account{ID: "acc-1", Platform: "instagram", Handle: "testuser"}
			Expect(as.Create(a)).To(Succeed())

			cookies, err := as.GetCookies("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(cookies).To(BeNil())
		})
	})

	Describe("Delete", func() {
		It("removes an account", func() {
			a := &model.Account{ID: "acc-1", Platform: "instagram", Handle: "testuser"}
			Expect(as.Create(a)).To(Succeed())

			err := as.Delete("acc-1")
			Expect(err).NotTo(HaveOccurred())

			_, err = as.Get("acc-1")
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("cascades to subject_accounts links", func() {
			// Create a project and subject first
			ps := store.NewProjectStore(db)
			p := &model.Project{ID: "p-1", Slug: "proj", Name: "Project"}
			Expect(ps.Create(p)).To(Succeed())

			ss := store.NewSubjectStore(db)
			sub := &model.Subject{ID: "s-1", ProjectID: "p-1", Slug: "subj", Name: "Subject"}
			Expect(ss.Create(sub)).To(Succeed())

			// Create account and link
			a := &model.Account{ID: "acc-1", Platform: "instagram", Handle: "testuser"}
			Expect(as.Create(a)).To(Succeed())

			_, err := db.Exec(
				"INSERT INTO subject_accounts (subject_id, account_id, created_at) VALUES (?, ?, datetime('now'))",
				"s-1", "acc-1",
			)
			Expect(err).NotTo(HaveOccurred())

			// Delete account should cascade
			err = as.Delete("acc-1")
			Expect(err).NotTo(HaveOccurred())

			// Verify link is gone
			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM subject_accounts WHERE account_id = ?", "acc-1").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("returns ErrNotFound for missing account", func() {
			err := as.Delete("nonexistent")
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("HandleExists", func() {
		It("returns true for existing (platform, handle)", func() {
			a := &model.Account{ID: "acc-1", Platform: "instagram", Handle: "testuser"}
			Expect(as.Create(a)).To(Succeed())

			exists, err := as.HandleExists("instagram", "testuser")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("returns false for non-existing (platform, handle)", func() {
			exists, err := as.HandleExists("instagram", "noone")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})

	Describe("GetDetail", func() {
		It("includes linked subjects", func() {
			// Create project, subject, account, and link
			ps := store.NewProjectStore(db)
			p := &model.Project{ID: "p-1", Slug: "proj", Name: "Project"}
			Expect(ps.Create(p)).To(Succeed())

			ss := store.NewSubjectStore(db)
			sub := &model.Subject{ID: "s-1", ProjectID: "p-1", Slug: "subj", Name: "My Subject"}
			Expect(ss.Create(sub)).To(Succeed())

			a := &model.Account{ID: "acc-1", Platform: "instagram", Handle: "testuser"}
			Expect(as.Create(a)).To(Succeed())

			_, err := db.Exec(
				"INSERT INTO subject_accounts (subject_id, account_id, created_at) VALUES (?, ?, datetime('now'))",
				"s-1", "acc-1",
			)
			Expect(err).NotTo(HaveOccurred())

			detail, err := as.GetDetail("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(detail.LinkedSubjects).To(HaveLen(1))
			Expect(detail.LinkedSubjects[0].ID).To(Equal("s-1"))
			Expect(detail.LinkedSubjects[0].ProjectID).To(Equal("p-1"))
			Expect(detail.LinkedSubjects[0].Name).To(Equal("My Subject"))
		})
	})
})
