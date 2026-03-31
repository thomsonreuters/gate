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

package authorizer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/thomsonreuters/gate/internal/config"
)

// authorizeCentral performs layer 1 authorization: issuer allowlist,
// required/forbidden claims, time restrictions.
func (a *Authorizer) authorizeCentral(req *Request) *DenialError {
	provider := a.findProvider(req.Issuer)
	if provider == nil {
		return newDenialError(ErrIssuerNotAllowed, "issuer not in allowed list",
			fmt.Sprintf("issuer: %s", req.Issuer))
	}

	for name, pattern := range provider.RequiredClaims {
		claim, ok := claimString(req.Claims, name)
		if !ok {
			return newDenialError(ErrRequiredClaimMismatch,
				fmt.Sprintf("required claim %q not present or not a string", name), "")
		}
		re := a.patterns[pattern]
		if re == nil {
			return newDenialError(ErrRequiredClaimMismatch,
				fmt.Sprintf("no compiled pattern for claim %q", name), "")
		}
		if !re.MatchString(claim) {
			return newDenialError(ErrRequiredClaimMismatch,
				fmt.Sprintf("required claim %q does not match pattern", name),
				fmt.Sprintf("expected: %s, got: %s", pattern, claim))
		}
	}

	for name, pattern := range provider.ForbiddenClaims {
		claim, ok := claimString(req.Claims, name)
		if !ok {
			continue
		}
		if re := a.patterns[pattern]; re != nil && re.MatchString(claim) {
			return newDenialError(ErrForbiddenClaimMatched,
				fmt.Sprintf("forbidden claim %q matches pattern", name),
				fmt.Sprintf("pattern: %s, value: %s", pattern, claim))
		}
	}

	if provider.TimeRestrictions != nil {
		if denial := a.validateTime(provider.TimeRestrictions); denial != nil {
			return denial
		}
	}

	return nil
}

// claimString returns the claim value as a string and whether it was present and a string.
func claimString(claims map[string]any, name string) (string, bool) {
	v, ok := claims[name]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// findProvider returns the provider config for the given issuer, or nil if not found.
func (a *Authorizer) findProvider(issuer string) *config.ProviderConfig {
	for i := range a.config.Providers {
		if a.config.Providers[i].Issuer == issuer {
			return &a.config.Providers[i]
		}
	}
	return nil
}

// validateTime checks whether the current UTC time is within the allowed days and hour range.
func (a *Authorizer) validateTime(tr *config.TimeRestriction) *DenialError {
	now := a.now().UTC()

	if len(tr.AllowedDays) > 0 {
		current := now.Weekday().String()
		if !slices.ContainsFunc(tr.AllowedDays, func(d config.AllowedDays) bool {
			return strings.EqualFold(string(d), current)
		}) {
			return newDenialError(ErrTimeRestriction, "current day is not allowed",
				fmt.Sprintf("current: %s", current))
		}
	}

	if tr.AllowedHours != nil {
		hour := now.Hour()
		if !isHourAllowed(hour, tr.AllowedHours) {
			return newDenialError(ErrTimeRestriction, "current hour is not allowed",
				fmt.Sprintf("allowed: %d-%d, current: %d UTC",
					tr.AllowedHours.Start, tr.AllowedHours.End, hour))
		}
	}

	return nil
}

// isHourAllowed returns true if hour falls within the allowed range
// (supports overnight windows when Start > End).
func isHourAllowed(hour int, window *config.HourRange) bool {
	if window.Start <= window.End {
		return hour >= window.Start && hour <= window.End
	}
	return hour >= window.Start || hour <= window.End
}
