// Package agy is the host installer adapter for the Antigravity (agy) host: it
// links native skills, writes the user and project MCP configuration under
// ~/.gemini/config, and verifies the result by reading it back.
package agy

import (
	"path/filepath"

	"issueops/internal/port"
)

// VerifyActivation reads back the issueops entry in the user MCP config
// (~/.gemini/config/mcp_config.json) and returns activation evidence for it. It
// fails when the entry is missing or differs from what Install writes.
func (installer Installer) VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	geminiRoot := filepath.Join(req.Home, ".gemini", "config")
	mcpPath := filepath.Join(geminiRoot, "mcp_config.json")
	expectedDigest, err := installer.deps.VerifyJSONMapEntry(mcpPath, []string{"mcpServers"}, "issueops", "agy MCP readback", func() (map[string]any, error) {
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
