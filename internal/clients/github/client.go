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

// Package github provides a client for interacting with the GitHub API
// to generate installation access tokens for GitHub Apps.
//
// The client uses google/go-github for typed API calls, handles GitHub App
// JWT authentication, and caches installation IDs to minimize API calls.
package github

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	gogithub "github.com/google/go-github/v83/github"
	"github.com/thomsonreuters/gate/internal/utils"
	"golang.org/x/sync/singleflight"
)

const (
	// jwtExpiry is the JWT exp claim; GitHub accepts up to 10 minutes.
	jwtExpiry = 10 * time.Minute
	// jwtClockSkew backdates iat to allow for clock drift between this service and GitHub.
	jwtClockSkew = 5 * time.Second
)

// ClientIface defines the GitHub App client operations for token issuance and repository access.
// Implementations must be safe for concurrent use.
type ClientIface interface {
	// RequestToken requests an installation access token scoped to
	// the given repository and permissions.
	// req.Repository must be in "owner/repo" format. Returns error for invalid repo or API failure.
	RequestToken(ctx context.Context, req *TokenRequest) (*TokenResponse, error)
	// RevokeToken revokes an installation access token, rendering it unusable.
	// The token parameter is used to authenticate the DELETE /installation/token call.
	RevokeToken(ctx context.Context, token string) error
	// RateLimit returns current rate limit status. The /rate_limit endpoint does not consume quota.
	RateLimit(ctx context.Context, token string) (RateLimitInfo, error)
	// GetContents fetches file content from repository at path. Repository must be "owner/repo".
	// Returns error if the path is not found, is a directory, or the API call fails.
	GetContents(ctx context.Context, repository string, path string) ([]byte, error)
}

// Client manages GitHub API interactions for token generation.
type Client struct {
	clientID        string
	key             *rsa.PrivateKey
	baseURL         string
	httpClient      *http.Client
	installations   *installationCache
	installationSF  singleflight.Group
	contentsTokens  *tokenCache
	tokenMintSF     singleflight.Group
	tokenReadyDelay time.Duration
}

// ghClient returns a go-github client authenticated with the given token.
func (c *Client) ghClient(token string) (*gogithub.Client, error) {
	gh := gogithub.NewClient(c.httpClient).WithAuthToken(token)
	if c.baseURL != DefaultBaseURL {
		u, err := url.Parse(strings.TrimRight(c.baseURL, "/") + "/")
		if err != nil {
			return nil, fmt.Errorf("parsing base URL %q: %w", c.baseURL, err)
		}
		gh.BaseURL = u
	}
	return gh, nil
}

func (c *Client) RequestToken(ctx context.Context, req *TokenRequest) (*TokenResponse, error) {
	owner, repo, err := parseRepository(req.Repository)
	if err != nil {
		return nil, err
	}

	id, err := c.getInstallationID(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("getting installation ID: %w", err)
	}

	token, err := c.createInstallationToken(ctx, id, repo, req.Permissions)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func (c *Client) RateLimit(ctx context.Context, token string) (RateLimitInfo, error) {
	gh, err := c.ghClient(token)
	if err != nil {
		return RateLimitInfo{}, err
	}

	limits, _, err := gh.RateLimit.Get(ctx)
	if err != nil {
		return RateLimitInfo{}, fmt.Errorf("querying rate limit: %w", err)
	}

	if limits.Core == nil {
		return RateLimitInfo{}, nil
	}

	return RateLimitInfo{
		Remaining: limits.Core.Remaining,
		ResetAt:   limits.Core.Reset.Time,
	}, nil
}

func (c *Client) RevokeToken(ctx context.Context, token string) error {
	gh, err := c.ghClient(token)
	if err != nil {
		return err
	}
	resp, err := gh.Apps.RevokeInstallationToken(ctx)
	if err != nil {
		// 401 means the token is already invalid (expired or revoked),
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil
		}
		return fmt.Errorf("revoking installation token: %w", err)
	}
	return nil
}

