package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
)

const (
	usageAuditRequestPromptLimit = 64 * 1024
	usageAuditSystemPromptLimit  = 128 * 1024
	usageAuditOutputTextLimit    = 64 * 1024
	usageAuditSummaryLimit       = 512
	usageAuditToolArgsLimit      = 4 * 1024
)

// ToolCallAudit stores a lightweight, safe representation of one tool call.
type ToolCallAudit struct {
	ID                 string `json:"id,omitempty"`
	Type               string `json:"type,omitempty"`
	Name               string `json:"name,omitempty"`
	ArgumentsSummary   string `json:"arguments_summary,omitempty"`
	ArgumentsSHA256    string `json:"arguments_sha256,omitempty"`
	ArgumentsBytes     int    `json:"arguments_bytes,omitempty"`
	ArgumentsTruncated bool   `json:"arguments_truncated,omitempty"`
}

// UsageAuditPayload is the structured audit payload persisted alongside usage/error logs.
type UsageAuditPayload struct {
	RequestPrompt         string
	SystemPromptSummary   string
	SystemPromptText      string
	DeveloperPromptText   string
	ToolNames             []string
	ToolCallNames         []string
	ToolCallsJSON         []ToolCallAudit
	OutputSummary         string
	OutputText            string
	RequestBodySHA256     string
	RequestBodyBytes      int
	RequestBodyTruncated  bool
	ResponseTextTruncated bool
	TurnMetadata          map[string]any
	IPLocation            map[string]any
	AuditCaptureVersion   int16
}

// ExtractUsageAuditFromRequest extracts audit fields from an OpenAI-compatible request body.
func ExtractUsageAuditFromRequest(body []byte, headers map[string]string) UsageAuditPayload {
	audit := UsageAuditPayload{
		RequestBodySHA256:   sha256Hex(body),
		RequestBodyBytes:    len(body),
		TurnMetadata:        extractTurnMetadata(headers),
		IPLocation:          map[string]any{},
		AuditCaptureVersion: 1,
	}

	audit.RequestPrompt, audit.RequestBodyTruncated = truncateAuditText(ExtractRequestPrompt(body, nil), usageAuditRequestPromptLimit)

	systemText, developerText := extractAuditPromptTexts(body)
	audit.SystemPromptText, audit.RequestBodyTruncated = truncateAuditTextMerge(audit.RequestBodyTruncated, systemText, usageAuditSystemPromptLimit)
	audit.DeveloperPromptText, audit.RequestBodyTruncated = truncateAuditTextMerge(audit.RequestBodyTruncated, developerText, usageAuditSystemPromptLimit)
	audit.SystemPromptSummary, _ = truncateAuditText(collapseAuditWhitespace(strings.TrimSpace(strings.Join(nonEmptyStrings(audit.SystemPromptText, audit.DeveloperPromptText), "\n\n"))), usageAuditSummaryLimit)
	audit.ToolNames = extractAuditToolNames(body)

	return audit
}

// ApplyUsageAuditResponse augments an audit payload with response text and tool calls.
func ApplyUsageAuditResponse(audit *UsageAuditPayload, responseBody []byte) {
	if audit == nil || len(responseBody) == 0 {
		return
	}
	if audit.TurnMetadata == nil {
		audit.TurnMetadata = map[string]any{}
	}
	if audit.IPLocation == nil {
		audit.IPLocation = map[string]any{}
	}
	if audit.AuditCaptureVersion == 0 {
		audit.AuditCaptureVersion = 1
	}

	text := extractAuditOutputText(responseBody)
	if text != "" {
		audit.OutputText, audit.ResponseTextTruncated = truncateAuditText(text, usageAuditOutputTextLimit)
		audit.OutputSummary, _ = truncateAuditText(collapseAuditWhitespace(audit.OutputText), usageAuditSummaryLimit)
	}

	calls := extractAuditToolCalls(responseBody)
	if len(calls) > 0 {
		audit.ToolCallsJSON = append(audit.ToolCallsJSON, calls...)
		for _, call := range calls {
			if call.Name != "" {
				audit.ToolCallNames = appendUniqueString(audit.ToolCallNames, call.Name)
			}
		}
	}
}

