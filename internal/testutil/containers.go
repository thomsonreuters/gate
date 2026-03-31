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

//go:build integration

package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/docker/go-connections/nat"
	"github.com/golang-migrate/migrate/v4"
	migratePostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcLocalstack "github.com/testcontainers/testcontainers-go/modules/localstack"
	tcPostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcRedis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/thomsonreuters/gate/internal/db/migrations"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostgresContainer holds the GORM DB connection to a Testcontainers PostgreSQL instance.
type PostgresContainer struct {
	DB *gorm.DB
}

// NewPostgresContainer starts a PostgreSQL container and returns a
// PostgresContainer with a connected GORM DB.
// The container is terminated when the test finishes.
func NewPostgresContainer(t *testing.T) *PostgresContainer {
	t.Helper()
	ctx := context.Background()

	container, err := tcPostgres.Run(ctx,
		"postgres:16-alpine",
		tcPostgres.WithDatabase("gate_test"),
		tcPostgres.WithUsername("test"),
		tcPostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Discard,
	})
	require.NoError(t, err)

	return &PostgresContainer{DB: db}
}

// NewRedisContainer starts a Redis container and returns a connected client.
// The container is terminated when the test finishes.
func NewRedisContainer(t *testing.T) *redis.Client {
	t.Helper()
	ctx := context.Background()

	container, err := tcRedis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: endpoint})
	require.NoError(t, client.Ping(ctx).Err())
	t.Cleanup(func() { _ = client.Close() })

	return client
}

// NewDynamoDBContainer starts a LocalStack container with DynamoDB and returns
// a connected client. It creates the standard tables (logs, rate_limits).
// The container is terminated when the test finishes.
func NewDynamoDBContainer(t *testing.T) *dynamodb.Client {
	t.Helper()
	ctx := context.Background()

	container, err := tcLocalstack.Run(ctx,
		"localstack/localstack:latest",
		testcontainers.WithEnv(map[string]string{
			"SERVICES": "dynamodb",
		}),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready.").WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, nat.Port("4566/tcp"))
	require.NoError(t, err)

	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "test")),
	)
	require.NoError(t, err)

	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = &endpoint
	})

	createDynamoDBTables(t, ctx, client)

	return client
}

func createDynamoDBTables(t *testing.T, ctx context.Context, client *dynamodb.Client) {
	t.Helper()

	auditTable := "audit_logs"
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: &auditTable,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: new("request_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: new("timestamp"), AttributeType: types.ScalarAttributeTypeN},
			{AttributeName: new("caller"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: new("target_repository"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: new("request_id"), KeyType: types.KeyTypeHash},
			{AttributeName: new("timestamp"), KeyType: types.KeyTypeRange},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: new("CallerIndex"),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: new("caller"), KeyType: types.KeyTypeHash},
					{AttributeName: new("timestamp"), KeyType: types.KeyTypeRange},
				},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
			},
			{
				IndexName: new("RepositoryIndex"),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: new("target_repository"), KeyType: types.KeyTypeHash},
					{AttributeName: new("timestamp"), KeyType: types.KeyTypeRange},
				},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	require.NoError(t, err, "creating audit_logs table")

	rateLimitsTable := "rate_limits"
	_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: &rateLimitsTable,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: new("client_id"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: new("client_id"), KeyType: types.KeyTypeHash},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	require.NoError(t, err, "creating rate_limits table")
}

// RunPostgresMigrations applies all embedded SQL migrations against the given
// GORM database using golang-migrate. Use this after NewPostgresContainer to
// set up the schema.
func RunPostgresMigrations(t *testing.T, db *gorm.DB) {
	t.Helper()

	sqlDB, err := db.DB()
	require.NoError(t, err)

	source, err := iofs.New(migrations.PostgresFS, "postgres")
	require.NoError(t, err)

	driver, err := migratePostgres.WithInstance(sqlDB, &migratePostgres.Config{})
	require.NoError(t, err)

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	require.NoError(t, err)

	err = m.Up()
	require.NoError(t, err)
}
