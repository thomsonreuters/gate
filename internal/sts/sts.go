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

// Package sts implements the Security Token Service exchange workflow.
//
// The Service orchestrates the full token exchange: OIDC validation,
// two-layer authorization, GitHub App selection, token generation,
// rate limit tracking, and audit logging.
package sts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/thomsonreuters/gate/internal/clients/github"
	"github.com/thomsonreuters/gate/internal/config"
	"github.com/thomsonreuters/gate/internal/sts/audit"
	"github.com/thomsonreuters/gate/internal/sts/authorizer"
	"github.com/thomsonreuters/gate/internal/sts/oidc"
	"github.com/thomsonreuters/gate/internal/sts/selector"
	"github.com/thomsonreuters/gate/internal/utils"
	"golang.org/x/sync/errgroup"
)

const (
	revocationInterval    = 1 * time.Minute
	revocationConcurrency = 5
)

// validGitHubName matches valid GitHub owner/repo name characters: alphanumeric, hyphens, underscores, and dots.
var validGitHubName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

var (
	// ErrNilConfig is returned when NewService is called with nil config.
	ErrNilConfig = errors.New("config is required")
	// ErrNilSelector is returned when NewService dependencies have a nil Selector.
	ErrNilSelector = errors.New("selector is required")
	// ErrNilAudit is returned when NewService dependencies have a nil Audit backend.
	ErrNilAudit = errors.New("audit backend is required")
	// ErrNoApps is returned when config has no GitHub apps configured.
	ErrNoApps = errors.New("at least one GitHub app is required")
)

// Exchanger defines the interface for the token exchange operation.
type Exchanger interface {
	// Exchange validates the OIDC token, authorizes the request,
	// selects an app, and returns a GitHub installation token
	// or an *ExchangeError.
	Exchange(ctx context.Context, requestID string, req *ExchangeRequest) (*ExchangeResponse, error)
}

// Compile-time check that *Service implements Exchanger.
var _ Exchanger = (*Service)(nil)

// ExchangeRequest represents an incoming token exchange request.
type ExchangeRequest struct {
	OIDCToken            string            `json:"oidc_token"`
	TargetRepository     string            `json:"target_repository"`
	PolicyName           string            `json:"policy_name,omitempty"`
	RequestedPermissions map[string]string `json:"requested_permissions,omitempty"`
	RequestedTTL         int               `json:"requested_ttl,omitempty"`
}

// ExchangeResponse represents a successful token exchange result.
type ExchangeResponse struct {
	Token         string            `json:"token"`
	ExpiresAt     time.Time         `json:"expires_at"`
	MatchedPolicy string            `json:"matched_policy"`
	Permissions   map[string]string `json:"permissions"`
	RequestID     string            `json:"request_id"`
}

// Service orchestrates the complete STS token exchange workflow.
type Service struct {
	oidc         *oidc.Validator
	authorizer   *authorizer.Authorizer
	selector     *selector.Selector
	audit        audit.AuditEntryBackend
	clients      map[string]github.ClientIface
	tokenTracker *TokenTracker
	cancelRevoke context.CancelFunc
	maxTTL       int
	logger       *slog.Logger
}

// Dependencies holds external dependencies that cannot be derived from config alone.
type Dependencies struct {
	Selector *selector.Selector
	Audit    audit.AuditEntryBackend
	Logger   *slog.Logger
}

// NewService creates a fully wired Service from config and external dependencies.
// It builds the OIDC validator, GitHub clients, and authorizer internally.
func NewService(cfg *config.Config, dependencies Dependencies) (*Service, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if dependencies.Selector == nil {
		return nil, ErrNilSelector
	}
	if dependencies.Audit == nil {
		return nil, ErrNilAudit
	}
	if len(cfg.GitHubApps) == 0 {
		return nil, ErrNoApps
	}
	if dependencies.Logger == nil {
		dependencies.Logger = slog.Default()
	}

	oidcValidator, err := oidc.NewValidator(cfg.OIDC.Audience, cfg.Policy.Issuers())
	if err != nil {
		return nil, fmt.Errorf("creating OIDC validator: %w", err)
	}

	clients, err := buildGitHubClients(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating GitHub clients: %w", err)
	}

	auth, err := authorizer.NewAuthorizer(&cfg.Policy, dependencies.Selector, clients, dependencies.Logger)
	if err != nil {
		return nil, fmt.Errorf("creating authorizer: %w", err)
	}

	revokeCtx, cancelRevoke := context.WithCancel(context.Background())

	svc := &Service{
		oidc:         oidcValidator,
		authorizer:   auth,
		selector:     dependencies.Selector,
		audit:        dependencies.Audit,
		clients:      clients,
		tokenTracker: NewTokenTracker(),
		cancelRevoke: cancelRevoke,
		maxTTL:       cfg.Policy.MaxTokenTTL,
		logger:       dependencies.Logger,
	}

	svc.runRevocationLoop(revokeCtx)

	return svc, nil
}

