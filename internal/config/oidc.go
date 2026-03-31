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
	// KeyOIDCAudience is the Viper key for the OIDC token audience.
	KeyOIDCAudience = "oidc.audience"
	// DefaultOIDCAudience is the default audience value when not set.
	DefaultOIDCAudience = "gate"
)

// ErrInvalidOIDCAudience is returned when the OIDC audience is empty.
var ErrInvalidOIDCAudience = errors.New("invalid OIDC audience")

// OIDCConfig holds OIDC token audience configuration.
type OIDCConfig struct {
	Audience string `mapstructure:"audience"`
}

// Validate validates the OIDC configuration.
func (o *OIDCConfig) Validate() error {
	if o.Audience == "" {
		return ErrInvalidOIDCAudience
	}
	return nil
}
