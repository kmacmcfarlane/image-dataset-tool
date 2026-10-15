package store_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("AccountStore session expiry", func() {
	var (
		tmpDir string
		db     *sql.DB
		as     *store.AccountStore
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "account-session-test-*")
		Expect(err).NotTo(HaveOccurred())

		db, err = store.OpenDB(filepath.Join(tmpDir, "test.sqlite"))
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())

		as = store.NewAccountStore(db, make([]byte, crypto.KeySize))
		Expect(as.Create(&model.Account{ID: "acc-1", Platform: "instagram", Handle: "testuser"})).To(Succeed())
	})

	AfterEach(func() {
		db.Close()
		os.RemoveAll(tmpDir)
	})

	Describe("UpdateCookiesWithSessionExpiry", func() {
		It("stores cookies and expiry together", func() {
			want := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
			Expect(as.UpdateCookiesWithSessionExpiry("acc-1", []byte("sessionid=abc"), want)).To(Succeed())

			cookies, err := as.GetCookies("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(cookies)).To(Equal("sessionid=abc"))

			got, err := as.GetSessionExpiry("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(want))
		})

		It("returns ErrNotFound for a missing account", func() {
			err := as.UpdateCookiesWithSessionExpiry("nonexistent", []byte("x"), time.Now())
			Expect(err).To(MatchError(store.ErrNotFound))
		})
	})

	Describe("UpdateSessionExpiry", func() {
		It("stores the expiry so GetSessionExpiry returns it in UTC", func() {
			want := time.Date(2026, 5, 27, 12, 30, 0, 0, time.FixedZone("X", 2*3600))
			Expect(as.UpdateSessionExpiry("acc-1", want)).To(Succeed())

			got, err := as.GetSessionExpiry("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Equal(want)).To(BeTrue())
			Expect(got.Location()).To(Equal(time.UTC))
		})

		It("returns ErrNotFound for a missing account", func() {
			err := as.UpdateSessionExpiry("nonexistent", time.Now())
			Expect(err).To(MatchError(store.ErrNotFound))
		})
	})

	Describe("GetSessionExpiry", func() {
		It("returns zero time when the expiry was never set", func() {
			got, err := as.GetSessionExpiry("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.IsZero()).To(BeTrue())
		})

		It("returns ErrNotFound for a missing account", func() {
			_, err := as.GetSessionExpiry("nonexistent")
			Expect(err).To(MatchError(store.ErrNotFound))
		})

		It("returns a parse error for a malformed stored value", func() {
			_, err := db.Exec(`UPDATE accounts SET session_expires_at = 'not-a-time' WHERE id = 'acc-1'`)
			Expect(err).NotTo(HaveOccurred())

			_, err = as.GetSessionExpiry("acc-1")
			Expect(err).To(MatchError(ContainSubstring("parse session_expires_at")))
		})
	})
})
