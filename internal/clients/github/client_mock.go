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
	"context"

	"github.com/stretchr/testify/mock"
)

// MockClient is a testify mock implementing ClientIface.
type MockClient struct {
	mock.Mock
}

//nolint:errcheck // mock: panic on unexpected type is intentional
func (m *MockClient) RequestToken(ctx context.Context, req *TokenRequest) (*TokenResponse, error) {
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*TokenResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockClient) RevokeToken(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockClient) RateLimit(ctx context.Context, token string) (RateLimitInfo, error) {
	args := m.Called(ctx, token)
	if info := args.Get(0); info != nil {
		return info.(RateLimitInfo), args.Error(1) //nolint:errcheck // mock type assertion
	}
	return RateLimitInfo{}, args.Error(1)
}

//nolint:errcheck // mock: panic on unexpected type is intentional
func (m *MockClient) GetContents(ctx context.Context, repository string, path string) ([]byte, error) {
	args := m.Called(ctx, repository, path)
	if data := args.Get(0); data != nil {
		return data.([]byte), args.Error(1)
	}
	return nil, args.Error(1)
}
