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

// Package logger provides structured logging for the application.
// It defines LogLevel and LogFormat types, configures the global slog logger,
// and provides a GORM logger adapter (see gorm.go) that delegates to slog.
package logger

import (
	"log/slog"
	"os"
)

// GetSlogLevel returns the slog level for the given level string.
func GetSlogLevel(level LogLevel) slog.Level {
	switch level {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelInfo:
		return slog.LevelInfo
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelError:
		return slog.LevelError
	}
	return slog.LevelInfo
}

// SetGlobalLogger sets the default slog logger with the given level and format.
func SetGlobalLogger(level LogLevel, format LogFormat) {
	opts := &slog.HandlerOptions{Level: GetSlogLevel(level)}
	var handler slog.Handler

	if format == LogFormatJSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// InitDefaultLevel initializes the default logger with INFO level and JSON format.
func InitDefaultLevel() {
	SetGlobalLogger(LogLevelInfo, LogFormatJSON)
}
