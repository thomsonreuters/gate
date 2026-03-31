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

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ggicci/httpin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/sts"
)

type mockService struct {
	resp *sts.ExchangeResponse
	err  error
}

var _ sts.Exchanger = (*mockService)(nil)

func (m *mockService) Exchange(_ context.Context, _ string, _ *sts.ExchangeRequest) (*sts.ExchangeResponse, error) {
	return m.resp, m.err
}

func TestNewExchangeHandler(t *testing.T) {
	t.Parallel()
	h := NewExchangeHandler(&mockService{})
	require.NotNil(t, h)
}

func TestHTTPStatusCode(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 400, httpStatusCode(sts.ErrInvalidRequest))
	assert.Equal(t, 401, httpStatusCode(sts.ErrInvalidToken))
	assert.Equal(t, 404, httpStatusCode(sts.ErrPolicyNotFound))
	assert.Equal(t, 403, httpStatusCode(sts.ErrTrustPolicyNotFound))
	assert.Equal(t, 404, httpStatusCode(sts.ErrRepositoryNotFound))
	assert.Equal(t, 429, httpStatusCode(sts.ErrRateLimited))
	assert.Equal(t, 500, httpStatusCode(sts.ErrInternalError))
	assert.Equal(t, 500, httpStatusCode(sts.ErrPolicyLoadFailed))
	assert.Equal(t, 502, httpStatusCode(sts.ErrGitHubAPIError))
	assert.Equal(t, 403, httpStatusCode(sts.ErrIssuerNotAllowed))
}

func TestExchange_MissingInput(t *testing.T) {
	t.Parallel()
	h := NewExchangeHandler(&mockService{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/exchange", nil)
	w := httptest.NewRecorder()

	h.Exchange(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, sts.ErrInvalidRequest, resp.Code)
}

func TestExchange_Success(t *testing.T) {
	t.Parallel()

	svc := &mockService{
		resp: &sts.ExchangeResponse{
			Token:         "ghs_test_token", // #nosec G101 -- test fixture, not a credential
			ExpiresAt:     time.Now().Add(time.Hour),
			MatchedPolicy: "deploy-prod",
			Permissions:   map[string]string{"contents": "read"},
			RequestID:     "req-123",
		},
	}
	h := NewExchangeHandler(svc)

	input := &ExchangeInput{
		Body: sts.ExchangeRequest{
			OIDCToken:        "oidc-token",
			TargetRepository: "org/repo",
		},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/exchange", nil)
	ctx := context.WithValue(req.Context(), httpin.Input, input)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Exchange(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp sts.ExchangeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ghs_test_token", resp.Token)
	assert.Equal(t, "deploy-prod", resp.MatchedPolicy)
}

func TestExchange_ExchangeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        *sts.ExchangeError
		wantStatus int
	}{
		{
			name:       "invalid_request",
			err:        &sts.ExchangeError{Code: sts.ErrInvalidRequest, Message: "bad request", RequestID: "req-1"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid_token",
			err:        &sts.ExchangeError{Code: sts.ErrInvalidToken, Message: "bad token", RequestID: "req-2"},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "issuer_not_allowed",
			err:        &sts.ExchangeError{Code: sts.ErrIssuerNotAllowed, Message: "denied", RequestID: "req-3"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "policy_not_found",
			err:        &sts.ExchangeError{Code: sts.ErrPolicyNotFound, Message: "not found", RequestID: "req-4"},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "trust_policy_not_found",
			err:        &sts.ExchangeError{Code: sts.ErrTrustPolicyNotFound, Message: "trust policy file not found", RequestID: "req-4b"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "repository_not_found",
			err:        &sts.ExchangeError{Code: sts.ErrRepositoryNotFound, Message: "repository not found", RequestID: "req-4c"},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "github_api_error",
			err:        &sts.ExchangeError{Code: sts.ErrGitHubAPIError, Message: "github error", RequestID: "req-5"},
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "internal_error",
			err:        &sts.ExchangeError{Code: sts.ErrInternalError, Message: "internal", RequestID: "req-6"},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := &mockService{err: tt.err}
			h := NewExchangeHandler(svc)

			input := &ExchangeInput{
				Body: sts.ExchangeRequest{OIDCToken: "tok", TargetRepository: "org/repo"},
			}
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/exchange", nil)
			ctx := context.WithValue(req.Context(), httpin.Input, input)
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			h.Exchange(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var resp ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, tt.err.Code, resp.Code)
			assert.Equal(t, tt.err.RequestID, resp.RequestID)
		})
	}
}

func TestExchange_RateLimitedWithRetryAfter(t *testing.T) {
	t.Parallel()

	svc := &mockService{
		err: &sts.ExchangeError{
			Code:              sts.ErrRateLimited,
			Message:           "rate limited",
			RequestID:         "req-rl",
			RetryAfterSeconds: 60,
		},
	}
	h := NewExchangeHandler(svc)

	input := &ExchangeInput{
		Body: sts.ExchangeRequest{OIDCToken: "tok", TargetRepository: "org/repo"},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/exchange", nil)
	ctx := context.WithValue(req.Context(), httpin.Input, input)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Exchange(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
}

func TestExchange_NonExchangeError(t *testing.T) {
	t.Parallel()

	svc := &mockService{err: errors.New("unexpected")}
	h := NewExchangeHandler(svc)

	input := &ExchangeInput{
		Body: sts.ExchangeRequest{OIDCToken: "tok", TargetRepository: "org/repo"},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/exchange", nil)
	ctx := context.WithValue(req.Context(), httpin.Input, input)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Exchange(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, sts.ErrInternalError, resp.Code)
}
