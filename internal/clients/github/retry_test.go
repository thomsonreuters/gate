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

package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fastRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
		Multiplier:     2.0,
		JitterFraction: 0.0,
	}
}

func doGet(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func TestRetryTransport_StatusBehavior(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		statusCode   int
		failCount    int32
		wantStatus   int
		wantAttempts int32
	}{
		{"success_first_attempt", http.StatusOK, 0, http.StatusOK, 1},
		{"retries_on_5xx", http.StatusBadGateway, 3, http.StatusOK, 3},
		{"retries_on_429", http.StatusTooManyRequests, 2, http.StatusOK, 2},
		{"retries_on_400", http.StatusBadRequest, 2, http.StatusOK, 2},
		{"retries_on_401", http.StatusUnauthorized, 2, http.StatusOK, 2},
		{"retries_on_403", http.StatusForbidden, 2, http.StatusOK, 2},
		{"retries_on_404", http.StatusNotFound, 2, http.StatusOK, 2},
		{"exhausts_max_attempts", http.StatusServiceUnavailable, 999, http.StatusServiceUnavailable, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count := attempts.Add(1)
				if count < tt.failCount {
					w.WriteHeader(tt.statusCode)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(server.Close)

			client := &http.Client{Transport: &retryTransport{next: http.DefaultTransport, cfg: fastRetryConfig()}}
			resp := doGet(t, client, server.URL)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			assert.Equal(t, tt.wantAttempts, attempts.Load())
		})
	}
}

func TestRetryTransport_PreservesRequestBody(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	var bodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		bodies = append(bodies, string(buf[:n]))

		if count < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := &http.Client{Transport: &retryTransport{next: http.DefaultTransport, cfg: fastRetryConfig()}}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader(`{"test":"data"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, bodies, 2)
	assert.JSONEq(t, `{"test":"data"}`, bodies[0])
	assert.JSONEq(t, `{"test":"data"}`, bodies[1])
}

func TestRetryTransport_RespectsContextCancellation(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	cfg := fastRetryConfig()
	cfg.MaxAttempts = 10
	cfg.InitialBackoff = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	client := &http.Client{Transport: &retryTransport{next: http.DefaultTransport, cfg: cfg}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Less(t, attempts.Load(), int32(10))
}

func TestShouldRetry(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{"200 OK", http.StatusOK, false},
		{"201 Created", http.StatusCreated, false},
		{"204 No Content", http.StatusNoContent, false},
		{"301 Moved Permanently", http.StatusMovedPermanently, false},
		{"400 Bad Request", http.StatusBadRequest, true},
		{"401 Unauthorized", http.StatusUnauthorized, true},
		{"403 Forbidden", http.StatusForbidden, true},
		{"404 Not Found", http.StatusNotFound, true},
		{"408 Request Timeout", http.StatusRequestTimeout, true},
		{"429 Too Many Requests", http.StatusTooManyRequests, true},
		{"500 Internal Server Error", http.StatusInternalServerError, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := &http.Response{StatusCode: tt.status}
			assert.Equal(t, tt.want, shouldRetry(resp))
		})
	}
}

func TestShouldRetry_NilResponse(t *testing.T) {
	t.Parallel()
	assert.True(t, shouldRetry(nil))
}

func TestCalculateBackoff(t *testing.T) {
	t.Parallel()
	rt := &retryTransport{cfg: &RetryConfig{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     20 * time.Second,
		Multiplier:     2.0,
		JitterFraction: 0.0,
	}}

	assert.Equal(t, 1*time.Second, rt.calculateBackoff(1))
	assert.Equal(t, 2*time.Second, rt.calculateBackoff(2))
	assert.Equal(t, 4*time.Second, rt.calculateBackoff(3))
	assert.Equal(t, 8*time.Second, rt.calculateBackoff(4))
	assert.Equal(t, 16*time.Second, rt.calculateBackoff(5))
}

func TestCalculateBackoff_MaxCap(t *testing.T) {
	t.Parallel()
	rt := &retryTransport{cfg: &RetryConfig{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     3 * time.Second,
		Multiplier:     2.0,
		JitterFraction: 0.0,
	}}

	assert.Equal(t, 2*time.Second, rt.calculateBackoff(2))
	assert.Equal(t, 3*time.Second, rt.calculateBackoff(3))
	assert.Equal(t, 3*time.Second, rt.calculateBackoff(5))
}
