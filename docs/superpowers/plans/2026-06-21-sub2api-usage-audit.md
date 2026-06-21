# Sub2API Usage Audit Enhancements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 Usage 使用记录体系里增强请求审计能力，让成功请求写入 `usage_logs`，失败请求写入 `ops_error_logs`，管理员可在使用记录页查看摘要、预览完整上下文并导出。

**Architecture:** 后端新增一个窄接口的 audit extractor，只负责从 request body、headers、response/SSE 事件里提取结构化审计字段。成功链路把 audit payload 随现有 usage log 异步写入 `usage_logs`，失败链路把同类字段随 ops error logger 写入 `ops_error_logs`。前端继续沿用 `UsageView` / `UsageTable`，新增可隐藏摘要列、预览弹窗和导出字段，不新建独立请求日志中心。

**Tech Stack:** Go 1.x、Gin、Ent、PostgreSQL JSONB、Vue 3、TypeScript、pnpm、Vitest、Docker Compose、Tailscale。

---

## File Structure

### Backend schema and migrations

- Modify: `backend/ent/schema/usage_log.go`
- Modify: `backend/ent/schema/ops_error_log.go`
- Create: `backend/migrations/146_usage_audit_fields.sql`
- Create: `backend/internal/service/usage_audit.go`
- Create: `backend/internal/service/usage_audit_test.go`

`usage_audit.go` 只做提取和截断，不依赖 repository，不写库。

### Backend write path

- Modify: `backend/internal/service/usage_log.go`
- Modify: `backend/internal/service/usage_service.go`
- Modify: `backend/internal/repository/usage_log_repo.go`
- Modify: `backend/internal/handler/gateway_handler_responses.go`
- Modify: `backend/internal/handler/gateway_handler_chat_completions.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Modify: `backend/internal/handler/openai_gateway_chat_completions.go`
- Modify: `backend/internal/handler/ops_error_logger.go`
- Modify: `backend/internal/service/ops_models.go`
- Modify: `backend/internal/service/ops_service.go`
- Modify: `backend/internal/repository/ops_repo.go`

### Backend DTO/API

- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Test: `backend/internal/handler/dto/mappers_usage_test.go`

### Frontend usage view

- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/views/admin/UsageView.vue`
- Modify: `frontend/src/components/admin/usage/UsageTable.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`
- Test: `frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts`
- Test: `frontend/src/views/admin/__tests__/UsageView.spec.ts`

### Runtime verification

- Use: `docker-compose.yml`
- Current local URL: `http://127.0.0.1:8420`
- Current observed Tailscale IP: `100.126.43.55`
- Tailscale URL pattern: `http://$(tailscale ip -4):8420`

Before external testing, run `tailscale ip -4` again and use the live value.

---

### Task 1: Add audit fields to `usage_logs` and `ops_error_logs`

**Files:**
- Modify: `backend/ent/schema/usage_log.go`
- Modify: `backend/ent/schema/ops_error_log.go`
- Create: `backend/migrations/146_usage_audit_fields.sql`
- Test: `backend/internal/repository/migrations_schema_integration_test.go`

- [ ] **Step 1: Write the migration**

Create `backend/migrations/146_usage_audit_fields.sql`:

```sql
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
  ADD COLUMN IF NOT EXISTS request_body_truncated BOOLEAN DEFAULT false,
  ADD COLUMN IF NOT EXISTS response_text_truncated BOOLEAN DEFAULT false,
  ADD COLUMN IF NOT EXISTS turn_metadata JSONB DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS ip_location JSONB DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS audit_capture_version SMALLINT DEFAULT 1;

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
  ADD COLUMN IF NOT EXISTS request_body_truncated BOOLEAN DEFAULT false,
  ADD COLUMN IF NOT EXISTS turn_metadata JSONB DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS ip_location JSONB DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS audit_capture_version SMALLINT DEFAULT 1;

CREATE INDEX IF NOT EXISTS usage_logs_request_body_sha256_idx
  ON usage_logs (request_body_sha256)
  WHERE request_body_sha256 IS NOT NULL;

CREATE INDEX IF NOT EXISTS ops_error_logs_request_body_sha256_idx
  ON ops_error_logs (request_body_sha256)
  WHERE request_body_sha256 IS NOT NULL;
```

- [ ] **Step 2: Update Ent schema for `usage_logs`**

In `backend/ent/schema/usage_log.go`, after `field.Text("request_prompt")...`, add:

```go
field.Text("system_prompt_summary").Optional().Nillable(),
field.Text("system_prompt_text").Optional().Nillable(),
field.Text("developer_prompt_text").Optional().Nillable(),
field.JSON("tool_names", []string{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
field.JSON("tool_call_names", []string{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
field.JSON("tool_calls_json", []map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
field.Text("output_summary").Optional().Nillable(),
field.Text("output_text").Optional().Nillable(),
field.String("request_body_sha256").MaxLen(64).Optional().Nillable(),
field.Int("request_body_bytes").Optional().Nillable(),
field.Bool("request_body_truncated").Default(false),
field.Bool("response_text_truncated").Default(false),
field.JSON("turn_metadata", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
field.JSON("ip_location", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
field.Int16("audit_capture_version").Default(1),
```

In `Indexes()`, add:

```go
index.Fields("request_body_sha256"),
```

- [ ] **Step 3: Update Ent schema for `ops_error_logs`**

Open `backend/ent/schema/ops_error_log.go` and add the same audit fields that apply to failed requests:

```go
field.Text("request_prompt").Optional().Nillable(),
field.Text("system_prompt_summary").Optional().Nillable(),
field.Text("system_prompt_text").Optional().Nillable(),
field.Text("developer_prompt_text").Optional().Nillable(),
field.JSON("tool_names", []string{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
field.JSON("tool_call_names", []string{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
field.Text("output_summary").Optional().Nillable(),
field.String("request_body_sha256").MaxLen(64).Optional().Nillable(),
field.Int("request_body_bytes").Optional().Nillable(),
field.Bool("request_body_truncated").Default(false),
field.JSON("turn_metadata", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
field.JSON("ip_location", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
field.Int16("audit_capture_version").Default(1),
```

