package mcpcli

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	DefaultHTTPAddress = "127.0.0.1:47831"
	HTTPEndpointPath   = "/mcp"

	httpMaxRequestBodyBytes = 4 << 20
	httpShutdownDrain       = 10 * time.Second
	httpReadHeaderTimeout   = 10 * time.Second
)

type HTTPOptions struct {
	// Address is the exact loopback host:port clients must send as Host.
	Address string
	Bearer  string
	// Logger receives allowlisted access events only: HTTP method and status,
	// tool name, outcome, and duration. Nil discards them.
	Logger *slog.Logger
}

// ValidateHTTPAddress accepts only a literal loopback IP with an explicit port.
func ValidateHTTPAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		return fmt.Errorf("mcp http address %q must be host:port", address)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("mcp http address %q must use a loopback IP literal", address)
	}
	return nil
}

// NewHTTPHandler serves one stateless Streamable HTTP endpoint. Every tool
// must carry an explicit authority class; workspace tools bind a request-local
// capability, so no caller identity or workspace lives on the shared server.
func NewHTTPHandler(deps MCPDependencies, options HTTPOptions) (http.Handler, error) {
	if err := ValidateHTTPAddress(options.Address); err != nil {
		return nil, err
	}
	if err := validateHTTPBearer(options.Bearer); err != nil {
		return nil, err
	}
	if err := validateHTTPToolClassification(deps.Catalog); err != nil {
		return nil, err
	}
	access := options.Logger
	if access == nil {
		access = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	server := newSDKServer(deps, sdkServerOptionsWithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), transportHTTP, access)
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		Stateless:                    true,
		JSONResponse:                 false,
		PropagateRequestCancellation: true,
		MaxRequestBodyBytes:          httpMaxRequestBodyBytes,
	})
	return &httpGuard{next: streamable, address: options.Address, bearer: options.Bearer, access: access}, nil
}

type httpGuard struct {
	next    http.Handler
	address string
	bearer  string
	access  *slog.Logger
}

func (g *httpGuard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	defer func() {
		g.access.Info("mcp_http_request", "method", r.Method, "status", recorder.status, "duration_ms", time.Since(started).Milliseconds())
	}()
	switch {
	case r.URL.Path != HTTPEndpointPath:
		http.Error(recorder, "not found", http.StatusNotFound)
	case r.Host != g.address:
		http.Error(recorder, "forbidden host", http.StatusForbidden)
	case r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+g.address:
		http.Error(recorder, "forbidden origin", http.StatusForbidden)
	case !g.authorized(r.Header.Get("Authorization")):
		recorder.Header().Set("WWW-Authenticate", `Bearer realm="issueops"`)
		http.Error(recorder, "unauthorized", http.StatusUnauthorized)
	default:
		r.Body = http.MaxBytesReader(recorder, r.Body, httpMaxRequestBodyBytes)
		g.next.ServeHTTP(recorder, r)
	}
}

func (g *httpGuard) authorized(header string) bool {
	token, ok := strings.CutPrefix(header, "Bearer ")
	return ok && subtle.ConstantTimeCompare([]byte(token), []byte(g.bearer)) == 1
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status, r.wroteHeader = status, true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// httpToolHandler binds correlation only from this request's traceparent
// header and logs the tool outcome without arguments or results.
func httpToolHandler(next mcp.ToolHandler, name string, bindTrace func(context.Context, string) (context.Context, bool), access *slog.Logger) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		started := time.Now()
		if bindTrace != nil {
			raw := ""
			if req.Extra != nil && req.Extra.Header != nil {
				raw = req.Extra.Header.Get("traceparent")
			}
			bound, valid := bindTrace(ctx, raw)
			ctx = bound
			if !valid {
				access.Warn("mcp_http_trace", "tool", name, "outcome", "invalid_traceparent_ignored")
			}
		}
		result, err := next(ctx, req)
		outcome := "ok"
		if err != nil {
			outcome = "protocol_error"
		} else if result != nil && result.IsError {
			outcome = "tool_error"
		}
		access.Info("mcp_http_tool", "tool", name, "outcome", outcome, "duration_ms", time.Since(started).Milliseconds())
		return result, err
	}
}

// ServeHTTP serves until ctx ends, then stops accepting, drains in-flight
// requests for up to ten seconds, and finally cancels whatever remains.
func ServeHTTP(ctx context.Context, listener net.Listener, handler http.Handler) error {
	base, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		BaseContext:       func(net.Listener) context.Context { return base },
		ErrorLog:          log.New(io.Discard, "", 0),
	}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	drain, stop := context.WithTimeout(context.Background(), httpShutdownDrain)
	defer stop()
	shutdownErr := server.Shutdown(drain)
	cancel()
	if errors.Is(shutdownErr, context.DeadlineExceeded) {
		shutdownErr = server.Close()
	}
	if err := <-served; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return shutdownErr
}
