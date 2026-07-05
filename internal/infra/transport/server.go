// Package transport wires the MCP server to its stdio or HTTP/SSE transports.
// Business logic lives entirely in the use-case and adapter layers; this
// package only performs server lifecycle management.
package transport

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	mcpadapter "github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/mcp"
)

// NewMCPServer creates an MCPServer with all 14 tools registered and FR-021
// structured-logging middleware applied to every tool invocation.
func NewMCPServer(svc mcpadapter.Services) *mcpserver.MCPServer {
	srv := mcpserver.NewMCPServer(
		"tahu",
		"0.1.0",
		mcpserver.WithToolHandlerMiddleware(loggingMiddleware()),
	)
	mcpadapter.RegisterTools(srv, svc)
	return srv
}

// ServeStdio starts the MCP stdio transport and blocks until the process
// receives SIGTERM/SIGINT or an error occurs (FR-015).
func ServeStdio(_ context.Context, srv *mcpserver.MCPServer) error {
	if err := mcpserver.ServeStdio(srv); err != nil {
		return fmt.Errorf("transport.ServeStdio: %w", err)
	}
	return nil
}

// ServeHTTP starts an HTTP/SSE MCP server on addr and blocks until ctx is
// cancelled. It also exposes GET /healthz returning 200 OK (FR-017).
//
// The SSE and message endpoints are served at /sse and /message respectively.
// All other paths are handled by the SSEServer.
func ServeHTTP(ctx context.Context, srv *mcpserver.MCPServer, addr string) error {
	baseURL := "http://" + addr

	// Build the SSEServer. WithBaseURL tells it how to construct client URLs.
	sseSrv := mcpserver.NewSSEServer(srv, mcpserver.WithBaseURL(baseURL))

	// Our mux routes /healthz locally and delegates everything else to the
	// SSEServer (which handles /sse and /message internally).
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", sseSrv)

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("HTTP/SSE MCP server listening", slog.String("addr", addr))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("transport.ServeHTTP: listen: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		// context.WithoutCancel strips the parent cancellation so the shutdown
		// window is not immediately cancelled alongside ctx.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		// Shut down SSE sessions first, then the HTTP listener.
		_ = sseSrv.Shutdown(shutdownCtx)
		return httpSrv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// ---------------------------------------------------------------------------
// FR-021 structured logging middleware
// ---------------------------------------------------------------------------

// contextKey is an unexported type for context keys in this package,
// preventing collisions with keys from other packages.
type contextKey string

const requestIDKey contextKey = "request_id"

// RequestIDFromContext returns the request_id stored by loggingMiddleware,
// or "" if the context carries no request_id (FR-021).
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// loggingMiddleware returns a ToolHandlerMiddleware that emits a JSON log line
// for every MCP tool invocation with the fields required by FR-021:
// request_id, tool, duration_ms, level, and (on error) error.
// The request_id is also propagated via context so downstream handlers can
// correlate their own log lines to the originating MCP call.
func loggingMiddleware() mcpserver.ToolHandlerMiddleware {
	return func(next mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			requestID := uuid.New().String()
			ctx = context.WithValue(ctx, requestIDKey, requestID)
			tool := req.Params.Name
			start := time.Now()

			result, err := next(ctx, req)

			durationMS := time.Since(start).Milliseconds()

			attrs := []any{
				slog.String("request_id", requestID),
				slog.String("tool", tool),
				slog.Int64("duration_ms", durationMS),
			}
			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
				slog.Log(ctx, slog.LevelError, "tool call", attrs...)
			} else {
				slog.Log(ctx, slog.LevelInfo, "tool call", attrs...)
			}

			return result, err
		}
	}
}
