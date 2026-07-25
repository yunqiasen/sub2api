package repository

import (
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func nullStringSliceJSON(v []string) any {
	if len(v) == 0 {
		return nil
	}
	payload, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return string(payload)
}

func stringSliceFromNullJSON(v sql.NullString) []string {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(v.String), &out); err != nil || len(out) == 0 {
		return nil
	}
	return out
}

func nullToolCallAuditSliceJSON(v []service.ToolCallAudit) any {
	if len(v) == 0 {
		return nil
	}
	payload, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return string(payload)
}

func toolCallAuditSliceFromNullJSON(v sql.NullString) []service.ToolCallAudit {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil
	}
	var out []service.ToolCallAudit
	if err := json.Unmarshal([]byte(v.String), &out); err != nil || len(out) == 0 {
		return nil
	}
	return out
}

func nullStringAnyMapJSON(v map[string]any) any {
	if len(v) == 0 {
		return nil
	}
	payload, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return string(payload)
}

func stringAnyMapFromNullJSON(v sql.NullString) map[string]any {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(v.String), &out); err != nil || len(out) == 0 {
		return nil
	}
	return out
}
