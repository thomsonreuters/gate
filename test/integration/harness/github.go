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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// GitHub is a mock GitHub API server for testing.
// It supports policy file serving, installation lookup, and token creation.
type GitHub struct {
	t      *testing.T
	server *httptest.Server

	mu            sync.RWMutex
	policies      map[string]policy  // repo -> policy content
	installations map[string]int64   // repo -> installation ID
	tokens        map[int64]token    // installation ID -> token config
	errors        map[string]failure // path pattern -> error to return
	requests      []Request          // recorded requests
	latency       time.Duration      // simulated latency
}

// policy holds policy file content and metadata.
type policy struct {
	content  string
	path     string
	encoding string
}

// token holds token creation configuration.
type token struct {
	value       string
	permissions map[string]string
	expiresAt   time.Time
}

// failure holds error simulation configuration.
type failure struct {
	status     int
	message    string
	retryAfter int
}

// Request represents a recorded HTTP request.
type Request struct {
	Method string
	Path   string
	Time   time.Time
}

// newGitHub creates a new mock GitHub server.
func newGitHub(t *testing.T) *GitHub {
	t.Helper()

	g := &GitHub{
		t:             t,
		policies:      make(map[string]policy),
		installations: make(map[string]int64),
		tokens:        make(map[int64]token),
		errors:        make(map[string]failure),
		requests:      make([]Request, 0),
	}

	g.server = httptest.NewServer(http.HandlerFunc(g.handler))
	return g
}

// Close shuts down the mock server and releases all resources.
// This method is safe to call multiple times.
func (g *GitHub) Close() {
	if g.server != nil {
		g.server.Close()
	}
}

// APIURL returns the base URL of the mock GitHub API server.
// Use this URL to configure the STS service to use the mock GitHub.
func (g *GitHub) APIURL() string {
	return g.server.URL
}

// SetPolicy configures a policy file response for a repository.
func (g *GitHub) SetPolicy(repo, content string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.policies[repo] = policy{
		content:  content,
		path:     ".github/trust-policy.yaml",
		encoding: "base64",
	}
}

// SetPolicyFromFile loads a policy from a fixture file.
func (g *GitHub) SetPolicyFromFile(repo, path string) {
	g.t.Helper()

	// #nosec G304 -- test fixture loading from controlled paths
	content, err := os.ReadFile(path)
	if err != nil {
		g.t.Fatalf("failed to read policy fixture %s: %v", path, err)
	}

	g.SetPolicy(repo, string(content))
}

// SetInstallation configures the installation ID for a repository.
func (g *GitHub) SetInstallation(repo string, id int64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.installations[repo] = id
}

// SetToken configures the token response for an installation.
func (g *GitHub) SetToken(id int64, value string, permissions map[string]string, expiresAt time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.tokens[id] = token{
		value:       value,
		permissions: permissions,
		expiresAt:   expiresAt,
	}
}

// SetError configures an error response for a path pattern.
// The pattern matches if the request path contains the pattern string.
func (g *GitHub) SetError(pattern string, status int, message string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.errors[pattern] = failure{
		status:  status,
		message: message,
	}
}

// SetLatency configures simulated network latency for all requests.
// This is useful for testing timeout behavior and slow network conditions.
func (g *GitHub) SetLatency(d time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.latency = d
}

// SetRateLimit configures a rate limit error for a path pattern.
// The rate limit response includes a Retry-After header with the specified value.
func (g *GitHub) SetRateLimit(pattern string, retryAfter int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.errors[pattern] = failure{
		status:     http.StatusForbidden,
		message:    "API rate limit exceeded",
		retryAfter: retryAfter,
	}
}

// RequestCount returns the total number of requests received by the mock server.
func (g *GitHub) RequestCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return len(g.requests)
}

// WasRequested checks if any request path contains the specified substring.
// Use this to verify that expected API endpoints were called.
func (g *GitHub) WasRequested(path string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, r := range g.requests {
		if strings.Contains(r.Path, path) {
			return true
		}
	}
	return false
}

