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

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/sts"
	"github.com/thomsonreuters/gate/test/integration/harness"
)

func TestTTL_SuccessScenarios(t *testing.T) {
	tests := []struct {
		name       string
		serverOpts []harness.ServerOption
		ttl        int
		maxExpiry  int
	}{
		{
			name:       "default used when not requested",
			serverOpts: []harness.ServerOption{harness.WithDefaultTTL(1800), harness.WithMaxTTL(3600)},
			ttl:        0,
			maxExpiry:  1800,
		},
		{
			name:       "custom within limits",
			serverOpts: []harness.ServerOption{harness.WithDefaultTTL(3600), harness.WithMaxTTL(7200)},
			ttl:        1800,
			maxExpiry:  1800,
		},
		{
			name:       "exactly at maximum",
			serverOpts: []harness.ServerOption{harness.WithDefaultTTL(900), harness.WithMaxTTL(1800)},
			ttl:        1800,
			maxExpiry:  1800,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)
			harness.StartServer(t, ctx, tt.serverOpts...)

			ctx.SetupPolicy(harness.DefaultRepo, "contents_read_metadata_read.tpl.yaml")

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        ctx.DefaultToken(),
				TargetRepository: harness.DefaultRepo,
				RequestedTTL:     tt.ttl,
			})
			require.NoError(t, err)
			require.Nil(t, got.Error)
			assert.NotEmpty(t, got.Response.Token)

			limit := time.Now().Add(time.Duration(tt.maxExpiry) * time.Second)
			assert.False(t, got.Response.ExpiresAt.After(limit.Add(30*time.Second)),
				"expires_at %v exceeds expected TTL %v (+30s tolerance)",
				got.Response.ExpiresAt, limit)
		})
	}
}

func TestTTL_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name         string
		serverOpts   []harness.ServerOption
		requestedTTL int
	}{
		{
			name:         "exceeds maximum rejected",
			serverOpts:   []harness.ServerOption{harness.WithDefaultTTL(900), harness.WithMaxTTL(1800)},
			requestedTTL: 7200,
		},
		{
			name:         "negative rejected",
			requestedTTL: -100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)
			harness.StartServer(t, ctx, tt.serverOpts...)

			ctx.SetupPolicy(harness.DefaultRepo, "contents_read_metadata_read.tpl.yaml")

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        ctx.DefaultToken(),
				TargetRepository: harness.DefaultRepo,
				RequestedTTL:     tt.requestedTTL,
			})
			require.NoError(t, err)

			require.NotNil(t, got.Error)
			assert.Equal(t, sts.ErrInvalidRequest, got.Error.Code)
		})
	}
}
