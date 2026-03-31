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

package server

import (
	"fmt"
	"log/slog"

	"github.com/thomsonreuters/gate/internal/config"
	"github.com/thomsonreuters/gate/internal/sts"
	auditbackends "github.com/thomsonreuters/gate/internal/sts/audit/backends"
	"github.com/thomsonreuters/gate/internal/sts/selector"
	selectorbackends "github.com/thomsonreuters/gate/internal/sts/selector/backends"
)

// buildDependencies constructs the STS service and its external dependencies from config.
// On error, any already-created resources are closed before returning.
func (s *Server) buildDependencies() (*sts.Service, []func() error, error) {
	cfg := s.cfg
	ctx := s.ctx

	store, err := selectorbackends.NewStore(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("creating selector store: %w", err)
	}

	apps := buildApps(cfg)
	sel, err := selector.NewSelector(apps, store)
	if err != nil {
		_ = store.Close()
		return nil, nil, fmt.Errorf("creating selector: %w", err)
	}

	audit, err := auditbackends.NewBackend(ctx, cfg)
	if err != nil {
		_ = store.Close()
		return nil, nil, fmt.Errorf("creating audit backend: %w", err)
	}

	service, err := sts.NewService(cfg, sts.Dependencies{
		Selector: sel,
		Audit:    audit,
		Logger:   slog.Default(),
	})
	if err != nil {
		_ = audit.Close()
		_ = store.Close()
		return nil, nil, fmt.Errorf("creating STS service: %w", err)
	}

	// LIFO: service first (closes audit internally), then store.
	closers := []func() error{service.Close, store.Close}

	return service, closers, nil
}

// buildApps builds the selector app list from the GitHub apps config.
func buildApps(cfg *config.Config) []selector.App {
	apps := make([]selector.App, 0, len(cfg.GitHubApps))
	for _, appCfg := range cfg.GitHubApps {
		apps = append(apps, selector.App{
			ClientID:     appCfg.ClientID,
			Organization: appCfg.Organization,
		})
	}
	return apps
}
