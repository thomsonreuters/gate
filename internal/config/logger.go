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

	"github.com/thomsonreuters/gate/internal/logger"
)

const (
	// KeyLoggerLevel is the Viper key for the log level.
	KeyLoggerLevel = "logger.level"
	// KeyLoggerFormat is the Viper key for the log format (e.g. json, text).
	KeyLoggerFormat = "logger.format"
)

var (
	// ErrInvalidLogLevel is returned when the configured log level is not supported.
	ErrInvalidLogLevel = errors.New("invalid log level")
	// ErrInvalidLogFormat is returned when the configured log format is not supported.
	ErrInvalidLogFormat = errors.New("invalid log format")
)

// LoggerConfig holds logging level and format settings.
type LoggerConfig struct {
	Level  logger.LogLevel  `mapstructure:"level"`
	Format logger.LogFormat `mapstructure:"format"`
}

// Validate validates the logger configuration.
func (l *LoggerConfig) Validate() error {
	if l.Level != "" && !logger.IsValidLogLevel(l.Level) {
		return ErrInvalidLogLevel
	}
	if l.Format != "" && !logger.IsValidLogFormat(l.Format) {
		return ErrInvalidLogFormat
	}
	return nil
}
