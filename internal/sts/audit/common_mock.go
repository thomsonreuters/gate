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

package audit

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockBackend is a testify mock implementing AuditEntryBackend.
type MockBackend struct {
	mock.Mock
}

func (m *MockBackend) Log(ctx context.Context, entry *AuditEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockBackend) Close() error {
	args := m.Called()
	return args.Error(0)
}
