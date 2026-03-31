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

import "errors"

const (
	// KeyOriginEnabled is the Viper key for enabling origin verification.
	KeyOriginEnabled = "origin.enabled"
	// KeyOriginHeaderName is the Viper key for the origin verification header name.
	KeyOriginHeaderName = "origin.header_name"
	// KeyOriginHeaderValue is the Viper key for the expected origin verification header value.
	KeyOriginHeaderValue = "origin.header_value"
)

var (
	// ErrInvalidOriginHeaderName is returned when origin verification
	// is enabled but header_name is empty.
	ErrInvalidOriginHeaderName = errors.New("origin header name is required")
	// ErrInvalidOriginHeaderValue is returned when origin verification
	// is enabled but header_value is empty.
	ErrInvalidOriginHeaderValue = errors.New("origin header value is required")
)

// OriginConfig holds origin verification settings (shared secret header).
type OriginConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	HeaderName  string `mapstructure:"header_name"`
	HeaderValue string `mapstructure:"header_value"`
}

// Validate validates the origin verification configuration.
func (o *OriginConfig) Validate() error {
	if o.Enabled {
		if o.HeaderName == "" {
			return ErrInvalidOriginHeaderName
		}
		if o.HeaderValue == "" {
			return ErrInvalidOriginHeaderValue
		}
	}
	return nil
}
