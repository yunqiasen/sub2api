package service

import (
	"strings"
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
	require.Contains(t, audit.SystemPromptSummary, "You are Codex")
	require.Equal(t, []string{"exec_command", "mcp__context7__query_docs"}, audit.ToolNames)
	require.Equal(t, "s1", audit.TurnMetadata["session_id"])
	require.Len(t, audit.RequestBodySHA256, 64)
	require.Equal(t, len(body), audit.RequestBodyBytes)
	require.False(t, audit.RequestBodyTruncated)
	require.Equal(t, int16(1), audit.AuditCaptureVersion)
}

func TestExtractUsageAuditFromChatCompletionsRequest(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4.1",
		"messages":[
			{"role":"system","content":"System text"},
			{"role":"developer","content":[{"type":"text","text":"Developer text"}]},
			{"role":"user","content":"User text"}
		],
		"tools":[{"type":"function","function":{"name":"search_docs"}}]
	}`)

	audit := ExtractUsageAuditFromRequest(body, nil)

	require.Equal(t, "User text", audit.RequestPrompt)
	require.Equal(t, "System text", audit.SystemPromptText)
	require.Equal(t, "Developer text", audit.DeveloperPromptText)
	require.Equal(t, []string{"search_docs"}, audit.ToolNames)
}

func TestApplyUsageAuditFromResponsesResponse(t *testing.T) {
	audit := UsageAuditPayload{}
	body := []byte(`{
		"output_text":"done text",
		"output":[
			{"type":"function_call","name":"exec_command","call_id":"call_1","arguments":"{\"cmd\":\"ls\"}"},
			{"type":"message","content":[{"type":"output_text","text":"done text"}]}
		]
	}`)

	ApplyUsageAuditResponse(&audit, body)

	require.Equal(t, "done text", audit.OutputText)
	require.Equal(t, "done text", audit.OutputSummary)
	require.Equal(t, []string{"exec_command"}, audit.ToolCallNames)
	require.Len(t, audit.ToolCallsJSON, 1)
	require.Equal(t, "exec_command", audit.ToolCallsJSON[0].Name)
	require.Equal(t, "call_1", audit.ToolCallsJSON[0].ID)
	require.Len(t, audit.ToolCallsJSON[0].ArgumentsSHA256, 64)
}

func TestApplyUsageAuditFromChatCompletionResponse(t *testing.T) {
	audit := UsageAuditPayload{}
	body := []byte(`{
		"choices":[{"message":{
			"content":"chat output",
			"tool_calls":[{"id":"tool_1","type":"function","function":{"name":"search_docs","arguments":"{\"q\":\"audit\"}"}}]
		}}]
	}`)

	ApplyUsageAuditResponse(&audit, body)

	require.Equal(t, "chat output", audit.OutputText)
	require.Equal(t, []string{"search_docs"}, audit.ToolCallNames)
	require.Len(t, audit.ToolCallsJSON, 1)
	require.Equal(t, "tool_1", audit.ToolCallsJSON[0].ID)
	require.Equal(t, "search_docs", audit.ToolCallsJSON[0].Name)
}

func TestUsageAuditTruncatesLargeFields(t *testing.T) {
	bigPrompt := strings.Repeat("p", usageAuditRequestPromptLimit+16)
	body := []byte(`{"messages":[{"role":"user","content":"` + bigPrompt + `"}]}`)

	audit := ExtractUsageAuditFromRequest(body, nil)

	require.True(t, audit.RequestBodyTruncated)
	require.Len(t, audit.RequestPrompt, usageAuditRequestPromptLimit)
	require.Equal(t, len(body), audit.RequestBodyBytes)

	bigOutput := strings.Repeat("x", usageAuditOutputTextLimit+16)
	ApplyUsageAuditResponse(&audit, []byte(`{"output_text":"`+bigOutput+`"}`))

	require.True(t, audit.ResponseTextTruncated)
	require.Len(t, audit.OutputText, usageAuditOutputTextLimit)
}
