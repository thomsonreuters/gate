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

package telemetry

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/config"
)

func TestInit_DisabledReturnsNoopShutdown(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	shutdown, err := Init(ctx, &config.OTelConfig{Enabled: false})
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	assert.NoError(t, shutdown(ctx))
}

func TestProbeCollector_Reachable(t *testing.T) {
	t.Parallel()

	lc := net.ListenConfig{}
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { require.NoError(t, ln.Close()) }()

	cfg := &config.OTelConfig{
		Endpoint:        ln.Addr().String(),
		ExporterTimeout: 2 * time.Second,
	}
	assert.NoError(t, probeCollector(t.Context(), cfg))
}

func TestProbeCollector_Unreachable(t *testing.T) {
	t.Parallel()

	// Use a port that is almost certainly not listening.
	cfg := &config.OTelConfig{
		Endpoint:        "127.0.0.1:1",
		ExporterTimeout: 500 * time.Millisecond,
	}
	assert.Error(t, probeCollector(t.Context(), cfg))
}

func TestInit_FailsWhenCollectorUnreachable(t *testing.T) {
	t.Parallel()

	cfg := &config.OTelConfig{
		Enabled:         true,
		Endpoint:        "127.0.0.1:1",
		Protocol:        "grpc",
		SampleRate:      1.0,
		ExporterTimeout: 500 * time.Millisecond,
	}

	shutdown, err := Init(t.Context(), cfg)
	require.Error(t, err)
	assert.Nil(t, shutdown)
	assert.Contains(t, err.Error(), fmt.Sprintf("OTel collector unreachable at %s", cfg.Endpoint))
}