Add an index on `request_body_sha256`.

- [ ] **Step 4: Generate Ent code**

Run:

```bash
cd backend
go generate ./ent
```

Expected: command exits `0`, and generated files under `backend/ent/` include the new fields.

- [ ] **Step 5: Run schema-related tests**

Run:

```bash
cd backend
go test -tags=integration ./internal/repository -run TestMigrationsSchema -count=1
```

Expected: PASS. If no exact test name matches, run:

```bash
cd backend
go test -tags=integration ./internal/repository -run Migration -count=1
```

- [ ] **Step 6: Commit schema work**

```bash
git add backend/ent backend/migrations/146_usage_audit_fields.sql
git commit -m "feat: add usage audit storage fields"
```

---

### Task 2: Create the request/response audit extractor

**Files:**
- Create: `backend/internal/service/usage_audit.go`
- Create: `backend/internal/service/usage_audit_test.go`

- [ ] **Step 1: Write extractor tests first**

Create `backend/internal/service/usage_audit_test.go`:

```go
package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractUsageAuditFromResponsesRequest(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.1-codex",
		"instructions":"You are Codex. Use tools carefully.",
		"tools":[{"type":"function","name":"exec_command"},{"type":"function","name":"mcp__context7__query_docs"}],
		"input":[{"role":"user","content":[{"type":"input_text","text":"帮我看使用记录"}]}]
	}`)

	audit := ExtractUsageAuditFromRequest(body, map[string]string{
		"X-Codex-Turn-Metadata": `{"session_id":"s1","thread_id":"t1","turn_id":"u1"}`,
	})

	require.Equal(t, "帮我看使用记录", audit.RequestPrompt)
	require.Contains(t, audit.SystemPromptText, "You are Codex")
	require.Equal(t, []string{"exec_command", "mcp__context7__query_docs"}, audit.ToolNames)
	require.Equal(t, "s1", audit.TurnMetadata["session_id"])
	require.Len(t, audit.RequestBodySHA256, 64)
	require.Equal(t, len(body), audit.RequestBodyBytes)
}

func TestExtractUsageAuditFromChatCompletionsRequest(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4.1",
		"messages":[
			{"role":"system","content":"System text"},
			{"role":"developer","content":"Developer text"},
			{"role":"user","content":"User asks"}
		],
		"tools":[{"type":"function","function":{"name":"search_docs"}}]
	}`)

	audit := ExtractUsageAuditFromRequest(body, nil)

	require.Equal(t, "User asks", audit.RequestPrompt)
	require.Equal(t, "System text", audit.SystemPromptText)
	require.Equal(t, "Developer text", audit.DeveloperPromptText)
	require.Equal(t, []string{"search_docs"}, audit.ToolNames)
}

func TestApplyUsageAuditResponseCapturesToolCallsAndOutput(t *testing.T) {
	audit := UsageAuditPayload{}
	ApplyUsageAuditResponse(&audit, []byte(`{
		"output":[
			{"type":"message","content":[{"type":"output_text","text":"hello world"}]},
			{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"pwd\"}"}
		]
	}`))

	require.Equal(t, "hello world", audit.OutputText)
	require.Equal(t, "hello world", audit.OutputSummary)
	require.Equal(t, []string{"exec_command"}, audit.ToolCallNames)
	require.Len(t, audit.ToolCalls, 1)
}

func TestAuditTruncatesAndHashesLargeText(t *testing.T) {
	large := make([]byte, MaxAuditTextBytes+128)
	for i := range large {
		large[i] = 'a'
	}
	body, err := json.Marshal(map[string]any{"prompt": string(large)})
	require.NoError(t, err)

	audit := ExtractUsageAuditFromRequest(body, nil)

	require.True(t, audit.RequestBodyTruncated)
	require.LessOrEqual(t, len([]byte(audit.RequestPrompt)), MaxAuditTextBytes)
	require.Len(t, audit.RequestBodySHA256, 64)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
go test ./internal/service -run 'TestExtractUsageAudit|TestApplyUsageAudit|TestAuditTruncates' -count=1
```

Expected: FAIL with undefined `ExtractUsageAuditFromRequest`, `UsageAuditPayload`, or `MaxAuditTextBytes`.

- [ ] **Step 3: Implement extractor**

Create `backend/internal/service/usage_audit.go`:

