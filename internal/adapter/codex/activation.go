package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/port"
)

// VerifyActivation reads back ~/.codex/config.toml and hooks.json under
// req.CodexHome and returns one NativeActivationEvidence each for the MCP
// block and the hooks. It fails when config.toml does not contain exactly one
// canonical issueops MCP server block (ending the file) or when the hooks do
// not match the expected hook configuration.
func (installer Installer) VerifyActivation(req port.NativeInstallRequest) ([]port.NativeActivationEvidence, error) {
	configPath := filepath.Join(req.CodexHome, "config.toml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	text := string(config)
	expectedBlock := installer.codexGlobalBlock(req)
	wantEnvSections := 1
	if req.MCPTransport == mcpTransportHTTP {
		wantEnvSections = 0
	}
	if strings.Count(text, "[mcp_servers.issueops]") != 1 || strings.Count(text, "[mcp_servers.issueops.env]") != wantEnvSections ||
		!strings.HasSuffix(text, expectedBlock) {
		return nil, fmt.Errorf("Codex MCP readback does not contain exactly one canonical issueops server") //nolint:staticcheck,nolintlint // error string starts with the proper noun Codex
	}
	mcpDigest, err := installer.deps.SemanticSHA256(map[string]any{
		"host": "codex", "surface": "mcp", "block": expectedBlock,
	})
	if err != nil {
		return nil, err
	}
	hooksPath := filepath.Join(req.CodexHome, "hooks.json")
	hooksDigest, err := installer.deps.VerifyHookActivation(hooksPath, HooksConfig(req.BinPath))
	if err != nil {
		return nil, fmt.Errorf("Codex hook readback failed: %w", err) //nolint:staticcheck,nolintlint // error string starts with the proper noun Codex
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
