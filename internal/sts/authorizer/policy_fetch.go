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
	"path/filepath"
	"strings"

	"github.com/thomsonreuters/gate/internal/clients/github"
	"gopkg.in/yaml.v3"
)

// fetchPolicy retrieves a trust policy from the target repository.
// Results are cached. If the path already has a .yaml/.yml extension it is
// used directly; otherwise both extensions are tried in order.
func (a *Authorizer) fetchPolicy(ctx context.Context, repository, pathTemplate string) (*PolicyFile, error) {
	parts := strings.SplitN(repository, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid repository format: %s", repository)
	}
	org := parts[0]
	resolved := strings.ReplaceAll(pathTemplate, "{org}", org)

	cacheKey := repository + ":" + resolved
	if cached := a.cache.get(cacheKey); cached != nil {
		return cached, nil
	}

	v, err, _ := a.fetchSF.Do(cacheKey, func() (any, error) {
		if cached := a.cache.get(cacheKey); cached != nil {
			return cached, nil
		}
		return a.fetchPolicyFromGitHub(ctx, repository, resolved, cacheKey)
	})
	if err != nil {
		return nil, err
	}
	pf, ok := v.(*PolicyFile)
	if !ok {
		return nil, fmt.Errorf("unexpected type from policy fetch: %T", v)
	}
	return pf, nil
}

// fetchPolicyFromGitHub fetches the trust policy file from the repository
// via the selected GitHub App and parses/validates it.
func (a *Authorizer) fetchPolicyFromGitHub(ctx context.Context, repository, resolved, cacheKey string) (*PolicyFile, error) {
	app, err := a.selector.SelectApp(ctx, repository)
	if err != nil {
		return nil, fmt.Errorf("selecting GitHub App: %w", err)
	}

	client, ok := a.clients[app.ClientID]
	if !ok {
		return nil, fmt.Errorf("no GitHub client for app %s", app.ClientID)
	}

	paths := extensionVariants(resolved)

	for _, fullPath := range paths {
		content, err := client.GetContents(ctx, repository, fullPath)
		if err == nil {
			policy, parseErr := parseAndValidate(content)
			if parseErr != nil {
				return nil, parseErr
			}
			a.cache.set(cacheKey, policy)
			return policy, nil
		}
		if errors.Is(err, github.ErrRepositoryNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrRepositoryNotAccessible, repository)
		}
		if !errors.Is(err, github.ErrFileNotFound) {
			return nil, fmt.Errorf("fetching %s: %w", fullPath, err)
		}
	}

	return nil, fmt.Errorf("%w at %s", ErrPolicyFileNotFound, strings.Join(paths, ", "))
}

// extensionVariants returns the paths to try when fetching a trust policy.
// If the path already has a .yaml or .yml extension, it is used as-is.
// Otherwise, both .yaml and .yml variants are returned.
func extensionVariants(path string) []string {
	ext := filepath.Ext(path)
	if ext == ".yaml" || ext == ".yml" {
		return []string{path}
	}
	return []string{path + ".yaml", path + ".yml"}
}

// parseAndValidate unmarshals the YAML into a PolicyFile and runs Validate.
func parseAndValidate(content []byte) (*PolicyFile, error) {
	var pf PolicyFile
	if err := yaml.Unmarshal(content, &pf); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	if err := pf.Validate(); err != nil {
		return nil, fmt.Errorf("invalid trust policy: %w", err)
	}
	return &pf, nil
}
