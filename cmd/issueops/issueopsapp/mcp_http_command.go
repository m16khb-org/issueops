package issueopsapp

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"issueops/cmd/issueops/mcpcli"
	mcpserviceadapter "issueops/internal/adapter/mcpservice"
	statestore "issueops/internal/adapter/outbound/state"
)

// runMCPHTTP serves the shared Streamable HTTP endpoint in the foreground.
// Supervisor install and start/stop belong to the service port.
func runMCPHTTP(args []string) error {
	fs := flag.NewFlagSet("issueops mcp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	httpMode := fs.Bool("http", false, "serve the shared Streamable HTTP endpoint")
	address := fs.String("addr", mcpcli.DefaultHTTPAddress, "loopback host:port to listen on")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*httpMode {
		return fmt.Errorf("issueops mcp: use --http to serve HTTP, or no flags for stdio")
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected mcp argument %q", fs.Arg(0))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serveMCPHTTP(ctx, *address, statestore.StateDir(), issueOpsMCPHTTPDependencies(), os.Stderr)
}

func serveMCPHTTP(ctx context.Context, address, stateDir string, deps mcpcli.MCPDependencies, diagnostics io.Writer) error {
	if err := mcpcli.ValidateHTTPAddress(address); err != nil {
		return err
	}
	bearer, err := mcpcli.EnsureHTTPBearer(stateDir)
	if err != nil {
		return fmt.Errorf("mcp http bearer: %w", err)
	}
	instance, err := mcpserviceadapter.AcquireInstance(stateDir)
	if err != nil {
		return fmt.Errorf("mcp http: %w", err)
	}
	defer instance.Release()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("mcp http: %s is unavailable (conflict): %w", address, err)
	}
	bound := listener.Addr().String()
	handler, err := mcpcli.NewHTTPHandler(deps, mcpcli.HTTPOptions{
		Address: bound, Bearer: bearer, Logger: slog.New(slog.NewTextHandler(diagnostics, nil)),
	})
	if err != nil {
		_ = listener.Close()
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		_ = listener.Close()
		return err
	}
	record, err := instance.Publish(mcpServiceProcessInspector(), executable)
	if err != nil {
		_ = listener.Close()
		return err
	}
	handler = mcpserviceadapter.IdentityHandler(handler, record)
	fmt.Fprintf(diagnostics, "issueops mcp http ready url=http://%s%s pid=%d\n", bound, mcpcli.HTTPEndpointPath, os.Getpid())
	return mcpcli.ServeHTTP(ctx, listener, handler)
}
