package service_test

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("AccountService session expiry", func() {
	var (
		svc     *service.AccountService
		as      *store.AccountStore
		cleanup func()
	)

	BeforeEach(func() {
		tmpDir, err := os.MkdirTemp("", "account-svc-session-test-*")
		Expect(err).NotTo(HaveOccurred())

		db, err := store.OpenDB(filepath.Join(tmpDir, "test.sqlite"))
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())

		as = store.NewAccountStore(db, make([]byte, crypto.KeySize))
		svc = service.NewAccountService(as)
		cleanup = func() {
			db.Close()
			os.RemoveAll(tmpDir)
		}
	})

	AfterEach(func() {
		cleanup()
	})

	It("sets session_expires_at to now + IGSessionTTL when cookies are imported", func() {
		a, err := svc.Create("instagram", "testuser")
		Expect(err).NotTo(HaveOccurred())

		before := time.Now().UTC().Truncate(time.Second)
		Expect(svc.UpdateCookies(a.ID, "sessionid=abc; csrftoken=xyz; ds_user_id=123")).To(Succeed())
		after := time.Now().UTC()

		got, err := as.GetSessionExpiry(a.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeTemporally(">=", before.Add(service.IGSessionTTL)))
		Expect(got).To(BeTemporally("<=", after.Add(service.IGSessionTTL)))
	})

	It("leaves session_expires_at unset when the cookies are rejected", func() {
		a, err := svc.Create("instagram", "testuser")
		Expect(err).NotTo(HaveOccurred())

		Expect(svc.UpdateCookies(a.ID, "sessionid=abc")).NotTo(Succeed())

		got, err := as.GetSessionExpiry(a.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.IsZero()).To(BeTrue())
	})
})
