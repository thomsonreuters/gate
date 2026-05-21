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
	"context"
	"errors"
	"log/slog"
)

// FanoutHandler dispatches each record to every child handler.
type FanoutHandler struct {
	children []slog.Handler
}

// NewFanoutHandler builds a FanoutHandler from one or more child handlers.
// If called with no children, Enabled always returns false, so slog will
// short-circuit before invoking Handle — records are dropped silently
// without ever reaching this handler.
func NewFanoutHandler(children ...slog.Handler) *FanoutHandler {
	return &FanoutHandler{children: children}
}

// Enabled returns true if any child reports the level enabled.
func (h *FanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, c := range h.children {
		if c.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle dispatches the record to each child; errors are joined.
func (h *FanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, c := range h.children {
		if !c.Enabled(ctx, r.Level) {
			continue
		}
		if err := c.Handle(ctx, r.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// WithAttrs returns a new FanoutHandler with attrs applied to each child.
func (h *FanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(h.children))
	for i, c := range h.children {
		next[i] = c.WithAttrs(attrs)
	}
	return &FanoutHandler{children: next}
}

// WithGroup returns a new FanoutHandler with the group applied to each child.
func (h *FanoutHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(h.children))
	for i, c := range h.children {
		next[i] = c.WithGroup(name)
	}
	return &FanoutHandler{children: next}
}
