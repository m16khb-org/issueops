package claude

import (
	"os"

	"issueops/internal/port"
)

const mcpTransportHTTP = "http"

func (installer Installer) writeClaudeUserMCP(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "claude_user_mcp_config"}
	config, err := installer.deps.MergeJSONMapFile(path, "mcpServers", "issueops", req.DryRun, func() (map[string]any, error) {
		return claudeUserMCPServer(req), nil
	})
	if err != nil {
		return file, err
	}
	if req.MCPTransport == mcpTransportHTTP && !req.DryRun {
		if err := os.Chmod(path, 0o600); err != nil && !os.IsNotExist(err) {
			return file, err
		}
	}
	return installer.deps.WriteJSONPlan(path, file.Kind, config, 0o600, req.DryRun)
}

func claudeUserMCPServer(req port.NativeInstallRequest) map[string]any {
	if req.MCPTransport == mcpTransportHTTP {
		return map[string]any{
			"type":    "http",
			"url":     req.MCPURL,
			"headers": map[string]any{"Authorization": "Bearer " + req.MCPBearer},
		}
	}
	return map[string]any{
		"type":    "stdio",
		"command": req.BinPath,
		"args":    []string{"mcp"},
		"env": map[string]any{
			"ISSUEOPS_ROOT": req.Root,
		},
	}
}

func (installer Installer) removeClaudeProjectMCP(path string, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "claude_project_mcp_config"}
	config, removed, err := installer.deps.RemoveJSONMapEntry(path, "mcpServers", "issueops_project")
	if err != nil || !removed {
		return file, err
	}
	return installer.deps.WriteJSONPlan(path, file.Kind, config, 0o644, dryRun)
}
