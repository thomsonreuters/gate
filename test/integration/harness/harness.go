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

// Package harness provides a test harness for integration testing the STS token exchange:
// mock GitHub and OIDC servers, HTTP client, and helpers to build contexts and run exchanges.
package harness

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thomsonreuters/gate/internal/testutil"
)

const (
	// DefaultRepo is the default target repository for harness tests.
	DefaultRepo = "example-org/example-repo"
	// DefaultInstallationID is the default GitHub App installation ID used in mock responses.
	DefaultInstallationID = int64(123456)
	// DefaultPolicyTemplate is the default trust policy fixture name under fixtures/policies.
	DefaultPolicyTemplate = "contents_read_metadata_read.tpl.yaml"
)

// Context holds test state and provides methods for test execution.
// Cleanup is registered automatically via t.Cleanup — callers never
// need to call Close or Cleanup manually.
type Context struct {
	t *testing.T

	GitHub *GitHub
	OIDC   *testutil.OIDCServer
	Client *Client
}

// New creates a new test Context with mock servers initialized.
// Cleanup is automatic via t.Cleanup.
func New(t *testing.T) *Context {
	t.Helper()

	key := testutil.GenerateRSAKeyObject(t)

	oidc := testutil.NewOIDCServer(t, key)
	gh := newGitHub(t)

	t.Cleanup(func() {
		oidc.Close()
		gh.Close()
	})

	return &Context{
		t:      t,
		GitHub: gh,
		OIDC:   oidc,
	}
}

// SignTokenWithClaims returns a signed OIDC token with the given claims.
func (c *Context) SignTokenWithClaims(claims map[string]any) string {
	c.t.Helper()
	return c.OIDC.SignTokenWithClaims(c.t, claims)
}

// IssuerURL returns the OIDC issuer URL for the test context.
func (c *Context) IssuerURL() string {
	return c.OIDC.URL
}

// GitHubAPIURL returns the mock GitHub API base URL.
func (c *Context) GitHubAPIURL() string {
	return c.GitHub.APIURL()
}

// FixturePath returns the path to a fixture file under the test
// fixtures directory (relative to cwd).
func (c *Context) FixturePath(parts ...string) string {
	c.t.Helper()

	base, err := os.Getwd()
	if err != nil {
		c.t.Fatalf("failed to get working directory: %v", err)
	}

	elements := append([]string{base, "fixtures"}, parts...)
	return filepath.Join(elements...)
}

// LoadTemplate reads a fixture file under fixtures/ and substitutes
// {{ISSUER_URL}} with the context issuer URL.
func (c *Context) LoadTemplate(parts ...string) string {
	c.t.Helper()

	path := c.FixturePath(parts...)
	// #nosec G304 -- test fixture loading from controlled paths
	content, err := os.ReadFile(path)
	if err != nil {
		c.t.Fatalf("failed to read template %s: %v", path, err)
	}

	result := string(content)
	result = strings.ReplaceAll(result, "{{ISSUER_URL}}", c.IssuerURL())
	return result
}

// DefaultClaims returns standard OIDC claims for the context
// (iss, sub, aud, exp, repository, ref, etc.).
func (c *Context) DefaultClaims() map[string]any {
	now := time.Now()
	return map[string]any{
		"iss":        c.IssuerURL(),
		"sub":        "repo:" + DefaultRepo + ":ref:refs/heads/main",
		"aud":        c.IssuerURL(),
		"exp":        now.Add(1 * time.Hour).Unix(),
		"iat":        now.Unix(),
		"nbf":        now.Unix(),
		"repository": DefaultRepo,
		"ref":        "refs/heads/main",
	}
}

// ClaimsWith returns DefaultClaims with the given overrides merged in.
func (c *Context) ClaimsWith(overrides map[string]any) map[string]any {
	claims := c.DefaultClaims()
	maps.Copy(claims, overrides)
	return claims
}

// DefaultToken returns a signed OIDC token with DefaultClaims.
func (c *Context) DefaultToken() string {
	return c.SignTokenWithClaims(c.DefaultClaims())
}

// TokenWith returns a signed OIDC token with DefaultClaims merged with overrides.
func (c *Context) TokenWith(overrides map[string]any) string {
	return c.SignTokenWithClaims(c.ClaimsWith(overrides))
}

// SetupPolicy sets the trust policy and installation ID for the given
// repo using the named template fixture.
func (c *Context) SetupPolicy(repo, template string) {
	c.t.Helper()
	c.GitHub.SetPolicy(repo, c.LoadTemplate("policies", template))
	c.GitHub.SetInstallation(repo, DefaultInstallationID)
}

// SetupDefaultPolicy calls SetupPolicy with DefaultRepo and DefaultPolicyTemplate.
func (c *Context) SetupDefaultPolicy() {
	c.t.Helper()
	c.SetupPolicy(DefaultRepo, DefaultPolicyTemplate)
}

// ExchangeDefault performs an exchange request with DefaultToken and DefaultRepo.
func (c *Context) ExchangeDefault() (*Result, error) {
	return c.Client.Exchange(c.t.Context(), &ExchangeRequest{
		OIDCToken:        c.DefaultToken(),
		TargetRepository: DefaultRepo,
	})
}