// buildGitHubClients constructs a map of ClientID to GitHub client
// from config, loading each app's private key from disk.
func buildGitHubClients(cfg *config.Config) (map[string]github.ClientIface, error) {
	clients := make(map[string]github.ClientIface, len(cfg.GitHubApps))

	for _, appCfg := range cfg.GitHubApps {
		keyPEM, err := os.ReadFile(appCfg.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("reading private key for app %s: %w", appCfg.ClientID, err)
		}

		client, err := github.New(github.Options{
			BaseURL:    cfg.Policy.GitHubAPIBaseURL,
			ClientID:   appCfg.ClientID,
			PrivateKey: string(keyPEM),
		})
		if err != nil {
			return nil, fmt.Errorf("creating client for app %s: %w", appCfg.ClientID, err)
		}

		clients[appCfg.ClientID] = client
	}

	return clients, nil
}

// Exchange exchanges an OIDC token for a GitHub installation token.
// It validates the OIDC token, authorizes the request against policies,
// selects an appropriate GitHub App, and generates a scoped token.
// All operations are audited and rate-limited.
func (s *Service) Exchange(ctx context.Context, requestID string, req *ExchangeRequest) (*ExchangeResponse, error) {
	if err := s.validateRequest(req); err != nil {
		return nil, &ExchangeError{
			Code:      ErrInvalidRequest,
			Message:   "Invalid request",
			Details:   err.Error(),
			RequestID: requestID,
		}
	}

	claims, err := s.oidc.Validate(ctx, req.OIDCToken)
	if err != nil {
		return nil, &ExchangeError{
			Code:      ErrInvalidToken,
			Message:   "OIDC token validation failed",
			Details:   err.Error(),
			RequestID: requestID,
		}
	}

	authReq := &authorizer.Request{
		Claims:               claimsToMap(claims),
		Issuer:               claims.Issuer,
		TargetRepository:     req.TargetRepository,
		PolicyName:           req.PolicyName,
		RequestedPermissions: req.RequestedPermissions,
		RequestedTTL:         req.RequestedTTL,
	}

	result := s.authorizer.Authorize(ctx, authReq)
	if !result.Allowed {
		s.auditDenied(ctx, requestID, claims, req, string(result.DenyReason.Code))
		return nil, &ExchangeError{
			Code:      string(result.DenyReason.Code),
			Message:   result.DenyReason.Message,
			Details:   result.DenyReason.Details,
			RequestID: requestID,
		}
	}

	app, err := s.selector.SelectApp(ctx, req.TargetRepository)
	if err != nil {
		if exhausted, ok := errors.AsType[*selector.ExhaustedError](err); ok {
			s.auditDenied(ctx, requestID, claims, req, ErrRateLimited)
			return nil, &ExchangeError{
				Code:              ErrRateLimited,
				Message:           "All GitHub Apps exhausted rate limits",
				RequestID:         requestID,
				RetryAfterSeconds: exhausted.RetryAfter,
			}
		}
		s.auditDenied(ctx, requestID, claims, req, ErrAppSelectionFailed)
		return nil, &ExchangeError{
			Code:      ErrInternalError,
			Message:   "Failed to select GitHub App",
			Details:   err.Error(),
			RequestID: requestID,
		}
	}

	client, ok := s.clients[app.ClientID]
	if !ok {
		s.auditDenied(ctx, requestID, claims, req, ErrClientNotFound)
		return nil, &ExchangeError{
			Code:      ErrInternalError,
			Message:   "GitHub client not found for app",
			Details:   app.ClientID,
			RequestID: requestID,
		}
	}

	tokenResp, err := client.RequestToken(ctx, &github.TokenRequest{
		Repository:  req.TargetRepository,
		Permissions: result.EffectivePermissions,
	})
	if err != nil {
		s.auditDenied(ctx, requestID, claims, req, ErrGitHubAPIError)
		return nil, &ExchangeError{
			Code:      ErrGitHubAPIError,
			Message:   "Failed to request GitHub token",
			Details:   err.Error(),
			RequestID: requestID,
		}
	}

	rateInfo, err := client.RateLimit(ctx, tokenResp.Token)
	if err != nil {
		s.logger.WarnContext(ctx, "rate limit query failed", "client_id", app.ClientID, "error", err)
	} else {
		if err := s.selector.RecordUsage(ctx, app.ClientID, rateInfo.Remaining, rateInfo.ResetAt); err != nil {
			s.logger.WarnContext(ctx, "record usage failed", "client_id", app.ClientID, "error", err)
		}
	}

	expires := capExpiry(tokenResp.ExpiresAt, result.EffectiveTTL)

	tokenHash := utils.HashString(tokenResp.Token)
	s.tokenTracker.Record(tokenHash, tokenResp.Token, expires)

	if err := s.auditGranted(ctx, requestID, claims, req, result, app.ClientID, tokenHash); err != nil {
		return nil, &ExchangeError{
			Code:      ErrInternalError,
			Message:   "Audit log failed",
			Details:   err.Error(),
			RequestID: requestID,
		}
	}

	return &ExchangeResponse{
		Token:         tokenResp.Token,
		ExpiresAt:     expires,
		MatchedPolicy: result.MatchedPolicy,
		Permissions:   result.EffectivePermissions,
		RequestID:     requestID,
	}, nil
}

