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

package harness

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ggicci/httpin"
	httpin_integration "github.com/ggicci/httpin/integration"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/thomsonreuters/gate/cmd/server/handlers"
	"github.com/thomsonreuters/gate/internal/config"
	"github.com/thomsonreuters/gate/internal/sts"
	"github.com/thomsonreuters/gate/internal/sts/audit"
	"github.com/thomsonreuters/gate/internal/sts/selector"
	selectorbackends "github.com/thomsonreuters/gate/internal/sts/selector/backends"
	"github.com/thomsonreuters/gate/internal/testutil"
)

func init() {
	httpin_integration.UseGochiURLParam("path", chi.URLParam)
}

// TestApp describes a GitHub App for use in integration tests.
type TestApp struct {
	ClientID     string
	Organization string
}

// Server wraps the STS service and HTTP test server for integration testing.
type Server struct {
	t       *testing.T
	HTTP    *httptest.Server
	service *sts.Service
	store   selector.Store
}

// ServerOption configures the Server during creation.
type ServerOption func(*serverConfig)

type serverConfig struct {
	configuration         *config.Config
	requireExplicitPolicy bool
	defaultTTL            int
	maxTTL                int
	maxPermissions        map[string]string
	apps                  []*TestApp
}

// WithConfiguration sets the full config for the test server; policy
// and OIDC are still overridden from Context.
func WithConfiguration(cfg *config.Config) ServerOption {
	return func(c *serverConfig) {
		c.configuration = cfg
	}
}

// WithRequireExplicitPolicy sets whether the server requires an
// explicit repository policy to grant tokens.
func WithRequireExplicitPolicy(require bool) ServerOption {
	return func(c *serverConfig) {
		c.requireExplicitPolicy = require
	}
}

// WithDefaultTTL sets the default token TTL in seconds for the test server policy.
func WithDefaultTTL(ttl int) ServerOption {
	return func(c *serverConfig) {
		c.defaultTTL = ttl
	}
}

// WithMaxTTL sets the maximum token TTL in seconds for the test server policy.
func WithMaxTTL(ttl int) ServerOption {
	return func(c *serverConfig) {
		c.maxTTL = ttl
	}
}

// WithMaxPermissions sets the maximum allowed permissions for the test server policy.
func WithMaxPermissions(permissions map[string]string) ServerOption {
	return func(c *serverConfig) {
		c.maxPermissions = permissions
	}
}

// WithGitHubApps sets the GitHub Apps (client ID and org) used by the test server.
func WithGitHubApps(apps []*TestApp) ServerOption {
	return func(c *serverConfig) {
		c.apps = apps
	}
}

