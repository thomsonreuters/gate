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
	clientID      string
	key           *rsa.PrivateKey
	baseURL       string
	httpClient    *http.Client
	installations *installationCache
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
	parts := strings.Split(req.Repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("%w: %s", ErrInvalidRepository, req.Repository)
	}
	owner, repo := parts[0], parts[1]

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
		// which achieves the same goal as revocation.
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil
		}
		return fmt.Errorf("revoking installation token: %w", err)
	}
	return nil
}

func (c *Client) GetContents(ctx context.Context, repository string, path string) ([]byte, error) {
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("%w: %s", ErrInvalidRepository, repository)
	}
	owner, repo := parts[0], parts[1]

	id, err := c.getInstallationID(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("getting installation ID: %w", err)
	}

	token, err := c.createInstallationToken(ctx, id, repo, map[string]string{"contents": "read"})
	if err != nil {
		return nil, fmt.Errorf("getting contents token: %w", err)
	}

	gh, err := c.ghClient(token.Token)
	if err != nil {
		return nil, err
	}

	fileContent, _, resp, err := gh.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
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

// getInstallationID returns the app installation ID for owner/repo, using cache when available.
func (c *Client) getInstallationID(ctx context.Context, owner, repo string) (int64, error) {
	key := owner + "/" + repo

	if id := c.installations.get(key); id != 0 {
		return id, nil
	}

	appJWT, err := c.generateAppJWT()
	if err != nil {
		return 0, fmt.Errorf("generating app JWT: %w", err)
	}

	gh, err := c.ghClient(appJWT)
	if err != nil {
		return 0, err
	}

	installation, resp, err := gh.Apps.FindRepositoryInstallation(ctx, owner, repo)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return 0, fmt.Errorf("%w: %s/%s", ErrRepositoryNotFound, owner, repo)
		}
		return 0, fmt.Errorf("fetching installation ID: %w", err)
	}

	c.installations.set(key, installation.GetID())
	return installation.GetID(), nil
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

	token, _, err := gh.Apps.CreateInstallationToken(ctx, id, opts)
	if err != nil {
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
		clientID:      opts.ClientID,
		key:           key,
		baseURL:       strings.TrimRight(opts.BaseURL, "/"),
		httpClient:    &http.Client{Timeout: opts.Timeout, Transport: transport},
		installations: newInstallationCache(),
	}, nil
}
