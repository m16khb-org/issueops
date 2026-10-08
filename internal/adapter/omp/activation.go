package omp

import (
	"fmt"
	"os"
	"path/filepath"

	"issueops/internal/port"
)

func (installer Installer) VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	agentRoot := ompAgentRoot(req.Home)
	mcpPath := filepath.Join(agentRoot, "mcp.json")
	expectedDigest, err := installer.deps.VerifyJSONMapEntry(mcpPath, "mcpServers", "issueops", "omp MCP readback", func() (map[string]any, error) {
		return installer.ompUserMCPServer(req)
	}, installer.deps.SemanticSHA256)
	if err != nil {
		return nil, err
	}

	extensionPath := filepath.Join(agentRoot, "extensions", "issueops.js")
	extension, err := os.ReadFile(extensionPath)
	if err != nil {
		return nil, err
	}
	expectedExtension := installer.lifecycleExtension(req.BinPath)
	if string(extension) != expectedExtension {
		return nil, fmt.Errorf("omp lifecycle extension does not match the canonical managed content")
	}
	extensionDigest, err := installer.deps.SemanticSHA256(map[string]any{
		"host": "omp", "surface": "hooks", "content": expectedExtension,
	})
	if err != nil {
		return nil, err
	}

	mcpEvidence, err := installer.deps.CaptureNativeActivationEvidence("omp", "mcp", mcpPath, expectedDigest)
	if err != nil {
		return nil, err
	}
	hookEvidence, err := installer.deps.CaptureNativeActivationEvidence("omp", "hooks", extensionPath, extensionDigest)
	if err != nil {
		return nil, err
	}
	return []port.NativeActivationEvidence{mcpEvidence, hookEvidence}, nil
}
