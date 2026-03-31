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

package config

import "errors"

const (
	// KeySelectorType is the Viper key for the selector store type (memory, redis, or dynamodb).
	KeySelectorType = "selector.type"
	// KeySelectorRedisAddress is the Viper key for the Redis server address.
	KeySelectorRedisAddress = "selector.redis.address"
	// KeySelectorRedisPassword is the Viper key for the Redis password.
	KeySelectorRedisPassword = "selector.redis.password" // #nosec G101 -- config key name, not a credential
	// KeySelectorRedisDB is the Viper key for the Redis database index.
	KeySelectorRedisDB = "selector.redis.db"
	// KeySelectorRedisTLS is the Viper key for enabling TLS to Redis.
	KeySelectorRedisTLS = "selector.redis.tls"
	// KeySelectorDynamoDBTableName is the Viper key for the selector DynamoDB table name.
	KeySelectorDynamoDBTableName = "selector.dynamodb.table_name"
	// KeySelectorDynamoDBTTLMinutes is the Viper key for selector DynamoDB entry TTL in minutes.
	KeySelectorDynamoDBTTLMinutes = "selector.dynamodb.ttl_minutes"
)

const (
	// DefaultSelectorStoreType is the default selector store when type is not set (in-memory).
	DefaultSelectorStoreType = SelectorStoreTypeMemory
	// DefaultRedisDB is the default Redis database index when not set.
	DefaultRedisDB = 0
)

const (
	// MaxSelectorDynamoDBTTLMinutes is the maximum TTL in minutes
	// for selector DynamoDB entries (24 hours).
	MaxSelectorDynamoDBTTLMinutes = 1440
)

var (
	// ErrInvalidSelectorStoreType is returned when the selector type
	// is not memory, redis, or dynamodb.
	ErrInvalidSelectorStoreType = errors.New("invalid selector store type")
	// ErrInvalidRedisConfig is returned when the selector type is redis but redis config is missing.
	ErrInvalidRedisConfig = errors.New("redis config is required")
	// ErrInvalidRedisAddress is returned when the Redis address is empty.
	ErrInvalidRedisAddress = errors.New("redis address is required")
	// ErrInvalidRedisDB is returned when the Redis DB index is negative.
	ErrInvalidRedisDB = errors.New("redis db must be positive")
	// ErrInvalidSelectorDynamoDBConfig is returned when the selector type
	// is dynamodb but dynamodb config is missing.
	ErrInvalidSelectorDynamoDBConfig = errors.New("selector dynamodb config is required")
	// ErrInvalidSelectorDynamoDBTable is returned when the selector DynamoDB table name is empty.
	ErrInvalidSelectorDynamoDBTable = errors.New("selector dynamodb table name is required")
	// ErrInvalidSelectorDynamoDBTTL is returned when TTL minutes are not in [0, 1440].
	ErrInvalidSelectorDynamoDBTTL = errors.New("selector dynamodb TTL minutes must be between 0 and 1440")
)

// SelectorStoreType identifies the session/store backend: memory, redis, or dynamodb.
type SelectorStoreType string

const (
	// SelectorStoreTypeMemory uses in-process memory (no persistence).
	SelectorStoreTypeMemory SelectorStoreType = "memory"
	// SelectorStoreTypeRedis uses Redis for session storage.
	SelectorStoreTypeRedis SelectorStoreType = "redis"
	// SelectorStoreTypeDynamoDB uses DynamoDB for session storage.
	SelectorStoreTypeDynamoDB SelectorStoreType = "dynamodb"
)

// RedisConfig holds Redis connection settings for the selector store.
type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password" json:"-"`
	DB       int    `mapstructure:"db"`
	TLS      bool   `mapstructure:"tls"`
}

// Validate validates the Redis configuration.
func (r *RedisConfig) Validate() error {
	if r.Address == "" {
		return ErrInvalidRedisAddress
	}
	if r.DB < 0 {
		return ErrInvalidRedisDB
	}
	return nil
}

// SelectorDynamoDBConfig holds DynamoDB table and TTL settings for the selector store.
type SelectorDynamoDBConfig struct {
	TableName  string `mapstructure:"table_name"`
	TTLMinutes int    `mapstructure:"ttl_minutes"`
}

// Validate validates the DynamoDB selector configuration.
func (d *SelectorDynamoDBConfig) Validate() error {
	if d.TableName == "" {
		return ErrInvalidSelectorDynamoDBTable
	}
	if d.TTLMinutes < 0 || d.TTLMinutes > MaxSelectorDynamoDBTTLMinutes {
		return ErrInvalidSelectorDynamoDBTTL
	}
	return nil
}

// SelectorConfig holds selector store configuration.
type SelectorConfig struct {
	Type     SelectorStoreType       `mapstructure:"type"`
	Redis    *RedisConfig            `mapstructure:"redis"`
	DynamoDB *SelectorDynamoDBConfig `mapstructure:"dynamodb"`
}

// Validate validates the selector configuration based on type.
func (c *SelectorConfig) Validate() error {
	if c.Type == "" {
		return nil
	}

	switch c.Type {
	case SelectorStoreTypeMemory:
		return nil
	case SelectorStoreTypeRedis:
		if c.Redis == nil {
			return ErrInvalidRedisConfig
		}
		return c.Redis.Validate()
	case SelectorStoreTypeDynamoDB:
		if c.DynamoDB == nil {
			return ErrInvalidSelectorDynamoDBConfig
		}
		return c.DynamoDB.Validate()
	default:
		return ErrInvalidSelectorStoreType
	}
}
