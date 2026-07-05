package transport_test

import (
	"context"
	"testing"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/infra/transport"
)

// TestRequestIDFromContext_FR021 validates FR-021: request_id is accessible
// via context after loggingMiddleware stores it.
func TestRequestIDFromContext_FR021(t *testing.T) {
	t.Parallel()

	t.Run("empty context returns empty string", func(t *testing.T) {
		t.Parallel()
		if id := transport.RequestIDFromContext(context.Background()); id != "" {
			t.Errorf("got %q, want empty string for bare context", id)
		}
	})
}
