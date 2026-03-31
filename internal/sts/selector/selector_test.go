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

package selector_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/sts/selector"
	"github.com/thomsonreuters/gate/internal/sts/selector/backends"
)

func TestNewSelector(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		store := backends.NewMemoryStore()
		t.Cleanup(func() { _ = store.Close() })

		sel, err := selector.NewSelector([]selector.App{{ClientID: "client-1", Organization: "example-org"}}, store)
		require.NoError(t, err)
		require.NotNil(t, sel)
	})

	t.Run("empty_apps", func(t *testing.T) {
		t.Parallel()
		store := backends.NewMemoryStore()
		t.Cleanup(func() { _ = store.Close() })

		sel, err := selector.NewSelector(nil, store)
		require.ErrorIs(t, err, selector.ErrNoApps)
		require.Nil(t, sel)

		sel, err = selector.NewSelector([]selector.App{}, store)
		require.ErrorIs(t, err, selector.ErrNoApps)
		require.Nil(t, sel)
	})

	t.Run("nil_store", func(t *testing.T) {
		t.Parallel()

		sel, err := selector.NewSelector([]selector.App{{ClientID: "client-1", Organization: "example-org"}}, nil)
		require.ErrorIs(t, err, selector.ErrNilStore)
		require.Nil(t, sel)
	})
}

func TestSelectApp_NoState(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := backends.NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	sel, err := selector.NewSelector([]selector.App{
		{ClientID: "client-1", Organization: "example-org"},
		{ClientID: "client-2", Organization: "example-org"},
	}, store)
	require.NoError(t, err)

	seen := make(map[string]bool)
	for range 100 {
		app, err := sel.SelectApp(ctx, "example-org/example-repo")
		require.NoError(t, err)
		require.NotNil(t, app)
		seen[app.ClientID] = true
	}
	assert.True(t, seen["client-1"], "client-1 should be selected at least once")
	assert.True(t, seen["client-2"], "client-2 should be selected at least once")
}

func TestSelectApp_ExpiredState(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := backends.NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	sel, err := selector.NewSelector([]selector.App{
		{ClientID: "client-1", Organization: "example-org"},
		{ClientID: "client-2", Organization: "example-org"},
	}, store)
	require.NoError(t, err)

	require.NoError(t, store.SetState(ctx, "client-1", &selector.RateLimitState{
		Remaining:   0,
		ResetAt:     time.Now().Add(-time.Minute),
		LastUpdated: time.Now(),
	}))

	seen := make(map[string]bool)
	for range 100 {
		app, err := sel.SelectApp(ctx, "example-org/example-repo")
		require.NoError(t, err)
		require.NotNil(t, app)
		seen[app.ClientID] = true
	}
	assert.True(t, seen["client-1"], "expired app should be eligible for selection")
	assert.True(t, seen["client-2"], "unknown app should be eligible for selection")
}

func TestSelectApp_HighestRemaining(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := backends.NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	sel, err := selector.NewSelector([]selector.App{
		{ClientID: "client-1", Organization: "example-org"},
		{ClientID: "client-2", Organization: "example-org"},
		{ClientID: "client-3", Organization: "example-org"},
	}, store)
	require.NoError(t, err)

	resetAt := time.Now().Add(time.Hour)
	require.NoError(t, store.SetState(ctx, "client-1", &selector.RateLimitState{Remaining: 5, ResetAt: resetAt, LastUpdated: time.Now()}))
	require.NoError(t, store.SetState(ctx, "client-2", &selector.RateLimitState{Remaining: 50, ResetAt: resetAt, LastUpdated: time.Now()}))
	require.NoError(t, store.SetState(ctx, "client-3", &selector.RateLimitState{Remaining: 20, ResetAt: resetAt, LastUpdated: time.Now()}))

	app, err := sel.SelectApp(ctx, "example-org/example-repo")
	require.NoError(t, err)
	require.NotNil(t, app)
	assert.Equal(t, "client-2", app.ClientID, "should pick app with most headroom")
}

func TestSelectApp_Exhausted(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := backends.NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	sel, err := selector.NewSelector([]selector.App{
		{ClientID: "client-1", Organization: "example-org"},
		{ClientID: "client-2", Organization: "example-org"},
	}, store)
	require.NoError(t, err)

	resetAt := time.Now().Add(30 * time.Second)
	require.NoError(t, store.SetState(ctx, "client-1", &selector.RateLimitState{Remaining: 0, ResetAt: resetAt, LastUpdated: time.Now()}))
	require.NoError(t, store.SetState(ctx, "client-2", &selector.RateLimitState{Remaining: 0, ResetAt: resetAt, LastUpdated: time.Now()}))

	app, err := sel.SelectApp(ctx, "example-org/example-repo")
	require.Nil(t, app)
	var exhausted *selector.ExhaustedError
	require.ErrorAs(t, err, &exhausted)
	assert.GreaterOrEqual(t, exhausted.RetryAfter, selector.MinRetrySeconds)
	assert.LessOrEqual(t, exhausted.RetryAfter, selector.DefaultRetrySeconds)
}

