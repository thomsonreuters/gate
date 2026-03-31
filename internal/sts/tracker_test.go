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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenTracker_Record(t *testing.T) {
	t.Parallel()
	tracker := NewTokenTracker()

	tracker.Record("hash-1", "token-1", time.Now().Add(time.Hour))
	tracker.Record("hash-2", "token-2", time.Now().Add(time.Hour))

	expired := tracker.GetExpired()
	assert.Empty(t, expired, "no tokens should be expired yet")
}

func TestTokenTracker_GetExpired(t *testing.T) {
	t.Parallel()
	tracker := NewTokenTracker()

	tracker.Record("expired-1", "token-a", time.Now().Add(-time.Minute))
	tracker.Record("expired-2", "token-b", time.Now().Add(-time.Hour))
	tracker.Record("active-1", "token-c", time.Now().Add(time.Hour))

	expired := tracker.GetExpired()
	assert.Len(t, expired, 2)
	assert.Equal(t, "token-a", expired["expired-1"])
	assert.Equal(t, "token-b", expired["expired-2"])
	assert.NotContains(t, expired, "active-1")
}

func TestTokenTracker_GetExpired_DoesNotRemove(t *testing.T) {
	t.Parallel()
	tracker := NewTokenTracker()

	tracker.Record("hash-1", "token-1", time.Now().Add(-time.Minute))

	first := tracker.GetExpired()
	require.Len(t, first, 1)

	second := tracker.GetExpired()
	assert.Len(t, second, 1, "GetExpired should not remove entries")
}

func TestTokenTracker_GetExpired_Empty(t *testing.T) {
	t.Parallel()
	tracker := NewTokenTracker()

	expired := tracker.GetExpired()
	assert.Empty(t, expired)
}

func TestTokenTracker_Remove(t *testing.T) {
	t.Parallel()
	tracker := NewTokenTracker()

	tracker.Record("hash-1", "token-1", time.Now().Add(-time.Minute))
	tracker.Record("hash-2", "token-2", time.Now().Add(-time.Minute))
	tracker.Record("hash-3", "token-3", time.Now().Add(-time.Minute))

	tracker.Remove("hash-1")
	tracker.Remove("hash-3")

	expired := tracker.GetExpired()
	assert.Len(t, expired, 1)
	assert.Equal(t, "token-2", expired["hash-2"])
}

func TestTokenTracker_Remove_NonExistentHash(t *testing.T) {
	t.Parallel()
	tracker := NewTokenTracker()

	tracker.Record("hash-1", "token-1", time.Now().Add(-time.Minute))

	tracker.Remove("nonexistent")

	expired := tracker.GetExpired()
	assert.Len(t, expired, 1)
}

func TestTokenTracker_RecordOverwrite(t *testing.T) {
	t.Parallel()
	tracker := NewTokenTracker()

	tracker.Record("hash-1", "old-token", time.Now().Add(-time.Minute))
	tracker.Record("hash-1", "new-token", time.Now().Add(time.Hour))

	expired := tracker.GetExpired()
	assert.Empty(t, expired, "overwritten entry should have new expiry")
}
