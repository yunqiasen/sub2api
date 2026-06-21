package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func extractUsageAuditForRequest(c *gin.Context, body []byte, parsed *service.ParsedRequest) service.UsageAuditPayload {
	audit := service.ExtractUsageAuditFromRequest(body, map[string]string{
		"X-Codex-Turn-Metadata": c.GetHeader("X-Codex-Turn-Metadata"),
	})
	if prompt := strings.TrimSpace(service.ExtractRequestPrompt(body, parsed)); prompt != "" {
		audit.RequestPrompt = prompt
	}
	return audit
}
