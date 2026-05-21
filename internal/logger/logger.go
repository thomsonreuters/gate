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

	"github.com/thomsonreuters/gate/internal/constants"
	"github.com/thomsonreuters/gate/internal/logger/slogext"
	"go.opentelemetry.io/contrib/bridges/otelslog"
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

// SetGlobalLogger sets the default slog logger. When otelEnabled is true,
// records are fanned out to stdout (with trace_id/span_id enrichment) and
// to the OTel log provider via the otelslog bridge.
func SetGlobalLogger(level LogLevel, format LogFormat, otelEnabled bool) {
	opts := &slog.HandlerOptions{Level: GetSlogLevel(level)}

	var stdoutHandler slog.Handler
	if format == LogFormatJSON {
		stdoutHandler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		stdoutHandler = slog.NewTextHandler(os.Stdout, opts)
	}

	var handler slog.Handler = stdoutHandler
	if otelEnabled {
		enriched := slogext.NewTraceHandler(stdoutHandler)
		bridge := otelslog.NewHandler(constants.ProgramIdentifier)
		handler = slogext.NewFanoutHandler(enriched, bridge)
	}

	slog.SetDefault(slog.New(handler))
	slog.Debug("Global slog logger configured",
		slog.String("level", level.String()),
		slog.String("format", format.String()),
		slog.Bool("otel_fanout", otelEnabled),
	)
}

// InitDefaultLevel initializes the default logger with INFO level, JSON format,
// and OTel disabled. It's safe to call before config is loaded.
func InitDefaultLevel() {
	SetGlobalLogger(LogLevelInfo, LogFormatJSON, false)
}
