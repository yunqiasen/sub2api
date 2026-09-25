package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPartialStreamUsageResult_PreservesAudit(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	body := []byte("event: content_block_delta\ndata: {\"delta\":{\"type\":\"text_delta\",\"text\":\"partial answer\"}}\n\n")
	result := partialStreamUsageResult(c, &http.Response{Header: http.Header{"X-Request-Id": {"partial-request"}}}, &streamingResult{
		usage:             &ClaudeUsage{InputTokens: 10, OutputTokens: 2},
		responseAuditBody: body,
	}, "model", "upstream-model", time.Now(), io.ErrUnexpectedEOF)
	require.NotNil(t, result)
	require.Equal(t, body, result.ResponseAuditBody)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Equal(t, "partial-request", result.RequestID)
}

func TestPartialStreamUsageResult_SkipsFailoverAudit(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	result := partialStreamUsageResult(c, &http.Response{Header: make(http.Header)}, &streamingResult{
		usage:             &ClaudeUsage{InputTokens: 10},
		responseAuditBody: []byte("partial"),
	}, "model", "upstream-model", time.Now(), &UpstreamFailoverError{})
	require.Nil(t, result)
}
