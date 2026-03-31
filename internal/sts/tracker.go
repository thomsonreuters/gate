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

package sts

import (
	"sync"
	"time"
)

type tokenEntry struct {
	token   string
	expires time.Time
}

// TokenTracker tracks issued tokens in memory keyed by token hash.
type TokenTracker struct {
	mu      sync.RWMutex
	entries map[string]*tokenEntry
}

// NewTokenTracker creates an empty tracker.
func NewTokenTracker() *TokenTracker {
	return &TokenTracker{
		entries: make(map[string]*tokenEntry),
	}
}

// Record stores a token keyed by its hash with the given expiry.
func (c *TokenTracker) Record(hash, token string, expires time.Time) {
	c.mu.Lock()
	c.entries[hash] = &tokenEntry{token: token, expires: expires}
	c.mu.Unlock()
}

// GetExpired returns all entries whose expiry has passed without removing them.
func (c *TokenTracker) GetExpired() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	expired := make(map[string]string)
	for hash, entry := range c.entries {
		if now.After(entry.expires) {
			expired[hash] = entry.token
		}
	}
	return expired
}

// Remove deletes a single hash from the tracker.
func (c *TokenTracker) Remove(hash string) {
	c.mu.Lock()
	delete(c.entries, hash)
	c.mu.Unlock()
}
