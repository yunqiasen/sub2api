package service

import "strings"

// usageAuditBodyCollector keeps a bounded response snapshot for the usage audit
// record without retaining an unbounded upstream response in the request worker.
type usageAuditBodyCollector struct {
	builder strings.Builder
	limit   int
}

func newUsageAuditBodyCollector() *usageAuditBodyCollector {
	return &usageAuditBodyCollector{limit: usageAuditOutputTextLimit * 4}
}

func (c *usageAuditBodyCollector) AppendString(value string) {
	if c == nil || value == "" || c.limit <= 0 || c.builder.Len() >= c.limit {
		return
	}
	remaining := c.limit - c.builder.Len()
	if len(value) > remaining {
		value = value[:remaining]
	}
	_, _ = c.builder.WriteString(value)
}

func (c *usageAuditBodyCollector) AppendSSEData(data string) {
	if c == nil || strings.TrimSpace(data) == "" {
		return
	}
	c.AppendString("data: ")
	c.AppendString(data)
	c.AppendString("\n\n")
}

func (c *usageAuditBodyCollector) Bytes() []byte {
	if c == nil || c.builder.Len() == 0 {
		return nil
	}
	return []byte(c.builder.String())
}
