package mcpcli

import (
	"context"
	"io"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func initSDKServer(deps MCPDependencies) *mcp.Server {
	return initSDKServerWithDiagnostics(deps, io.Discard)
}

func sdkServerOptions() *mcp.ServerOptions {
	return sdkServerOptionsWithDiagnostics(io.Discard)
}

func serveMCPStreamWithDependencies(input io.Reader, output io.Writer, diagnostics io.Writer, deps MCPDependencies) error {
	return ServeMCPStreamContextWithDependencies(context.Background(), input, output, diagnostics, deps)
}
