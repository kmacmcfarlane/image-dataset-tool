package natsutil

import (
	"fmt"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	logrus "github.com/sirupsen/logrus"
)

// Server wraps an embedded NATS server with JetStream and provides
// an in-process client connection.
type Server struct {
	ns   *server.Server
	nc   *nats.Conn
	js   jetstream.JetStream
	cfg  Config
}

// New creates and starts an embedded NATS server with JetStream enabled.
// The server uses in-process connections only (no network port).
// It creates the media stream and durable pull consumers per config.
func New(cfg Config) (*Server, error) {
	log := logrus.WithField("component", "nats")

	opts := &server.Options{
		DontListen: true, // in-process only, no TCP port
		JetStream:  true,
		StoreDir:   cfg.DataDir,
		MaxPayload: int32(cfg.MaxPayloadKB) * 1024,
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("create nats server: %w", err)
	}

	// Start the server (non-blocking).
	ns.Start()

	// Wait for the server to be ready.
	if !ns.ReadyForConnections(10 * time.Second) {
		ns.Shutdown()
		return nil, fmt.Errorf("nats server not ready within timeout")
	}

	log.Info("Embedded NATS server started")

	// Create in-process client connection.
	nc, err := nats.Connect(nats.DefaultURL, nats.InProcessServer(ns))
	if err != nil {
		ns.Shutdown()
		return nil, fmt.Errorf("connect to embedded nats: %w", err)
	}

	// Create JetStream context.
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		ns.Shutdown()
		return nil, fmt.Errorf("create jetstream context: %w", err)
	}

	s := &Server{
		ns:  ns,
		nc:  nc,
		js:  js,
		cfg: cfg,
	}

	// Set up stream and consumers.
	if err := s.setupStream(); err != nil {
		nc.Close()
		ns.Shutdown()
		return nil, fmt.Errorf("setup stream: %w", err)
	}

	log.Info("NATS JetStream stream and consumers configured")
	return s, nil
}

// JS returns the JetStream interface for publishing and subscribing.
func (s *Server) JS() jetstream.JetStream {
	return s.js
}

// Conn returns the underlying NATS connection.
func (s *Server) Conn() *nats.Conn {
	return s.nc
}

// Shutdown gracefully stops the NATS server, draining the connection first.
func (s *Server) Shutdown() {
	log := logrus.WithField("component", "nats")

	if s.nc != nil {
		if err := s.nc.Drain(); err != nil {
			log.WithError(err).Warn("Error draining NATS connection")
		}
	}

	if s.ns != nil {
		s.ns.Shutdown()
		s.ns.WaitForShutdown()
	}

	log.Info("Embedded NATS server stopped")
}