func extractAuditPromptTexts(body []byte) (systemText string, developerText string) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return "", ""
	}
	root := gjson.ParseBytes(body)
	var systemParts []string
	var developerParts []string

	if instructions := strings.TrimSpace(root.Get("instructions").String()); instructions != "" {
		systemParts = append(systemParts, instructions)
	}
	if system := strings.TrimSpace(extractTextFromGJSONContent(root.Get("system"))); system != "" {
		systemParts = append(systemParts, system)
	}
	if systemInstruction := strings.TrimSpace(extractTextFromGJSONContent(root.Get("system_instruction"))); systemInstruction != "" {
		systemParts = append(systemParts, systemInstruction)
	}

	appendRolePromptParts := func(items gjson.Result) {
		if !items.IsArray() {
			return
		}
		items.ForEach(func(_, item gjson.Result) bool {
			role := strings.TrimSpace(item.Get("role").String())
			text := strings.TrimSpace(extractTextFromGJSONContent(item.Get("content")))
			if text == "" {
				text = strings.TrimSpace(item.Get("text").String())
			}
			switch role {
			case "system":
				if text != "" {
					systemParts = append(systemParts, text)
				}
			case "developer":
				if text != "" {
					developerParts = append(developerParts, text)
				}
			}
			return true
		})
	}

	appendRolePromptParts(root.Get("messages"))
	appendRolePromptParts(root.Get("input"))

	return strings.TrimSpace(strings.Join(systemParts, "\n\n")), strings.TrimSpace(strings.Join(developerParts, "\n\n"))
}

func extractAuditToolNames(body []byte) []string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil
	}
	root := gjson.ParseBytes(body)
	var out []string
	appendTool := func(tool gjson.Result) {
		candidates := []string{
			tool.Get("name").String(),
			tool.Get("function.name").String(),
			tool.Get("type").String(),
		}
		for _, candidate := range candidates {
			name := strings.TrimSpace(candidate)
			if name != "" && name != "function" {
				out = appendUniqueString(out, name)
				return
			}
		}
	}
	if tools := root.Get("tools"); tools.IsArray() {
		tools.ForEach(func(_, tool gjson.Result) bool {
			appendTool(tool)
			return true
		})
	}
	if functions := root.Get("functions"); functions.IsArray() {
		functions.ForEach(func(_, fn gjson.Result) bool {
			if name := strings.TrimSpace(fn.Get("name").String()); name != "" {
				out = appendUniqueString(out, name)
			}
			return true
		})
	}
	return out
}

func extractAuditOutputText(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	root := gjson.ParseBytes(body)
	if text := strings.TrimSpace(root.Get("output_text").String()); text != "" {
		return text
	}
	var parts []string
	if choices := root.Get("choices"); choices.IsArray() {
		choices.ForEach(func(_, choice gjson.Result) bool {
			if text := strings.TrimSpace(extractTextFromGJSONContent(choice.Get("message.content"))); text != "" {
				parts = append(parts, text)
			}
			if text := strings.TrimSpace(extractTextFromGJSONContent(choice.Get("delta.content"))); text != "" {
				parts = append(parts, text)
			}
			return true
		})
	}
	if output := root.Get("output"); output.IsArray() {
		output.ForEach(func(_, item gjson.Result) bool {
			if text := strings.TrimSpace(extractTextFromGJSONContent(item.Get("content"))); text != "" {
				parts = append(parts, text)
			}
			return true
		})
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

func extractAuditToolCalls(body []byte) []ToolCallAudit {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil
	}
	root := gjson.ParseBytes(body)
	var calls []ToolCallAudit

	if output := root.Get("output"); output.IsArray() {
		output.ForEach(func(_, item gjson.Result) bool {
			typeName := strings.TrimSpace(item.Get("type").String())
			if !strings.Contains(typeName, "function_call") && !strings.Contains(typeName, "tool_call") && typeName != "custom_tool_call" {
				return true
			}
			name := strings.TrimSpace(firstNonEmptyAuditString(item.Get("name").String(), item.Get("function.name").String()))
			if name == "" {
				return true
			}
			calls = append(calls, newToolCallAudit(firstNonEmptyAuditString(item.Get("call_id").String(), item.Get("id").String()), typeName, name, item.Get("arguments").String()))
			return true
		})
	}

	if choices := root.Get("choices"); choices.IsArray() {
		choices.ForEach(func(_, choice gjson.Result) bool {
			collectChatToolCalls(choice.Get("message.tool_calls"), &calls)
			collectChatToolCalls(choice.Get("delta.tool_calls"), &calls)
			return true
		})
	}

	return calls
}

func collectChatToolCalls(items gjson.Result, calls *[]ToolCallAudit) {
	if !items.IsArray() || calls == nil {
		return
	}
	items.ForEach(func(_, item gjson.Result) bool {
		name := strings.TrimSpace(item.Get("function.name").String())
		if name == "" {
			name = strings.TrimSpace(item.Get("name").String())
		}
		if name == "" {
			return true
		}
		*calls = append(*calls, newToolCallAudit(item.Get("id").String(), firstNonEmptyAuditString(item.Get("type").String(), "function"), name, item.Get("function.arguments").String()))
		return true
	})
}

func newToolCallAudit(id, typ, name, arguments string) ToolCallAudit {
	arguments = strings.TrimSpace(arguments)
	summary, truncated := truncateAuditText(arguments, usageAuditToolArgsLimit)
	call := ToolCallAudit{
		ID:                 strings.TrimSpace(id),
		Type:               strings.TrimSpace(typ),
		Name:               strings.TrimSpace(name),
		ArgumentsSummary:   summary,
		ArgumentsBytes:     len(arguments),
		ArgumentsTruncated: truncated,
	}
	if arguments != "" {
		call.ArgumentsSHA256 = sha256Hex([]byte(arguments))
	}
	return call
}

func extractTurnMetadata(headers map[string]string) map[string]any {
	if len(headers) == 0 {
		return map[string]any{}
	}
	var raw string
	for key, value := range headers {
		if strings.EqualFold(strings.TrimSpace(key), "X-Codex-Turn-Metadata") {
			raw = strings.TrimSpace(value)
			break
		}
	}
	if raw == "" {
		return map[string]any{}
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil || parsed == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(parsed))
	for key, value := range parsed {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		switch v := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				out[key] = trimmed
			}
		case float64, bool, nil:
			out[key] = v
		}
	}
	return out
}

