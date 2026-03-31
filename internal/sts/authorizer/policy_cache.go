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

package authorizer

import (
	"sync"
	"time"
)

const (
	// maxPolicyCacheEntries caps the number of cached policy files to limit memory.
	maxPolicyCacheEntries = 500
	// defaultPolicyCacheTTL is the default cache TTL when not specified (avoids stale policies).
	defaultPolicyCacheTTL = 5 * time.Minute
)

// policyEntry is a cached policy file.
type policyEntry struct {
	policy    *PolicyFile
	fetchedAt time.Time
}

// policyCache is a cache of policy files.
type policyCache struct {
	mu      sync.RWMutex
	entries map[string]*policyEntry
	ttl     time.Duration
}

// newPolicyCache creates a cache with the given TTL (or defaultPolicyCacheTTL if 0).
func newPolicyCache(ttl time.Duration) *policyCache {
	if ttl == 0 {
		ttl = defaultPolicyCacheTTL
	}
	return &policyCache{
		entries: make(map[string]*policyEntry),
		ttl:     ttl,
	}
}

// get returns the cached policy for the key, or nil if missing or expired.
func (c *policyCache) get(key string) *PolicyFile {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	if !ok || time.Since(e.fetchedAt) > c.ttl {
		return nil
	}
	return e.policy
}

// set stores the policy in the cache, evicting the oldest entry if at capacity.
func (c *policyCache) set(key string, p *PolicyFile) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= maxPolicyCacheEntries && c.entries[key] == nil {
		var oldestKey string
		var oldestTime time.Time
		for k, e := range c.entries {
			if oldestKey == "" || e.fetchedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = e.fetchedAt
			}
		}
		if oldestKey != "" {
			delete(c.entries, oldestKey)
		}
	}

	c.entries[key] = &policyEntry{policy: p, fetchedAt: time.Now()}
}
