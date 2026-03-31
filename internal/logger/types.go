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

package logger

import "slices"

// LogLevel represents a named logging severity.
type LogLevel string

// String returns the string representation of the log level.
func (l LogLevel) String() string {
	return string(l)
}

const (
	// LogLevelDebug is the debug severity level.
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo is the info severity level.
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn is the warning severity level.
	LogLevelWarn LogLevel = "warn"
	// LogLevelError is the error severity level.
	LogLevelError LogLevel = "error"
)

// ValidLogLevels lists all accepted log level strings.
var ValidLogLevels = []LogLevel{
	LogLevelDebug,
	LogLevelInfo,
	LogLevelWarn,
	LogLevelError,
}

// IsValidLogLevel reports whether the given level string is recognized.
func IsValidLogLevel(level LogLevel) bool {
	return slices.Contains(ValidLogLevels, level)
}

// LogFormat represents a log output format.
type LogFormat string

// String returns the string representation of the log format.
func (f LogFormat) String() string {
	return string(f)
}

const (
	// LogFormatJSON outputs log entries as JSON (one object per line).
	LogFormatJSON LogFormat = "json"
	// LogFormatText outputs log entries as human-readable text.
	LogFormatText LogFormat = "text"
)

// ValidLogFormats lists all accepted log format strings.
var ValidLogFormats = []LogFormat{
	LogFormatJSON,
	LogFormatText,
}

// IsValidLogFormat reports whether the given format string is recognized.
func IsValidLogFormat(format LogFormat) bool {
	return slices.Contains(ValidLogFormats, format)
}
