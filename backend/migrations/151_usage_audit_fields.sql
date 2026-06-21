-- Usage audit fields for Sub2API fork.
-- Successful requests extend usage_logs; failed requests extend ops_error_logs.
-- Do not create a separate request log table.

ALTER TABLE usage_logs
  ADD COLUMN IF NOT EXISTS system_prompt_summary TEXT,
  ADD COLUMN IF NOT EXISTS system_prompt_text TEXT,
  ADD COLUMN IF NOT EXISTS developer_prompt_text TEXT,
  ADD COLUMN IF NOT EXISTS tool_names JSONB DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS tool_call_names JSONB DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS tool_calls_json JSONB DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS output_summary TEXT,
  ADD COLUMN IF NOT EXISTS output_text TEXT,
  ADD COLUMN IF NOT EXISTS request_body_sha256 VARCHAR(64),
  ADD COLUMN IF NOT EXISTS request_body_bytes INTEGER,
  ADD COLUMN IF NOT EXISTS request_body_truncated BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS response_text_truncated BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS turn_metadata JSONB DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS ip_location JSONB DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS audit_capture_version SMALLINT NOT NULL DEFAULT 1;

ALTER TABLE ops_error_logs
  ADD COLUMN IF NOT EXISTS request_prompt TEXT,
  ADD COLUMN IF NOT EXISTS system_prompt_summary TEXT,
  ADD COLUMN IF NOT EXISTS system_prompt_text TEXT,
  ADD COLUMN IF NOT EXISTS developer_prompt_text TEXT,
  ADD COLUMN IF NOT EXISTS tool_names JSONB DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS tool_call_names JSONB DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS output_summary TEXT,
  ADD COLUMN IF NOT EXISTS request_body_sha256 VARCHAR(64),
  ADD COLUMN IF NOT EXISTS request_body_bytes INTEGER,
  ADD COLUMN IF NOT EXISTS request_body_truncated BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS turn_metadata JSONB DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS ip_location JSONB DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS audit_capture_version SMALLINT NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS usage_logs_request_body_sha256_idx
  ON usage_logs (request_body_sha256)
  WHERE request_body_sha256 IS NOT NULL;

CREATE INDEX IF NOT EXISTS ops_error_logs_request_body_sha256_idx
  ON ops_error_logs (request_body_sha256)
  WHERE request_body_sha256 IS NOT NULL;