```go
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
)

const (
	AuditCaptureVersion = int16(1)
	MaxAuditTextBytes   = 64 * 1024
	MaxSystemTextBytes  = 128 * 1024
	MaxToolCallsBytes   = 32 * 1024
)

type ToolCallAudit struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

type UsageAuditPayload struct {
	RequestPrompt        string
	SystemPromptSummary  string
	SystemPromptText     string
	DeveloperPromptText  string
	ToolNames            []string
	ToolCallNames        []string
	ToolCalls            []ToolCallAudit
	OutputSummary        string
	OutputText           string
	RequestBodySHA256    string
	RequestBodyBytes     int
	RequestBodyTruncated bool
	ResponseTextTruncated bool
	TurnMetadata         map[string]any
	IPLocation           map[string]any
	AuditCaptureVersion  int16
}

func ExtractUsageAuditFromRequest(body []byte, headers map[string]string) UsageAuditPayload {
	audit := UsageAuditPayload{
		RequestPrompt:       ExtractRequestPrompt(body, nil),
		RequestBodySHA256:   sha256Hex(body),
		RequestBodyBytes:    len(body),
		TurnMetadata:        extractTurnMetadata(headers),
		IPLocation:          map[string]any{},
		AuditCaptureVersion: AuditCaptureVersion,
	}

	audit.SystemPromptText, audit.DeveloperPromptText = extractSystemAndDeveloperText(body)
	audit.SystemPromptSummary = summarizeAuditText(audit.SystemPromptText, 240)
	audit.ToolNames = uniqueStrings(extractToolNames(body))

	audit.RequestPrompt, audit.RequestBodyTruncated = truncateAuditText(audit.RequestPrompt, MaxAuditTextBytes)
	audit.SystemPromptText, _ = truncateAuditText(audit.SystemPromptText, MaxSystemTextBytes)
	audit.DeveloperPromptText, _ = truncateAuditText(audit.DeveloperPromptText, MaxSystemTextBytes)
	return audit
}

func ApplyUsageAuditResponse(audit *UsageAuditPayload, body []byte) {
	if audit == nil || len(body) == 0 {
		return
	}
	text := extractOutputText(body)
	audit.OutputText, audit.ResponseTextTruncated = truncateAuditText(text, MaxAuditTextBytes)
	audit.OutputSummary = summarizeAuditText(audit.OutputText, 240)
	audit.ToolCalls = truncateToolCalls(extractToolCalls(body))
	names := make([]string, 0, len(audit.ToolCalls))
	for _, call := range audit.ToolCalls {
		if strings.TrimSpace(call.Name) != "" {
			names = append(names, strings.TrimSpace(call.Name))
		}
	}
	audit.ToolCallNames = uniqueStrings(names)
}

func ApplyUsageAuditSSEEvent(audit *UsageAuditPayload, eventName string, data []byte) {
	if audit == nil || len(data) == 0 {
		return
	}
	result := gjson.ParseBytes(data)
	name := strings.TrimSpace(result.Get("item.name").String())
	if name == "" {
		name = strings.TrimSpace(result.Get("name").String())
	}
	if name != "" && strings.Contains(eventName, "output_item") {
		audit.ToolCalls = truncateToolCalls(append(audit.ToolCalls, ToolCallAudit{Name: name}))
		audit.ToolCallNames = uniqueStrings(append(audit.ToolCallNames, name))
	}
	if delta := strings.TrimSpace(result.Get("delta").String()); delta != "" {
		next := audit.OutputText + delta
		audit.OutputText, audit.ResponseTextTruncated = truncateAuditText(next, MaxAuditTextBytes)
		audit.OutputSummary = summarizeAuditText(audit.OutputText, 240)
	}
}

func extractSystemAndDeveloperText(body []byte) (string, string) {
	var systemParts []string
	var developerParts []string
	if instructions := strings.TrimSpace(gjson.GetBytes(body, "instructions").String()); instructions != "" {
		systemParts = append(systemParts, instructions)
	}
	msgs := gjson.GetBytes(body, "messages")
	if msgs.IsArray() {
		msgs.ForEach(func(_, msg gjson.Result) bool {
			role := strings.TrimSpace(msg.Get("role").String())
			content := strings.TrimSpace(extractTextFromContent(msg.Get("content").Value()))
			switch role {
			case "system":
				if content != "" {
					systemParts = append(systemParts, content)
				}
			case "developer":
				if content != "" {
					developerParts = append(developerParts, content)
				}
			}
			return true
		})
	}
	return strings.Join(systemParts, "\n\n"), strings.Join(developerParts, "\n\n")
}

func extractToolNames(body []byte) []string {
	var names []string
	tools := gjson.GetBytes(body, "tools")
	if tools.IsArray() {
		tools.ForEach(func(_, tool gjson.Result) bool {
			for _, path := range []string{"name", "function.name"} {
				if name := strings.TrimSpace(tool.Get(path).String()); name != "" {
					names = append(names, name)
					return true
				}
			}
			return true
		})
	}
	functions := gjson.GetBytes(body, "functions")
	if functions.IsArray() {
		functions.ForEach(func(_, fn gjson.Result) bool {
			if name := strings.TrimSpace(fn.Get("name").String()); name != "" {
				names = append(names, name)
			}
			return true
		})
	}
	return names
}

func extractOutputText(body []byte) string {
	var parts []string
	for _, path := range []string{"output_text", "choices.0.message.content", "content.0.text"} {
		if text := strings.TrimSpace(gjson.GetBytes(body, path).String()); text != "" {
			parts = append(parts, text)
		}
	}
	output := gjson.GetBytes(body, "output")
	if output.IsArray() {
		output.ForEach(func(_, item gjson.Result) bool {
			content := item.Get("content")
			if content.IsArray() {
				content.ForEach(func(_, part gjson.Result) bool {
					if text := strings.TrimSpace(part.Get("text").String()); text != "" {
						parts = append(parts, text)
					}
					return true
				})
			}
			return true
		})
	}
	return strings.Join(parts, "\n")
}

func extractToolCalls(body []byte) []ToolCallAudit {
	var calls []ToolCallAudit
	output := gjson.GetBytes(body, "output")
	if output.IsArray() {
		output.ForEach(func(_, item gjson.Result) bool {
			if name := strings.TrimSpace(item.Get("name").String()); name != "" {
				calls = append(calls, ToolCallAudit{Name: name, Arguments: truncateStringBytes(item.Get("arguments").String(), MaxToolCallsBytes)})
			}
			return true
		})
	}
	toolCalls := gjson.GetBytes(body, "choices.0.message.tool_calls")
	if toolCalls.IsArray() {
		toolCalls.ForEach(func(_, item gjson.Result) bool {
			if name := strings.TrimSpace(item.Get("function.name").String()); name != "" {
				calls = append(calls, ToolCallAudit{Name: name, Arguments: truncateStringBytes(item.Get("function.arguments").String(), MaxToolCallsBytes)})
			}
			return true
		})
	}
	return calls
}

func extractTurnMetadata(headers map[string]string) map[string]any {
	if len(headers) == 0 {
		return map[string]any{}
	}
	raw := strings.TrimSpace(headers["X-Codex-Turn-Metadata"])
	if raw == "" {
		raw = strings.TrimSpace(headers["x-codex-turn-metadata"])
	}
	if raw == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func truncateAuditText(value string, maxBytes int) (string, bool) {
	if len([]byte(value)) <= maxBytes {
		return value, false
	}
	return truncateStringBytes(value, maxBytes), true
}

func truncateStringBytes(value string, maxBytes int) string {
	if maxBytes <= 0 || len([]byte(value)) <= maxBytes {
		return value
	}
	b := []byte(value)
	for maxBytes > 0 && (b[maxBytes]&0xC0) == 0x80 {
		maxBytes--
	}
	return string(b[:maxBytes])
}

func summarizeAuditText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func truncateToolCalls(calls []ToolCallAudit) []ToolCallAudit {
	encoded, err := json.Marshal(calls)
	if err == nil && len(encoded) <= MaxToolCallsBytes {
		return calls
	}
	out := make([]ToolCallAudit, 0, len(calls))
	total := 2
	for _, call := range calls {
		encodedCall, err := json.Marshal(call)
		if err != nil {
			continue
		}
		if total+len(encodedCall)+1 > MaxToolCallsBytes {
			break
		}
		out = append(out, call)
		total += len(encodedCall) + 1
	}
	return out
}
```

