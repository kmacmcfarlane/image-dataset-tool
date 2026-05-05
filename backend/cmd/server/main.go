package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	logrus "github.com/sirupsen/logrus"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
	health "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/health"
	healthsvr "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/http/health/server"
	genqueues "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/queues"
	queuessvr "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/http/queues/server"
	gensettings "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/settings"
	settingssvr "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/http/settings/server"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/datadir"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/reconciler"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
	goahttp "goa.design/goa/v3/http"
)

func main() {
	// --- Startup initialization ---

	// 1. Bootstrap data directory (creates dirs if missing, resolves path).
	dir, err := datadir.Bootstrap()
	if err != nil {
		logrus.Fatalf("Failed to bootstrap data directory: %v", err)
	}

	// 2a. Auto-provision secret.key on first start (dev convenience).
	if err := datadir.EnsureSecretKey(dir); err != nil {
		logrus.Fatalf("Failed to provision encryption key: %v", err)
	}

	// 2b. Load encryption key — fail fast with clear error on missing/bad perms.
	keyPath := datadir.SecretKeyPath(dir)
	encKey, err := crypto.LoadKey(keyPath)
	if err != nil {
		logrus.Fatalf("Failed to load encryption key: %v", err)
	}

	// 3. Open SQLite database and run migrations.
	db, err := store.OpenDB(datadir.DBPath(dir))
	if err != nil {
		logrus.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := store.Migrate(db); err != nil {
		logrus.Fatalf("Failed to run database migrations: %v", err)
	}

	// 4. Run filesystem reconciler (must complete before NATS consumers start).
	if err := reconciler.Run(db, dir); err != nil {
		logrus.Fatalf("Failed to run filesystem reconciler: %v", err)
	}

	// 5. Start embedded NATS JetStream server.
	natsDataDir := filepath.Join(dir, "nats")
	natsCfg := natsutil.DefaultConfig(natsDataDir)
	natsSrv, err := natsutil.New(natsCfg)
	if err != nil {
		logrus.Fatalf("Failed to start embedded NATS server: %v", err)
	}
	defer natsSrv.Shutdown()

	// --- HTTP server ---

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Load application config (optional; errors are non-fatal).
	cfg, err := api.LoadConfig()
	if err != nil {
		logrus.WithError(err).Warn("Failed to load config.yaml — using empty config")
		cfg = map[string]any{}
	}

	// Create service implementations
	healthSvc := api.NewHealthService()
	queuesSvc := api.NewQueuesService(natsSrv.JS())
	secretsStore := store.NewSecretsStore(db, encKey)
	settingsSvc := api.NewSettingsService(secretsStore, dir, cfg)

	// Create endpoints
	healthEndpoints := health.NewEndpoints(healthSvc)
	queuesEndpoints := genqueues.NewEndpoints(queuesSvc)
	settingsEndpoints := gensettings.NewEndpoints(settingsSvc)

	// Create HTTP mux
	mux := goahttp.NewMuxer()

	// Create and mount health server
	healthServer := healthsvr.New(healthEndpoints, mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	healthsvr.Mount(mux, healthServer)

	// Create and mount queues server
	queuesServer := queuessvr.New(queuesEndpoints, mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	queuessvr.Mount(mux, queuesServer)

	// Create and mount settings server
	settingsServer := settingssvr.New(settingsEndpoints, mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	settingssvr.Mount(mux, settingsServer)

	// Create HTTP server
	addr := fmt.Sprintf(":%s", port)
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	logrus.Infof("Starting server on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logrus.Fatalf("Server failed: %v", err)
	}
}
