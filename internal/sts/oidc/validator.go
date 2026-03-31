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

// Package oidc validates OIDC tokens for the STS exchange flow.
//
// Uses coreos/go-oidc for discovery, JWKS caching, key rotation,
// and JWT verification. The issuer allowlist is the security boundary —
// only pre-approved issuers trigger network requests.
package oidc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

var (
	// ErrEmptyAudience is returned when the validator is created with an empty audience.
	ErrEmptyAudience = errors.New("audience is required")
	// ErrEmptyToken is returned when Validate is called with an empty token string.
	ErrEmptyToken = errors.New("token is empty")
	// ErrMalformedToken is returned when the token cannot be parsed as a JWT.
	ErrMalformedToken = errors.New("malformed token")
	// ErrMissingIssuer is returned when the token has no issuer claim.
	ErrMissingIssuer = errors.New("token missing issuer claim")
	// ErrIssuerDenied is returned when the token issuer is not in the allowed list.
	ErrIssuerDenied = errors.New("issuer not in allowed list")
	// ErrEmptyIssuers is returned when NewValidator is called with no allowed issuers.
	ErrEmptyIssuers = errors.New("at least one allowed issuer is required")
)

// Validator validates tokens from a fixed set of allowed issuers.
type Validator struct {
	audience  string
	issuers   map[string]struct{}
	providers sync.Map // issuer → *gooidc.Provider (cached after first discovery)
}

// NewValidator creates a validator for the given audience and allowed issuers.
// Both are required — a production STS must never accept arbitrary issuers.
func NewValidator(audience string, allowedIssuers []string) (*Validator, error) {
	if audience == "" {
		return nil, ErrEmptyAudience
	}
	if len(allowedIssuers) == 0 {
		return nil, ErrEmptyIssuers
	}

	issuers := make(map[string]struct{}, len(allowedIssuers))
	for _, iss := range allowedIssuers {
		issuers[iss] = struct{}{}
	}

	return &Validator{audience: audience, issuers: issuers}, nil
}

// Claims holds the validated token data consumed by authorization and audit.
type Claims struct {
	Issuer    string         // Token issuer (e.g. https://token.actions.githubusercontent.com).
	Subject   string         // Token subject (e.g. repo:org/repo:ref:refs/heads/main).
	Audience  []string       // Intended audience; must include the configured audience.
	ExpiresAt time.Time      // Token expiration time.
	IssuedAt  time.Time      // Token issuance time.
	Custom    map[string]any // Non-standard claims (repository, ref, actor, etc.).
}

// Validate verifies the OIDC token signature, claims, and expiry.
// It checks the issuer against the allowlist before performing network-based verification.
// Returns extracted claims on success or an error if validation fails.
func (o *Validator) Validate(ctx context.Context, rawToken string) (*Claims, error) {
	if rawToken == "" {
		return nil, ErrEmptyToken
	}

	parsed, err := jwt.ParseString(rawToken, jwt.WithVerify(false))
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	issuer := parsed.Issuer()

	if issuer == "" {
		return nil, ErrMissingIssuer
	}

	if _, ok := o.issuers[issuer]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrIssuerDenied, issuer)
	}

	provider, err := o.provider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discovering issuer %s: %w", issuer, err)
	}

	if _, err := provider.Verifier(&gooidc.Config{ClientID: o.audience}).Verify(ctx, rawToken); err != nil {
		return nil, fmt.Errorf("verifying token: %w", err)
	}

	return &Claims{
		Issuer:    parsed.Issuer(),
		Subject:   parsed.Subject(),
		Audience:  parsed.Audience(),
		ExpiresAt: parsed.Expiration(),
		IssuedAt:  parsed.IssuedAt(),
		Custom:    parsed.PrivateClaims(),
	}, nil
}

// provider returns a cached OIDC provider or creates one via discovery.
func (o *Validator) provider(ctx context.Context, issuer string) (*gooidc.Provider, error) {
	if p, ok := o.providers.Load(issuer); ok {
		provider, ok := p.(*gooidc.Provider)
		if !ok {
			return nil, errors.New("unexpected provider type")
		}
		return provider, nil
	}

	p, err := gooidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}

	actual, _ := o.providers.LoadOrStore(issuer, p)
	provider, ok := actual.(*gooidc.Provider)
	if !ok {
		return nil, errors.New("unexpected provider type")
	}
	return provider, nil
}
