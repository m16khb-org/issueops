package opencode

import (
	"fmt"
	"os"
	"path/filepath"

	"issueops/internal/port"
)

// VerifyActivation reads back the managed MCP entry by meaning and the plugin
// by exact bytes before the install transaction commits.
func (installer Installer) VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	configRoot := installer.configRoot(req.Home)
	mcpPath := filepath.Join(configRoot, "opencode.json")
	expectedDigest, err := installer.deps.VerifyJSONMapEntry(mcpPath, mcpServersPath, "issueops", "opencode MCP readback", func() (map[string]any, error) {
		return installer.userMCPServer(req)
	}, installer.deps.SemanticSHA256)
	if err != nil {
		return nil, err
	}

	pluginPath := filepath.Join(configRoot, "plugins", "issueops.js")
	plugin, err := os.ReadFile(pluginPath)
	if err != nil {
		return nil, err
	}
	expectedPlugin := installer.lifecycle(req.BinPath)
	if string(plugin) != expectedPlugin {
		return nil, fmt.Errorf("opencode lifecycle plugin does not match the canonical managed content")
	}
	pluginDigest, err := installer.deps.SemanticSHA256(map[string]any{
		"host": host, "surface": "hooks", "content": expectedPlugin,
	})
	if err != nil {
		return nil, err
	}

	mcpEvidence, err := installer.deps.CaptureNativeActivationEvidence(host, "mcp", mcpPath, expectedDigest)
	if err != nil {
		return nil, err
	}
	hookEvidence, err := installer.deps.CaptureNativeActivationEvidence(host, "hooks", pluginPath, pluginDigest)
	if err != nil {
		return nil, err
	}
	return []port.NativeActivationEvidence{mcpEvidence, hookEvidence}, nil
}
