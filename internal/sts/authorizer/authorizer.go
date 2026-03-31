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

// Package authorizer implements two-layer authorization for token exchange requests.
//
// Layer 1 (Central): Validates against organization-wide policies —
// issuer allowlist, required/forbidden claims, time restrictions.
//
// Layer 2 (Repository): Validates against per-repo trust policies —
// rule matching, permission resolution, TTL enforcement.
package authorizer

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/thomsonreuters/gate/internal/clients/github"
	"github.com/thomsonreuters/gate/internal/config"
	"github.com/thomsonreuters/gate/internal/sts/selector"
	"golang.org/x/sync/singleflight"
)

// Authorizer performs two-layer authorization for token exchange requests.
type Authorizer struct {
	config   *config.PolicyConfig
	selector *selector.Selector
	clients  map[string]github.ClientIface
	cache    *policyCache
	fetchSF  singleflight.Group
	patterns map[string]*regexp.Regexp
	now      func() time.Time
	logger   *slog.Logger
}

// NewAuthorizer creates an authorizer. Claim patterns from provider configs
// are compiled once; an error is returned if any pattern is invalid.
func NewAuthorizer(
	cfg *config.PolicyConfig,
	sel *selector.Selector,
	clients map[string]github.ClientIface,
	logger *slog.Logger,
) (*Authorizer, error) {
	if logger == nil {
		logger = slog.Default()
	}

	patterns, err := compileClaimPatterns(cfg.Providers)
	if err != nil {
		return nil, err
	}

	return &Authorizer{
		config:   cfg,
		selector: sel,
		clients:  clients,
		cache:    newPolicyCache(0),
		patterns: patterns,
		now:      time.Now,
		logger:   logger,
	}, nil
}

// compileClaimPatterns compiles required and forbidden claim regex
// patterns from all providers; returns error if any pattern is invalid.
func compileClaimPatterns(providers []config.ProviderConfig) (map[string]*regexp.Regexp, error) {
	patterns := make(map[string]*regexp.Regexp)
	add := func(claim, pattern string) error {
		if _, ok := patterns[pattern]; ok {
			return nil
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid claim pattern for %q: %w", claim, err)
		}
		patterns[pattern] = re
		return nil
	}

	for _, p := range providers {
		for claim, pattern := range p.RequiredClaims {
			if err := add(claim, pattern); err != nil {
				return nil, err
			}
		}
		for claim, pattern := range p.ForbiddenClaims {
			if err := add(claim, pattern); err != nil {
				return nil, err
			}
		}
	}
	return patterns, nil
}

// Request represents a token exchange request submitted for authorization.
type Request struct {
	Claims               map[string]any
	Issuer               string
	TargetRepository     string
	PolicyName           string
	RequestedPermissions map[string]string
	RequestedTTL         int
}

// Result represents the authorization decision.
type Result struct {
	Allowed              bool
	MatchedPolicy        string
	EffectivePermissions map[string]string
	EffectiveTTL         int
	DenyReason           *DenialError
}

// denied returns a Result with Allowed false and the given DenialError.
func denied(err *DenialError) *Result {
	return &Result{Allowed: false, DenyReason: err}
}

// Authorize performs two-layer authorization. Denials are returned in
// Result.DenyReason, never as Go errors.
func (a *Authorizer) Authorize(ctx context.Context, req *Request) *Result {
	if req.Issuer == "" {
		return denied(newDenialError(ErrIssuerNotAllowed, "issuer is required", ""))
	}
	if req.TargetRepository == "" {
		return denied(newDenialError(ErrPolicyLoadFailed, "target repository is required", ""))
	}

	if denial := a.authorizeCentral(req); denial != nil {
		return a.logResult(ctx, req, denied(denial))
	}

	return a.logResult(ctx, req, a.authorizeRepository(ctx, req))
}

// logResult logs the authorization result and returns it unchanged.
func (a *Authorizer) logResult(ctx context.Context, req *Request, r *Result) *Result {
	if r.Allowed {
		a.logger.LogAttrs(ctx, slog.LevelInfo, "authorization granted",
			slog.String("policy", r.MatchedPolicy),
			slog.String("repository", req.TargetRepository),
			slog.Int("ttl", r.EffectiveTTL),
		)
	} else {
		a.logger.LogAttrs(ctx, slog.LevelWarn, "authorization denied",
			slog.String("code", string(r.DenyReason.Code)),
			slog.String("message", r.DenyReason.Message),
			slog.String("details", r.DenyReason.Details),
			slog.String("issuer", req.Issuer),
			slog.String("repository", req.TargetRepository),
		)
	}
	return r
}