// StartServer creates and starts a new STS test server.
// Cleanup is registered via t.Cleanup — callers never need to call Close.
func StartServer(t *testing.T, ctx *Context, options ...ServerOption) *Server {
	t.Helper()

	sc := &serverConfig{
		defaultTTL: 3600,
		maxTTL:     7200,
		maxPermissions: map[string]string{
			"contents":      "write",
			"metadata":      "read",
			"packages":      "write",
			"actions":       "write",
			"issues":        "write",
			"pull_requests": "write",
		},
	}

	for _, opt := range options {
		opt(sc)
	}

	var cfg *config.Config
	if sc.configuration != nil {
		cfg = sc.configuration
	} else {
		cfg = &config.Config{
			Policy: config.PolicyConfig{
				Version:               "1.0",
				TrustPolicyPath:       ".github/trust-policy.yaml",
				DefaultTokenTTL:       sc.defaultTTL,
				MaxTokenTTL:           sc.maxTTL,
				RequireExplicitPolicy: sc.requireExplicitPolicy,
				MaxPermissions:        sc.maxPermissions,
				GitHubAPIBaseURL:      ctx.GitHubAPIURL(),
				GitHubRawBaseURL:      ctx.GitHubAPIURL(),
				Providers: []config.ProviderConfig{
					{
						Name:   "github-actions",
						Issuer: ctx.IssuerURL(),
					},
				},
			},
			OIDC: config.OIDCConfig{
				Audience: ctx.IssuerURL(),
			},
			Server: config.ServerConfig{
				Port:            8080,
				ReadTimeout:     30 * time.Second,
				WriteTimeout:    30 * time.Second,
				ShutdownTimeout: 10 * time.Second,
				RequestTimeout:  30 * time.Second,
				IdleTimeout:     10 * time.Second,
				WaitTimeout:     10 * time.Second,
			},
		}
	}

	cfg.Policy.GitHubAPIBaseURL = ctx.GitHubAPIURL()
	cfg.Policy.GitHubRawBaseURL = ctx.GitHubAPIURL()
	if len(cfg.Policy.Providers) == 0 {
		cfg.Policy.Providers = []config.ProviderConfig{
			{Name: "github-actions", Issuer: ctx.IssuerURL()},
		}
	} else {
		cfg.Policy.Providers[0].Issuer = ctx.IssuerURL()
	}

	var testApps []*TestApp
	if len(sc.apps) > 0 {
		testApps = sc.apps
	} else {
		testApps = []*TestApp{{ClientID: "client-1", Organization: "example-org"}}
	}

	ghApps := make([]config.GitHubAppConfig, 0, len(testApps))
	selApps := make([]selector.App, 0, len(testApps))
	for _, ta := range testApps {
		key := testutil.GenerateRSAKeyObject(t)
		keyPath := testutil.WriteKeyFile(t, key)
		ghApps = append(ghApps, config.GitHubAppConfig{
			ClientID:       ta.ClientID,
			PrivateKeyPath: keyPath,
			Organization:   ta.Organization,
		})
		selApps = append(selApps, selector.App{
			ClientID:     ta.ClientID,
			Organization: ta.Organization,
		})
	}
	cfg.GitHubApps = ghApps

	store := selectorbackends.NewMemoryStore()
	sel, err := selector.NewSelector(selApps, store)
	if err != nil {
		t.Fatalf("create selector: %v", err)
	}

	backend := newMockAuditBackend()
	logger := slog.Default()

	svc, err := sts.NewService(cfg, sts.Dependencies{
		Selector: sel,
		Audit:    backend,
		Logger:   logger,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	exchangeHandler := handlers.NewExchangeHandler(svc)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Heartbeat("/health"))
	router.Route("/api/v1", func(r chi.Router) {
		r.With(httpin.NewInput(handlers.ExchangeInput{})).Post("/exchange", exchangeHandler.Exchange)
	})

	httpServer := httptest.NewServer(router)
	const clientTimeout = 30 * time.Second
	ctx.Client = newClient(httpServer.URL, clientTimeout)

	s := &Server{
		t:       t,
		HTTP:    httpServer,
		service: svc,
		store:   store,
	}

	t.Cleanup(s.Close)

	return s
}

// Close shuts down the server and releases resources.
func (s *Server) Close() {
	if s.HTTP != nil {
		s.HTTP.Close()
	}
	if s.service != nil {
		if err := s.service.Close(); err != nil {
			s.t.Logf("failed to close service: %v", err)
		}
	}
	if s.store != nil {
		if err := s.store.Close(); err != nil {
			s.t.Logf("failed to close store: %v", err)
		}
	}
}

type mockAuditBackend struct {
	mu      sync.Mutex
	entries []*audit.AuditEntry
}

// newMockAuditBackend returns an in-memory audit backend that appends
// entries for inspection in tests.
func newMockAuditBackend() *mockAuditBackend {
	return &mockAuditBackend{
		entries: make([]*audit.AuditEntry, 0),
	}
}

func (m *mockAuditBackend) Log(_ context.Context, entry *audit.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockAuditBackend) Close() error {
	return nil
}
