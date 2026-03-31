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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	domain "github.com/thomsonreuters/gate/internal/sts/selector"
	"github.com/thomsonreuters/gate/internal/sts/selector/backends"
	"github.com/thomsonreuters/gate/internal/testutil"
)

func newRedisStore(t *testing.T) *backends.RedisStore {
	t.Helper()
	return backends.NewRedisStore(testutil.NewRedisContainer(t))
}

func TestRedis_Lifecycle(t *testing.T) {
	t.Parallel()
	storeLifecycle(t, newRedisStore(t))
}

func TestRedis_StaleWriteDiscarded(t *testing.T) {
	t.Parallel()
	storeConditionalWrite(t, newRedisStore(t))
}

func TestRedis_NewWindowOverwrites(t *testing.T) {
	t.Parallel()
	storeNewWindow(t, newRedisStore(t))
}

func TestRedis_FirstWriteSucceeds(t *testing.T) {
	t.Parallel()
	store := newRedisStore(t)
	ctx := t.Context()

	_, err := store.GetState(ctx, "first-app")
	require.ErrorIs(t, err, domain.ErrStateNotFound)

	state := rateLimitState(4999, 1)
	require.NoError(t, store.SetState(ctx, "first-app", state))

	got, err := store.GetState(ctx, "first-app")
	require.NoError(t, err)
	assert.Equal(t, 4999, got.Remaining)
}

func TestRedis_GetAllStates(t *testing.T) {
	t.Parallel()
	storeGetAll(t, newRedisStore(t))
}

func TestRedis_SetNilDeletes(t *testing.T) {
	t.Parallel()
	store := newRedisStore(t)
	ctx := t.Context()

	require.NoError(t, store.SetState(ctx, "del-app", rateLimitState(42, 1)))
	require.NoError(t, store.SetState(ctx, "del-app", nil))
}
