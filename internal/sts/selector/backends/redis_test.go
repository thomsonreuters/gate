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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/sts/selector"
)

func TestRedisStore_Compiles(t *testing.T) {
	t.Parallel()
	var _ selector.Store = (*RedisStore)(nil)
}

func TestUnmarshalHash(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		fields := map[string]string{
			"remaining":    "42",
			"reset_at":     "1700000000",
			"last_updated": "1699999000",
		}
		state, err := unmarshalHash(fields)
		require.NoError(t, err)
		assert.Equal(t, 42, state.Remaining)
		assert.Equal(t, int64(1700000000), state.ResetAt.Unix())
		assert.Equal(t, int64(1699999000), state.LastUpdated.Unix())
	})

	t.Run("missing_remaining", func(t *testing.T) {
		t.Parallel()
		_, err := unmarshalHash(map[string]string{
			"reset_at":     "1700000000",
			"last_updated": "1699999000",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "remaining")
	})

	t.Run("missing_reset_at", func(t *testing.T) {
		t.Parallel()
		_, err := unmarshalHash(map[string]string{
			"remaining":    "42",
			"last_updated": "1699999000",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reset_at")
	})

	t.Run("missing_last_updated", func(t *testing.T) {
		t.Parallel()
		_, err := unmarshalHash(map[string]string{
			"remaining": "42",
			"reset_at":  "1700000000",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "last_updated")
	})

	t.Run("invalid_remaining", func(t *testing.T) {
		t.Parallel()
		_, err := unmarshalHash(map[string]string{
			"remaining":    "abc",
			"reset_at":     "1700000000",
			"last_updated": "1699999000",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "remaining")
	})
}
