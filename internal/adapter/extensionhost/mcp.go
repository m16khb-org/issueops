package extensionhost

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"issueops/internal/port"
)

const (
	mcpCatalogSHA256Env = "ISSUEOPS_MCP_CATALOG_SHA256"
	// mcpCatalogHeader carries the catalog digest on the HTTP entry so the
	// host's config-keyed catalog cache turns over when the advertised tools
	// change. The server ignores it.
	mcpCatalogHeader = "X-Issueops-Mcp-Catalog-Sha256"
	mcpTransportHTTP = "http"
)

func (installer Installer) writeUserMCP(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: installer.spec.Host + "_user_mcp_config"}
	config, err := installer.deps.MergeJSONMapFile(path, "mcpServers", "issueops", req.DryRun, func() (map[string]any, error) {
		return installer.userMCPServer(req)
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

func (installer Installer) removeProjectMCP(path string, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: installer.spec.Host + "_project_mcp_config"}
	config, removed, err := installer.deps.RemoveJSONMapEntry(path, "mcpServers", "issueops_project")
	if err != nil || !removed {
		return file, err
	}
	return installer.deps.WriteJSONPlan(path, file.Kind, config, 0o644, dryRun)
}

func (installer Installer) writeProjectMCP(path, kind string, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: kind}
	config, err := installer.projectMCPConfig()
	if err != nil {
		return file, err
	}
	return installer.deps.WriteJSONPlan(path, kind, config, 0o644, dryRun)
}

func (installer Installer) userMCPServer(req port.NativeInstallRequest) (map[string]any, error) {
	if req.MCPTransport == mcpTransportHTTP {
		catalogSHA256, err := installer.catalogSHA256()
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"type": "http",
			"url":  req.MCPURL,
			"headers": map[string]any{
				"Authorization":  "Bearer " + req.MCPBearer,
				mcpCatalogHeader: catalogSHA256,
			},
		}, nil
	}
	return installer.mcpServer(req.BinPath, req.Root)
}

func (installer Installer) catalogSHA256() (string, error) {
	name := installer.spec.DisplayName
	if installer.deps.MCPCatalogSHA256 == nil {
		return "", fmt.Errorf("%s MCP catalog digest is not configured", name)
	}
	catalogSHA256, err := installer.deps.MCPCatalogSHA256()
	if err != nil {
		return "", fmt.Errorf("compute %s MCP catalog digest: %w", name, err)
	}
	if catalogSHA256 == "" {
		return "", fmt.Errorf("compute %s MCP catalog digest: empty digest", name)
	}
	return catalogSHA256, nil
}

func (installer Installer) projectMCPConfig() (map[string]any, error) {
	server, err := installer.mcpServer("./bin/issueops", ".")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"mcpServers": map[string]any{
			"issueops_project": server,
		},
	}, nil
}

func (installer Installer) mcpServer(command, root string) (map[string]any, error) {
	catalogSHA256, err := installer.catalogSHA256()
	if err != nil {
		return nil, err
	}
	server := map[string]any{
		"command": command,
		"args":    []string{"mcp"},
		"env": map[string]any{
			"ISSUEOPS_ROOT":     root,
			mcpCatalogSHA256Env: catalogSHA256,
		},
	}
	if installer.spec.StdioType != "" {
		server["type"] = installer.spec.StdioType
	}
	return server, nil
}