- [ ] **Step 4: Run extractor tests**

Run:

```bash
cd backend
go test ./internal/service -run 'TestExtractUsageAudit|TestApplyUsageAudit|TestAuditTruncates' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit extractor**

```bash
git add backend/internal/service/usage_audit.go backend/internal/service/usage_audit_test.go
git commit -m "feat: extract usage audit metadata"
```

---

### Task 3: Persist audit fields for successful usage records

**Files:**
- Modify: `backend/internal/service/usage_log.go`
- Modify: `backend/internal/service/usage_service.go`
- Modify: `backend/internal/repository/usage_log_repo.go`
- Modify: gateway handlers that already pass `RequestPrompt`
- Test: `backend/internal/service/openai_gateway_record_usage_test.go`
- Test: `backend/internal/repository/usage_log_repo_request_type_test.go`

- [ ] **Step 1: Extend service structs**

In `backend/internal/service/usage_log.go`, add these fields to `UsageLog` after `RequestPrompt *string`:

```go
SystemPromptSummary   *string
SystemPromptText      *string
DeveloperPromptText   *string
ToolNames             []string
ToolCallNames         []string
ToolCallsJSON         []ToolCallAudit
OutputSummary         *string
OutputText            *string
RequestBodySHA256     *string
RequestBodyBytes      *int
RequestBodyTruncated  bool
ResponseTextTruncated bool
TurnMetadata          map[string]any
IPLocation            map[string]any
AuditCaptureVersion   int16
```

In `backend/internal/service/usage_service.go`, add the same fields to `CreateUsageLogRequest`.

- [ ] **Step 2: Add a helper to copy audit payload**

In `backend/internal/service/usage_service.go`, add:

```go
func ApplyUsageAuditToCreateRequest(req *CreateUsageLogRequest, audit UsageAuditPayload) {
	if req == nil {
		return
	}
	if audit.RequestPrompt != "" {
		req.RequestPrompt = &audit.RequestPrompt
	}
	if audit.SystemPromptSummary != "" {
		req.SystemPromptSummary = &audit.SystemPromptSummary
	}
	if audit.SystemPromptText != "" {
		req.SystemPromptText = &audit.SystemPromptText
	}
	if audit.DeveloperPromptText != "" {
		req.DeveloperPromptText = &audit.DeveloperPromptText
	}
	if audit.OutputSummary != "" {
		req.OutputSummary = &audit.OutputSummary
	}
	if audit.OutputText != "" {
		req.OutputText = &audit.OutputText
	}
	if audit.RequestBodySHA256 != "" {
		req.RequestBodySHA256 = &audit.RequestBodySHA256
	}
	if audit.RequestBodyBytes > 0 {
		req.RequestBodyBytes = &audit.RequestBodyBytes
	}
	req.ToolNames = audit.ToolNames
	req.ToolCallNames = audit.ToolCallNames
	req.ToolCallsJSON = audit.ToolCalls
	req.RequestBodyTruncated = audit.RequestBodyTruncated
	req.ResponseTextTruncated = audit.ResponseTextTruncated
	req.TurnMetadata = audit.TurnMetadata
	req.IPLocation = audit.IPLocation
	if audit.AuditCaptureVersion > 0 {
		req.AuditCaptureVersion = audit.AuditCaptureVersion
	}
}
```

- [ ] **Step 3: Update repository insert/select columns**

In `backend/internal/repository/usage_log_repo.go`, extend `usageLogSelectColumns` by inserting after `request_prompt`:

```sql
system_prompt_summary, system_prompt_text, developer_prompt_text, tool_names, tool_call_names, tool_calls_json, output_summary, output_text, request_body_sha256, request_body_bytes, request_body_truncated, response_text_truncated, turn_metadata, ip_location, audit_capture_version
```

Update scan destinations in the row scanner to include these nullable strings, JSON raw fields, nullable ints and booleans. Decode JSONB fields into service struct fields with a helper:

```go
func scanJSONBytes[T any](raw []byte, fallback T) T {
	if len(raw) == 0 {
		return fallback
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return fallback
	}
	return out
}
```

Update all insert builders that currently include `request_prompt` to include the new columns and argument values.

- [ ] **Step 4: Pass audit payload from OpenAI Responses handler**

In `backend/internal/handler/gateway_handler_responses.go`, near the existing `RequestPrompt: service.ExtractRequestPrompt(body, parsedReq)`, create audit before forwarding:

```go
audit := service.ExtractUsageAuditFromRequest(body, map[string]string{
	"X-Codex-Turn-Metadata": c.GetHeader("X-Codex-Turn-Metadata"),
})
```

When response result/body is available, call:

```go
service.ApplyUsageAuditResponse(&audit, responseBody)
```

When creating `service.CreateUsageLogRequest`, call:

```go
service.ApplyUsageAuditToCreateRequest(&usageReq, audit)
```

Keep existing `RequestPrompt` fallback if a path already sets it.

- [ ] **Step 5: Repeat for Chat Completions and OpenAI gateway handlers**

Apply the same pattern in:

- `backend/internal/handler/gateway_handler_chat_completions.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/openai_gateway_chat_completions.go`

- [ ] **Step 6: Add persistence test**

In `backend/internal/service/openai_gateway_record_usage_test.go`, add a test that creates a request with `instructions`, `tools`, user input and response text, then asserts the submitted usage record contains:

```go
require.NotNil(t, got.RequestPrompt)
require.Contains(t, *got.RequestPrompt, "用户输入")
require.NotNil(t, got.SystemPromptSummary)
require.Contains(t, *got.SystemPromptSummary, "Codex")
require.Contains(t, got.ToolNames, "exec_command")
require.NotNil(t, got.RequestBodySHA256)
```

- [ ] **Step 7: Run targeted tests**

Run:

```bash
cd backend
go test ./internal/service -run 'TestOpenAIGateway.*Usage|TestExtractUsageAudit' -count=1
go test ./internal/repository -run 'Test.*UsageLog' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit usage persistence**

