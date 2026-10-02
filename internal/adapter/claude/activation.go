package claude

import (
	"fmt"
	"path/filepath"

	"issueops/internal/port"
)

func (installer Installer) VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	mcpPath := filepath.Join(req.Home, ".claude.json")
	expectedDigest, err := installer.deps.VerifyJSONMapEntry(mcpPath, "mcpServers", "issueops", "Claude MCP readback", func() (map[string]any, error) {
		return claudeUserMCPServer(req), nil
	}, installer.deps.SemanticSHA256)
	if err != nil {
		return nil, err
	}
	hooksPath := filepath.Join(req.Home, ".claude", "settings.json")
	hooksDigest, err := installer.deps.VerifyHookActivation(hooksPath, installer.claudeSettingsConfig(req.BinPath))
	if err != nil {
		return nil, fmt.Errorf("Claude hook readback failed: %w", err)
	}
	mcpEvidence, err := installer.deps.CaptureNativeActivationEvidence("claude", "mcp", mcpPath, expectedDigest)
	if err != nil {
		return nil, err
	}
	hookEvidence, err := installer.deps.CaptureNativeActivationEvidence("claude", "hooks", hooksPath, hooksDigest)
	if err != nil {
		return nil, err
	}
	return []port.NativeActivationEvidence{mcpEvidence, hookEvidence}, nil
}
