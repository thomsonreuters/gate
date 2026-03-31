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

package backends

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/sts/selector"
)

func TestMemoryStore_Compiles(t *testing.T) {
	t.Parallel()
	var _ selector.Store = (*MemoryStore)(nil)
}

func TestMemoryStore_GetSet(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	state := &selector.RateLimitState{
		Remaining:   100,
		ResetAt:     time.Now().Add(time.Hour),
		LastUpdated: time.Now(),
	}

	require.NoError(t, store.SetState(ctx, "app1", state))

	got, err := store.GetState(ctx, "app1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 100, got.Remaining)
	assert.False(t, got.LastUpdated.IsZero())
}

func TestMemoryStore_NotFound(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	_, err := store.GetState(ctx, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, selector.ErrStateNotFound)
}

func TestMemoryStore_FreshnessCheck(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	fresh := &selector.RateLimitState{
		Remaining:   50,
		ResetAt:     time.Now().Add(time.Hour),
		LastUpdated: time.Now(),
	}
	require.NoError(t, store.SetState(ctx, "app1", fresh))

	stale := &selector.RateLimitState{
		Remaining:   80,
		ResetAt:     fresh.ResetAt,
		LastUpdated: time.Now(),
	}
	require.NoError(t, store.SetState(ctx, "app1", stale))

	got, err := store.GetState(ctx, "app1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 50, got.Remaining, "stale write should be ignored")
}

func TestMemoryStore_GetAllStates(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	require.NoError(
		t,
		store.SetState(ctx, "app1", &selector.RateLimitState{Remaining: 10, ResetAt: time.Now().Add(time.Hour), LastUpdated: time.Now()}),
	)
	require.NoError(
		t,
		store.SetState(ctx, "app2", &selector.RateLimitState{Remaining: 20, ResetAt: time.Now().Add(time.Hour), LastUpdated: time.Now()}),
	)

	all, err := store.GetAllStates(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Equal(t, 10, all["app1"].Remaining)
	assert.Equal(t, 20, all["app2"].Remaining)
}

func TestMemoryStore_Close(t *testing.T) {
	t.Parallel()
	require.NoError(t, NewMemoryStore().Close())
}

func TestMemoryStore_Concurrent(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			state := &selector.RateLimitState{
				Remaining:   n,
				ResetAt:     time.Now().Add(time.Hour),
				LastUpdated: time.Now(),
			}
			_ = store.SetState(ctx, "app1", state)
			_, _ = store.GetState(ctx, "app1")
			_, _ = store.GetAllStates(ctx)
		}(i)
	}
	wg.Wait()
}
