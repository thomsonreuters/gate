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

package backends

import (
	"context"
	"sync"

	"github.com/thomsonreuters/gate/internal/sts/selector"
)

// MemoryStore is an in-memory implementation of selector.Store.
type MemoryStore struct {
	mu     sync.RWMutex
	states map[string]*selector.RateLimitState
}

// NewMemoryStore returns a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		states: make(map[string]*selector.RateLimitState),
	}
}

func (m *MemoryStore) GetState(_ context.Context, clientID string) (*selector.RateLimitState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.states[clientID]
	if !ok || state == nil {
		return nil, selector.ErrStateNotFound
	}
	return copyState(state), nil
}

func (m *MemoryStore) SetState(_ context.Context, clientID string, state *selector.RateLimitState) error {
	if state == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing := m.states[clientID]; !state.IsFresherThan(existing) {
		return nil
	}
	m.states[clientID] = copyState(state)
	return nil
}

func (m *MemoryStore) GetAllStates(_ context.Context) (map[string]*selector.RateLimitState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]*selector.RateLimitState, len(m.states))
	for k, v := range m.states {
		if v != nil {
			out[k] = copyState(v)
		}
	}
	return out, nil
}

func (m *MemoryStore) Close() error {
	return nil
}

// copyState returns a shallow copy of the rate limit state, or nil if s is nil.
func copyState(s *selector.RateLimitState) *selector.RateLimitState {
	if s == nil {
		return nil
	}
	return &selector.RateLimitState{
		Remaining:   s.Remaining,
		ResetAt:     s.ResetAt,
		LastUpdated: s.LastUpdated,
	}
}
