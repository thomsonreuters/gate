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
	"errors"
)

const (
	// KeyOTelEnabled is the Viper key for enabling the OpenTelemetry pipeline.
	KeyOTelEnabled = "otel.enabled"
	// KeyOTelServiceName is the Viper key for the OTel service.name resource attribute.
	KeyOTelServiceName = "otel.service_name"
	// KeyOTelEndpoint is the Viper key for the OTLP/gRPC collector endpoint.
	KeyOTelEndpoint = "otel.endpoint"
	// KeyOTelProtocol is the Viper key for the OTLP transport protocol (only "grpc" is supported).
	KeyOTelProtocol = "otel.protocol"
	// KeyOTelInsecure is the Viper key for disabling TLS on the OTLP/gRPC connection.
	KeyOTelInsecure = "otel.insecure"
	// KeyOTelSampleRate is the Viper key for the trace sampler ratio (0.0-1.0).
	KeyOTelSampleRate = "otel.sample_rate"

	// DefaultOTelServiceName is the default service.name when not set.
	DefaultOTelServiceName = "gate"
	// DefaultOTelEndpoint is the default OTLP/gRPC collector endpoint.
	DefaultOTelEndpoint = "localhost:4317"
	// DefaultOTelProtocol is the only supported OTLP transport.
	DefaultOTelProtocol = "grpc"
	// DefaultOTelInsecure disables TLS by default for local collectors.
	DefaultOTelInsecure = true
	// DefaultOTelSampleRate is the default trace sampler ratio.
	DefaultOTelSampleRate = 1.0
)

var (
	// ErrOTelEndpointRequired is returned when OTel is enabled without an endpoint.
	ErrOTelEndpointRequired = errors.New("otel endpoint is required when enabled")
	// ErrOTelInvalidProtocol is returned when otel.protocol is not "grpc".
	ErrOTelInvalidProtocol = errors.New("otel protocol must be \"grpc\"")
	// ErrOTelInvalidSampleRate is returned when otel.sample_rate is outside [0, 1].
	ErrOTelInvalidSampleRate = errors.New("otel sample_rate must be between 0.0 and 1.0")
)

// OTelConfig holds OpenTelemetry exporter and sampler configuration.
type OTelConfig struct {
	Enabled     bool    `mapstructure:"enabled"`
	ServiceName string  `mapstructure:"service_name"`
	Endpoint    string  `mapstructure:"endpoint"`
	Protocol    string  `mapstructure:"protocol"`
	Insecure    bool    `mapstructure:"insecure"`
	SampleRate  float64 `mapstructure:"sample_rate"`
}

// Validate validates OTel configuration. When disabled, all fields are accepted.
func (o *OTelConfig) Validate() error {
	if !o.Enabled {
		return nil
	}
	if o.Endpoint == "" {
		return ErrOTelEndpointRequired
	}
	if o.Protocol != DefaultOTelProtocol {
		return ErrOTelInvalidProtocol
	}
	if o.SampleRate < 0.0 || o.SampleRate > 1.0 {
		return ErrOTelInvalidSampleRate
	}
	return nil
}
