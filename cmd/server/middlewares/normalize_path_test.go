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
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripTrailingSlashes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		in          string
		want        string
		wantChanged bool
	}{
		{name: "empty", in: "", want: "", wantChanged: false},
		{name: "root preserved", in: "/", want: "/", wantChanged: false},
		{name: "no trailing slash", in: "/foo", want: "/foo", wantChanged: false},
		{name: "single trailing slash", in: "/foo/", want: "/foo", wantChanged: true},
		{name: "multiple trailing slashes", in: "/foo///", want: "/foo", wantChanged: true},
		{name: "nested path", in: "/api/v1/info/", want: "/api/v1/info", wantChanged: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, changed := stripTrailingSlashes(tt.in)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantChanged, changed)
		})
	}
}

func TestNormalizePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		rawPath     string
		wantPath    string
		wantRawPath string
	}{
		{name: "trailing slash stripped", path: "/health/", wantPath: "/health"},
		{name: "multiple trailing slashes stripped", path: "/health///", wantPath: "/health"},
		{name: "no trailing slash", path: "/health", wantPath: "/health"},
		{name: "root preserved", path: "/", wantPath: "/"},
		{name: "nested trailing slash", path: "/api/v1/info/", wantPath: "/api/v1/info"},
		{
			name:        "raw path stripped in lock-step with path",
			path:        "/api/v1/foo bar/",
			rawPath:     "/api/v1/foo%20bar/",
			wantPath:    "/api/v1/foo bar",
			wantRawPath: "/api/v1/foo%20bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				gotPath    string
				gotRawPath string
			)

			handler := NormalizePath(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotRawPath = r.URL.RawPath
			}))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			req.URL = &url.URL{Path: tt.path, RawPath: tt.rawPath}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.wantPath, gotPath)
			assert.Equal(t, tt.wantRawPath, gotRawPath)
		})
	}
}

func TestNormalizePath_PreservesCallerRequest(t *testing.T) {
	t.Parallel()

	handler := NormalizePath(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	originalURL := &url.URL{Path: "/health/", RawPath: "/health/", RawQuery: "x=1"}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.URL = originalURL

	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "/health/", originalURL.Path, "caller's URL.Path must not be mutated")
	assert.Equal(t, "/health/", originalURL.RawPath, "caller's URL.RawPath must not be mutated")
	assert.Same(t, originalURL, req.URL, "caller's *http.Request must still reference the original *url.URL")
}

func TestNormalizePath_PreservesQueryAndOtherFields(t *testing.T) {
	t.Parallel()

	var got *http.Request
	handler := NormalizePath(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	req.URL = &url.URL{Path: "/api/v1/exchange/", RawQuery: "code=abc&state=xyz", Fragment: "frag"}
	req.Header.Set("X-Test", "preserved")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "/api/v1/exchange", got.URL.Path)
	assert.Equal(t, "code=abc&state=xyz", got.URL.RawQuery, "query string must be preserved")
	assert.Equal(t, "frag", got.URL.Fragment, "fragment must be preserved")
	assert.Equal(t, http.MethodPost, got.Method, "method must be preserved")
	assert.Equal(t, "preserved", got.Header.Get("X-Test"), "headers must be preserved")
}

func TestNormalizePath_FastPathSkipsClone(t *testing.T) {
	t.Parallel()

	var observed *http.Request
	handler := NormalizePath(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		observed = r
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.URL = &url.URL{Path: "/health"}

	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Same(t, req, observed, "when no normalization is required the original request must be forwarded as-is")
}
