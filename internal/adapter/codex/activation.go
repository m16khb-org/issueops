package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/port"
)

func (installer Installer) VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	configPath := filepath.Join(req.CodexHome, "config.toml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	text := string(config)
	expectedBlock := installer.codexGlobalBlock(req)
	if strings.Count(text, "[mcp_servers.issueops]") != 1 || strings.Count(text, "[mcp_servers.issueops.env]") != 1 ||
		!strings.HasSuffix(text, expectedBlock) {
		return nil, fmt.Errorf("Codex MCP readback does not contain exactly one canonical issueops server")
	}
	mcpDigest, err := installer.deps.SemanticSHA256(map[string]any{
		"host": "codex", "surface": "mcp", "block": expectedBlock,
	})
	if err != nil {
		return nil, err
	}
	hooksPath := filepath.Join(req.CodexHome, "hooks.json")
	hooksDigest, err := installer.deps.VerifyHookActivation(hooksPath, codexHooksConfig(req.BinPath))
	if err != nil {
		return nil, fmt.Errorf("Codex hook readback failed: %w", err)
	}
	mcpEvidence, err := installer.deps.CaptureNativeActivationEvidence("codex", "mcp", configPath, mcpDigest)
	if err != nil {
		return nil, err
	}
	hookEvidence, err := installer.deps.CaptureNativeActivationEvidence("codex", "hooks", hooksPath, hooksDigest)
	if err != nil {
		return nil, err
	}
	return []port.NativeActivationEvidence{mcpEvidence, hookEvidence}, nil
}
