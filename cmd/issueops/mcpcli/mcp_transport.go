package mcpcli

import (
	"context"
	"io"
	"os"
)

func RunMCPWithDependencies(deps MCPDependencies) error {
	if os.Getenv("ISSUEOPS_MCP_DIRECT") == "1" {
		return ServeMCPStreamWithDependencies(os.Stdin, os.Stdout, os.Stderr, deps)
	}
	return RunMCPProxy()
}

func ServeMCPStreamWithDependencies(input io.Reader, output io.Writer, diagnostics io.Writer, deps MCPDependencies) error {
	return ServeMCPStreamContextWithDependencies(context.Background(), input, output, diagnostics, deps)
}

func ServeMCPStreamContext(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer) error {
	return ServeMCPStreamContextWithDependencies(ctx, input, output, diagnostics, MCPDependencies{})
}

func ServeMCPStreamContextWithDependencies(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer, deps MCPDependencies) error {
	return serveMCPStreamSDK(ctx, input, output, diagnostics, deps)
}
