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
	"slices"
)

const (
	// KeyFIPSEnabled is the Viper key for enabling FIPS 140 mode.
	KeyFIPSEnabled = "fips.enabled"
	// KeyFIPSMode is the Viper key for the FIPS mode (on or only).
	KeyFIPSMode = "fips.mode"
)

// FIPSMode is the FIPS 140 mode: "on" (prefer FIPS) or "only" (require FIPS).
type FIPSMode string

const (
	// FIPSModeOn prefers FIPS-approved crypto when available.
	FIPSModeOn FIPSMode = "on"
	// FIPSModeOnly requires FIPS-approved crypto only.
	FIPSModeOnly FIPSMode = "only"
)

// ValidFIPSModes lists the allowed FIPS mode values.
var ValidFIPSModes = []FIPSMode{
	FIPSModeOn,
	FIPSModeOnly,
}

// IsValidFIPSMode returns true if mode is "on" or "only".
func IsValidFIPSMode(mode FIPSMode) bool {
	return slices.Contains(ValidFIPSModes, mode)
}

// ErrInvalidFIPSMode is returned when the FIPS mode is not "on" or "only".
var ErrInvalidFIPSMode = errors.New("invalid FIPS mode")

// FIPSConfig holds FIPS 140 enablement and mode settings.
type FIPSConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Mode    FIPSMode `mapstructure:"mode"`
}

// Validate validates the FIPS configuration.
func (f *FIPSConfig) Validate() error {
	if f.Mode != "" && !IsValidFIPSMode(f.Mode) {
		return ErrInvalidFIPSMode
	}
	return nil
}
