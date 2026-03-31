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

package github

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidRepository is returned when the repository string is not in "owner/repo" format.
	ErrInvalidRepository = errors.New("invalid repository format (expected owner/repo)")
	// ErrFileNotFound is returned when the requested file path does not exist or is a directory.
	ErrFileNotFound = errors.New("file not found")
	// ErrRepositoryNotFound is returned when the repository is not
	// installed or not accessible to the app.
	ErrRepositoryNotFound = errors.New("repository not found or not accessible")
	// ErrRepositoryRequired is returned when the repository is required.
	ErrRepositoryRequired = errors.New("repository is required")
)

// NetworkError represents a network-level error (retryable).
type NetworkError struct {
	err error
}

// Error returns the error string representation of the network error.
func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error: %v", e.err)
}

// Unwrap returns the underlying error wrapped by NetworkError.
func (e *NetworkError) Unwrap() error {
	return e.err
}

// APIError represents a non-2xx HTTP response from the GitHub API.
type APIError struct {
	statusCode int
	message    string
}

// Error returns the formatted error message including the HTTP status code.
func (e *APIError) Error() string {
	return fmt.Sprintf("GitHub API error (status %d): %s", e.statusCode, e.message)
}

// StatusCode returns the HTTP status code from the GitHub API response.
func (e *APIError) StatusCode() int {
	return e.statusCode
}
