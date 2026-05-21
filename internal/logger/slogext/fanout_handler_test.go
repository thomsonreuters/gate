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

package slogext

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFanoutHandler_DispatchesToAllChildren(t *testing.T) {
	t.Parallel()

	var bufA, bufB bytes.Buffer
	hA := slog.NewJSONHandler(&bufA, &slog.HandlerOptions{Level: slog.LevelDebug})
	hB := slog.NewJSONHandler(&bufB, &slog.HandlerOptions{Level: slog.LevelDebug})

	fan := NewFanoutHandler(hA, hB)
	logger := slog.New(fan)
	logger.InfoContext(context.Background(), "fanout-test")

	assert.Contains(t, bufA.String(), "fanout-test")
	assert.Contains(t, bufB.String(), "fanout-test")
}

func TestFanoutHandler_EnabledReturnsTrueIfAnyChildEnabled(t *testing.T) {
	t.Parallel()

	errorOnly := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError})
	debugOk := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelDebug})

	fan := NewFanoutHandler(errorOnly, debugOk)
	assert.True(t, fan.Enabled(context.Background(), slog.LevelDebug))
}

func TestFanoutHandler_WithAttrsPropagatesToChildren(t *testing.T) {
	t.Parallel()

	var bufA, bufB bytes.Buffer
	hA := slog.NewJSONHandler(&bufA, &slog.HandlerOptions{Level: slog.LevelDebug})
	hB := slog.NewJSONHandler(&bufB, &slog.HandlerOptions{Level: slog.LevelDebug})

	fan := NewFanoutHandler(hA, hB)
	logger := slog.New(fan).With("svc", "gate")
	logger.InfoContext(context.Background(), "msg")

	assert.Contains(t, bufA.String(), `"svc":"gate"`)
	assert.Contains(t, bufB.String(), `"svc":"gate"`)
}
