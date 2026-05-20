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
	tokens          *tokenCache
	tokenMintSF     singleflight.Group
	tokenReadyDelay time.Duration
}

// mintOpts controls mintToken behavior.
type mintOpts struct {
	skipCache bool // skip cache and mint a fresh token
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

	id, err := c.getInstallationID(ctx, owner)
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

	perms := map[string]string{"contents": "read"}
	var opts *mintOpts

	// Retry once on 401/403 in case the token was stale; the second attempt mints fresh.
	for attempt := range 2 {
		tok, err := c.mintToken(ctx, owner, "", perms, opts)
		if err != nil {
			return nil, err
		}

		gh, err := c.ghClient(tok)
		if err != nil {
			return nil, err
		}

		fileContent, _, resp, err := gh.Repositories.GetContents(ctx, owner, repo, path, nil)
		if err != nil {
			if attempt == 0 && resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
				opts = &mintOpts{skipCache: true}
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
	return nil, fmt.Errorf("unexpected: exhausted GetContents retries for %s/%s", repository, path)
}

// mintToken returns a cached or freshly minted installation token.
// When repo is non-empty the token is scoped to that single repository;
// when empty the token covers every repo the installation can access.
// Tokens are keyed by '<owner>[/<repo>]'+'|<permissions>'.
func (c *Client) mintToken(ctx context.Context, owner, repo string, perms map[string]string, opts *mintOpts) (string, error) {
	key := owner
	if repo != "" {
		key = owner + "/" + repo
	}
	key += "|" + permissionsKey(perms)

	skipCache := opts != nil && opts.skipCache

	if skipCache {
		c.tokens.delete(key)
	} else if tok := c.tokens.get(key); tok != "" {
		return tok, nil
	}

	v, err, _ := c.tokenMintSF.Do(key, func() (any, error) {
		if !skipCache {
			if tok := c.tokens.get(key); tok != "" {
				return tok, nil
			}
		}

		id, err := c.getInstallationID(ctx, owner)
		if err != nil {
			return nil, fmt.Errorf("getting installation ID: %w", err)
		}

		resp, err := c.createInstallationToken(ctx, id, repo, perms)
		if err != nil {
			return nil, fmt.Errorf("minting installation token: %w", err)
		}

		if err := c.waitTokenReady(ctx); err != nil {
			return nil, err
		}

		c.tokens.set(key, resp.Token, resp.ExpiresAt)
		return resp.Token, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil //nolint:errcheck // singleflight guarantees string on nil error
}

// getInstallationID returns the app installation ID for the given owner
// (organization or user account). Cache and singleflight are keyed by owner.
func (c *Client) getInstallationID(ctx context.Context, owner string) (int64, error) {
	if id := c.installations.get(owner); id != 0 {
		return id, nil
	}

	v, err, _ := c.installationSF.Do(owner, func() (any, error) {
		if id := c.installations.get(owner); id != 0 {
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

		// Try organization first, fall back to user on 404.
		installation, resp, err := gh.Apps.FindOrganizationInstallation(ctx, owner)
		if err != nil && resp != nil && resp.StatusCode == http.StatusNotFound {
			installation, resp, err = gh.Apps.FindUserInstallation(ctx, owner)
		}
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("%w: %s", ErrInstallationNotFound, owner)
			}
			return nil, fmt.Errorf("fetching installation ID: %w", err)
		}

		c.installations.set(owner, installation.GetID())
		return installation.GetID(), nil
	})
	if err != nil {
		return 0, err
	}
	return v.(int64), nil //nolint:errcheck // singleflight guarantees int64 on nil error
}

// createInstallationToken creates an installation access token with the
// given permissions. When repository is non-empty the token is scoped to
// that single repo; when empty it covers all repos the installation can access.
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

	opts := &gogithub.InstallationTokenOptions{
		Permissions: installationPermissions,
	}
	if repository != "" {
		opts.Repositories = []string{repository}
	}

	token, resp, err := gh.Apps.CreateInstallationToken(ctx, id, opts)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity) {
			if repository != "" {
				return nil, fmt.Errorf("%w: %s", ErrRepositoryNotFound, repository)
			}
			return nil, fmt.Errorf("%w (installation %d)", ErrInstallationNotFound, id)
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
		tokens:          newTokenCache(),
		tokenReadyDelay: DefaultTokenReadyDelay,
	}, nil
}
