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

//go:build integration

package selector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	domain "github.com/thomsonreuters/gate/internal/sts/selector"
)

func rateLimitState(remaining int, reset time.Duration) *domain.RateLimitState {
	return &domain.RateLimitState{
		Remaining:   remaining,
		ResetAt:     time.Now().Add(reset).Truncate(time.Second),
		LastUpdated: time.Now().Truncate(time.Second),
	}
}

// storeLifecycle exercises the full get/set/update/getAll cycle on any Store.
func storeLifecycle(t *testing.T, store domain.Store) {
	t.Helper()
	ctx := t.Context()

	_, err := store.GetState(ctx, "test-app")
	require.ErrorIs(t, err, domain.ErrStateNotFound)

	state := rateLimitState(42, time.Hour)
	require.NoError(t, store.SetState(ctx, "test-app", state))

	got, err := store.GetState(ctx, "test-app")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 42, got.Remaining)
	assert.Equal(t, state.ResetAt.Unix(), got.ResetAt.Unix())
}

// storeConditionalWrite verifies that stale writes are discarded
// while fresher writes (lower remaining, same window) win.
func storeConditionalWrite(t *testing.T, store domain.Store) {
	t.Helper()
	ctx := t.Context()

	reset := time.Now().Add(time.Hour).Truncate(time.Second)

	fresh := &domain.RateLimitState{
		Remaining:   100,
		ResetAt:     reset,
		LastUpdated: time.Now().Truncate(time.Second),
	}
	require.NoError(t, store.SetState(ctx, "cas-app", fresh))

	lower := &domain.RateLimitState{
		Remaining:   50,
		ResetAt:     reset,
		LastUpdated: time.Now().Truncate(time.Second),
	}
	require.NoError(t, store.SetState(ctx, "cas-app", lower))

	got, err := store.GetState(ctx, "cas-app")
	require.NoError(t, err)
	assert.Equal(t, 50, got.Remaining)

	stale := &domain.RateLimitState{
		Remaining:   100,
		ResetAt:     reset,
		LastUpdated: time.Now().Truncate(time.Second),
	}
	require.NoError(t, store.SetState(ctx, "cas-app", stale))

	got, err = store.GetState(ctx, "cas-app")
	require.NoError(t, err)
	assert.Equal(t, 50, got.Remaining)
}

// storeNewWindow verifies that a new rate limit window always overwrites.
func storeNewWindow(t *testing.T, store domain.Store) {
	t.Helper()
	ctx := t.Context()

	old := rateLimitState(10, time.Hour)
	require.NoError(t, store.SetState(ctx, "window-app", old))

	current := rateLimitState(5000, 2*time.Hour)
	require.NoError(t, store.SetState(ctx, "window-app", current))

	got, err := store.GetState(ctx, "window-app")
	require.NoError(t, err)
	assert.Equal(t, 5000, got.Remaining)
	assert.Equal(t, current.ResetAt.Unix(), got.ResetAt.Unix())
}

// storeGetAll verifies that GetAllStates returns all stored entries.
func storeGetAll(t *testing.T, store domain.Store) {
	t.Helper()
	ctx := t.Context()

	for _, id := range []string{"all-1", "all-2", "all-3"} {
		require.NoError(t, store.SetState(ctx, id, rateLimitState(100, time.Hour)))
	}

	all, err := store.GetAllStates(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 3)
	assert.Contains(t, all, "all-1")
	assert.Contains(t, all, "all-2")
	assert.Contains(t, all, "all-3")
}
