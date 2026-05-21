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

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/thomsonreuters/gate/internal/config"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
)

// Init initializes the global OpenTelemetry tracer, meter, and logger
// providers and returns a shutdown function. If cfg.Enabled is false it
// returns a no-op shutdown without touching globals.
func Init(ctx context.Context, cfg *config.OTelConfig) (func(context.Context) error, error) {
	if !cfg.Enabled {
		slog.InfoContext(ctx, "OpenTelemetry disabled")
		return func(context.Context) error { return nil }, nil
	}

	slog.InfoContext(ctx, "Initializing OpenTelemetry",
		slog.String("endpoint", cfg.Endpoint),
		slog.String("protocol", cfg.Protocol),
		slog.String("service_name", cfg.ServiceName),
		slog.Bool("insecure", cfg.Insecure),
		slog.Float64("sample_rate", cfg.SampleRate),
	)

	res, err := newResource(cfg)
	if err != nil {
		return nil, fmt.Errorf("building resource: %w", err)
	}
	slog.DebugContext(ctx, "OTel resource built")

	var shutdowns []func(context.Context) error

	tp, err := newTracerProvider(ctx, cfg, res)
	if err != nil {
		return nil, fmt.Errorf("tracer provider: %w", err)
	}
	otel.SetTracerProvider(tp)
	shutdowns = append(shutdowns, tp.Shutdown)
	slog.DebugContext(ctx, "OTel tracer provider registered")

	mp, err := newMeterProvider(ctx, cfg, res)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, fmt.Errorf("meter provider: %w", err)
	}
	otel.SetMeterProvider(mp)
	shutdowns = append(shutdowns, mp.Shutdown)
	slog.DebugContext(ctx, "OTel meter provider registered")

	lp, err := newLoggerProvider(ctx, cfg, res)
	if err != nil {
		_ = mp.Shutdown(ctx)
		_ = tp.Shutdown(ctx)
		return nil, fmt.Errorf("logger provider: %w", err)
	}
	global.SetLoggerProvider(lp)
	shutdowns = append(shutdowns, lp.Shutdown)
	slog.DebugContext(ctx, "OTel logger provider registered")

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	slog.DebugContext(ctx, "OTel propagators configured", slog.String("propagators", "tracecontext,baggage"))

	if err := runtime.Start(); err != nil {
		slog.WarnContext(ctx, "OTel runtime instrumentation failed to start; runtime metrics unavailable", slog.Any("error", err))
	} else {
		slog.DebugContext(ctx, "OTel runtime metrics started")
	}

	slog.InfoContext(ctx, "OpenTelemetry initialized")

	return func(ctx context.Context) error {
		slog.DebugContext(ctx, "Shutting down OpenTelemetry providers")
		var errs []error
		// Shut down in reverse registration order so dependents finish first.
		for i := len(shutdowns) - 1; i >= 0; i-- {
			if err := shutdowns[i](ctx); err != nil {
				slog.WarnContext(ctx, "OTel provider shutdown failed",
					slog.Int("index", i),
					slog.Any("error", err),
				)
				errs = append(errs, err)
			}
		}
		if len(errs) == 0 {
			slog.InfoContext(ctx, "OpenTelemetry shut down")
		}
		return errors.Join(errs...)
	}, nil
}