```bash
git add backend/internal/service backend/internal/repository/usage_log_repo.go backend/internal/handler/gateway_handler_responses.go backend/internal/handler/gateway_handler_chat_completions.go backend/internal/handler/openai_gateway_handler.go backend/internal/handler/openai_gateway_chat_completions.go
git commit -m "feat: persist usage audit metadata"
```

---

### Task 4: Persist audit fields for failed requests in ops logs

**Files:**
- Modify: `backend/internal/handler/ops_error_logger.go`
- Modify: `backend/internal/service/ops_models.go`
- Modify: `backend/internal/service/ops_service.go`
- Modify: `backend/internal/repository/ops_repo.go`
- Test: `backend/internal/handler/ops_error_logger_test.go`
- Test: `backend/internal/repository/ops_repo_request_details.go`

- [ ] **Step 1: Extend ops error model**

In `backend/internal/service/ops_models.go`, add audit fields to the ops error create/request model:

```go
RequestPrompt       *string
SystemPromptSummary *string
SystemPromptText    *string
DeveloperPromptText *string
ToolNames           []string
ToolCallNames       []string
OutputSummary       *string
RequestBodySHA256   *string
RequestBodyBytes    *int
RequestBodyTruncated bool
TurnMetadata        map[string]any
IPLocation          map[string]any
AuditCaptureVersion int16
```

- [ ] **Step 2: Capture request audit before forwarding can fail**

In handlers that call `ops_error_logger`, attach audit payload to context before upstream call:

```go
audit := service.ExtractUsageAuditFromRequest(body, map[string]string{
	"X-Codex-Turn-Metadata": c.GetHeader("X-Codex-Turn-Metadata"),
})
service.SetUsageAuditContext(c.Request.Context(), audit)
```

Add context helpers in `backend/internal/service/usage_audit.go`:

```go
type usageAuditContextKey struct{}

func ContextWithUsageAudit(ctx context.Context, audit UsageAuditPayload) context.Context {
	return context.WithValue(ctx, usageAuditContextKey{}, audit)
}

func UsageAuditFromContext(ctx context.Context) (UsageAuditPayload, bool) {
	audit, ok := ctx.Value(usageAuditContextKey{}).(UsageAuditPayload)
	return audit, ok
}
```

Import `context` in `usage_audit.go`.

- [ ] **Step 3: Apply audit in ops error logger**

In `backend/internal/handler/ops_error_logger.go`, when building the ops error payload:

```go
if audit, ok := service.UsageAuditFromContext(c.Request.Context()); ok {
	payload.RequestPrompt = stringPtr(audit.RequestPrompt)
	payload.SystemPromptSummary = stringPtr(audit.SystemPromptSummary)
	payload.SystemPromptText = stringPtr(audit.SystemPromptText)
	payload.DeveloperPromptText = stringPtr(audit.DeveloperPromptText)
	payload.ToolNames = audit.ToolNames
	payload.ToolCallNames = audit.ToolCallNames
	payload.OutputSummary = stringPtr(audit.OutputSummary)
	payload.RequestBodySHA256 = stringPtr(audit.RequestBodySHA256)
	if audit.RequestBodyBytes > 0 {
		payload.RequestBodyBytes = &audit.RequestBodyBytes
	}
	payload.RequestBodyTruncated = audit.RequestBodyTruncated
	payload.TurnMetadata = audit.TurnMetadata
	payload.IPLocation = audit.IPLocation
	payload.AuditCaptureVersion = audit.AuditCaptureVersion
}
```

Use existing pointer helper if present; otherwise add a private `stringPtr` helper.

- [ ] **Step 4: Update ops repository insert/select**

In `backend/internal/repository/ops_repo.go`, add the new ops fields to insert SQL and detail select SQL. Decode JSON fields with the same safe helper pattern used in usage repo.

- [ ] **Step 5: Add ops tests**

In `backend/internal/handler/ops_error_logger_test.go`, add a test that injects usage audit into request context, triggers an upstream error log, and asserts stored payload has:

```go
require.Contains(t, *got.RequestPrompt, "用户输入")
require.Contains(t, *got.SystemPromptSummary, "Codex")
require.Contains(t, got.ToolNames, "exec_command")
require.NotNil(t, got.RequestBodySHA256)
```

- [ ] **Step 6: Run ops tests**

Run:

