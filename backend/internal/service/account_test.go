package service_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("AccountService", func() {
	var (
		tmpDir  string
		svc     *service.AccountService
		as      *store.AccountStore
		cleanup func()
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "account-svc-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err := store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())

		err = store.Migrate(db)
		Expect(err).NotTo(HaveOccurred())

		encKey := make([]byte, crypto.KeySize)
		for i := range encKey {
			encKey[i] = byte(i)
		}

		as = store.NewAccountStore(db, encKey)
		svc = service.NewAccountService(as)

		cleanup = func() {
			db.Close()
			os.RemoveAll(tmpDir)
		}
	})

	AfterEach(func() {
		cleanup()
	})

	Describe("Create", func() {
		It("creates an account with normalized platform", func() {
			a, err := svc.Create("Instagram", "testuser")
			Expect(err).NotTo(HaveOccurred())
			Expect(a.ID).NotTo(BeEmpty())
			Expect(a.Platform).To(Equal("instagram"))
			Expect(a.Handle).To(Equal("testuser"))
		})

		It("trims whitespace from handle", func() {
			a, err := svc.Create("instagram", "  testuser  ")
			Expect(err).NotTo(HaveOccurred())
			Expect(a.Handle).To(Equal("testuser"))
		})

		It("returns ErrBadRequest for empty platform", func() {
			_, err := svc.Create("", "handle")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("bad request")))
		})

		It("returns ErrBadRequest for empty handle", func() {
			_, err := svc.Create("instagram", "")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("bad request")))
		})

		It("returns ErrConflict for duplicate (platform, handle)", func() {
			_, err := svc.Create("instagram", "user1")
			Expect(err).NotTo(HaveOccurred())

			_, err = svc.Create("instagram", "user1")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("conflict")))
		})
	})

	Describe("Get", func() {
		It("returns an existing account", func() {
			created, err := svc.Create("instagram", "testuser")
			Expect(err).NotTo(HaveOccurred())

			got, err := svc.Get(created.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Handle).To(Equal("testuser"))
		})

		It("returns ErrNotFound for missing account", func() {
			_, err := svc.Get("nonexistent")
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("List", func() {
		It("returns all accounts", func() {
			_, err := svc.Create("instagram", "user1")
			Expect(err).NotTo(HaveOccurred())
			_, err = svc.Create("tiktok", "user2")
			Expect(err).NotTo(HaveOccurred())

			list, err := svc.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(HaveLen(2))
		})
	})

	Describe("UpdateCookies", func() {
		It("stores cookies for an existing account", func() {
			a, err := svc.Create("instagram", "testuser")
			Expect(err).NotTo(HaveOccurred())

			err = svc.UpdateCookies(a.ID, "session_id=abc123")
			Expect(err).NotTo(HaveOccurred())

			// Verify has_cookies via Get
			got, err := svc.Get(a.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.HasCookies).To(BeTrue())
		})

		It("returns ErrNotFound for missing account", func() {
			err := svc.UpdateCookies("nonexistent", "cookies")
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("Delete", func() {
		It("deletes an existing account", func() {
			a, err := svc.Create("instagram", "testuser")
			Expect(err).NotTo(HaveOccurred())

			err = svc.Delete(a.ID)
			Expect(err).NotTo(HaveOccurred())

			_, err = svc.Get(a.ID)
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("returns ErrNotFound for missing account", func() {
			err := svc.Delete("nonexistent")
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})
})
