package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const opsUsageAuditContextKey = "ops_usage_audit"

func extractUsageAuditForRequest(c *gin.Context, body []byte, parsed *service.ParsedRequest) service.UsageAuditPayload {
	if audit, ok := getOpsUsageAuditContext(c); ok {
		return audit
	}
	audit := service.ExtractUsageAuditFromRequest(body, map[string]string{
		"X-Codex-Turn-Metadata": c.GetHeader("X-Codex-Turn-Metadata"),
	})
	if prompt := strings.TrimSpace(service.ExtractRequestPrompt(body, parsed)); prompt != "" {
		audit.RequestPrompt = prompt
	}
	setOpsUsageAuditContext(c, audit)
	return audit
}

func setOpsUsageAuditContext(c *gin.Context, audit service.UsageAuditPayload) {
	if c == nil {
		return
	}
	c.Set(opsUsageAuditContextKey, audit)
}

func getOpsUsageAuditContext(c *gin.Context) (service.UsageAuditPayload, bool) {
	if c == nil {
		return service.UsageAuditPayload{}, false
	}
	v, ok := c.Get(opsUsageAuditContextKey)
	if !ok {
		return service.UsageAuditPayload{}, false
	}
	audit, ok := v.(service.UsageAuditPayload)
	return audit, ok
}
