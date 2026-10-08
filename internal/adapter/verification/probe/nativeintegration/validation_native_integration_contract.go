package nativeintegration

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

func nativeIntegrationRequiredPaths(root, home string, codexSkills, claudeSkills, omoSkills, ompSkills []string) []string {
	paths := []string{
		filepath.Join(root, "configs", "codex", "mcp.config.toml"),
		filepath.Join(root, "configs", "codex", "hooks.json"),
		filepath.Join(root, "configs", "claude", "mcp.project.json"),
		filepath.Join(root, "configs", "omo", "mcp.json"),
		filepath.Join(root, "configs", "omo", "issueops.js"),
		filepath.Join(root, "configs", "omp", "mcp.json"),
		filepath.Join(root, "configs", "omp", "issueops.js"),
	}
	for _, nativeSkill := range codexSkills {
		paths = append(paths, filepath.Join(home, ".codex", "skills", nativeSkill, "SKILL.md"))
	}
	for _, nativeSkill := range claudeSkills {
		paths = append(paths, filepath.Join(home, ".claude", "skills", nativeSkill, "SKILL.md"))
	}
	for _, nativeSkill := range omoSkills {
		paths = append(paths, filepath.Join(home, ".omo", "agent", "skills", nativeSkill, "SKILL.md"))
	}
	for _, nativeSkill := range ompSkills {
		paths = append(paths, filepath.Join(home, ".omp", "agent", "skills", nativeSkill, "SKILL.md"))
	}
	return paths
}

func nativeIntegrationPathErrors(paths []string, deps nativeIntegrationValidationDeps) []string {
	errs := []string{}
	for _, path := range paths {
		if !deps.exists(path) {
			errs = append(errs, "missing "+path)
		}
	}
	return errs
}

func nativeIntegrationCodexConfigErrors(root, home string, deps nativeIntegrationValidationDeps) []string {
	errs := []string{}
	if b, err := deps.readFile(filepath.Join(home, ".codex", "config.toml")); err != nil || !strings.Contains(string(b), "[mcp_servers.issueops]") {
		errs = append(errs, "Codex MCP config missing issueops")
	}
	expectedBinary, err := deps.canonicalHarnessBinary(root)
	if err != nil {
		errs = append(errs, "resolve stable native root: "+err.Error())
		return errs
	}
	if b, err := deps.readFile(filepath.Join(home, ".codex", "hooks.json")); err != nil || !deps.hasThinCodexContextHooks(string(b), expectedBinary) {
		errs = append(errs, "Codex thin context hooks missing issueops context hook surface")
	}
	return errs
}

func (deps nativeIntegrationValidationDeps) hasThinCodexContextHooks(config, expectedBinary string) bool {
	if deps.CodexHooksConfig == nil || deps.VerifyHookConfigActivation == nil {
		return false
	}
	var actual map[string]any
	if json.Unmarshal([]byte(config), &actual) != nil {
		return false
	}
	_, err := deps.VerifyHookConfigActivation(actual, deps.CodexHooksConfig(expectedBinary))
	return err == nil
}

// lifecycleHost names one pi-style host whose MCP config and lifecycle
// extension module are installed under the user's home.
type lifecycleHost struct {
	label         string
	mcpPath       string
	extensionPath string
	render        func(string) string
	events        string
}

func nativeIntegrationOmoConfigErrors(root, home string, deps nativeIntegrationValidationDeps) []string {
	return nativeIntegrationLifecycleHostErrors(root, deps, lifecycleHost{
		label:         "Omo",
		mcpPath:       filepath.Join(home, ".omo", "mcp.json"),
		extensionPath: filepath.Join(home, ".omo", "extensions", "issueops.js"),
		render:        deps.OmoLifecycleExtension,
		events:        "session_start/session_compact",
	})
}

func nativeIntegrationOmpConfigErrors(root, home string, deps nativeIntegrationValidationDeps) []string {
	return nativeIntegrationLifecycleHostErrors(root, deps, lifecycleHost{
		label:         "omp",
		mcpPath:       filepath.Join(home, ".omp", "agent", "mcp.json"),
		extensionPath: filepath.Join(home, ".omp", "agent", "extensions", "issueops.js"),
		render:        deps.OmpLifecycleExtension,
		events:        "session_start/session_switch/session_compact",
	})
}

