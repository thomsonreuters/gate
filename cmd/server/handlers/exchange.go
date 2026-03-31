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
	"net/http"
	"strconv"

	"github.com/ggicci/httpin"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/thomsonreuters/gate/internal/sts"
)

// ExchangeInput is the httpin-decoded input for the exchange endpoint.
type ExchangeInput struct {
	Body sts.ExchangeRequest `in:"body=json"`
}

// ExchangeHandler handles POST /api/v1/exchange requests.
type ExchangeHandler struct {
	service sts.Exchanger
}

// ErrorResponse is the JSON body returned for all exchange failures.
type ErrorResponse struct {
	Code              string `json:"error_code"`
	Message           string `json:"error"`
	RequestID         string `json:"request_id"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
}

// maxBodySize is the maximum request body size in bytes.
const maxBodySize = 1 << 20 // 1 MB

// NewExchangeHandler creates an ExchangeHandler with the given STS service.
func NewExchangeHandler(service sts.Exchanger) *ExchangeHandler {
	return &ExchangeHandler{service: service}
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
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	requestID := middleware.GetReqID(r.Context())

	input, ok := r.Context().Value(httpin.Input).(*ExchangeInput)
	if !ok {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, &ErrorResponse{Code: sts.ErrInvalidRequest, Message: "Invalid request", RequestID: requestID})
		return
	}

	resp, err := h.service.Exchange(r.Context(), requestID, &input.Body)
	if err != nil {
		var exchangeErr *sts.ExchangeError
		if errors.As(err, &exchangeErr) {
			status := httpStatusCode(exchangeErr.Code)
			if exchangeErr.Code == sts.ErrRateLimited && exchangeErr.RetryAfterSeconds > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(exchangeErr.RetryAfterSeconds))
			}
			render.Status(r, status)
			render.JSON(w, r, &ErrorResponse{
				Code:              exchangeErr.Code,
				Message:           exchangeErr.Message,
				RequestID:         exchangeErr.RequestID,
				RetryAfterSeconds: exchangeErr.RetryAfterSeconds,
			})
			return
		}

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, &ErrorResponse{
			Code:      sts.ErrInternalError,
			Message:   "Internal server error",
			RequestID: requestID,
		})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, resp)
}