// Close releases resources held by the service.
func (s *Service) Close() error {
	if s.cancelRevoke != nil {
		s.cancelRevoke()
	}
	if s.audit != nil {
		return s.audit.Close()
	}
	return nil
}

// runRevocationLoop starts a background goroutine that periodically calls
// revokeExpiredTokens at a fixed interval until the context is cancelled.
func (s *Service) runRevocationLoop(ctx context.Context) {
	ticker := time.NewTicker(revocationInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.revokeExpiredTokens(ctx)
			}
		}
	}()
}

// revokeExpiredTokens fetches all expired tokens from the tracker and revokes
// them concurrently via the GitHub API, removing each successfully revoked
// token from the tracker.
func (s *Service) revokeExpiredTokens(ctx context.Context) {
	expired := s.tokenTracker.GetExpired()
	if len(expired) == 0 || len(s.clients) == 0 {
		return
	}

	var client github.ClientIface
	for _, c := range s.clients {
		client = c
		break
	}

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(revocationConcurrency)

	for hash, token := range expired {
		group.Go(func() error {
			if err := client.RevokeToken(ctx, token); err != nil {
				s.logger.ErrorContext(ctx, "failed to revoke token", "token_hash", hash, "error", err)
				return nil
			}
			s.tokenTracker.Remove(hash)
			return nil
		})
	}

	_ = group.Wait()
}

// validateRequest checks required fields and target_repository format;
// returns an error for invalid input.
func (s *Service) validateRequest(req *ExchangeRequest) error {
	if req.OIDCToken == "" {
		return errors.New("oidc_token is required")
	}
	if req.TargetRepository == "" {
		return errors.New("target_repository is required")
	}
	if parts := strings.SplitN(
		req.TargetRepository,
		"/",
		3,
	); len(parts) != 2 || !validGitHubName.MatchString(parts[0]) ||
		!validGitHubName.MatchString(parts[1]) {
		return errors.New("target_repository must be in owner/repo format (alphanumeric, hyphens, underscores, dots only)")
	}
	if req.RequestedTTL < 0 {
		return errors.New("requested_ttl cannot be negative")
	}
	if s.maxTTL > 0 && req.RequestedTTL > s.maxTTL {
		return fmt.Errorf("requested_ttl (%d) exceeds maximum (%d)", req.RequestedTTL, s.maxTTL)
	}
	return nil
}

// auditGranted writes an audit log entry for a successful token exchange.
func (s *Service) auditGranted(
	ctx context.Context,
	requestID string,
	claims *oidc.Claims,
	req *ExchangeRequest,
	result *authorizer.Result,
	clientID, tokenHash string,
) error {
	return s.audit.Log(ctx, &audit.AuditEntry{
		RequestID:        requestID,
		Timestamp:        time.Now().Unix(),
		Caller:           claims.Subject,
		Claims:           flattenClaims(claims),
		TargetRepository: req.TargetRepository,
		PolicyName:       result.MatchedPolicy,
		Permissions:      result.EffectivePermissions,
		Outcome:          audit.OutcomeGranted,
		TokenHash:        tokenHash,
		TTL:              result.EffectiveTTL,
		GitHubClientID:   clientID,
	})
}

// auditDenied writes an audit log entry for a denied token exchange.
func (s *Service) auditDenied(ctx context.Context, requestID string, claims *oidc.Claims, req *ExchangeRequest, reason string) {
	if err := s.audit.Log(ctx, &audit.AuditEntry{
		RequestID:        requestID,
		Timestamp:        time.Now().Unix(),
		Caller:           claims.Subject,
		Claims:           flattenClaims(claims),
		TargetRepository: req.TargetRepository,
		PolicyName:       req.PolicyName,
		Outcome:          audit.OutcomeDenied,
		DenyReason:       reason,
	}); err != nil {
		s.logger.WarnContext(ctx, "audit log failed", "request_id", requestID, "error", err)
	}
}

// claimsToMap converts OIDC claims to the map[string]any format expected by the authorizer.
func claimsToMap(c *oidc.Claims) map[string]any {
	m := make(map[string]any, len(c.Custom)+4)
	m["iss"] = c.Issuer
	m["sub"] = c.Subject
	m["aud"] = c.Audience
	if !c.ExpiresAt.IsZero() {
		m["exp"] = c.ExpiresAt.Unix()
	}
	maps.Copy(m, c.Custom)
	return m
}

// flattenClaims converts OIDC claims to map[string]string for audit storage.
func flattenClaims(c *oidc.Claims) map[string]string {
	m := make(map[string]string, len(c.Custom)+2)
	m["iss"] = c.Issuer
	m["sub"] = c.Subject
	for k, v := range c.Custom {
		m[k] = fmt.Sprintf("%v", v)
	}
	return m
}

// capExpiry returns the earlier of the GitHub token expiry and the
// max expiry implied by ttlSeconds.
func capExpiry(ghExpiry time.Time, ttlSeconds int) time.Time {
	maxExpiry := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	if ghExpiry.After(maxExpiry) {
		return maxExpiry
	}
	return ghExpiry
}
