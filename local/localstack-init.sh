#!/bin/bash
# Copyright 2026 Thomson Reuters
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

echo "Creating DynamoDB tables for local development..."

awslocal dynamodb create-table \
    --table-name audit_logs \
    --attribute-definitions \
        AttributeName=request_id,AttributeType=S \
        AttributeName=timestamp,AttributeType=N \
        AttributeName=caller,AttributeType=S \
        AttributeName=target_repository,AttributeType=S \
    --key-schema \
        AttributeName=request_id,KeyType=HASH \
        AttributeName=timestamp,KeyType=RANGE \
    --global-secondary-indexes \
        '[
            {
                "IndexName": "CallerIndex",
                "KeySchema": [
                    {"AttributeName": "caller", "KeyType": "HASH"},
                    {"AttributeName": "timestamp", "KeyType": "RANGE"}
                ],
                "Projection": {"ProjectionType": "ALL"}
            },
            {
                "IndexName": "RepositoryIndex",
                "KeySchema": [
                    {"AttributeName": "target_repository", "KeyType": "HASH"},
                    {"AttributeName": "timestamp", "KeyType": "RANGE"}
                ],
                "Projection": {"ProjectionType": "ALL"}
            }
        ]' \
    --billing-mode PAY_PER_REQUEST \
    --tags Key=Environment,Value=local

awslocal dynamodb create-table \
    --table-name rate_limits \
    --attribute-definitions \
        AttributeName=client_id,AttributeType=S \
    --key-schema \
        AttributeName=client_id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --tags Key=Environment,Value=local

echo "DynamoDB tables created successfully."
awslocal dynamodb list-tables