```bash
cd backend
go test ./internal/handler -run 'TestOpsErrorLogger' -count=1
go test ./internal/repository -run 'Test.*Ops.*Request|Test.*Ops.*Error' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit ops persistence**

```bash
git add backend/internal/handler/ops_error_logger.go backend/internal/service/ops_models.go backend/internal/service/ops_service.go backend/internal/repository/ops_repo.go backend/internal/handler/ops_error_logger_test.go
git commit -m "feat: persist ops audit metadata"
```

---

### Task 5: Expose audit fields in admin Usage API

**Files:**
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Test: `backend/internal/handler/dto/mappers_usage_test.go`

- [ ] **Step 1: Extend DTO types**

In `backend/internal/handler/dto/types.go`, add to `UsageLog`:

```go
SystemPromptSummary   *string          `json:"system_prompt_summary,omitempty"`
SystemPromptText      *string          `json:"system_prompt_text,omitempty"`
DeveloperPromptText   *string          `json:"developer_prompt_text,omitempty"`
ToolNames             []string         `json:"tool_names,omitempty"`
ToolCallNames         []string         `json:"tool_call_names,omitempty"`
ToolCallsJSON         []service.ToolCallAudit `json:"tool_calls_json,omitempty"`
OutputSummary         *string          `json:"output_summary,omitempty"`
OutputText            *string          `json:"output_text,omitempty"`
RequestBodySHA256     *string          `json:"request_body_sha256,omitempty"`
RequestBodyBytes      *int             `json:"request_body_bytes,omitempty"`
RequestBodyTruncated  bool             `json:"request_body_truncated"`
ResponseTextTruncated bool             `json:"response_text_truncated"`
TurnMetadata          map[string]any   `json:"turn_metadata,omitempty"`
IPLocation            map[string]any   `json:"ip_location,omitempty"`
AuditCaptureVersion   int16            `json:"audit_capture_version,omitempty"`
```

If importing `service` in DTO creates a cycle, define a local DTO struct:

```go
type ToolCallAudit struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}
```

and map field-by-field.

- [ ] **Step 2: Map service fields to DTO**

In `backend/internal/handler/dto/mappers.go`, update `usageLogFromServiceUser`:

```go
SystemPromptSummary:   l.SystemPromptSummary,
SystemPromptText:      l.SystemPromptText,
DeveloperPromptText:   l.DeveloperPromptText,
ToolNames:             l.ToolNames,
ToolCallNames:         l.ToolCallNames,
ToolCallsJSON:         mapToolCalls(l.ToolCallsJSON),
OutputSummary:         l.OutputSummary,
OutputText:            l.OutputText,
RequestBodySHA256:     l.RequestBodySHA256,
RequestBodyBytes:      l.RequestBodyBytes,
RequestBodyTruncated:  l.RequestBodyTruncated,
ResponseTextTruncated: l.ResponseTextTruncated,
TurnMetadata:          l.TurnMetadata,
IPLocation:            l.IPLocation,
AuditCaptureVersion:   l.AuditCaptureVersion,
```

- [ ] **Step 3: Add mapper test**

In `backend/internal/handler/dto/mappers_usage_test.go`, add:

```go
func TestUsageLogFromServiceAdminIncludesAuditFields(t *testing.T) {
	summary := "system summary"
	prompt := "user prompt"
	output := "output summary"
	log := &service.UsageLog{
		ID: 1,
		RequestID: "req_1",
		RequestPrompt: &prompt,
		SystemPromptSummary: &summary,
		ToolNames: []string{"exec_command"},
		ToolCallNames: []string{"exec_command"},
		OutputSummary: &output,
		TurnMetadata: map[string]any{"thread_id":"t1"},
		AuditCaptureVersion: 1,
	}

	dto := UsageLogFromServiceAdmin(log)
	require.Equal(t, "user prompt", *dto.RequestPrompt)
	require.Equal(t, "system summary", *dto.SystemPromptSummary)
	require.Equal(t, []string{"exec_command"}, dto.ToolNames)
	require.Equal(t, "t1", dto.TurnMetadata["thread_id"])
}
```

- [ ] **Step 4: Run DTO tests**

Run:

```bash
cd backend
go test ./internal/handler/dto -run TestUsageLogFromServiceAdminIncludesAuditFields -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit API DTO work**

```bash
git add backend/internal/handler/dto/types.go backend/internal/handler/dto/mappers.go backend/internal/handler/dto/mappers_usage_test.go
git commit -m "feat: expose usage audit fields"
```

---

### Task 6: Enhance Usage table columns, preview modal, and export

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/views/admin/UsageView.vue`
- Modify: `frontend/src/components/admin/usage/UsageTable.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`
- Test: `frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts`
- Test: `frontend/src/views/admin/__tests__/UsageView.spec.ts`

- [ ] **Step 1: Extend frontend `AdminUsageLog` type**

In `frontend/src/types/index.ts`, add fields to `AdminUsageLog` or the shared usage log interface:

```ts
system_prompt_summary?: string | null
system_prompt_text?: string | null
developer_prompt_text?: string | null
tool_names?: string[]
tool_call_names?: string[]
tool_calls_json?: Array<{ name: string; arguments?: string }>
output_summary?: string | null
output_text?: string | null
request_body_sha256?: string | null
request_body_bytes?: number | null
request_body_truncated?: boolean
response_text_truncated?: boolean
turn_metadata?: Record<string, unknown>
ip_location?: Record<string, unknown>
audit_capture_version?: number
```

- [ ] **Step 2: Add columns to `UsageView`**

In `frontend/src/views/admin/UsageView.vue`, append these columns to `allColumns` after `request_prompt`:

```ts
{ key: 'system_prompt_summary', label: t('admin.usage.systemPrompt'), sortable: false },
{ key: 'tool_call_names', label: t('admin.usage.actualCalls'), sortable: false },
{ key: 'output_summary', label: t('admin.usage.outputSummary'), sortable: false },
{ key: 'actions', label: t('common.actions'), sortable: false }
```

Keep `system_prompt_summary`, `tool_call_names`, `output_summary` hidden by default by adding their keys to the initialized `hiddenColumns` set. Keep `actions` visible.

- [ ] **Step 3: Add export fields**

In `exportToExcel`, add headers:

```ts
t('admin.usage.systemPrompt'),
t('admin.usage.toolNames'),
t('admin.usage.actualCalls'),
t('admin.usage.outputSummary'),
t('admin.usage.requestBodyHash'),
t('admin.usage.truncated')
```

Add row values:

```ts
log.system_prompt_summary || '',
(log.tool_names || []).join(', '),
(log.tool_call_names || []).join(', '),
log.output_summary || '',
log.request_body_sha256 || '',
[log.request_body_truncated ? 'request' : '', log.response_text_truncated ? 'response' : ''].filter(Boolean).join(', ')
```

- [ ] **Step 4: Add preview modal state**

In `frontend/src/components/admin/usage/UsageTable.vue`, add:

```ts
const auditDialogVisible = ref(false)
const auditDialogLog = ref<AdminUsageLog | null>(null)