// handler is the HTTP handler for the mock server.
func (g *GitHub) handler(w http.ResponseWriter, r *http.Request) {
	g.recordRequest(r)

	g.mu.RLock()
	latency := g.latency
	g.mu.RUnlock()

	if latency > 0 {
		time.Sleep(latency)
	}

	if g.checkError(w, r) {
		return
	}

	path := r.URL.Path

	switch {
	case strings.HasPrefix(path, "/repos/") && strings.Contains(path, "/contents/"):
		g.handleContents(w, r)
	case strings.HasPrefix(path, "/repos/") && strings.HasSuffix(path, "/installation"):
		g.handleInstallation(w, r)
	case strings.HasPrefix(path, "/app/installations/") && strings.HasSuffix(path, "/access_tokens"):
		g.handleAccessToken(w, r)
	default:
		g.writeError(w, http.StatusNotFound, "Not Found")
	}
}

// recordRequest saves the request for later inspection.
func (g *GitHub) recordRequest(r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.requests = append(g.requests, Request{
		Method: r.Method,
		Path:   r.URL.Path,
		Time:   time.Now(),
	})
}

// checkError returns true if an error was configured for this path.
func (g *GitHub) checkError(w http.ResponseWriter, r *http.Request) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for pattern, fail := range g.errors {
		if strings.Contains(r.URL.Path, pattern) {
			if fail.retryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(fail.retryAfter))
			}
			g.writeError(w, fail.status, fail.message)
			return true
		}
	}
	return false
}

// handleContents handles GET /repos/{owner}/{repo}/contents/{path} requests.
func (g *GitHub) handleContents(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/repos/"), "/")
	if len(parts) < 4 {
		g.writeError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	repo := fmt.Sprintf("%s/%s", parts[0], parts[1])

	g.mu.RLock()
	p, ok := g.policies[repo]
	g.mu.RUnlock()

	if !ok {
		g.writeError(w, http.StatusNotFound, "File not found")
		return
	}

	content := base64.StdEncoding.EncodeToString([]byte(p.content))

	response := map[string]any{
		"name":     "trust-policy.yaml",
		"path":     p.path,
		"sha":      "abc123",
		"size":     len(p.content),
		"encoding": "base64",
		"content":  content,
		"type":     "file",
	}

	g.writeJSON(w, http.StatusOK, response)
}

// handleInstallation handles GET /repos/{owner}/{repo}/installation requests.
func (g *GitHub) handleInstallation(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/repos/"), "/installation")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		g.writeError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	repo := fmt.Sprintf("%s/%s", parts[0], parts[1])

	g.mu.RLock()
	id, ok := g.installations[repo]
	g.mu.RUnlock()

	if !ok {
		g.writeError(w, http.StatusNotFound, "Installation not found")
		return
	}

	response := map[string]any{
		"id":         id,
		"account":    map[string]string{"login": parts[0]},
		"repository": map[string]string{"full_name": repo},
	}

	g.writeJSON(w, http.StatusOK, response)
}

// handleAccessToken handles POST /app/installations/{id}/access_tokens requests.
func (g *GitHub) handleAccessToken(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/app/installations/")
	path = strings.TrimSuffix(path, "/access_tokens")

	var id int64
	if _, err := fmt.Sscanf(path, "%d", &id); err != nil {
		g.writeError(w, http.StatusBadRequest, "Invalid installation ID")
		return
	}

	g.mu.RLock()
	mock, ok := g.tokens[id]
	g.mu.RUnlock()

	if !ok {
		// Return a default token if not configured
		mock = token{
			value:       fmt.Sprintf("ghs_test_token_%d", id),
			permissions: map[string]string{"contents": "read"},
			expiresAt:   time.Now().Add(1 * time.Hour),
		}
	}

	response := map[string]any{
		"token":       mock.value,
		"expires_at":  mock.expiresAt.Format(time.RFC3339),
		"permissions": mock.permissions,
	}

	g.writeJSON(w, http.StatusCreated, response)
}

// writeJSON writes a JSON response.
func (g *GitHub) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		g.t.Logf("failed to encode JSON response: %v", err)
	}
}

// writeError writes a JSON error response.
func (g *GitHub) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": message,
	}); err != nil {
		g.t.Logf("failed to encode JSON error: %v", err)
	}
}
