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
	"net/url"
)

// NormalizePath is an HTTP middleware that strips trailing slashes from
// r.URL.Path (and r.URL.RawPath, when set) before the request reaches
// subsequent middleware or handlers. The root path "/" is preserved.
//
// Why this exists: chi's middleware.StripSlashes only rewrites the chi
// RouteContext's RoutePath; it does not modify r.URL.Path. Middleware that
// reads r.URL.Path directly (for example middleware.Heartbeat, structured
// loggers, or OpenTelemetry span-name formatters) therefore continues to see
// the unnormalized path. Registering NormalizePath at the top of the chain
// closes that gap so every downstream observer sees a single canonical path.
//
// NormalizePath does not mutate the caller's *http.Request. If normalization
// is required it performs a shallow clone of the request and its URL (mirroring
// net/http.StripPrefix); otherwise it forwards the request unchanged.
func NormalizePath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, pathChanged := stripTrailingSlashes(r.URL.Path)
		raw, rawChanged := stripTrailingSlashes(r.URL.RawPath)
		if !pathChanged && !rawChanged {
			next.ServeHTTP(w, r)
			return
		}

		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = path
		r2.URL.RawPath = raw
		next.ServeHTTP(w, r2)
	})
}

// stripTrailingSlashes removes every trailing '/' from p, except when p itself
// is "/". It returns the cleaned path and whether any change was made so the
// caller can avoid unnecessary allocations on already-canonical paths.
func stripTrailingSlashes(p string) (string, bool) {
	end := len(p)
	for end > 1 && p[end-1] == '/' {
		end--
	}
	if end == len(p) {
		return p, false
	}
	return p[:end], true
}
