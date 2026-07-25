package repository

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsInsertErrorLogArgs_PersistsUsageAuditFields(t *testing.T) {
	bodyBytes := 456
	input := &service.OpsInsertErrorLogInput{
		RequestID:            "req-ops-audit",
		UserAgent:            "ua",
		RequestPrompt:        "user prompt",
		SystemPromptSummary:  stringPtrForOpsTest("system summary"),
		SystemPromptText:     stringPtrForOpsTest("system text"),
		DeveloperPromptText:  stringPtrForOpsTest("developer text"),
		ToolNames:            []string{"exec_command"},
		ToolCallNames:        []string{"exec_command"},
		OutputSummary:        stringPtrForOpsTest("output summary"),
		RequestBodySHA256:    stringPtrForOpsTest(strings.Repeat("a", 64)),
		RequestBodyBytes:     &bodyBytes,
		RequestBodyTruncated: true,
		TurnMetadata:         map[string]any{"session_id": "s1"},
		IPLocation:           map[string]any{"source": "direct"},
		AuditCaptureVersion:  1,
		ErrorPhase:           "upstream",
		ErrorType:            "upstream_error",
		IsBusinessLimited:    false,
		CreatedAt:            time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	args := opsInsertErrorLogArgs(input)

	require.Len(t, args, 51)
	require.Equal(t, "ua", requireOpsNullStringArg(t, args, 16).String)
	require.Equal(t, "user prompt", requireOpsNullStringArg(t, args, 17).String)
	require.Equal(t, "system summary", requireOpsNullStringArg(t, args, 18).String)
	require.Equal(t, "system text", requireOpsNullStringArg(t, args, 19).String)
	require.Equal(t, "developer text", requireOpsNullStringArg(t, args, 20).String)
	require.JSONEq(t, `["exec_command"]`, requireOpsStringArg(t, args, 21))
	require.JSONEq(t, `["exec_command"]`, requireOpsStringArg(t, args, 22))
	require.Equal(t, "output summary", requireOpsNullStringArg(t, args, 23).String)
	require.Equal(t, strings.Repeat("a", 64), requireOpsNullStringArg(t, args, 24).String)
	require.Equal(t, int64(bodyBytes), requireOpsNullInt64Arg(t, args, 25).Int64)
	require.Equal(t, true, args[26])
	require.JSONEq(t, `{"session_id":"s1"}`, requireOpsStringArg(t, args, 27))
	require.JSONEq(t, `{"source":"direct"}`, requireOpsStringArg(t, args, 28))
	require.Equal(t, int16(1), args[29])
	require.Equal(t, "upstream", args[30])
}

func stringPtrForOpsTest(value string) *string { return &value }

func requireOpsNullStringArg(t *testing.T, args []any, index int) sql.NullString {
	t.Helper()
	value, ok := args[index].(sql.NullString)
	require.True(t, ok)
	return value
}

func requireOpsNullInt64Arg(t *testing.T, args []any, index int) sql.NullInt64 {
	t.Helper()
	value, ok := args[index].(sql.NullInt64)
	require.True(t, ok)
	return value
}

func requireOpsStringArg(t *testing.T, args []any, index int) string {
	t.Helper()
	value, ok := args[index].(string)
	require.True(t, ok)
	return value
}
