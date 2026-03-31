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

package selector

import (
	"context"
	"fmt"
	"time"
)

// Store persists and retrieves GitHub App rate limit states.
// Implementations must be safe for concurrent use.
type Store interface {
	// GetState returns the rate limit state for the app, or ErrStateNotFound if absent.
	GetState(ctx context.Context, clientID string) (*RateLimitState, error)
	// SetState updates the state for the app; stale writes may be ignored by the implementation.
	SetState(ctx context.Context, clientID string, state *RateLimitState) error
	// GetAllStates returns the current state for all known apps.
	GetAllStates(ctx context.Context) (map[string]*RateLimitState, error)
	// Close releases resources held by the store.
	Close() error
}

// App represents a GitHub App available for token issuance.
type App struct {
	ClientID     string
	Organization string
}

// RateLimitState tracks the GitHub API rate limit for a single App.
type RateLimitState struct {
	Remaining   int
	ResetAt     time.Time
	LastUpdated time.Time
}

// IsExpired returns true when the rate limit window has reset.
func (s *RateLimitState) IsExpired() bool {
	return time.Now().After(s.ResetAt)
}

// HasCapacity returns true when there are remaining API calls available.
func (s *RateLimitState) HasCapacity() bool {
	return s.Remaining > 0
}

// IsFresherThan reports whether s represents a more recent observation
// than other. A state is fresher when it belongs to a newer rate limit
// window, or to the same window but with lower remaining (more usage observed).
func (s *RateLimitState) IsFresherThan(other *RateLimitState) bool {
	if other == nil {
		return true
	}
	if s.ResetAt.After(other.ResetAt) {
		return true
	}
	if s.ResetAt.Equal(other.ResetAt) && s.Remaining < other.Remaining {
		return true
	}
	return false
}

// ExhaustedError is returned when all GitHub Apps are rate-limited.
type ExhaustedError struct {
	RetryAfter int
}

// Error returns the formatted error message with retry-after duration.
func (e *ExhaustedError) Error() string {
	return fmt.Sprintf("all GitHub Apps exhausted; retry after %ds", e.RetryAfter)
}

const (
	// DefaultRetrySeconds is the default retry-after value when no reset time is known.
	DefaultRetrySeconds = 60
	// MinRetrySeconds is the minimum retry-after value returned to clients.
	MinRetrySeconds = 1
)
