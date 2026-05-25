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

// Package handlers provides HTTP request handlers for the Gate API.
package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ggicci/httpin"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/thomsonreuters/gate/internal/sts"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// ExchangeInput is the httpin-decoded input for the exchange endpoint.
type ExchangeInput struct {
	Body sts.ExchangeRequest `in:"body=json"`
}

// ExchangeHandler handles POST /api/v1/exchange requests.
type ExchangeHandler struct {
	service             sts.Exchanger
	tracer              trace.Tracer
	exchangeCount       metric.Int64Counter
	exchangeDuration    metric.Float64Histogram
	tokenIssuedCount    metric.Int64Counter
	callerExchangeCount metric.Int64Counter
}

// ErrorResponse is the JSON body returned for all exchange failures.
type ErrorResponse struct {
	Code              string `json:"error_code"`
	Message           string `json:"error"`
	RequestID         string `json:"request_id"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
}

// NewExchangeHandler creates an ExchangeHandler with the given STS service.
// It pulls a tracer and meters from the global OTel providers.
func NewExchangeHandler(service sts.Exchanger) (*ExchangeHandler, error) {
	meter := otel.GetMeterProvider().Meter("exchange")
	counter, err := meter.Int64Counter("token_exchange_total",
		metric.WithDescription("Token exchange requests by outcome"))
	if err != nil {
		return nil, fmt.Errorf("registering token exchange counter: %w", err)
	}

	histogram, err := meter.Float64Histogram("token_exchange_duration_seconds",
		metric.WithUnit("s"),
		metric.WithDescription("Token exchange handler latency in seconds"))
	if err != nil {
		return nil, fmt.Errorf("registering token exchange duration histogram: %w", err)
	}

	tokenIssued, err := meter.Int64Counter("token_issued_total",
		metric.WithDescription("Tokens issued by repository"))
	if err != nil {
		return nil, fmt.Errorf("registering token issued counter: %w", err)
	}

	callerExchange, err := meter.Int64Counter("caller_exchange_total",
		metric.WithDescription("Exchange attempts by caller identity and outcome"))
	if err != nil {
		return nil, fmt.Errorf("registering caller exchange counter: %w", err)
	}

	return &ExchangeHandler{
		service:             service,
		tracer:              otel.GetTracerProvider().Tracer("exchange"),
		exchangeCount:       counter,
		exchangeDuration:    histogram,
		tokenIssuedCount:    tokenIssued,
		callerExchangeCount: callerExchange,
	}, nil
}

// httpStatusCode maps STS error codes to HTTP status codes for API responses.
func httpStatusCode(code string) int {
	switch code {
	case sts.ErrInvalidRequest:
		return http.StatusBadRequest
	case sts.ErrInvalidToken:
		return http.StatusUnauthorized
	case sts.ErrRateLimited:
		return http.StatusTooManyRequests
	case sts.ErrTrustPolicyNotFound:
		return http.StatusForbidden
	case sts.ErrPolicyNotFound, sts.ErrRepositoryNotFound:
		return http.StatusNotFound
	case sts.ErrGitHubAPIError:
		return http.StatusBadGateway
	case sts.ErrPolicyLoadFailed, sts.ErrInternalError:
		return http.StatusInternalServerError
	default:
		// Authorization denial codes (ISSUER_NOT_ALLOWED, NO_RULES_MATCHED, etc.)
		return http.StatusForbidden
	}
}

// Exchange delegates to the STS service and writes the response.
func (h *ExchangeHandler) Exchange(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetReqID(r.Context())
	ctx, span := h.tracer.Start(r.Context(), "TokenExchange")
	defer span.End()

	start := time.Now()
	outcome := "ok"
	var callerIssuer, callerSubject string

	defer func() {
		r := recover()
		if r != nil {
			outcome = "panic"
			span.SetStatus(codes.Error, "handler panic")
			span.SetAttributes(attribute.String("panic.value", fmt.Sprintf("%v", r)))
			slog.ErrorContext(ctx, "Exchange handler panic", slog.Any("panic", r))
		}
		h.exchangeDuration.Record(ctx, time.Since(start).Seconds())
		h.exchangeCount.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
		if callerIssuer != "" || callerSubject != "" {
			h.callerExchangeCount.Add(ctx, 1, metric.WithAttributes(
				attribute.String("sub", callerSubject),
				attribute.String("issuer", callerIssuer),
				attribute.String("outcome", outcome),
			))
		}
		if r != nil {
			panic(r)
		}
	}()

	input, ok := r.Context().Value(httpin.Input).(*ExchangeInput)
	if !ok {
		outcome = h.writeError(w, r, span, sts.ErrInvalidRequest, "Invalid request", requestID, 0, nil)
		return
	}
	span.SetAttributes(attribute.String("repository", input.Body.TargetRepository))

	response, err := h.service.Exchange(ctx, requestID, &input.Body)
	if err != nil {
		var exErr *sts.ExchangeError
		if errors.As(err, &exErr) {
			callerIssuer = exErr.Issuer
			callerSubject = exErr.Subject
			outcome = h.writeError(w, r, span, exErr.Code, exErr.Message, exErr.RequestID, exErr.RetryAfterSeconds, err)
			return
		}
		outcome = h.writeError(w, r, span, sts.ErrInternalError, "Internal server error", requestID, 0, err)
		return
	}

	callerIssuer = response.Issuer
	callerSubject = response.Subject
	h.tokenIssuedCount.Add(ctx, 1, metric.WithAttributes(
		attribute.String("repository", input.Body.TargetRepository),
	))

	span.SetAttributes(
		attribute.String("matched_policy", response.MatchedPolicy),
	)

	render.Status(r, http.StatusOK)
	render.JSON(w, r, response)
}

// writeError renders a JSON error response, records the failure on the
// span, and returns the metric outcome label (lowercased error code).
func (h *ExchangeHandler) writeError(w http.ResponseWriter, r *http.Request, span trace.Span,
	code, message, requestID string, retryAfter int, err error) string {
	if err != nil {
		span.RecordError(err)
	}
	span.SetStatus(codes.Error, message)

	if code == sts.ErrRateLimited && retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	}

	render.Status(r, httpStatusCode(code))
	render.JSON(w, r, &ErrorResponse{
		Code:              code,
		Message:           message,
		RequestID:         requestID,
		RetryAfterSeconds: retryAfter,
	})
	return strings.ToLower(code)
}
