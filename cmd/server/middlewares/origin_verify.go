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
	"crypto/subtle"
	"log/slog"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/thomsonreuters/gate/internal/config"
)

// OriginVerify creates middleware that validates requests contain a shared secret header.
// Uses constant-time comparison to prevent timing attacks.
//
// When origin verification is disabled via config, the middleware is a no-op.
// Returns 403 Forbidden when the header is missing or the value doesn't match.
func OriginVerify(cfg config.OriginConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if !cfg.Enabled {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			value := r.Header.Get(cfg.HeaderName)

			if value == "" || subtle.ConstantTimeCompare([]byte(value), []byte(cfg.HeaderValue)) != 1 {
				reqID := chimw.GetReqID(r.Context())

				slog.WarnContext(r.Context(), "origin verification failed",
					"request_id", reqID,
					"header", cfg.HeaderName,
					"method", r.Method,
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr)

				render.Status(r, http.StatusForbidden)
				render.JSON(w, r, map[string]string{
					"error":      "Forbidden",
					"error_code": "ORIGIN_VERIFICATION_FAILED",
					"request_id": reqID,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
