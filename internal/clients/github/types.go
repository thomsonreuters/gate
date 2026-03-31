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

package github

import (
	"sync"
	"time"
)

// TokenRequest represents a request for an installation access token.
type TokenRequest struct {
	Repository  string
	Permissions map[string]string
}

// TokenResponse represents the GitHub API response for an installation access token.
type TokenResponse struct {
	Token     string
	ExpiresAt time.Time
}

// RateLimitInfo contains rate limit information from the GitHub API.
type RateLimitInfo struct {
	Remaining int
	ResetAt   time.Time
}

const (
	// maxInstallationCacheEntries caps the in-memory installation ID cache size.
	maxInstallationCacheEntries = 1000
	// installationCacheTTL evicts entries to avoid stale IDs after GitHub App scope changes.
	installationCacheTTL = 24 * time.Hour
)

type installationCache struct {
	mu      sync.RWMutex
	entries map[string]installationEntry
}

type installationEntry struct {
	id      int64
	fetched time.Time
}

// newInstallationCache creates an empty installation ID cache.
func newInstallationCache() *installationCache {
	return &installationCache{
		entries: make(map[string]installationEntry),
	}
}

// get retrieves an installation ID from the cache.
// Returns 0 if the key is not found or the entry has expired.
func (c *installationCache) get(key string) int64 {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return 0
	}

	if time.Since(e.fetched) > installationCacheTTL {
		c.mu.Lock()
		if re, ok := c.entries[key]; ok && time.Since(re.fetched) > installationCacheTTL {
			delete(c.entries, key)
		}
		c.mu.Unlock()
		return 0
	}

	return e.id
}

// set stores an installation ID in the cache.
// Evicts the oldest entry when at capacity.
func (c *installationCache) set(key string, id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= maxInstallationCacheEntries && c.entries[key].id == 0 {
		var oldest string
		var oldestTime time.Time
		for k, e := range c.entries {
			if oldest == "" || e.fetched.Before(oldestTime) {
				oldest = k
				oldestTime = e.fetched
			}
		}
		if oldest != "" {
			delete(c.entries, oldest)
		}
	}

	c.entries[key] = installationEntry{
		id:      id,
		fetched: time.Now(),
	}
}
