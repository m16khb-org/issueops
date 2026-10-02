package omo

import (
	"fmt"
	"os"
	"path/filepath"

	"issueops/internal/port"
)

func (installer Installer) VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	omoRoot := filepath.Join(req.Home, ".omo")
	mcpPath := filepath.Join(omoRoot, "mcp.json")
	expectedDigest, err := installer.deps.VerifyJSONMapEntry(mcpPath, "mcpServers", "issueops", "Omo MCP readback", func() (map[string]any, error) {
		return installer.omoUserMCPServer(req)
	}, installer.deps.SemanticSHA256)
	if err != nil {
		return nil, err
	}

	extensionPath := filepath.Join(omoRoot, "extensions", "issueops.js")
	extension, err := os.ReadFile(extensionPath)
	if err != nil {
		return nil, err
	}
	expectedExtension := installer.lifecycleExtension(req.BinPath)
	if string(extension) != expectedExtension {
		return nil, fmt.Errorf("Omo lifecycle extension does not match the canonical managed content")
	}
	extensionDigest, err := installer.deps.SemanticSHA256(map[string]any{
		"host": "omo", "surface": "hooks", "content": expectedExtension,
	})
	if err != nil {
		return nil, err
	}

	mcpEvidence, err := installer.deps.CaptureNativeActivationEvidence("omo", "mcp", mcpPath, expectedDigest)
	if err != nil {
		return nil, err
	}
	hookEvidence, err := installer.deps.CaptureNativeActivationEvidence("omo", "hooks", extensionPath, extensionDigest)
	if err != nil {
		return nil, err
	}
	return []port.NativeActivationEvidence{mcpEvidence, hookEvidence}, nil
}