func (c *Client) GetContents(ctx context.Context, repository string, path string) ([]byte, error) {
	owner, repo, err := parseRepository(repository)
	if err != nil {
		return nil, err
	}

	// Retry once: if a cached token yields 401/403, evict it and mint a fresh one.
	for attempt := range 2 {
		tok, cached, err := c.contentsToken(ctx, repository, owner, repo)
		if err != nil {
			return nil, err
		}

		gh, err := c.ghClient(tok)
		if err != nil {
			return nil, err
		}

		fileContent, _, resp, err := gh.Repositories.GetContents(ctx, owner, repo, path, nil)
		if err != nil {
			if attempt == 0 && cached && resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
				c.contentsTokens.delete(repository)
				continue
			}
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("%w: %s/%s", ErrFileNotFound, repository, path)
			}
			return nil, fmt.Errorf("fetching contents: %w", err)
		}
		if fileContent == nil {
			return nil, fmt.Errorf("%w: %s/%s is a directory", ErrFileNotFound, repository, path)
		}

		content, err := fileContent.GetContent()
		if err != nil {
			return nil, fmt.Errorf("decoding content: %w", err)
		}
		return []byte(content), nil
	}
	return nil, errors.New("unexpected: exhausted GetContents retry")
}

// contentsToken returns a repo-scoped contents:read token, serving from
// cache when possible. On a cache miss, concurrent callers for the same
// repository are coalesced so only one mint + replication delay occurs.
// The bool indicates whether the token was served from cache.
func (c *Client) contentsToken(ctx context.Context, repository, owner, repo string) (string, bool, error) {
	if tok := c.contentsTokens.get(repository); tok != "" {
		return tok, true, nil
	}

	perms := map[string]string{"contents": "read"}
	tok, err := c.mintToken(ctx, repository, owner, repo, perms, c.contentsTokens)
	if err != nil {
		return "", false, err
	}
	return tok, false, nil
}

// mintToken creates a new installation token with the given permissions,
// waits for edge replication, and stores the result in cache. Concurrent
// calls for the same repository+permissions share a single in-flight
// mint via singleflight.
func (c *Client) mintToken(ctx context.Context, repository, owner, repo string, perms map[string]string, cache *tokenCache) (string, error) {
	sfKey := repository + "|" + permissionsKey(perms)

	v, err, _ := c.tokenMintSF.Do(sfKey, func() (any, error) {
		if tok := cache.get(repository); tok != "" {
			return tok, nil
		}

		id, err := c.getInstallationID(ctx, owner, repo)
		if err != nil {
			return nil, fmt.Errorf("getting installation ID: %w", err)
		}

		token, err := c.createInstallationToken(ctx, id, repo, perms)
		if err != nil {
			return nil, fmt.Errorf("minting installation token: %w", err)
		}

		if err := c.waitTokenReady(ctx); err != nil {
			return nil, err
		}

		cache.set(repository, token.Token, token.ExpiresAt)
		return token.Token, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil //nolint:errcheck // singleflight guarantees string on nil error
}

// getInstallationID returns the app installation ID for owner. Installations
// are per-account, so cache and singleflight are keyed by owner; repo is
// only used for the API lookup on cache miss.
func (c *Client) getInstallationID(ctx context.Context, owner, repo string) (int64, error) {
	key := owner

	if id := c.installations.get(key); id != 0 {
		return id, nil
	}

	v, err, _ := c.installationSF.Do(key, func() (any, error) {
		if id := c.installations.get(key); id != 0 {
			return id, nil
		}

		appJWT, err := c.generateAppJWT()
		if err != nil {
			return nil, fmt.Errorf("generating app JWT: %w", err)
		}

		gh, err := c.ghClient(appJWT)
		if err != nil {
			return nil, err
		}

		installation, resp, err := gh.Apps.FindRepositoryInstallation(ctx, owner, repo)
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("%w: %s/%s", ErrRepositoryNotFound, owner, repo)
			}
			return nil, fmt.Errorf("fetching installation ID: %w", err)
		}

		c.installations.set(key, installation.GetID())
		return installation.GetID(), nil
	})
	if err != nil {
		return 0, err
	}
	return v.(int64), nil //nolint:errcheck // singleflight guarantees int64 on nil error
}

