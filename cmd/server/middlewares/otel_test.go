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
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
)

func TestChiRouteLabeler_AddsRoutePatternAttribute(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()

	var capturedAttrs []attribute.KeyValue
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			labeler := &otelhttp.Labeler{}
			ctx := otelhttp.ContextWithLabeler(req.Context(), labeler)
			req = req.WithContext(ctx)
			next.ServeHTTP(w, req)
			capturedAttrs = labeler.Get()
		})
	})
	r.Use(ChiRouteLabeler)
	r.Get("/api/v1/users/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/42", nil)
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var found bool
	for _, kv := range capturedAttrs {
		if string(kv.Key) == "http.route" {
			assert.Equal(t, "/api/v1/users/{id}", kv.Value.AsString())
			found = true
		}
	}
	assert.True(t, found, "expected http.route attribute")
}
