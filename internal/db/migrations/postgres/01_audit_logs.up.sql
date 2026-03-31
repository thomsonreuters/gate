-- Copyright 2026 Thomson Reuters
--
-- Licensed under the Apache License, Version 2.0 (the "License");
-- you may not use this file except in compliance with the License.
-- You may obtain a copy of the License at
--
--     http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software
-- distributed under the License is distributed on an "AS IS" BASIS,
-- WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
-- See the License for the specific language governing permissions and
-- limitations under the License.

CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(255) NOT NULL,
    timestamp BIGINT NOT NULL,
    caller TEXT NOT NULL,
    claims JSONB,
    target_repository VARCHAR(255) NOT NULL,
    policy_name VARCHAR(255) NOT NULL,
    permissions JSONB,
    outcome VARCHAR(50) NOT NULL,
    deny_reason VARCHAR(255),
    token_hash VARCHAR(128),
    ttl INTEGER,
    github_client_id VARCHAR(50),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_audit_logs_request_id ON audit_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_logs_caller ON audit_logs(caller);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target_repository ON audit_logs(target_repository);
