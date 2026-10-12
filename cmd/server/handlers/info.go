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
	"crypto/fips140"
	"net/http"

	"github.com/go-chi/render"
)

// InfoHandler serves build and configuration information.
type InfoHandler struct {
	providersSHA256 string
}

// InfoResponse is the JSON body returned by the info endpoint.
type InfoResponse struct {
	FIPSEnabled     bool   `json:"fips_enabled"`
	ProvidersSHA256 string `json:"providers_sha256"`
}

// NewInfoHandler creates an InfoHandler reporting providersSHA256, the
// fingerprint of the trusted issuers (see config.PolicyConfig.IssuersFingerprint).
func NewInfoHandler(providersSHA256 string) *InfoHandler {
	return &InfoHandler{providersSHA256: providersSHA256}
}

// GetInfo writes build and configuration information as JSON.
func (h *InfoHandler) GetInfo(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, &InfoResponse{
		FIPSEnabled:     fips140.Enabled(),
		ProvidersSHA256: h.providersSHA256,
	})
}
