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
	"bytes"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"
)

// RetryConfig defines retry behavior for GitHub API HTTP calls.
type RetryConfig struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
	JitterFraction float64
}

// DefaultRetryConfig provides sensible retry defaults for GitHub API calls.
var DefaultRetryConfig = &RetryConfig{
	MaxAttempts:    3,
	InitialBackoff: 100 * time.Millisecond,
	MaxBackoff:     2 * time.Second,
	Multiplier:     2.0,
	JitterFraction: 0.1,
}

// retryTransport wraps an http.RoundTripper with automatic retry on
// transient failures (5xx, 429, 408, network errors).
type retryTransport struct {
	next http.RoundTripper
	cfg  *RetryConfig
}

// RoundTrip implements http.RoundTripper with automatic retries for transient failures.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
	}

	var lastResp *http.Response
	var lastErr error

	for attempt := range t.cfg.MaxAttempts {
		if attempt > 0 {
			backoff := t.calculateBackoff(attempt)
			select {
			case <-time.After(backoff):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}

		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err := t.next.RoundTrip(req)
		if err == nil && !shouldRetry(resp) {
			return resp, nil
		}
		if err != nil && !isTransient(err) {
			return resp, err
		}

		lastResp = resp
		lastErr = err

		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, nil
}

var retryableStatusCodes = map[int]bool{
	http.StatusBadRequest:      true,
	http.StatusNotFound:        true,
	http.StatusRequestTimeout:  true,
	http.StatusTooManyRequests: true,
}

func shouldRetry(resp *http.Response) bool {
	if resp == nil {
		return true
	}
	return resp.StatusCode >= http.StatusInternalServerError || retryableStatusCodes[resp.StatusCode]
}

// isTransient returns true for timeouts, DNS/connection errors, and connection reset/refused/EOF.
func isTransient(err error) bool {
	if err == nil {
		return false
	}

	if netErr, ok := errors.AsType[net.Error](err); ok {
		return netErr.Timeout()
	}

	if _, ok := errors.AsType[*net.OpError](err); ok {
		return true
	}

	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return true
	}

	msg := err.Error()
	return strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "EOF")
}

// calculateBackoff returns exponential backoff with jitter for the given attempt number.
func (t *retryTransport) calculateBackoff(attempt int) time.Duration {
	backoff := float64(t.cfg.InitialBackoff)
	for range attempt {
		backoff *= t.cfg.Multiplier
	}
	if backoff > float64(t.cfg.MaxBackoff) {
		backoff = float64(t.cfg.MaxBackoff)
	}

	//nolint:gosec // G404: math/rand is acceptable for non-cryptographic retry jitter
	jitter := backoff * t.cfg.JitterFraction * (rand.Float64()*2 - 1)
	backoff += jitter

	if backoff < 0 {
		backoff = float64(t.cfg.InitialBackoff)
	}
	return time.Duration(backoff)
}
