package main

import (
	"fmt"
	"net/http"
	"os"

	logrus "github.com/sirupsen/logrus"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
	health "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/health"
	healthsvr "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/http/health/server"
	goahttp "goa.design/goa/v3/http"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create service implementations
	healthSvc := api.NewHealthService()

	// Create endpoints
	healthEndpoints := health.NewEndpoints(healthSvc)

	// Create HTTP mux
	mux := goahttp.NewMuxer()

	// Create and mount health server
	healthServer := healthsvr.New(healthEndpoints, mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	healthsvr.Mount(mux, healthServer)

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
