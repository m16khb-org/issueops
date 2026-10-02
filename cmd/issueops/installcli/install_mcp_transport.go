package installcli

import (
	"context"
	"fmt"

	"issueops/internal/contract/mcpservice"
	"issueops/internal/port"
)

const (
	mcpTransportHTTP  = "http"
	mcpTransportStdio = "stdio"
)

type MCPServiceSetup interface {
	ReadBearer() (string, error)
	Prepare(context.Context) (string, error)
	EnsureRunning(context.Context, string) (mcpservice.Status, error)
}

func (c Command) resolveMCPTransport(flagValue string) (string, error) {
	transport := flagValue
	if transport == "" {
		transport = c.DefaultMCPTransport
	}
	if transport == "" {
		transport = mcpTransportStdio
	}
	if transport != mcpTransportHTTP && transport != mcpTransportStdio {
		return "", fmt.Errorf("invalid --mcp-transport %q: expected http or stdio", transport)
	}
	return transport, nil
}

// applyMCPTransport orders HTTP setup before any host config merge:
// dry-run only reads the existing credential; begin prepares the credential
// and supervisor unit; a real install additionally requires the service to
// run the target build and answer an authenticated MCP call.
func (c Command) applyMCPTransport(ctx context.Context, req *port.NativeInstallRequest, transport, step string) error {
	req.MCPTransport = transport
	if transport != mcpTransportHTTP || step == "abort" {
		return nil
	}
	if c.MCPService == nil {
		return fmt.Errorf("mcp http service setup is not configured; use --mcp-transport=stdio")
	}
	req.MCPURL = c.MCPURL
	if !req.DryRun {
		if err := c.preflightHostPlan(*req); err != nil {
			return err
		}
	}
	if req.DryRun {
		bearer, err := c.MCPService.ReadBearer()
		req.MCPBearer = bearer
		return err
	}
	bearer, err := c.MCPService.Prepare(ctx)
	if err != nil {
		return fmt.Errorf("prepare mcp http service: %w", err)
	}
	req.MCPBearer = bearer
	if step == "begin" {
		return nil
	}
	status, err := c.MCPService.EnsureRunning(ctx, req.BinPath)
	if err != nil {
		return fmt.Errorf("mcp http service is not ready (status=%s error_code=%s); host MCP configs were not changed: %w", status.Status, status.ErrorCode, err)
	}
	return nil
}

// preflightHostPlan validates the root and every host plan without writing
// anything, so a failed install never leaves a credential, unit, or running
// service behind. The plan carries no bearer because nothing is written.
func (c Command) preflightHostPlan(req port.NativeInstallRequest) error {
	req.DryRun = true
	result, err := c.InstallNative(req)
	if err == nil && !result.OK {
		err = fmt.Errorf("native installer dry-run preflight reported ok=false")
	}
	if err != nil {
		return fmt.Errorf("native install preflight failed before the mcp http service was changed: %w", err)
	}
	return nil
}
