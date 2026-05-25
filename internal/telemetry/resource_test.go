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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/config"
	"github.com/thomsonreuters/gate/internal/constants"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

func TestNewResource_UsesServiceNameFromConfig(t *testing.T) {
	t.Parallel()
	r, err := newResource(&config.OTelConfig{ServiceName: "test-service"})
	require.NoError(t, err)
	attrs := r.Attributes()

	var found bool
	for _, a := range attrs {
		if a.Key == semconv.ServiceNameKey {
			assert.Equal(t, "test-service", a.Value.AsString())
			found = true
		}
	}
	assert.True(t, found, "service.name attribute not found")
}

func TestNewResource_FallsBackToProgramIdentifier(t *testing.T) {
	t.Parallel()
	r, err := newResource(&config.OTelConfig{ServiceName: ""})
	require.NoError(t, err)

	var found bool
	for _, a := range r.Attributes() {
		if a.Key == semconv.ServiceNameKey {
			assert.Equal(t, constants.ProgramIdentifier, a.Value.AsString())
			found = true
		}
	}
	assert.True(t, found)
}

func TestNewResource_IncludesVersion(t *testing.T) {
	t.Parallel()
	r, err := newResource(&config.OTelConfig{ServiceName: "x"})
	require.NoError(t, err)

	var found bool
	for _, a := range r.Attributes() {
		if a.Key == semconv.ServiceVersionKey {
			assert.Equal(t, constants.Version, a.Value.AsString())
			found = true
		}
	}
	assert.True(t, found)
}
