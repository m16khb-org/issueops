package agy

import (
	"path/filepath"

	"issueops/internal/port"
)

func (installer Installer) VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	geminiRoot := filepath.Join(req.Home, ".gemini", "config")
	mcpPath := filepath.Join(geminiRoot, "mcp_config.json")
	expectedDigest, err := installer.deps.VerifyJSONMapEntry(mcpPath, "mcpServers", "issueops", "agy MCP readback", func() (map[string]any, error) {
		return installer.agyUserMCPServer(req)
	}, installer.deps.SemanticSHA256)
	if err != nil {
		return nil, err
	}

	mcpEvidence, err := installer.deps.CaptureNativeActivationEvidence("agy", "mcp", mcpPath, expectedDigest)
	if err != nil {
		return nil, err
	}
	return []port.NativeActivationEvidence{mcpEvidence}, nil
}