func extractTextFromGJSONContent(content gjson.Result) string {
	if !content.Exists() {
		return ""
	}
	switch content.Type {
	case gjson.String:
		return content.String()
	case gjson.JSON:
		if content.IsArray() {
			var parts []string
			content.ForEach(func(_, part gjson.Result) bool {
				if text := strings.TrimSpace(part.Get("text").String()); text != "" {
					parts = append(parts, text)
				}
				return true
			})
			return strings.Join(parts, "")
		}
	}
	return ""
}

func truncateAuditTextMerge(existing bool, text string, limit int) (string, bool) {
	truncatedText, truncated := truncateAuditText(text, limit)
	return truncatedText, existing || truncated
}

func truncateAuditText(text string, limit int) (string, bool) {
	if limit <= 0 || len(text) <= limit {
		return text, false
	}
	return text[:limit], true
}

func collapseAuditWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func appendUniqueString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func firstNonEmptyAuditString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ApplyUsageAuditToUsageLog copies non-empty audit fields into a UsageLog before persistence.
func ApplyUsageAuditToUsageLog(log *UsageLog, audit UsageAuditPayload) {
	if log == nil {
		return
	}
	if audit.RequestPrompt != "" {
		log.RequestPrompt = stringPtr(audit.RequestPrompt)
	}
	log.SystemPromptSummary = stringPtrIfNotEmpty(audit.SystemPromptSummary)
	log.SystemPromptText = stringPtrIfNotEmpty(audit.SystemPromptText)
	log.DeveloperPromptText = stringPtrIfNotEmpty(audit.DeveloperPromptText)
	log.ToolNames = append([]string(nil), audit.ToolNames...)
	log.ToolCallNames = append([]string(nil), audit.ToolCallNames...)
	log.ToolCallsJSON = append([]ToolCallAudit(nil), audit.ToolCallsJSON...)
	log.OutputSummary = stringPtrIfNotEmpty(audit.OutputSummary)
	log.OutputText = stringPtrIfNotEmpty(audit.OutputText)
	log.RequestBodySHA256 = stringPtrIfNotEmpty(audit.RequestBodySHA256)
	if audit.RequestBodyBytes > 0 {
		bytes := audit.RequestBodyBytes
		log.RequestBodyBytes = &bytes
	}
	log.RequestBodyTruncated = audit.RequestBodyTruncated
	log.ResponseTextTruncated = audit.ResponseTextTruncated
	if len(audit.TurnMetadata) > 0 {
		log.TurnMetadata = copyStringAnyMap(audit.TurnMetadata)
	}
	if len(audit.IPLocation) > 0 {
		log.IPLocation = copyStringAnyMap(audit.IPLocation)
	}
	if audit.AuditCaptureVersion > 0 {
		log.AuditCaptureVersion = audit.AuditCaptureVersion
	}
}

func stringPtr(value string) *string {
	return &value
}

func stringPtrIfNotEmpty(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func copyStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
