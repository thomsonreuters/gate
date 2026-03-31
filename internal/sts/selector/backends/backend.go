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

// Package backends provides selector store implementations.
package backends

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/thomsonreuters/gate/internal/config"
	"github.com/thomsonreuters/gate/internal/db"
	"github.com/thomsonreuters/gate/internal/sts/selector"
)

// NewStore creates a selector.Store from the application config.
func NewStore(ctx context.Context, cfg *config.Config) (selector.Store, error) {
	switch cfg.Selector.Type {
	case config.SelectorStoreTypeMemory:
		return NewMemoryStore(), nil
	case config.SelectorStoreTypeRedis:
		redisCfg := cfg.Selector.Redis
		opts := &redis.Options{
			Addr:     redisCfg.Address,
			Password: redisCfg.Password,
			DB:       redisCfg.DB,
		}
		if redisCfg.TLS {
			opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		return NewRedisStore(redis.NewClient(opts)), nil
	case config.SelectorStoreTypeDynamoDB:
		client, err := db.NewDynamoDB(ctx, cfg.AWSRegion)
		if err != nil {
			return nil, fmt.Errorf("creating dynamodb client: %w", err)
		}
		return NewDynamoDBStore(client, cfg.Selector.DynamoDB.TableName, cfg.Selector.DynamoDB.TTLMinutes), nil
	default:
		return nil, fmt.Errorf("unknown selector store type: %s", cfg.Selector.Type)
	}
}
