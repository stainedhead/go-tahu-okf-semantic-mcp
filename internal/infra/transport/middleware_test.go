package transport

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoggingMiddleware_PanicRecovered verifies that a panicking tool handler
// is recovered and returns an error rather than crashing the daemon (FR-023).
func TestLoggingMiddleware_PanicRecovered(t *testing.T) {
	middleware := loggingMiddleware()
	handler := middleware(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		panic("tool panic")
	})
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "panicked")
	assert.Nil(t, result)
}
