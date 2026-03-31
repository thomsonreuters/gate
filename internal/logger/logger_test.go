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

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSlogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level LogLevel
		want  slog.Level
	}{
		{"debug", LogLevelDebug, slog.LevelDebug},
		{"info", LogLevelInfo, slog.LevelInfo},
		{"warn", LogLevelWarn, slog.LevelWarn},
		{"error", LogLevelError, slog.LevelError},
		{"unknown defaults to info", LogLevel("unknown"), slog.LevelInfo},
		{"empty defaults to info", LogLevel(""), slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, GetSlogLevel(tt.level))
		})
	}
}

func TestIsValidLogLevel(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidLogLevel(LogLevelDebug))
	assert.True(t, IsValidLogLevel(LogLevelInfo))
	assert.True(t, IsValidLogLevel(LogLevelWarn))
	assert.True(t, IsValidLogLevel(LogLevelError))
	assert.False(t, IsValidLogLevel(LogLevel("unknown")))
	assert.False(t, IsValidLogLevel(LogLevel("")))
}

func TestIsValidLogFormat(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidLogFormat(LogFormatJSON))
	assert.True(t, IsValidLogFormat(LogFormatText))
	assert.False(t, IsValidLogFormat(LogFormat("xml")))
	assert.False(t, IsValidLogFormat(LogFormat("")))
}

func TestLogLevel_String(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "debug", LogLevelDebug.String())
	assert.Equal(t, "info", LogLevelInfo.String())
}

func TestLogFormat_String(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "json", LogFormatJSON.String())
	assert.Equal(t, "text", LogFormatText.String())
}

func TestSetGlobalLogger(t *testing.T) {
	ctx := t.Context()

	SetGlobalLogger(LogLevelDebug, LogFormatJSON)
	assert.True(t, slog.Default().Enabled(ctx, slog.LevelDebug))

	SetGlobalLogger(LogLevelError, LogFormatText)
	assert.False(t, slog.Default().Enabled(ctx, slog.LevelDebug))
	assert.True(t, slog.Default().Enabled(ctx, slog.LevelError))

	InitDefaultLevel()
}

func TestInitDefaultLevel(t *testing.T) {
	InitDefaultLevel()
	assert.True(t, slog.Default().Enabled(t.Context(), slog.LevelInfo))
}
