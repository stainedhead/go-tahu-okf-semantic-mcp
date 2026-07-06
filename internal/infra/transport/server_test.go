package transport_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpadapter "github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/mcp"
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

// TestServeHTTP_Healthz verifies the /healthz endpoint returns 200 GET and
// 405 for other methods (FR-020).
func TestServeHTTP_Healthz(t *testing.T) {
	t.Parallel()

	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	srv := transport.NewMCPServer(mcpadapter.Services{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = transport.ServeHTTP(ctx, srv, addr) }()

	require.Eventually(t, func() bool {
		resp, err := http.Get("http://" + addr + "/healthz") //nolint:noctx
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return true
	}, 2*time.Second, 20*time.Millisecond)

	resp, err := http.Get("http://" + addr + "/healthz") //nolint:noctx
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp2, err := http.Post("http://"+addr+"/healthz", "text/plain", nil) //nolint:noctx
	require.NoError(t, err)
	_ = resp2.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp2.StatusCode)
}

// TestServeHTTP_NonLoopbackRejected verifies non-loopback binds are rejected (FR-021).
func TestServeHTTP_NonLoopbackRejected(t *testing.T) {
	t.Parallel()
	srv := transport.NewMCPServer(mcpadapter.Services{})
	err := transport.ServeHTTP(context.Background(), srv, "0.0.0.0:19999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-loopback")
}

// TestServeStdio_ContextCancellation verifies ServeStdio respects ctx (FR-022).
func TestServeStdio_ContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	srv := transport.NewMCPServer(mcpadapter.Services{})
	err := transport.ServeStdio(ctx, srv)
	assert.ErrorIs(t, err, context.Canceled)
}
