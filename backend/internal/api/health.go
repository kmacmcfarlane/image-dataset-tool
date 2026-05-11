package api

import (
	"context"
	"database/sql"
	"os"

	logrus "github.com/sirupsen/logrus"

	health "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/health"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
)

// HealthChecker provides health status for external subsystems.
type HealthChecker interface {
	// CheckNATS returns nil if the embedded NATS server is healthy.
	CheckNATS() error
}

// HealthService implements the health service interface.
type HealthService struct {
	db      *sql.DB
	checker HealthChecker
	dataDir string
	keyPath string
}

// NewHealthService creates a new health service with subsystem dependencies.
func NewHealthService(db *sql.DB, checker HealthChecker, dataDir, keyPath string) *HealthService {
	return &HealthService{
		db:      db,
		checker: checker,
		dataDir: dataDir,
		keyPath: keyPath,
	}
}

// Check returns the current health status with per-subsystem detail.
// Returns 200 with HealthResult if all healthy, or the HealthResult as an
// "unhealthy" error (503) if any subsystem is failing.
func (s *HealthService) Check(_ context.Context) (*health.HealthResult, error) {
	log := logrus.WithField("component", "health")

	subs := make(map[string]*health.SubsystemStatus, 4)
	allOK := true

	// 1. SQLite connectivity
	subs["sqlite"] = s.checkSQLite()
	if !subs["sqlite"].OK {
		allOK = false
	}

	// 2. NATS embedded server
	subs["nats"] = s.checkNATS()
	if !subs["nats"].OK {
		allOK = false
	}

	// 3. $DATA_DIR writable
	subs["data_dir"] = s.checkDataDir()
	if !subs["data_dir"].OK {
		allOK = false
	}

	// 4. Encryption key
	subs["key"] = s.checkKey()
	if !subs["key"].OK {
		allOK = false
	}

	if allOK {
		return &health.HealthResult{
			Status:     "healthy",
			Subsystems: subs,
		}, nil
	}

	log.WithField("subsystems", subs).Warn("Health check failed")
	// Return as error so Goa maps to 503.
	return nil, &health.HealthResult{
		Status:     "unhealthy",
		Subsystems: subs,
	}
}

func (s *HealthService) checkSQLite() *health.SubsystemStatus {
	if s.db == nil {
		msg := "database not initialised"
		return &health.SubsystemStatus{OK: false, Message: &msg}
	}
	var n int
	if err := s.db.QueryRow("SELECT 1").Scan(&n); err != nil {
		msg := err.Error()
		return &health.SubsystemStatus{OK: false, Message: &msg}
	}
	msg := "ok"
	return &health.SubsystemStatus{OK: true, Message: &msg}
}

func (s *HealthService) checkNATS() *health.SubsystemStatus {
	if s.checker == nil {
		msg := "NATS checker not configured"
		return &health.SubsystemStatus{OK: false, Message: &msg}
	}
	if err := s.checker.CheckNATS(); err != nil {
		msg := err.Error()
		return &health.SubsystemStatus{OK: false, Message: &msg}
	}
	msg := "ok"
	return &health.SubsystemStatus{OK: true, Message: &msg}
}

func (s *HealthService) checkDataDir() *health.SubsystemStatus {
	// Check directory exists and is writable by attempting to create a temp file.
	f, err := os.CreateTemp(s.dataDir, ".health-check-*")
	if err != nil {
		msg := err.Error()
		return &health.SubsystemStatus{OK: false, Message: &msg}
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	msg := "writable"
	return &health.SubsystemStatus{OK: true, Message: &msg}
}

func (s *HealthService) checkKey() *health.SubsystemStatus {
	_, err := crypto.LoadKey(s.keyPath)
	if err != nil {
		msg := err.Error()
		return &health.SubsystemStatus{OK: false, Message: &msg}
	}
	msg := "valid"
	return &health.SubsystemStatus{OK: true, Message: &msg}
}
