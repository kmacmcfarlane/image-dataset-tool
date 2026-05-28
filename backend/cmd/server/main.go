package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
	genaccounts "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/accounts"
	genexports "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/exports"
	accountssvr "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/http/accounts/server"
	exportssvr "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/http/exports/server"
	health "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/health"
	healthsvr "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/http/health/server"
	genjobs "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/jobs"
	jobssvr "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/http/jobs/server"
	genprojects "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/projects"
	projectssvr "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/http/projects/server"
	genqueues "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/queues"
	queuessvr "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/http/queues/server"
	gensettings "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/settings"
	settingssvr "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/http/settings/server"
	gensubjects "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/subjects"
	subjectssvr "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/http/subjects/server"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/datadir"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/export/kohya"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/reconciler"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/slogutil"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
	goahttp "goa.design/goa/v3/http"
)

func main() {
	// Load application config first so log level can be read from it.
	cfg, err := api.LoadConfig()
	if err != nil {
		// Logging not yet configured; write to stderr manually.
		fmt.Fprintf(os.Stderr, "Failed to load config.yaml — using empty config: %v\n", err)
		cfg = map[string]any{}
	}

	// Configure structured JSON logging (reads log_level from config or LOG_LEVEL env).
	slogutil.Configure(cfg)

	// --- Startup initialization ---

	// 1. Bootstrap data directory (creates dirs if missing, resolves path).
	dir, err := datadir.Bootstrap()
	if err != nil {
		slog.Error("Failed to bootstrap data directory", "error", err)
		os.Exit(1)
	}

	// 2a. Auto-provision secret.key on first start (dev convenience).
	if err := datadir.EnsureSecretKey(dir); err != nil {
		slog.Error("Failed to provision encryption key", "error", err)
		os.Exit(1)
	}

	// 2b. Load encryption key — fail fast with clear error on missing/bad perms.
	keyPath := datadir.SecretKeyPath(dir)
	encKey, err := crypto.LoadKey(keyPath)
	if err != nil {
		slog.Error("Failed to load encryption key", "error", err)
		os.Exit(1)
	}

	// 3. Open SQLite database and run migrations.
	db, err := store.OpenDB(datadir.DBPath(dir))
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := store.Migrate(db); err != nil {
		slog.Error("Failed to run database migrations", "error", err)
		os.Exit(1)
	}

	// 4. Run filesystem reconciler (must complete before NATS consumers start).
	if err := reconciler.Run(db, dir); err != nil {
		slog.Error("Failed to run filesystem reconciler", "error", err)
		os.Exit(1)
	}

	// 5. Start embedded NATS JetStream server.
	natsDataDir := filepath.Join(dir, "nats")
	natsCfg := natsutil.DefaultConfig(natsDataDir)
	natsSrv, err := natsutil.New(natsCfg)
	if err != nil {
		slog.Error("Failed to start embedded NATS server", "error", err)
		os.Exit(1)
	}
	defer natsSrv.Shutdown()

	// --- SSE infrastructure ---
	sink := pipeline.NewChannelEventSink(256)
	ring := api.NewSSERingBuffer(1024)
	sseHandler := api.NewSSEHandler(sink, ring)
	go sseHandler.StartBroadcast()

	// --- HTTP server ---

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create service implementations.
	healthSvc := api.NewHealthService(db, natsSrv, dir, keyPath)
	queuesSvc := api.NewQueuesService(natsSrv.JS())
	secretsStore := store.NewSecretsStore(db, encKey)
	settingsSvc := api.NewSettingsService(secretsStore, dir, cfg)

	projectStore := store.NewProjectStore(db)
	projectSvc := service.NewProjectService(projectStore, dir)
	projectsAPISvc := api.NewProjectsService(projectSvc)

	subjectStore := store.NewSubjectStore(db)
	subjectSvc := service.NewSubjectService(subjectStore, projectStore, dir)
	subjectsAPISvc := api.NewSubjectsService(subjectSvc)

	accountStore := store.NewAccountStore(db, encKey)
	accountSvc := service.NewAccountService(accountStore)
	accountsAPISvc := api.NewAccountsService(accountSvc)

	sampleStore := store.NewSampleStore(db)

	jobStore := store.NewJobStore(db)
	jobSvc := service.NewJobService(jobStore)
	jobsAPISvc := api.NewJobsService(jobSvc)

	// --- Process pipeline ---
	processHandler := service.NewProcessHandler(
		sampleStore, subjectStore, projectStore, jobStore, sink, dir,
		service.DefaultProcessConfig(),
	)

	// --- Export pipeline ---
	kohyaWriter := kohya.NewWriter(db, dir)
	exportStore := store.NewExportStore(db)
	exportSvc := service.NewExportService(db, natsSrv.JS(), jobStore, dir)
	zipExportSvc := service.NewZipExportService(exportStore, dir)
	exportsAPISvc := api.NewExportsService(exportSvc, zipExportSvc)

	// Create endpoints.
	healthEndpoints := health.NewEndpoints(healthSvc)
	projectsEndpoints := genprojects.NewEndpoints(projectsAPISvc)
	subjectsEndpoints := gensubjects.NewEndpoints(subjectsAPISvc)
	accountsEndpoints := genaccounts.NewEndpoints(accountsAPISvc)
	jobsEndpoints := genjobs.NewEndpoints(jobsAPISvc)
	queuesEndpoints := genqueues.NewEndpoints(queuesSvc)
	settingsEndpoints := gensettings.NewEndpoints(settingsSvc)
	exportsEndpoints := genexports.NewEndpoints(exportsAPISvc)

	// Create HTTP mux.
	mux := goahttp.NewMuxer()

	// Create and mount Goa-generated servers.
	healthServer := healthsvr.New(healthEndpoints, mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	healthsvr.Mount(mux, healthServer)

	projectsServer := projectssvr.New(projectsEndpoints, mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	projectssvr.Mount(mux, projectsServer)

	subjectsServer := subjectssvr.New(subjectsEndpoints, mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	subjectssvr.Mount(mux, subjectsServer)

	accountsServer := accountssvr.New(accountsEndpoints, mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	accountssvr.Mount(mux, accountsServer)

	jobsServer := jobssvr.New(jobsEndpoints, mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	jobssvr.Mount(mux, jobsServer)

	queuesServer := queuessvr.New(queuesEndpoints, mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	queuessvr.Mount(mux, queuesServer)

	settingsServer := settingssvr.New(settingsEndpoints, mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	settingssvr.Mount(mux, settingsServer)

	exportsServer := exportssvr.New(exportsEndpoints, mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	exportssvr.Mount(mux, exportsServer)

	// Mount custom (non-Goa) handlers.
	setupHandler := api.NewSetupHandler(dir)
	mux.Handle("GET", "/api/setup/status", http.HandlerFunc(setupHandler.ServeStatus))
	mux.Handle("POST", "/api/setup/generate-key", http.HandlerFunc(setupHandler.ServeGenerateKey))

	mediaResolver := api.NewFileMediaResolver(dir)
	mediaHandler := api.NewMediaHandler(mediaResolver)
	mux.Handle("GET", "/v1/media/{sample_id}", http.HandlerFunc(mediaHandler.ServeImage))
	mux.Handle("GET", "/v1/media/{sample_id}/thumb", http.HandlerFunc(mediaHandler.ServeThumbnail))

	samplesHandler := api.NewSamplesHandler(sampleStore)
	mux.Handle("GET", "/v1/subjects/{sid}/samples", http.HandlerFunc(samplesHandler.ServeList))
	mux.Handle("PUT", "/v1/samples/{id}/status", http.HandlerFunc(samplesHandler.ServeUpdateStatus))
	mux.Handle("PUT", "/v1/samples/bulk-status", http.HandlerFunc(samplesHandler.ServeBulkUpdateStatus))

	editStore := store.NewEditStore(db)
	editService := service.NewEditService(editStore, sampleStore, dir)
	editsHandler := api.NewEditsHandler(editService)
	mux.Handle("GET", "/v1/samples/{id}/edits", http.HandlerFunc(editsHandler.ServeGetEdits))
	mux.Handle("PUT", "/v1/samples/{id}/edits", http.HandlerFunc(editsHandler.ServePutEdits))
	mux.Handle("DELETE", "/v1/samples/{id}/edits", http.HandlerFunc(editsHandler.ServeDeleteEdits))

	captionStore := store.NewCaptionStore(db)
	captionsHandler := api.NewCaptionsHandler(captionStore, sampleStore)
	mux.Handle("GET", "/v1/samples/{id}/captions", http.HandlerFunc(captionsHandler.ServeListCaptions))
	mux.Handle("PUT", "/v1/samples/{id}/captions", http.HandlerFunc(captionsHandler.ServeSaveCaption))

	mux.Handle("GET", "/api/sse", http.HandlerFunc(sseHandler.ServeHTTP))

	// --- Export NATS consumer ---
	exportHandler := service.NewExportHandler(kohyaWriter, jobStore)

	// Start the media.export consumer.
	tracker := pipeline.NewJobTracker(db)
	etaTracker := pipeline.NewETATracker()
	processConsumer := startProcessConsumer(natsSrv, tracker, sink, etaTracker, processHandler, dir)
	defer processConsumer.Stop()

	exportConsumer := startExportConsumer(natsSrv, tracker, sink, etaTracker, exportHandler, jobStore)
	defer exportConsumer.Stop()

	mux.Handle("GET", "/docs", api.DocsHandler())
	mux.Handle("GET", "/docs/openapi3.json", api.DocsSpecHandler())

	// Apply middleware stack: outermost first.
	var handler http.Handler = mux
	handler = api.RequestLoggingMiddleware(handler)
	handler = api.ErrorMiddleware(handler)
	handler = api.CORSMiddleware(handler)

	// Create HTTP server.
	addr := fmt.Sprintf(":%s", port)
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	slog.Info("Starting server", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

// startProcessConsumer creates and starts the media.process NATS pipeline consumer.
func startProcessConsumer(
	natsSrv *natsutil.Server,
	tracker pipeline.JobTracker,
	sink pipeline.EventSink,
	etaTracker *pipeline.ETATracker,
	handler *service.ProcessHandler,
	dataDir string,
) *pipeline.Consumer {
	js := natsSrv.JS()
	ctx := context.Background()

	cons, err := js.Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
	if err != nil {
		slog.Error("Failed to get process consumer", "error", err)
		os.Exit(1)
	}

	cfg := natsutil.DefaultConfig("").Consumers.MediaProcess

	pipelineConsumer := pipeline.NewConsumer(cons, pipeline.ConsumerOpts{
		ConsumerName: natsutil.ConsumerProcess,
		Subject:      natsutil.SubjectProcess,
		Concurrency:  cfg.Concurrency,
		MaxDeliver:   cfg.MaxDeliver,
		Handler:      handler.Handle,
		Tracker:      tracker,
		JS:           js,
		Logger:       slog.With("component", "process_consumer"),
		EventSink:    sink,
		ETATracker:   etaTracker,
		OnJobComplete: func(ctx context.Context, jobID string, status pipeline.JobStatus) {
			// Clean up tmp dir for this job.
			tmpDir := filepath.Join(dataDir, "tmp", jobID)
			if err := os.RemoveAll(tmpDir); err != nil {
				slog.Warn("Failed to clean up tmp dir", "path", tmpDir, "error", err)
			} else {
				slog.Info("Cleaned up job tmp dir", "path", tmpDir)
			}
		},
	})

	pipelineConsumer.Start(ctx)
	slog.Info("Process consumer started", "concurrency", cfg.Concurrency)
	return pipelineConsumer
}

// startExportConsumer creates and starts the media.export NATS pipeline consumer.
func startExportConsumer(
	natsSrv *natsutil.Server,
	tracker pipeline.JobTracker,
	sink pipeline.EventSink,
	etaTracker *pipeline.ETATracker,
	handler *service.ExportHandler,
	jobStore *store.JobStore,
) *pipeline.Consumer {
	js := natsSrv.JS()
	ctx := context.Background()

	cons, err := js.Consumer(ctx, natsutil.StreamName, natsutil.ConsumerExport)
	if err != nil {
		slog.Error("Failed to get export consumer", "error", err)
		os.Exit(1)
	}

	pipelineConsumer := pipeline.NewConsumer(cons, pipeline.ConsumerOpts{
		ConsumerName: natsutil.ConsumerExport,
		Subject:      natsutil.SubjectExport,
		Concurrency:  2,
		MaxDeliver:   5,
		Handler:      handler.Handle,
		Tracker:      tracker,
		JS:           js,
		Logger:       slog.With("component", "export_consumer"),
		EventSink:    sink,
		ETATracker:   etaTracker,
		OnJobComplete: func(ctx context.Context, jobID string, status pipeline.JobStatus) {
			if status == pipeline.JobStatusSucceeded || status == pipeline.JobStatusFailed {
				// Update sample_count from the completed_items.
				jr, err := jobStore.GetJob(jobID)
				if err != nil {
					slog.Error("Failed to get job for export result update",
						"error", err, "job_id", jobID)
					return
				}
				if err := jobStore.UpdateJobRunExportResult(jobID, jr.OutputPath, jr.CompletedItems); err != nil {
					slog.Error("Failed to update export result",
						"error", err, "job_id", jobID)
				}
			}
		},
	})

	pipelineConsumer.Start(ctx)
	slog.Info("Export consumer started", "concurrency", 2)
	return pipelineConsumer
}