func nativeIntegrationLifecycleHostErrors(root string, deps nativeIntegrationValidationDeps, host lifecycleHost) []string {
	stableRoot, err := deps.canonicalStableNativeRoot(root)
	if err != nil {
		return []string{"resolve stable native root for " + host.label + ": " + err.Error()}
	}
	expectedBinary := filepath.Join(stableRoot, "bin", "issueops")
	errs := []string{}
	if body, readErr := deps.readFile(host.mcpPath); readErr != nil || !hasCanonicalLifecycleHostMCP(body, expectedBinary, stableRoot) {
		errs = append(errs, host.label+" MCP config missing canonical issueops server")
	}
	if host.render == nil {
		errs = append(errs, host.label+" lifecycle extension renderer is unavailable")
	} else if body, readErr := deps.readFile(host.extensionPath); readErr != nil || string(body) != host.render(expectedBinary) {
		errs = append(errs, host.label+" lifecycle extension missing canonical "+host.events+" surface")
	}
	return errs
}

func hasCanonicalLifecycleHostMCP(body []byte, expectedBinary, root string) bool {
	var config map[string]any
	if json.Unmarshal(body, &config) != nil {
		return false
	}
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		return false
	}
	server, ok := servers["issueops"].(map[string]any)
	if !ok {
		return false
	}
	if server["type"] == "http" {
		if _, present := server["command"]; present {
			return false
		}
		if _, present := server["args"]; present {
			return false
		}
		address, _ := server["url"].(string)
		endpoint, err := url.Parse(address)
		if err != nil || endpoint.Scheme != "http" || !net.ParseIP(endpoint.Hostname()).IsLoopback() ||
			endpoint.Path != "/mcp" || endpoint.User != nil || endpoint.RawQuery != "" ||
			endpoint.ForceQuery || endpoint.Fragment != "" {
			return false
		}
		port, err := strconv.Atoi(endpoint.Port())
		if err != nil || port < 1 || port > 65535 {
			return false
		}
		headers, _ := server["headers"].(map[string]any)
		authorization, _ := headers["Authorization"].(string)
		bearer, ok := strings.CutPrefix(authorization, "Bearer ")
		if !ok || strings.ContainsAny(bearer, " \t\r\n") {
			return false
		}
		decoded, err := base64.RawURLEncoding.DecodeString(bearer)
		return err == nil && len(decoded) >= 32
	}
	if server["command"] != expectedBinary {
		return false
	}
	args, ok := server["args"].([]any)
	if !ok || len(args) != 1 || args[0] != "mcp" {
		return false
	}
	env, ok := server["env"].(map[string]any)
	return ok && env["ISSUEOPS_ROOT"] == root
}

func (deps nativeIntegrationValidationDeps) canonicalHarnessBinary(root string) (string, error) {
	stableRoot, err := deps.canonicalStableNativeRoot(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(stableRoot, "bin", "issueops"), nil
}

func (deps nativeIntegrationValidationDeps) canonicalStableNativeRoot(root string) (string, error) {
	if deps.ResolveStableNativeRoot == nil {
		return "", fmt.Errorf("stable native root resolver is unavailable")
	}
	return deps.ResolveStableNativeRoot(root)
}

func nativeIntegrationDuplicateWarningOutput(fixture string) ([]string, string) {
	duplicateWarnings := DetectClaudeMCPDuplicateWarnings(fixture)
	warningBytes, _ := json.MarshalIndent(map[string]any{
		"duplicate_mcp_warning_fixture": duplicateWarnings,
	}, "", "  ")
	errs := []string{}
	if len(duplicateWarnings) != 1 || duplicateWarnings[0].Server != "issueops" || !strings.Contains(duplicateWarnings[0].Message, "multiple scopes") {
		errs = append(errs, "Claude duplicate MCP warning fixture was not classified")
	}
	return errs, string(warningBytes)
}
