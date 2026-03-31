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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimitState_IsExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		resetAt time.Time
		want    bool
	}{
		{name: "past", resetAt: time.Now().Add(-time.Minute), want: true},
		{name: "future", resetAt: time.Now().Add(time.Hour), want: false},
		{name: "zero", resetAt: time.Time{}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &RateLimitState{ResetAt: tt.resetAt}
			assert.Equal(t, tt.want, s.IsExpired())
		})
	}
}

func TestRateLimitState_HasCapacity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		remaining int
		want      bool
	}{
		{name: "has_capacity", remaining: 100, want: true},
		{name: "zero", remaining: 0, want: false},
		{name: "negative", remaining: -1, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &RateLimitState{Remaining: tt.remaining}
			assert.Equal(t, tt.want, s.HasCapacity())
		})
	}
}

func TestRateLimitState_IsFresherThan(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name  string
		s     *RateLimitState
		other *RateLimitState
		want  bool
	}{
		{
			name:  "nil_other",
			s:     &RateLimitState{ResetAt: now},
			other: nil,
			want:  true,
		},
		{
			name:  "newer_window",
			s:     &RateLimitState{ResetAt: now.Add(time.Hour)},
			other: &RateLimitState{ResetAt: now},
			want:  true,
		},
		{
			name:  "older_window",
			s:     &RateLimitState{ResetAt: now},
			other: &RateLimitState{ResetAt: now.Add(time.Hour)},
			want:  false,
		},
		{
			name:  "same_window_lower_remaining",
			s:     &RateLimitState{ResetAt: now, Remaining: 100},
			other: &RateLimitState{ResetAt: now, Remaining: 200},
			want:  true,
		},
		{
			name:  "same_window_higher_remaining",
			s:     &RateLimitState{ResetAt: now, Remaining: 200},
			other: &RateLimitState{ResetAt: now, Remaining: 100},
			want:  false,
		},
		{
			name:  "identical",
			s:     &RateLimitState{ResetAt: now, Remaining: 100},
			other: &RateLimitState{ResetAt: now, Remaining: 100},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.s.IsFresherThan(tt.other))
		})
	}
}

func TestExhaustedError(t *testing.T) {
	t.Parallel()

	err := &ExhaustedError{RetryAfter: 42}
	assert.Contains(t, err.Error(), "42")
	assert.Contains(t, err.Error(), "exhausted")

	var target *ExhaustedError
	require.ErrorAs(t, err, &target)
	assert.Equal(t, 42, target.RetryAfter)
}

func TestApp(t *testing.T) {
	t.Parallel()

	app := App{ClientID: "client-1", Organization: "example-org"}
	assert.Equal(t, "client-1", app.ClientID)
	assert.Equal(t, "example-org", app.Organization)
}
