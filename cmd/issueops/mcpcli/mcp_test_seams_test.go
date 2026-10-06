package mcpcli

import (
	"io"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func initSDKServer(deps MCPDependencies) *mcp.Server {
	return initSDKServerWithDiagnostics(deps, io.Discard)
}

func sdkServerOptions() *mcp.ServerOptions {
	return sdkServerOptionsWithDiagnostics(io.Discard)
}
