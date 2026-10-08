package extensionhost

import (
	"fmt"
	"os"
	"path/filepath"

	"issueops/internal/port"
)

func (installer Installer) VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	host := installer.spec.Host
	configRoot := installer.configRoot(req.Home)
	mcpPath := filepath.Join(configRoot, "mcp.json")
	expectedDigest, err := installer.deps.VerifyJSONMapEntry(mcpPath, "mcpServers", "issueops", installer.spec.DisplayName+" MCP readback", func() (map[string]any, error) {
		return installer.userMCPServer(req)
	}, installer.deps.SemanticSHA256)
	if err != nil {
		return nil, err
	}

	extensionPath := filepath.Join(configRoot, "extensions", "issueops.js")
	extension, err := os.ReadFile(extensionPath)
	if err != nil {
		return nil, err
	}
	expectedExtension := installer.lifecycleExtension(req.BinPath)
	if string(extension) != expectedExtension {
		return nil, fmt.Errorf("%s lifecycle extension does not match the canonical managed content", installer.spec.DisplayName)
	}
	extensionDigest, err := installer.deps.SemanticSHA256(map[string]any{
		"host": host, "surface": "hooks", "content": expectedExtension,
	})
	if err != nil {
		return nil, err
	}

	mcpEvidence, err := installer.deps.CaptureNativeActivationEvidence(host, "mcp", mcpPath, expectedDigest)
	if err != nil {
		return nil, err
	}
	hookEvidence, err := installer.deps.CaptureNativeActivationEvidence(host, "hooks", extensionPath, extensionDigest)
	if err != nil {
		return nil, err
	}
	return []port.NativeActivationEvidence{mcpEvidence, hookEvidence}, nil
}
