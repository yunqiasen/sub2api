package service

import (
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
)

// ExtractRequestPrompt extracts a human-readable user prompt from an OpenAI-compatible request.
// It prefers parsed Chat Completions / Messages data when available, then falls back to raw body fields.
func ExtractRequestPrompt(body []byte, parsed *ParsedRequest) string {
	if prompt := extractRequestPromptFromParsedRequest(parsed); prompt != "" {
		return prompt
	}
	return extractRequestPromptFromBody(body)
}

func extractRequestPromptFromParsedRequest(parsed *ParsedRequest) string {
	if parsed == nil {
		return ""
	}
	raw := parsed.MessagesRaw()
	if len(raw) == 0 {
		return ""
	}

	var messages []map[string]any
	if err := json.Unmarshal(raw, &messages); err != nil {
		return ""
	}

	var parts []string
	for _, m := range messages {
		role, _ := m["role"].(string)
		if strings.TrimSpace(role) != "user" {
			continue
		}
		if text := strings.TrimSpace(extractTextFromContent(m["content"])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func extractRequestPromptFromBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	if prompt := strings.TrimSpace(gjson.GetBytes(body, "prompt").String()); prompt != "" {
		return prompt
	}

	var parts []string
	if msgs := gjson.GetBytes(body, "messages"); msgs.Exists() && msgs.IsArray() {
		msgs.ForEach(func(_, msg gjson.Result) bool {
			if strings.TrimSpace(msg.Get("role").String()) != "user" {
				return true
			}
			if text := strings.TrimSpace(extractTextFromContent(msg.Get("content").Value())); text != "" {
				parts = append(parts, text)
			}
			return true
		})
	}

	if len(parts) == 0 {
		parts = append(parts, extractRequestPromptFromInput(gjson.GetBytes(body, "input"))...)
	}

	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func extractRequestPromptFromInput(input gjson.Result) []string {
	if !input.Exists() {
		return nil
	}

	if input.Type == gjson.String {
		if text := strings.TrimSpace(input.String()); text != "" {
			return []string{text}
		}
		return nil
	}

	if input.IsArray() {
		var parts []string
		input.ForEach(func(_, item gjson.Result) bool {
			if text := extractRequestPromptFromInputItem(item); text != "" {
				parts = append(parts, text)
			}
			return true
		})
		return parts
	}

	if text := extractRequestPromptFromInputItem(input); text != "" {
		return []string{text}
	}
	return nil
}

func extractRequestPromptFromInputItem(item gjson.Result) string {
	if !item.Exists() {
		return ""
	}

	typ := strings.TrimSpace(item.Get("type").String())
	role := strings.TrimSpace(item.Get("role").String())
	switch {
	case role == "user":
		if text := strings.TrimSpace(extractTextFromContent(item.Get("content").Value())); text != "" {
			return text
		}
		if text := strings.TrimSpace(item.Get("text").String()); text != "" {
			return text
		}
	case typ == "input_text":
		if text := strings.TrimSpace(item.Get("text").String()); text != "" {
			return text
		}
	case typ == "message":
		if role == "user" {
			if text := strings.TrimSpace(extractTextFromContent(item.Get("content").Value())); text != "" {
				return text
			}
		}
	}

	return ""
}
