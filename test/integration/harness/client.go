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

package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the STS token exchange API.
// It wraps HTTP communication and provides typed request/response handling.
type Client struct {
	endpoint string
	timeout  time.Duration
	http     *http.Client
}

// ExchangeRequest represents a token exchange request.
type ExchangeRequest struct {
	OIDCToken            string            `json:"oidc_token"`
	TargetRepository     string            `json:"target_repository"`
	PolicyName           string            `json:"policy_name,omitempty"`
	RequestedPermissions map[string]string `json:"requested_permissions,omitempty"`
	RequestedTTL         int               `json:"requested_ttl,omitempty"`
}

// ExchangeResponse represents a successful token exchange response.
type ExchangeResponse struct {
	Token         string            `json:"token"`
	ExpiresAt     time.Time         `json:"expires_at"`
	MatchedPolicy string            `json:"matched_policy"`
	Permissions   map[string]string `json:"permissions"`
	RequestID     string            `json:"request_id"`
}

// ResponseError represents an error response from the STS API.
type ResponseError struct {
	Code              string `json:"error_code"`
	Message           string `json:"error"`
	Details           string `json:"details,omitempty"`
	RequestID         string `json:"request_id,omitempty"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
}

// Result wraps the outcome of an exchange request.
// Either Response or Error is set, never both.
type Result struct {
	Response   *ExchangeResponse
	Error      *ResponseError
	StatusCode int
}

// Error implements the error interface for ResponseError.
func (e *ResponseError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// newClient creates a new STS API client.
func newClient(endpoint string, timeout time.Duration) *Client {
	return &Client{
		endpoint: endpoint,
		timeout:  timeout,
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

// Exchange performs a token exchange request.
func (c *Client) Exchange(ctx context.Context, req *ExchangeRequest) (*Result, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/exchange", c.endpoint)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(request) // #nosec G704 -- test client targeting local httptest server
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	result := &Result{
		StatusCode: response.StatusCode,
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		var resp ExchangeResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}
		result.Response = &resp
	} else {
		var errResp ResponseError
		if err := json.Unmarshal(data, &errResp); err != nil {
			return nil, fmt.Errorf("unmarshal error response: %w", err)
		}
		result.Error = &errResp
	}

	return result, nil
}