func TestSelectApp_ExhaustedRetryAfterScopedToOrg(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := backends.NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	sel, err := selector.NewSelector([]selector.App{
		{ClientID: "client-1", Organization: "example-org"},
		{ClientID: "client-2", Organization: "other-org"},
	}, store)
	require.NoError(t, err)

	now := time.Now()
	// other-org app resets very soon — should NOT affect example-org's retry-after
	require.NoError(t, store.SetState(ctx, "client-2", &selector.RateLimitState{
		Remaining: 0, ResetAt: now.Add(2 * time.Second), LastUpdated: now,
	}))
	// example-org app resets later
	require.NoError(t, store.SetState(ctx, "client-1", &selector.RateLimitState{
		Remaining: 0, ResetAt: now.Add(45 * time.Second), LastUpdated: now,
	}))

	app, err := sel.SelectApp(ctx, "example-org/example-repo")
	require.Nil(t, app)
	var exhausted *selector.ExhaustedError
	require.ErrorAs(t, err, &exhausted)
	// retry-after must reflect example-org's app (≈45s), not other-org's (≈2s)
	assert.GreaterOrEqual(t, exhausted.RetryAfter, 40, "retry-after should be scoped to the requested org")
}

func TestSelectApp_InvalidRepository(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := backends.NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })
	sel, err := selector.NewSelector([]selector.App{{ClientID: "client-1", Organization: "example-org"}}, store)
	require.NoError(t, err)

	for _, repo := range []string{"", "nopath", "single/", "/only"} {
		app, err := sel.SelectApp(ctx, repo)
		require.Nil(t, app)
		require.ErrorIs(t, err, selector.ErrInvalidRepository)
	}
}

func TestSelectApp_NoMatchingOrg(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := backends.NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	sel, err := selector.NewSelector([]selector.App{{ClientID: "client-1", Organization: "example-org"}}, store)
	require.NoError(t, err)

	app, err := sel.SelectApp(ctx, "other-org/other-repo")
	require.Nil(t, app)
	require.ErrorIs(t, err, selector.ErrNoMatchingApp)
}

func TestRecordUsage(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := backends.NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	sel, err := selector.NewSelector([]selector.App{{ClientID: "client-1", Organization: "example-org"}}, store)
	require.NoError(t, err)

	resetAt := time.Now().Add(time.Hour)
	require.NoError(t, sel.RecordUsage(ctx, "client-1", 99, resetAt))

	state, err := store.GetState(ctx, "client-1")
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, 99, state.Remaining)
	assert.True(t, state.ResetAt.Equal(resetAt) || state.ResetAt.Sub(resetAt).Abs() < time.Second)
	assert.False(t, state.LastUpdated.IsZero())
}

func TestRetryAfter(t *testing.T) {
	t.Parallel()

	t.Run("no_states", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		store := backends.NewMemoryStore()
		t.Cleanup(func() { _ = store.Close() })
		sel, err := selector.NewSelector([]selector.App{{ClientID: "client-1", Organization: "example-org"}}, store)
		require.NoError(t, err)

		assert.Equal(t, selector.DefaultRetrySeconds, sel.RetryAfter(ctx))
	})

	t.Run("earliest_reset", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		store := backends.NewMemoryStore()
		t.Cleanup(func() { _ = store.Close() })
		sel, err := selector.NewSelector([]selector.App{{ClientID: "client-1", Organization: "example-org"}}, store)
		require.NoError(t, err)

		require.NoError(t, store.SetState(ctx, "client-1", &selector.RateLimitState{
			Remaining: 0, ResetAt: time.Now().Add(45 * time.Second), LastUpdated: time.Now(),
		}))

		retry := sel.RetryAfter(ctx)
		assert.GreaterOrEqual(t, retry, 40)
		assert.LessOrEqual(t, retry, 50)
	})

	t.Run("clamped_min", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		store := backends.NewMemoryStore()
		t.Cleanup(func() { _ = store.Close() })
		sel, err := selector.NewSelector([]selector.App{{ClientID: "client-1", Organization: "example-org"}}, store)
		require.NoError(t, err)

		require.NoError(t, store.SetState(ctx, "client-1", &selector.RateLimitState{
			Remaining: 0, ResetAt: time.Now().Add(500 * time.Millisecond), LastUpdated: time.Now(),
		}))

		assert.Equal(t, selector.MinRetrySeconds, sel.RetryAfter(ctx))
	})

	t.Run("clamped_max", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		store := backends.NewMemoryStore()
		t.Cleanup(func() { _ = store.Close() })
		sel, err := selector.NewSelector([]selector.App{{ClientID: "client-1", Organization: "example-org"}}, store)
		require.NoError(t, err)

		require.NoError(t, store.SetState(ctx, "client-1", &selector.RateLimitState{
			Remaining: 0, ResetAt: time.Now().Add(2 * time.Hour), LastUpdated: time.Now(),
		}))

		assert.Equal(t, selector.DefaultRetrySeconds, sel.RetryAfter(ctx))
	})
}
