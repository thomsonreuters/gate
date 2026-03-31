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

// Package selector implements GitHub App selection based on rate limit headroom.
package selector

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"time"
)

var (
	// ErrInvalidRepository is returned when the repository format is invalid.
	ErrInvalidRepository = errors.New("invalid repository format: expected owner/repo")

	// ErrNoApps is returned when no GitHub Apps are configured.
	ErrNoApps = errors.New("apps cannot be empty")

	// ErrNilStore is returned when a nil store is provided.
	ErrNilStore = errors.New("store cannot be nil")

	// ErrStateNotFound is returned when no rate limit state exists for an app.
	ErrStateNotFound = errors.New("rate limit state not found")

	// ErrNoMatchingApp is returned when no apps are configured for the target organization.
	ErrNoMatchingApp = errors.New("no GitHub App configured for organization")
)

// Selector chooses among GitHub Apps based on rate limit headroom.
type Selector struct {
	apps  []App
	store Store
}

// NewSelector creates a Selector. Returns an error if apps is empty or store is nil.
func NewSelector(apps []App, store Store) (*Selector, error) {
	if len(apps) == 0 {
		return nil, ErrNoApps
	}
	if store == nil {
		return nil, ErrNilStore
	}
	appsCopy := make([]App, len(apps))
	copy(appsCopy, apps)
	return &Selector{apps: appsCopy, store: store}, nil
}

// SelectApp returns an app with capacity for the given repository.
// Repository must be in "owner/repo" format. Returns *ExhaustedError when all
// matching apps are rate-limited.
func (s *Selector) SelectApp(ctx context.Context, repository string) (*App, error) {
	owner, err := parseOwner(repository)
	if err != nil {
		return nil, err
	}

	states, err := s.store.GetAllStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching states: %w", err)
	}

	var matching []App
	for _, app := range s.apps {
		if app.Organization == owner {
			matching = append(matching, app)
		}
	}

	if len(matching) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoMatchingApp, owner)
	}

	type candidate struct {
		app       *App
		remaining int
	}

	var candidates []candidate
	matchingStates := make(map[string]*RateLimitState, len(matching))

	for i := range matching {
		app := &matching[i]
		state, ok := states[app.ClientID]
		if ok && state != nil {
			matchingStates[app.ClientID] = state
		}
		if !ok || state == nil || state.IsExpired() {
			candidates = append(candidates, candidate{app: app, remaining: math.MaxInt})
			continue
		}
		if state.HasCapacity() {
			candidates = append(candidates, candidate{app: app, remaining: state.Remaining})
		}
	}

	if len(candidates) == 0 {
		return nil, &ExhaustedError{RetryAfter: retryAfterFromStates(matchingStates)}
	}

	maxRemaining := candidates[0].remaining
	for _, c := range candidates[1:] {
		if c.remaining > maxRemaining {
			maxRemaining = c.remaining
		}
	}

	var top []candidate
	for _, c := range candidates {
		if c.remaining == maxRemaining {
			top = append(top, c)
		}
	}

	return top[rand.IntN(len(top))].app, nil //nolint:gosec // G404: non-security load-balancing tie-break, cryptographic randomness not required
}

// RecordUsage updates the rate limit state for an app after an API call.
func (s *Selector) RecordUsage(ctx context.Context, clientID string, remaining int, resetAt time.Time) error {
	state := &RateLimitState{
		Remaining:   remaining,
		ResetAt:     resetAt,
		LastUpdated: time.Now(),
	}
	if err := s.store.SetState(ctx, clientID, state); err != nil {
		return fmt.Errorf("recording usage: %w", err)
	}
	return nil
}

// RetryAfter returns the seconds until the earliest app window resets.
// Clamped to [MinRetrySeconds, DefaultRetrySeconds].
func (s *Selector) RetryAfter(ctx context.Context) int {
	states, err := s.store.GetAllStates(ctx)
	if err != nil {
		return DefaultRetrySeconds
	}
	return retryAfterFromStates(states)
}

// retryAfterFromStates returns seconds until the earliest non-expired
// reset time, clamped to [MinRetrySeconds, DefaultRetrySeconds].
func retryAfterFromStates(states map[string]*RateLimitState) int {
	now := time.Now()
	var earliest time.Time
	for _, state := range states {
		if state != nil && !state.IsExpired() && state.ResetAt.After(now) {
			if earliest.IsZero() || state.ResetAt.Before(earliest) {
				earliest = state.ResetAt
			}
		}
	}

	if earliest.IsZero() {
		return DefaultRetrySeconds
	}

	secs := int(time.Until(earliest).Seconds())
	if secs < MinRetrySeconds {
		return MinRetrySeconds
	}
	if secs > DefaultRetrySeconds {
		return DefaultRetrySeconds
	}
	return secs
}

// parseOwner extracts the owner (first segment) from "owner/repo";
// returns ErrInvalidRepository if format is invalid.
func parseOwner(repository string) (string, error) {
	parts := strings.SplitN(repository, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ErrInvalidRepository
	}
	return parts[0], nil
}
