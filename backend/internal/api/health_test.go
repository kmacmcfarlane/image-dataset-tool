package api_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
	health "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/health"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}

// --- stub HealthChecker ---

type stubHealthChecker struct {
	err error
}

func (s *stubHealthChecker) CheckNATS() error { return s.err }

var _ = Describe("HealthService", func() {
	var (
		tmpDir  string
		db      *sql.DB
		keyPath string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "health-test-*")
		Expect(err).NotTo(HaveOccurred())

		// Create a valid secret key.
		keyPath = tmpDir + "/secret.key"
		key := make([]byte, crypto.KeySize)
		for i := range key {
			key[i] = byte(i + 1)
		}
		Expect(os.WriteFile(keyPath, key, 0600)).To(Succeed())

		// Open a real SQLite DB for the health check.
		dbPath := tmpDir + "/test.db"
		db, err = store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())
	})

	AfterEach(func() {
		if db != nil {
			db.Close()
		}
		os.RemoveAll(tmpDir)
	})

	Describe("Check", func() {
		Context("when all subsystems are healthy", func() {
			It("returns healthy status with all subsystems ok", func() {
				checker := &stubHealthChecker{err: nil}
				svc := api.NewHealthService(db, checker, tmpDir, keyPath)

				result, err := svc.Check(context.Background())
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Status).To(Equal("healthy"))
				Expect(result.Subsystems).To(HaveLen(4))
				Expect(result.Subsystems["sqlite"].OK).To(BeTrue())
				Expect(result.Subsystems["nats"].OK).To(BeTrue())
				Expect(result.Subsystems["data_dir"].OK).To(BeTrue())
				Expect(result.Subsystems["key"].OK).To(BeTrue())
			})
		})

		Context("when NATS is unhealthy", func() {
			It("returns unhealthy status with NATS subsystem failing", func() {
				checker := &stubHealthChecker{err: errors.New("NATS server is not running")}
				svc := api.NewHealthService(db, checker, tmpDir, keyPath)

				result, err := svc.Check(context.Background())
				// When unhealthy, the result is returned as an error.
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())

				var hr *health.HealthResult
				Expect(errors.As(err, &hr)).To(BeTrue())
				Expect(hr.Status).To(Equal("unhealthy"))
				Expect(hr.Subsystems["nats"].OK).To(BeFalse())
				Expect(*hr.Subsystems["nats"].Message).To(ContainSubstring("NATS"))
				// Other subsystems should still be ok.
				Expect(hr.Subsystems["sqlite"].OK).To(BeTrue())
				Expect(hr.Subsystems["data_dir"].OK).To(BeTrue())
				Expect(hr.Subsystems["key"].OK).To(BeTrue())
			})
		})

		Context("when key is missing", func() {
			It("reports key subsystem as unhealthy", func() {
				checker := &stubHealthChecker{err: nil}
				svc := api.NewHealthService(db, checker, tmpDir, tmpDir+"/nonexistent.key")

				result, err := svc.Check(context.Background())
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())

				var hr *health.HealthResult
				Expect(errors.As(err, &hr)).To(BeTrue())
				Expect(hr.Status).To(Equal("unhealthy"))
				Expect(hr.Subsystems["key"].OK).To(BeFalse())
			})
		})

		Context("when data_dir is not writable", func() {
			It("reports data_dir subsystem as unhealthy", func() {
				checker := &stubHealthChecker{err: nil}
				svc := api.NewHealthService(db, checker, "/nonexistent/path", keyPath)

				result, err := svc.Check(context.Background())
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())

				var hr *health.HealthResult
				Expect(errors.As(err, &hr)).To(BeTrue())
				Expect(hr.Status).To(Equal("unhealthy"))
				Expect(hr.Subsystems["data_dir"].OK).To(BeFalse())
			})
		})

		Context("when SQLite is down", func() {
			It("reports sqlite subsystem as unhealthy", func() {
				checker := &stubHealthChecker{err: nil}
				// Close the DB to simulate failure.
				db.Close()
				svc := api.NewHealthService(db, checker, tmpDir, keyPath)

				result, err := svc.Check(context.Background())
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())

				var hr *health.HealthResult
				Expect(errors.As(err, &hr)).To(BeTrue())
				Expect(hr.Status).To(Equal("unhealthy"))
				Expect(hr.Subsystems["sqlite"].OK).To(BeFalse())
				// Don't close again in AfterEach.
				db = nil
			})
		})
	})
})