const openAuditDialog = (row: AdminUsageLog) => {
  auditDialogLog.value = row
  auditDialogVisible.value = true
}

const formatStringList = (items?: string[] | null) => (items && items.length ? items.join(', ') : '-')
const formatJSONPreview = (value?: Record<string, unknown> | Array<unknown> | null) => {
  if (!value || (Array.isArray(value) && value.length === 0)) return '-'
  return JSON.stringify(value, null, 2)
}
```

- [ ] **Step 5: Add table cells**

Add cell templates:

```vue
<template #cell-system_prompt_summary="{ row }">
  <span v-if="row.system_prompt_summary" class="block max-w-[320px] truncate text-sm text-gray-600 dark:text-gray-400" :title="row.system_prompt_summary">
    {{ row.system_prompt_summary }}
  </span>
  <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
</template>

<template #cell-tool_call_names="{ row }">
  <span v-if="row.tool_call_names?.length" class="block max-w-[260px] truncate text-sm font-mono text-gray-600 dark:text-gray-400" :title="row.tool_call_names.join(', ')">
    {{ row.tool_call_names.join(', ') }}
  </span>
  <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
</template>

<template #cell-output_summary="{ row }">
  <span v-if="row.output_summary" class="block max-w-[320px] truncate text-sm text-gray-600 dark:text-gray-400" :title="row.output_summary">
    {{ row.output_summary }}
  </span>
  <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
</template>

<template #cell-actions="{ row }">
  <button type="button" class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="openAuditDialog(row)">
    {{ t('admin.usage.preview') }}
  </button>
</template>
```

- [ ] **Step 6: Add preview modal**

Add below the existing prompt dialog:

```vue
<BaseDialog :show="auditDialogVisible" :title="t('admin.usage.auditPreview')" width="wide" @close="auditDialogVisible = false">
  <div v-if="auditDialogLog" class="space-y-5 text-sm text-gray-700 dark:text-gray-200">
    <section>
      <h3 class="mb-2 font-semibold">{{ t('admin.usage.basicInfo') }}</h3>
      <div class="grid grid-cols-2 gap-2 text-xs md:grid-cols-3">
        <div>Request ID: {{ auditDialogLog.request_id || '-' }}</div>
        <div>IP: {{ auditDialogLog.ip_address || '-' }}</div>
        <div>Hash: {{ auditDialogLog.request_body_sha256 || '-' }}</div>
      </div>
    </section>
    <section>
      <h3 class="mb-2 font-semibold">{{ t('admin.usage.requestPrompt') }}</h3>
      <pre class="max-h-64 overflow-auto whitespace-pre-wrap rounded bg-gray-50 p-3 dark:bg-gray-900">{{ auditDialogLog.request_prompt || '-' }}</pre>
    </section>
    <section>
      <h3 class="mb-2 font-semibold">{{ t('admin.usage.systemPrompt') }}</h3>
      <pre class="max-h-64 overflow-auto whitespace-pre-wrap rounded bg-gray-50 p-3 dark:bg-gray-900">{{ auditDialogLog.system_prompt_text || auditDialogLog.system_prompt_summary || '-' }}</pre>
    </section>
    <section>
      <h3 class="mb-2 font-semibold">{{ t('admin.usage.tools') }}</h3>
      <div class="mb-2">{{ t('admin.usage.availableTools') }}: {{ formatStringList(auditDialogLog.tool_names) }}</div>
      <div>{{ t('admin.usage.actualCalls') }}: {{ formatStringList(auditDialogLog.tool_call_names) }}</div>
      <pre class="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded bg-gray-50 p-3 dark:bg-gray-900">{{ formatJSONPreview(auditDialogLog.tool_calls_json) }}</pre>
    </section>
    <section>
      <h3 class="mb-2 font-semibold">{{ t('admin.usage.outputSummary') }}</h3>
      <pre class="max-h-64 overflow-auto whitespace-pre-wrap rounded bg-gray-50 p-3 dark:bg-gray-900">{{ auditDialogLog.output_text || auditDialogLog.output_summary || '-' }}</pre>
    </section>
  </div>
