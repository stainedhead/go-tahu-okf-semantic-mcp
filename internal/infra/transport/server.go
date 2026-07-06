// Package transport wires the MCP server to its stdio or HTTP/SSE transports.
// Business logic lives entirely in the use-case and adapter layers; this
// package only performs server lifecycle management.
package transport

import (
	"context"
	"fmt"
	"log/slog"
	"net"
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

// ServeStdio starts the MCP stdio transport and blocks until ctx is cancelled
// or an error occurs (FR-022).
func ServeStdio(ctx context.Context, srv *mcpserver.MCPServer) error {
	errCh := make(chan error, 1)
	go func() {
		if err := mcpserver.ServeStdio(srv); err != nil {
			errCh <- fmt.Errorf("transport.ServeStdio: %w", err)
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// ServeHTTP starts an HTTP/SSE MCP server on addr and blocks until ctx is
// cancelled. It also exposes GET /healthz returning 200 OK (FR-017).
// Non-loopback bind addresses are rejected at startup (FR-021).
func ServeHTTP(ctx context.Context, srv *mcpserver.MCPServer, addr string) error {
	if err := validateBindAddr(addr); err != nil {
		return err
	}

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
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
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

// validateBindAddr returns an error when addr binds to a non-loopback interface.
// HTTP mode is loopback-only for security (FR-021).
func validateBindAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("transport: bind address %q is non-loopback; HTTP mode is loopback-only for security (use 127.0.0.1 or ::1)", addr)
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
// for every MCP tool invocation. It also recovers from tool handler panics so
// the daemon stays alive (FR-023).
func loggingMiddleware() mcpserver.ToolHandlerMiddleware {
	return func(next mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
			requestID := uuid.New().String()
			ctx = context.WithValue(ctx, requestIDKey, requestID)
			tool := req.Params.Name
			start := time.Now()

			defer func() {
				if r := recover(); r != nil {
					durationMS := time.Since(start).Milliseconds()
					slog.Error("tool handler panic",
						slog.String("request_id", requestID),
						slog.String("tool", tool),
						slog.Int64("duration_ms", durationMS),
						slog.Any("panic", r),
					)
					err = fmt.Errorf("internal error: tool %s panicked", tool)
					result = nil
				}
			}()

			result, err = next(ctx, req)

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
