package omo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"issueops/internal/port"
)

const (
	omoMCPCatalogSHA256Env = "ISSUEOPS_MCP_CATALOG_SHA256"
	// omoMCPCatalogHeader carries the catalog digest on the HTTP entry so Omo's
	// config-keyed catalog cache turns over when the advertised tools change.
	// The server ignores it.
	omoMCPCatalogHeader = "X-Issueops-Mcp-Catalog-Sha256"
	mcpTransportHTTP    = "http"
)

func (installer Installer) writeOmoUserMCP(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "omo_user_mcp_config"}
	config, err := installer.deps.MergeJSONMapFile(path, "mcpServers", "issueops", req.DryRun, func() (map[string]any, error) {
		return installer.omoUserMCPServer(req)
	})
	if err != nil {
		return file, err
	}
	if req.MCPTransport == mcpTransportHTTP && !req.DryRun {
		if err := os.Chmod(path, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return file, err
		}
	}
	return installer.deps.WriteJSONPlan(path, file.Kind, config, 0o600, req.DryRun)
}

func (installer Installer) removeOmoProjectMCP(path string, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "omo_project_mcp_config"}
	config, removed, err := installer.deps.RemoveJSONMapEntry(path, "mcpServers", "issueops_project")
	if err != nil || !removed {
		return file, err
	}
	return installer.deps.WriteJSONPlan(path, file.Kind, config, 0o644, dryRun)
}

func (installer Installer) writeOmoProjectMCP(path, kind string, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: kind}
	config, err := installer.omoProjectMCPConfig()
	if err != nil {
		return file, err
	}
	return installer.deps.WriteJSONPlan(path, kind, config, 0o644, dryRun)
}

func (installer Installer) omoUserMCPServer(req port.NativeInstallRequest) (map[string]any, error) {
	if req.MCPTransport == mcpTransportHTTP {
		catalogSHA256, err := installer.catalogSHA256()
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"type": "http",
			"url":  req.MCPURL,
			"headers": map[string]any{
				"Authorization":     "Bearer " + req.MCPBearer,
				omoMCPCatalogHeader: catalogSHA256,
			},
		}, nil
	}
	return installer.omoMCPServer(req.BinPath, req.Root)
}

func (installer Installer) catalogSHA256() (string, error) {
	if installer.deps.MCPCatalogSHA256 == nil {
		return "", fmt.Errorf("Omo MCP catalog digest is not configured")
	}
	catalogSHA256, err := installer.deps.MCPCatalogSHA256()
	if err != nil {
		return "", fmt.Errorf("compute Omo MCP catalog digest: %w", err)
	}
	if catalogSHA256 == "" {
		return "", fmt.Errorf("compute Omo MCP catalog digest: empty digest")
	}
	return catalogSHA256, nil
}

func (installer Installer) omoProjectMCPConfig() (map[string]any, error) {
	server, err := installer.omoMCPServer("./bin/issueops", ".")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"mcpServers": map[string]any{
			"issueops_project": server,
		},
	}, nil
}

func (installer Installer) omoMCPServer(command, root string) (map[string]any, error) {
	catalogSHA256, err := installer.catalogSHA256()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"command": command,
		"args":    []string{"mcp"},
		"env": map[string]any{
			"ISSUEOPS_ROOT":        root,
			omoMCPCatalogSHA256Env: catalogSHA256,
		},
	}, nil
}
