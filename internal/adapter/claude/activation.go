// Package claude is the host installer adapter for Claude Code: it links native
// skills, writes the user and project MCP configuration and the lifecycle hooks
// in settings.json, and verifies the result by reading it back.
package claude

import (
	"fmt"
	"path/filepath"

	"issueops/internal/port"
)

// VerifyActivation reads back the issueops MCP entry in ~/.claude.json and the
// lifecycle hooks in ~/.claude/settings.json and returns activation evidence for
// both. It fails when either differs from what Install writes.
func (installer Installer) VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	mcpPath := filepath.Join(req.Home, ".claude.json")
	expectedDigest, err := installer.deps.VerifyJSONMapEntry(mcpPath, []string{"mcpServers"}, "issueops", "Claude MCP readback", func() (map[string]any, error) {
		return claudeUserMCPServer(req), nil
	}, installer.deps.SemanticSHA256)
	if err != nil {
		return nil, err
	}
	hooksPath := filepath.Join(req.Home, ".claude", "settings.json")
	hooksDigest, err := installer.deps.VerifyHookActivation(hooksPath, installer.claudeSettingsConfig(req.BinPath))
	if err != nil {
		return nil, fmt.Errorf("Claude hook readback failed: %w", err) //nolint:staticcheck,nolintlint // error string starts with the proper noun Claude
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
