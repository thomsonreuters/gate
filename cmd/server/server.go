// Copyright 2026 Thomson Reuters
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package server implements the Gate HTTP server and its lifecycle.
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ggicci/httpin"
	httpin_integration "github.com/ggicci/httpin/integration"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/spf13/cobra"
	"github.com/thomsonreuters/gate/cmd/server/handlers"
	"github.com/thomsonreuters/gate/cmd/server/middlewares"
	"github.com/thomsonreuters/gate/internal/config"
)

const (
	healthPath = "/health"
)

// Server is the main HTTP server.
type Server struct {
	cfg     *config.Config
	router  *chi.Mux
	ctx     context.Context
	closers []func() error
}

// NewServer creates a new Server instance.
func NewServer(ctx context.Context, config *config.Config) *Server {
	return &Server{
		ctx: ctx,
		cfg: config,
	}
}

// Init initializes the HTTP server with routes, middleware, and configuration.
func (s *Server) Init() error {
	service, closers, err := s.buildDependencies()
	if err != nil {
		return fmt.Errorf("building dependencies: %w", err)
	}
	s.closers = closers

	infoHandler := handlers.NewInfoHandler()
	exchangeHandler := handlers.NewExchangeHandler(service)

	s.router = chi.NewRouter()
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(httplog.RequestLogger(slog.Default(), &httplog.Options{
		Level:         slog.LevelInfo,
		RecoverPanics: true,
		Skip: func(req *http.Request, respStatus int) bool {
			return req.URL.Path == healthPath && respStatus == http.StatusOK
		},
	}))
	s.router.Use(middleware.Timeout(s.cfg.Server.RequestTimeout))
	s.router.Use(middleware.StripSlashes)
	s.router.Use(middleware.CleanPath)
	s.router.Use(middleware.Heartbeat(healthPath))
	s.router.Use(middlewares.SecurityHeaders)

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Use(middlewares.OriginVerify(s.cfg.Origin))
		r.Get("/info", infoHandler.GetInfo)
		r.With(httpin.NewInput(handlers.ExchangeInput{})).Post("/exchange", exchangeHandler.Exchange)
	})

	return nil
}

// serve starts the HTTP server and handles graceful shutdown on context cancellation.
func (s *Server) serve() error {
	address := s.cfg.Server.GetAddr()

	httpServer := &http.Server{
		Handler:      s.router,
		Addr:         address,
		WriteTimeout: s.cfg.Server.WriteTimeout,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		IdleTimeout:  s.cfg.Server.IdleTimeout,
	}

	lc := net.ListenConfig{}
	ln, err := lc.Listen(s.ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("binding to %s: %w", address, err)
	}

	if s.cfg.Server.TLS.Enabled() {
		cert, err := tls.LoadX509KeyPair(s.cfg.Server.TLS.CertFilePath, s.cfg.Server.TLS.KeyFilePath)
		if err != nil {
			_ = ln.Close()
			return fmt.Errorf("loading TLS certificate: %w", err)
		}
		httpServer.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
		ln = tls.NewListener(ln, httpServer.TLSConfig)
		slog.InfoContext(s.ctx, "Starting server with TLS",
			"cert", s.cfg.Server.TLS.CertFilePath,
			"key", s.cfg.Server.TLS.KeyFilePath,
			"address", address,
		)
	} else {
		slog.InfoContext(s.ctx, "Starting server without TLS", "address", address)
	}

	errChannel := make(chan error, 1)
	go func() {
		if serveErr := httpServer.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			errChannel <- serveErr
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-c:
		slog.InfoContext(s.ctx, "Received shutdown signal", "signal", sig.String())
	case err := <-errChannel:
		return fmt.Errorf("server error: %w", err)
	}

	ctx, cancel := context.WithTimeout(s.ctx, s.cfg.Server.WaitTimeout)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		slog.ErrorContext(s.ctx, "Server shutdown failed", "error", err)
		return err
	}

	s.close()
	slog.InfoContext(s.ctx, "Server shutdown successfully")
	return nil
}

// close performs cleanup operations when shutting down the server.
func (s *Server) close() {
	for _, fn := range s.closers {
		if err := fn(); err != nil {
			slog.ErrorContext(s.ctx, "cleanup failed", "error", err)
		}
	}
}

// ServerCmd represents the server command.
var ServerCmd = &cobra.Command{
	Use:   "server",
	Short: "run the server",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		server := NewServer(ctx, config.GetCurrent())
		if err := server.Init(); err != nil {
			return err
		}
		return server.serve()
	},
}

func init() {
	httpin_integration.UseGochiURLParam("path", chi.URLParam)
}
