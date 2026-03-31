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
	"context"
	"errors"
	"fmt"
)

// authorizeRepository performs layer 2 authorization: loads trust
// policy, matches policy, resolves permissions and TTL.
func (a *Authorizer) authorizeRepository(ctx context.Context, req *Request) *Result {
	if a.config.RequireExplicitPolicy && req.PolicyName == "" {
		return denied(newDenialError(ErrPolicyNameRequired,
			"policy_name is required but not provided", ""))
	}

	file, err := a.fetchPolicy(ctx, req.TargetRepository, a.config.TrustPolicyPath)
	if err != nil {
		switch {
		case errors.Is(err, ErrRepositoryNotAccessible):
			return denied(newDenialError(ErrRepositoryNotFound,
				"repository not found or not accessible", err.Error()))
		case errors.Is(err, ErrPolicyFileNotFound):
			return denied(newDenialError(ErrTrustPolicyNotFound,
				"trust policy file not found in repository", err.Error()))
		default:
			return denied(newDenialError(ErrPolicyLoadFailed,
				"failed to load trust policy", err.Error()))
		}
	}

	var matched *TrustPolicy
	var denial *DenialError
	if req.PolicyName != "" {
		matched, denial = matchExplicit(file, req)
	} else {
		matched, denial = matchAutomatic(file, req)
	}
	if denial != nil {
		return denied(denial)
	}

	permissions, denial := a.resolvePermissions(req, matched)
	if denial != nil {
		return denied(denial)
	}

	return &Result{
		Allowed:              true,
		MatchedPolicy:        matched.Name,
		EffectivePermissions: permissions,
		EffectiveTTL:         a.resolveTTL(req, matched),
	}
}

// matchExplicit finds the trust policy named in the request and validates issuer and rules.
func matchExplicit(file *PolicyFile, req *Request) (*TrustPolicy, *DenialError) {
	for i := range file.TrustPolicies {
		p := &file.TrustPolicies[i]
		if p.Name != req.PolicyName {
			continue
		}
		if p.Issuer != req.Issuer {
			return nil, newDenialError(ErrIssuerNotAllowed,
				fmt.Sprintf("policy %q does not accept issuer %s", req.PolicyName, req.Issuer), "")
		}
		if evaluateRules(p, req.Claims) {
			return p, nil
		}
		return nil, newDenialError(ErrNoRulesMatched,
			fmt.Sprintf("policy %q rules did not match the request", req.PolicyName), "")
	}
	return nil, newDenialError(ErrPolicyNotFound,
		fmt.Sprintf("policy %q not found in repository", req.PolicyName), "")
}

// matchAutomatic finds the first trust policy that matches the request issuer and rules.
func matchAutomatic(file *PolicyFile, req *Request) (*TrustPolicy, *DenialError) {
	for i := range file.TrustPolicies {
		p := &file.TrustPolicies[i]
		if p.Issuer != req.Issuer {
			continue
		}
		if evaluateRules(p, req.Claims) {
			return p, nil
		}
	}
	return nil, newDenialError(ErrNoRulesMatched, "no policy rules matched the request", "")
}

// evaluateRules returns true if any rule in the policy matches the claims.
func evaluateRules(policy *TrustPolicy, claims map[string]any) bool {
	for i := range policy.Rules {
		if evaluateRule(&policy.Rules[i], claims) {
			return true
		}
	}
	return false
}

// evaluateRule returns true if the rule's conditions match the claims (AND/OR logic).
func evaluateRule(rule *PolicyRule, claims map[string]any) bool {
	logic := rule.Logic
	if logic == "" {
		logic = "AND"
	}

	if logic == "AND" {
		for i := range rule.Conditions {
			if !evaluateCondition(&rule.Conditions[i], claims) {
				return false
			}
		}
		return true
	}

	for i := range rule.Conditions {
		if evaluateCondition(&rule.Conditions[i], claims) {
			return true
		}
	}
	return false
}

// evaluateCondition returns true if the claim value matches the condition's pattern.
func evaluateCondition(cond *Condition, claims map[string]any) bool {
	claim, ok := claimString(claims, cond.Field)
	if !ok {
		return false
	}
	return cond.Matches(claim)
}

// resolveTTL returns the effective TTL from request, policy, and config defaults/caps.
func (a *Authorizer) resolveTTL(req *Request, policy *TrustPolicy) int {
	ttl := req.RequestedTTL
	if ttl == 0 {
		ttl = a.config.DefaultTokenTTL
	}
	if policy.TokenTTL > 0 && policy.TokenTTL < ttl {
		ttl = policy.TokenTTL
	}
	if ttl > a.config.MaxTokenTTL {
		ttl = a.config.MaxTokenTTL
	}
	return ttl
}
