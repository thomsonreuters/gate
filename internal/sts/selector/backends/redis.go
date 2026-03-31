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
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thomsonreuters/gate/internal/sts/selector"
)

const (
	// redisKeyPrefix is the prefix for Redis keys.
	redisKeyPrefix = "gate:ratelimit:"
	// ttlBuffer is added to key TTL so state remains readable slightly past the rate limit reset.
	ttlBuffer = 5 * time.Minute
	// scanCount is the batch size for SCAN when loading all keys.
	scanCount = 100
)

// setStateLua is the Lua script for atomic compare-and-swap SetState (see RedisStore).
//
//go:embed redis_setstate.lua
var setStateLua string

var setStateScript = redis.NewScript(setStateLua)

// RedisStore is a Redis-backed implementation of selector.Store.
//
// State is stored as Redis Hashes for efficient field access:
//
//	gate:ratelimit:{client_id} -> Hash{remaining, reset_at, last_updated}
//
// SetState uses a Lua script for atomic compare-and-swap to prevent stale
// writes from overwriting fresher state in multi-instance deployments.
// Keys automatically expire based on rate limit reset time plus a buffer.
type RedisStore struct {
	client redis.Cmdable
	prefix string
}

// NewRedisStore returns a new Redis store.
func NewRedisStore(client redis.Cmdable) *RedisStore {
	return &RedisStore{
		client: client,
		prefix: redisKeyPrefix,
	}
}

func (r *RedisStore) GetState(ctx context.Context, clientID string) (*selector.RateLimitState, error) {
	fields, err := r.client.HGetAll(ctx, r.key(clientID)).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall: %w", err)
	}
	if len(fields) == 0 {
		return nil, selector.ErrStateNotFound
	}
	return unmarshalHash(fields)
}

func (r *RedisStore) SetState(ctx context.Context, clientID string, state *selector.RateLimitState) error {
	if state == nil {
		return nil
	}

	duration := time.Until(state.ResetAt) + ttlBuffer
	duration = max(duration, ttlBuffer)
	ttl := int(duration.Seconds())

	err := setStateScript.Run(ctx, r.client, []string{r.key(clientID)},
		state.ResetAt.Unix(),
		state.Remaining,
		state.LastUpdated.Unix(),
		ttl,
	).Err()
	if err != nil {
		return fmt.Errorf("setstate lua: %w", err)
	}
	return nil
}

func (r *RedisStore) GetAllStates(ctx context.Context) (map[string]*selector.RateLimitState, error) {
	var keys []string
	var cursor uint64

	for {
		batch, nextCursor, err := r.client.Scan(ctx, cursor, r.prefix+"*", scanCount).Result()
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		keys = append(keys, batch...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if len(keys) == 0 {
		return map[string]*selector.RateLimitState{}, nil
	}

	pipe := r.client.Pipeline()
	commands := make([]*redis.MapStringStringCmd, len(keys))
	for i, key := range keys {
		commands[i] = pipe.HGetAll(ctx, key)
	}

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("pipeline exec: %w", err)
	}

	states := make(map[string]*selector.RateLimitState, len(keys))
	for i, cmd := range commands {
		fields, err := cmd.Result()
		if err != nil || len(fields) == 0 {
			continue
		}
		state, err := unmarshalHash(fields)
		if err != nil {
			slog.WarnContext(ctx, "skipping malformed Redis hash", "key", keys[i], "error", err)
			continue
		}
		clientID := strings.TrimPrefix(keys[i], r.prefix)
		states[clientID] = state
	}

	return states, nil
}

func (r *RedisStore) Close() error {
	if c, ok := r.client.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// key returns the Redis key for a client ID.
func (r *RedisStore) key(clientID string) string {
	return r.prefix + clientID
}

// unmarshalHash converts a Redis hash (remaining, reset_at, last_updated) to RateLimitState.
func unmarshalHash(fields map[string]string) (*selector.RateLimitState, error) {
	raw, ok := fields["remaining"]
	if !ok {
		return nil, errors.New("missing remaining field")
	}
	remaining, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid remaining: %w", err)
	}

	raw, ok = fields["reset_at"]
	if !ok {
		return nil, errors.New("missing reset_at field")
	}
	resetAt, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid reset_at: %w", err)
	}

	raw, ok = fields["last_updated"]
	if !ok {
		return nil, errors.New("missing last_updated field")
	}
	lastUpdated, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid last_updated: %w", err)
	}

	return &selector.RateLimitState{
		Remaining:   remaining,
		ResetAt:     time.Unix(resetAt, 0),
		LastUpdated: time.Unix(lastUpdated, 0),
	}, nil
}