</BaseDialog>
```

- [ ] **Step 7: Add i18n labels**

In `frontend/src/i18n/locales/zh.ts`, under `admin.usage`, add:

```ts
systemPrompt: '系统词',
actualCalls: '实际调用',
outputSummary: '输出摘要',
requestBodyHash: '请求 Hash',
truncated: '截断',
preview: '预览',
auditPreview: '请求审计预览',
basicInfo: '基础信息',
tools: '工具信息',
availableTools: '可用工具',
toolNames: '工具列表'
```

In `frontend/src/i18n/locales/en.ts`, add:

```ts
systemPrompt: 'System Prompt',
actualCalls: 'Actual Calls',
outputSummary: 'Output Summary',
requestBodyHash: 'Request Hash',
truncated: 'Truncated',
preview: 'Preview',
auditPreview: 'Request Audit Preview',
basicInfo: 'Basic Info',
tools: 'Tools',
availableTools: 'Available Tools',
toolNames: 'Tool Names'
```

- [ ] **Step 8: Add frontend tests**

In `frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts`, add a fixture row with audit fields and assert:

```ts
expect(wrapper.text()).toContain('exec_command')
await wrapper.get('button').trigger('click')
expect(wrapper.text()).toContain('Request Audit Preview')
expect(wrapper.text()).toContain('System text')
expect(wrapper.text()).toContain('Output text')
```

In `frontend/src/views/admin/__tests__/UsageView.spec.ts`, assert export row includes `system_prompt_summary`, `tool_call_names`, and `output_summary`.

- [ ] **Step 9: Run frontend tests**

Run:

```bash
pnpm --dir frontend exec vitest run frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts frontend/src/views/admin/__tests__/UsageView.spec.ts
pnpm --dir frontend run typecheck
```

Expected: PASS.

- [ ] **Step 10: Commit frontend work**

```bash
git add frontend/src/types/index.ts frontend/src/views/admin/UsageView.vue frontend/src/components/admin/usage/UsageTable.vue frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts frontend/src/views/admin/__tests__/UsageView.spec.ts
git commit -m "feat: show usage audit preview"
```

---

### Task 7: Runtime verification with Docker and Tailscale

**Files:**
- Use: `docker-compose.yml`
- Use: `.env`
- No source modifications in this task.

- [ ] **Step 1: Verify branch and no accidental runtime file staging**

Run:

```bash
git branch --show-current
git status --short
```

Expected:

```text
sub2-fork
```

No `.env`, `data/`, `postgres_data/`, `redis_data/`, or `logs/` entries should be staged.

- [ ] **Step 2: Rebuild and start mapped Docker project**

Run:

```bash
docker compose build sub2api
docker compose up -d
docker compose ps
```

Expected:

- `sub2api-fork` is `healthy`.
- `sub2api-fork-postgres` is `healthy`.
- `sub2api-fork-redis` is `healthy`.

- [ ] **Step 3: Verify local HTTP health**

Run:

```bash
curl -fsS http://127.0.0.1:8420/health
```

Expected:

```json
{"status":"ok"}
```

- [ ] **Step 4: Verify Tailscale access URL**

Run:

```bash
TS_IP=$(tailscale ip -4)
echo "http://${TS_IP}:8420"
curl -fsS "http://${TS_IP}:8420/health"
```

Expected:

```json
{"status":"ok"}
```

Current observed IP during plan writing was `100.126.43.55`; use the live `tailscale ip -4` output when testing.

- [ ] **Step 5: Verify database columns exist**

Run:

```bash
docker compose exec -T postgres psql -U "${POSTGRES_USER:-sub2api}" -d "${POSTGRES_DB:-sub2api}" -c "\d usage_logs" | grep -E 'system_prompt_summary|tool_names|output_summary|request_body_sha256'
docker compose exec -T postgres psql -U "${POSTGRES_USER:-sub2api}" -d "${POSTGRES_DB:-sub2api}" -c "\d ops_error_logs" | grep -E 'system_prompt_summary|tool_names|request_body_sha256'
```

Expected: each grep prints matching columns.

- [ ] **Step 6: Verify a new successful request stores audit data**

Use an existing test API key from the admin UI or a safe local test key. Send a minimal request through the mapped service:

```bash
curl -sS http://127.0.0.1:8420/v1/responses \
  -H 'Authorization: Bearer <TEST_API_KEY>' \
  -H 'Content-Type: application/json' \
  -H 'X-Codex-Turn-Metadata: {"session_id":"manual","thread_id":"usage-audit","turn_id":"1"}' \
  -d '{"model":"<AVAILABLE_MODEL>","instructions":"You are Codex audit test.","tools":[{"type":"function","name":"exec_command"}],"input":"Say usage audit ok"}'
```

Then query the newest row:

```bash
docker compose exec -T postgres psql -U "${POSTGRES_USER:-sub2api}" -d "${POSTGRES_DB:-sub2api}" -c "SELECT request_prompt, system_prompt_summary, tool_names, tool_call_names, output_summary, request_body_sha256 FROM usage_logs ORDER BY id DESC LIMIT 1;"
```

Expected:

- `request_prompt` includes `Say usage audit ok`.
- `system_prompt_summary` includes `Codex audit test`.
- `tool_names` includes `exec_command`.
- `request_body_sha256` is 64 hex chars.

- [ ] **Step 7: Verify Usage UI**

Open:

```text
http://127.0.0.1:8420
http://100.126.43.55:8420
```

Use live Tailscale IP from `tailscale ip -4` if it differs.

Expected:

- Admin Usage page loads.
- New usage row shows prompt summary.
- Preview opens and shows Basic Info, Request Prompt, System Prompt, Tools, Output Summary.
- Export includes added audit columns.

- [ ] **Step 8: Commit runtime verification note if needed**

Only if docs need a short operational note, append a verified access note to `docs/superpowers/specs/2026-06-21-sub2api-usage-audit-design.md`:

```markdown
## Runtime Access Check

Local health check: `http://127.0.0.1:8420/health`.
Tailscale health check uses `http://$(tailscale ip -4):8420/health`; observed during planning: `100.126.43.55`.
```

Commit only documentation changes:

```bash
git add docs/superpowers/specs/2026-06-21-sub2api-usage-audit-design.md
git commit -m "docs: add usage audit runtime check"
```

---

## Self-Review

### Spec coverage

- Existing Usage compatibility: Task 1, Task 3, Task 5, Task 6.
- No new `request_capture_logs` table: Task 1 only adds fields to existing `usage_logs` and `ops_error_logs`.
- Data saving priority: Task 1 through Task 4 define fields, extractor, usage write path and ops write path.
- Frontend summary / preview / export: Task 6.
- Docker mapped directory and Tailscale testing: Task 7.
- `sub2-fork` branch requirement: Task 7 Step 1.

### Placeholder scan

No placeholder steps are left. Every task names exact files, code blocks, commands, and expected results.

### Type consistency

The audit payload type is `UsageAuditPayload`. The persisted tool-call type is `ToolCallAudit`. DTO and frontend names use snake_case JSON keys matching planned database columns.
