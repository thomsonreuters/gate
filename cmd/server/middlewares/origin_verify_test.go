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

package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/config"
)

func TestOriginVerify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cfg    config.OriginConfig
		value  string
		status int
	}{
		{
			name: "valid header",
			cfg: config.OriginConfig{
				Enabled:     true,
				HeaderName:  "X-Origin-Verify",
				HeaderValue: "secret-value",
			},
			value:  "secret-value",
			status: http.StatusOK,
		},
		{
			name: "missing header",
			cfg: config.OriginConfig{
				Enabled:     true,
				HeaderName:  "X-Origin-Verify",
				HeaderValue: "secret-value",
			},
			value:  "",
			status: http.StatusForbidden,
		},
		{
			name: "invalid header",
			cfg: config.OriginConfig{
				Enabled:     true,
				HeaderName:  "X-Origin-Verify",
				HeaderValue: "secret-value",
			},
			value:  "wrong-value",
			status: http.StatusForbidden,
		},
		{
			name: "disabled",
			cfg: config.OriginConfig{
				Enabled: false,
			},
			value:  "",
			status: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := OriginVerify(tt.cfg)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
			if tt.value != "" {
				req.Header.Set(tt.cfg.HeaderName, tt.value)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.status, w.Code)
		})
	}
}

func TestOriginVerify_ResponseFormat(t *testing.T) {
	t.Parallel()

	cfg := config.OriginConfig{
		Enabled:     true,
		HeaderName:  "X-Origin-Verify",
		HeaderValue: "secret",
	}

	handler := OriginVerify(cfg)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "Forbidden", response["error"])
	assert.Equal(t, "ORIGIN_VERIFICATION_FAILED", response["error_code"])
}
