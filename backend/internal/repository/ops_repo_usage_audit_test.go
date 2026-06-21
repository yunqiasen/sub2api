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

	require.Len(t, args, 54)
	require.Equal(t, "ua", args[16].(sql.NullString).String)
	require.Equal(t, "user prompt", args[17].(sql.NullString).String)
	require.Equal(t, "system summary", args[18].(sql.NullString).String)
	require.Equal(t, "system text", args[19].(sql.NullString).String)
	require.Equal(t, "developer text", args[20].(sql.NullString).String)
	require.JSONEq(t, `["exec_command"]`, args[21].(string))
	require.JSONEq(t, `["exec_command"]`, args[22].(string))
	require.Equal(t, "output summary", args[23].(sql.NullString).String)
	require.Equal(t, strings.Repeat("a", 64), args[24].(sql.NullString).String)
	require.Equal(t, int64(bodyBytes), args[25].(sql.NullInt64).Int64)
	require.Equal(t, true, args[26])
	require.JSONEq(t, `{"session_id":"s1"}`, args[27].(string))
	require.JSONEq(t, `{"source":"direct"}`, args[28].(string))
	require.Equal(t, int16(1), args[29])
	require.Equal(t, "upstream", args[30])
}

func stringPtrForOpsTest(value string) *string { return &value }