// createInstallationToken creates a repository-scoped installation
// access token with the given permissions.
func (c *Client) createInstallationToken(ctx context.Context, id int64, repository string, permissions map[string]string) (*TokenResponse, error) {
	appJWT, err := c.generateAppJWT()
	if err != nil {
		return nil, fmt.Errorf("generating app JWT: %w", err)
	}

	gh, err := c.ghClient(appJWT)
	if err != nil {
		return nil, err
	}

	installationPermissions, err := c.toInstallationPermissions(permissions)
	if err != nil {
		return nil, fmt.Errorf("converting permissions: %w", err)
	}

	if repository == "" {
		return nil, ErrRepositoryRequired
	}

	opts := &gogithub.InstallationTokenOptions{
		Permissions:  installationPermissions,
		Repositories: []string{repository},
	}

	token, resp, err := gh.Apps.CreateInstallationToken(ctx, id, opts)
	if err != nil {
		// 404 (no installation) and 422 (repo outside installation scope)
		// match getInstallationID's ErrRepositoryNotFound contract.
		if resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity) {
			return nil, fmt.Errorf("%w: %s", ErrRepositoryNotFound, repository)
		}
		return nil, fmt.Errorf("creating installation token: %w", err)
	}

	return &TokenResponse{
		Token:     token.GetToken(),
		ExpiresAt: token.GetExpiresAt().Time,
	}, nil
}

// generateAppJWT creates an RS256 JWT for GitHub App authentication.
// The iat claim is backdated by 5 seconds to compensate for clock skew.
func (c *Client) generateAppJWT() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Add(-jwtClockSkew).Unix(),
		"exp": now.Add(jwtExpiry).Unix(),
		"iss": c.clientID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(c.key)
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}

	return tokenString, nil
}

// toInstallationPermissions converts a map[string]string to the go-github
// InstallationPermissions struct via JSON roundtrip. The map keys match the
// struct's JSON tags (e.g. "contents", "pull_requests"), and unknown keys
// are silently ignored.
func (c *Client) toInstallationPermissions(m map[string]string) (*gogithub.InstallationPermissions, error) {
	if len(m) == 0 {
		return nil, nil //nolint:nilnil // nil permissions intentionally means "no specific permissions"
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshaling permissions: %w", err)
	}
	p := &gogithub.InstallationPermissions{}
	if err := json.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("unmarshaling permissions: %w", err)
	}
	return p, nil
}

// waitTokenReady pauses for tokenReadyDelay to allow a newly minted
// installation token to replicate across GitHub's edge infrastructure.
func (c *Client) waitTokenReady(ctx context.Context) error {
	if c.tokenReadyDelay <= 0 {
		return nil
	}
	t := time.NewTimer(c.tokenReadyDelay)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("waiting for token replication: %w", ctx.Err())
	}
}

// New is the constructor for the GitHub client. It is a package-level
// variable so tests can replace it.
var New = newClient

// newClient builds a Client from options, applying defaults for BaseURL and Timeout.
func newClient(opts Options) (ClientIface, error) {
	if opts.BaseURL == "" {
		opts.BaseURL = DefaultBaseURL
	}
	if opts.Timeout == 0 {
		opts.Timeout = DefaultTimeout
	}

	key, err := utils.ParseRSAPrivateKey(opts.PrivateKey)
	if err != nil {
		return nil, err
	}

	transport := &retryTransport{
		next: http.DefaultTransport,
		cfg:  DefaultRetryConfig,
	}

	return &Client{
		clientID:        opts.ClientID,
		key:             key,
		baseURL:         strings.TrimRight(opts.BaseURL, "/"),
		httpClient:      &http.Client{Timeout: opts.Timeout, Transport: transport},
		installations:   newInstallationCache(),
		contentsTokens:  newTokenCache(),
		tokenReadyDelay: DefaultTokenReadyDelay,
	}, nil
}
