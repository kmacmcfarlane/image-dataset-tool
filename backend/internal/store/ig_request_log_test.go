package store_test

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("IGRequestLogStore", func() {
	var (
		logStore *store.IGRequestLogStore
		cleanup  func()
	)

	BeforeEach(func() {
		tmpDir, err := os.MkdirTemp("", "ig-reqlog-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err := store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())

		err = store.Migrate(db)
		Expect(err).NotTo(HaveOccurred())

		// Insert a test account so FK constraint is satisfied.
		_, err = db.Exec(`INSERT INTO accounts (id, platform, handle, created_at, updated_at)
			VALUES ('acc-1', 'instagram', 'testuser', datetime('now'), datetime('now'))`)
		Expect(err).NotTo(HaveOccurred())

		logStore = store.NewIGRequestLogStore(db)
		cleanup = func() {
			db.Close()
			os.RemoveAll(tmpDir)
		}
	})

	AfterEach(func() {
		cleanup()
	})

	Describe("Insert", func() {
		It("writes a request log entry", func() {
			entry := &model.IGRequestLog{
				Timestamp:      time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
				AccountID:      "acc-1",
				Endpoint:       "/api/v1/feed/user/53611243/?count=12",
				HTTPStatus:     200,
				IsThrottled:    false,
				IsDenied:       false,
				ResponseTimeMs: 150,
				PublicIP:       "1.2.3.4",
				ErrorDetail:    "",
			}
			err := logStore.Insert(entry)
			Expect(err).NotTo(HaveOccurred())
		})

		It("writes a throttled request log entry", func() {
			entry := &model.IGRequestLog{
				Timestamp:      time.Date(2026, 5, 20, 12, 1, 0, 0, time.UTC),
				AccountID:      "acc-1",
				Endpoint:       "/api/v1/feed/user/53611243/?count=12",
				HTTPStatus:     429,
				IsThrottled:    true,
				IsDenied:       false,
				ResponseTimeMs: 50,
				PublicIP:       "1.2.3.4",
				ErrorDetail:    "rate limited",
			}
			err := logStore.Insert(entry)
			Expect(err).NotTo(HaveOccurred())
		})

		It("writes a denied request log entry", func() {
			entry := &model.IGRequestLog{
				Timestamp:      time.Date(2026, 5, 20, 12, 2, 0, 0, time.UTC),
				AccountID:      "acc-1",
				Endpoint:       "/api/v1/feed/user/53611243/?count=12",
				HTTPStatus:     401,
				IsThrottled:    false,
				IsDenied:       true,
				ResponseTimeMs: 30,
				PublicIP:       "1.2.3.4",
				ErrorDetail:    "unauthorized",
			}
			err := logStore.Insert(entry)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("LastIPForAccount", func() {
		It("returns empty string when no prior requests exist", func() {
			ip, err := logStore.LastIPForAccount("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(ip).To(BeEmpty())
		})

		It("returns the most recent public IP for the account", func() {
			// Insert two entries with different IPs.
			entry1 := &model.IGRequestLog{
				Timestamp:      time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
				AccountID:      "acc-1",
				Endpoint:       "/api/v1/feed/user/53611243/",
				HTTPStatus:     200,
				ResponseTimeMs: 100,
				PublicIP:       "1.1.1.1",
			}
			entry2 := &model.IGRequestLog{
				Timestamp:      time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
				AccountID:      "acc-1",
				Endpoint:       "/api/v1/feed/user/53611243/",
				HTTPStatus:     200,
				ResponseTimeMs: 100,
				PublicIP:       "2.2.2.2",
			}
			Expect(logStore.Insert(entry1)).To(Succeed())
			Expect(logStore.Insert(entry2)).To(Succeed())

			ip, err := logStore.LastIPForAccount("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(ip).To(Equal("2.2.2.2"))
		})

		It("returns the later-inserted IP when two entries share the same second", func() {
			ts := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
			for _, ip := range []string{"4.4.4.4", "5.5.5.5"} {
				Expect(logStore.Insert(&model.IGRequestLog{
					Timestamp:  ts,
					AccountID:  "acc-1",
					Endpoint:   "/api/v1/feed/user/53611243/",
					HTTPStatus: 200,
					PublicIP:   ip,
				})).To(Succeed())
			}

			ip, err := logStore.LastIPForAccount("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(ip).To(Equal("5.5.5.5"))
		})

		It("skips entries with empty public IP", func() {
			entry1 := &model.IGRequestLog{
				Timestamp:      time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
				AccountID:      "acc-1",
				Endpoint:       "/api/v1/feed/user/53611243/",
				HTTPStatus:     200,
				ResponseTimeMs: 100,
				PublicIP:       "3.3.3.3",
			}
			entry2 := &model.IGRequestLog{
				Timestamp:      time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
				AccountID:      "acc-1",
				Endpoint:       "/api/v1/feed/user/53611243/",
				HTTPStatus:     200,
				ResponseTimeMs: 100,
				PublicIP:       "",
			}
			Expect(logStore.Insert(entry1)).To(Succeed())
			Expect(logStore.Insert(entry2)).To(Succeed())

			ip, err := logStore.LastIPForAccount("acc-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(ip).To(Equal("3.3.3.3"))
		})
	})
})
