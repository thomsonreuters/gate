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

package config

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOTelConfig_Validate_DisabledIsAlwaysValid(t *testing.T) {
	t.Parallel()
	c := &OTelConfig{Enabled: false}
	require.NoError(t, c.Validate())
}

func TestOTelConfig_Validate_EnabledRequiresEndpoint(t *testing.T) {
	t.Parallel()
	c := &OTelConfig{Enabled: true, Protocol: "grpc", SampleRate: 1.0, ExporterTimeout: DefaultOTelExporterTimeout}
	err := c.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOTelEndpointRequired)
}

func TestOTelConfig_Validate_EnabledRejectsNonGRPCProtocol(t *testing.T) {
	t.Parallel()
	c := &OTelConfig{Enabled: true, Endpoint: "localhost:4317", Protocol: "http", SampleRate: 1.0, ExporterTimeout: DefaultOTelExporterTimeout}
	err := c.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOTelInvalidProtocol)
}

func TestOTelConfig_Validate_SampleRateRange(t *testing.T) {
	t.Parallel()
	base := OTelConfig{Enabled: true, Endpoint: "localhost:4317", Protocol: "grpc", ExporterTimeout: DefaultOTelExporterTimeout}

	for _, rate := range []float64{-0.1, 1.1, math.NaN()} {
		c := base
		c.SampleRate = rate
		assert.ErrorIs(t, c.Validate(), ErrOTelInvalidSampleRate)
	}
	for _, rate := range []float64{0.0, 0.5, 1.0} {
		c := base
		c.SampleRate = rate
		assert.NoError(t, c.Validate())
	}
}

func TestOTelConfig_Validate_ExporterTimeout(t *testing.T) {
	t.Parallel()
	base := OTelConfig{Enabled: true, Endpoint: "localhost:4317", Protocol: "grpc", SampleRate: 1.0}

	c := base
	c.ExporterTimeout = 0
	assert.ErrorIs(t, c.Validate(), ErrOTelInvalidExporterTimeout)

	c = base
	c.ExporterTimeout = -1
	assert.ErrorIs(t, c.Validate(), ErrOTelInvalidExporterTimeout)

	c = base
	c.ExporterTimeout = DefaultOTelExporterTimeout
	assert.NoError(t, c.Validate())
}
