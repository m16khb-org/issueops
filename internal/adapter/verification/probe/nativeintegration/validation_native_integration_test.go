package nativeintegration

import (
	"encoding/json"
	"errors"
	"fmt"
	"issueops/internal/adapter/codex"
	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/domain/shelltoken"
	"issueops/internal/testsupport"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateNativeIntegrationWithDepsCoversSuccessAndMissingPaths(t *testing.T) {
	root := t.TempDir()
	validator := testNativeValidator()
	validator.ResolveStableNativeRoot = func(string) (string, error) { return root, nil }
	home := t.TempDir()
	existing := nativeIntegrationExpectedPaths(root, home)
	validator.ListSkillNames = func(string) ([]string, error) {
		return []string{"shared", "codex-only", "claude-only", "omo-only", "omp-only", "opencode-only"}, nil
	}
	validator.SkillNamesForHost = func(_ string, _ []string, host string) ([]string, []string) {
		switch host {
		case "codex":
			return []string{"shared", "codex-only"}, nil
		case "claude":
			return []string{"shared", "claude-only"}, nil
		case "omo":
			return []string{"shared", "omo-only"}, nil
		case "omp":
			return []string{"shared", "omp-only"}, nil
		case "opencode":
			return []string{"shared", "opencode-only"}, nil
		default:
			return nil, nil
		}
	}
	deps := nativeIntegrationValidationDeps{
		Validator:   validator,
		userHomeDir: func() (string, error) { return home, nil },
		exists:      func(path string) bool { return existing[path] },
		readFile: func(path string) ([]byte, error) {
			switch {
			case path == filepath.Join(home, ".omo", "mcp.json"):
				return fmt.Appendf(nil, `{"mcpServers":{"issueops":{"command":%q,"args":["mcp"],"env":{"ISSUEOPS_ROOT":%q}}}}`, nativeTestBinary(root), root), nil
			case path == filepath.Join(home, ".omo", "extensions", "issueops.js"):
				return []byte(hostprotocol.OmoLifecycleExtension(nativeTestBinary(root))), nil
			case path == filepath.Join(home, ".omp", "agent", "mcp.json"):
				return fmt.Appendf(nil, `{"mcpServers":{"issueops":{"type":"stdio","command":%q,"args":["mcp"],"env":{"ISSUEOPS_ROOT":%q}}}}`, nativeTestBinary(root), root), nil
			case path == filepath.Join(home, ".omp", "agent", "extensions", "issueops.js"):
				return []byte(hostprotocol.OmpLifecycleExtension(nativeTestBinary(root))), nil
			case path == filepath.Join(home, ".config", "opencode", "opencode.json"):
				return fmt.Appendf(nil, `{"model":"m","mcp":{"servers":{"issueops":{"type":"local","command":[%q,"mcp"],"environment":{"ISSUEOPS_ROOT":%q},"codemode":false}}}}`, nativeTestBinary(root), root), nil
			case path == filepath.Join(home, ".config", "opencode", "plugins", "issueops.js"):
				return []byte(hostprotocol.OpencodeLifecyclePlugin(nativeTestBinary(root))), nil
			}
			switch filepath.Base(path) {
			case "config.toml":
				return []byte("[mcp_servers.issueops]\ncommand = \"issueops\"\n"), nil
			case "hooks.json":
				return managedCodexHooksJSON(nativeTestBinary(root)), nil
			default:
				return nil, errors.New("unexpected read")
			}
		},
		duplicateWarningFixture: claudeMCPDuplicateWarningFixture,
	}

	step := validateNativeIntegrationWithDeps(root, deps)
	if !step.OK || step.Label != "native integration" || !strings.Contains(step.Stdout, "duplicate_mcp_warning_fixture") {
		t.Fatalf("unexpected success step: %#v", step)
	}

	for _, missingPath := range []string{
		filepath.Join(home, ".omo", "agent", "skills", "omo-only", "SKILL.md"),
		filepath.Join(home, ".omp", "agent", "skills", "omp-only", "SKILL.md"),
		filepath.Join(home, ".config", "opencode", "skills", "opencode-only", "SKILL.md"),
		filepath.Join(root, "configs", "opencode", "issueops.js"),
	} {
		existing[missingPath] = false
		failed := validateNativeIntegrationWithDeps(root, deps)
		if failed.OK || !strings.Contains(failed.Error, "missing "+missingPath) {
			t.Fatalf("expected missing path failure, got %#v", failed)
		}
		existing[missingPath] = true
	}
}

func TestNativeIntegrationOmoConfigAcceptsStableRootFromWorktree(t *testing.T) {
	worktreeRoot := t.TempDir()
	stableRoot := t.TempDir()
	home := t.TempDir()
	validator := testNativeValidator()
	validator.ResolveStableNativeRoot = func(string) (string, error) { return stableRoot, nil }
	expectedBinary := nativeTestBinary(stableRoot)
	deps := nativeIntegrationValidationDeps{
		Validator: validator,
		readFile: func(path string) ([]byte, error) {
			switch path {
			case filepath.Join(home, ".omo", "mcp.json"):
				return fmt.Appendf(nil, `{"mcpServers":{"issueops":{"command":%q,"args":["mcp"],"env":{"ISSUEOPS_ROOT":%q}}}}`, expectedBinary, stableRoot), nil
			case filepath.Join(home, ".omo", "extensions", "issueops.js"):
				return []byte(hostprotocol.OmoLifecycleExtension(expectedBinary)), nil
			default:
				return nil, errors.New("unexpected read")
			}
		},
	}

	if errs := nativeIntegrationOmoConfigErrors(worktreeRoot, home, deps); len(errs) != 0 {
		t.Fatalf("stable native config rejected from worktree: %v", errs)
	}
}

func TestNativeIntegrationOmpConfigAcceptsStableRootFromWorktree(t *testing.T) {
	worktreeRoot := t.TempDir()
	stableRoot := t.TempDir()
	home := t.TempDir()
	validator := testNativeValidator()
	validator.ResolveStableNativeRoot = func(string) (string, error) { return stableRoot, nil }
	expectedBinary := nativeTestBinary(stableRoot)
	deps := nativeIntegrationValidationDeps{
		Validator: validator,
		readFile: func(path string) ([]byte, error) {
			switch path {
			case filepath.Join(home, ".omp", "agent", "mcp.json"):
				return fmt.Appendf(nil, `{"mcpServers":{"issueops":{"type":"stdio","command":%q,"args":["mcp"],"env":{"ISSUEOPS_ROOT":%q}}}}`, expectedBinary, stableRoot), nil
			case filepath.Join(home, ".omp", "agent", "extensions", "issueops.js"):
				return []byte(hostprotocol.OmpLifecycleExtension(expectedBinary)), nil
			default:
				return nil, errors.New("unexpected read")
			}
		},
	}

	if errs := nativeIntegrationOmpConfigErrors(worktreeRoot, home, deps); len(errs) != 0 {
		t.Fatalf("stable native config rejected from worktree: %v", errs)
	}
}

// The omo module is not a valid omp extension: it lacks session_switch and the
// main-session id export, so the omp check must reject it.
func TestNativeIntegrationOmpConfigRejectsOmoLifecycleModule(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	validator := testNativeValidator()
	validator.ResolveStableNativeRoot = func(string) (string, error) { return root, nil }
	binary := nativeTestBinary(root)
	deps := nativeIntegrationValidationDeps{Validator: validator, readFile: func(path string) ([]byte, error) {
		switch path {
		case filepath.Join(home, ".omp", "agent", "mcp.json"):
			return fmt.Appendf(nil, `{"mcpServers":{"issueops":{"type":"stdio","command":%q,"args":["mcp"],"env":{"ISSUEOPS_ROOT":%q}}}}`, binary, root), nil
		case filepath.Join(home, ".omp", "agent", "extensions", "issueops.js"):
			return []byte(hostprotocol.OmoLifecycleExtension(binary)), nil
		default:
			return nil, errors.New("unexpected read")
		}
	}}
	errs := nativeIntegrationOmpConfigErrors(root, home, deps)
	if len(errs) != 1 || errs[0] != "omp lifecycle extension missing canonical session_start/session_switch/session_compact surface" {
		t.Fatalf("omo module accepted as omp extension: %v", errs)
	}
}

func TestNativeIntegrationOpencodeConfigChecksMCPServersAndPluginBytes(t *testing.T) {
	root, configRoot := t.TempDir(), t.TempDir()
	validator := testNativeValidator()
	validator.ResolveStableNativeRoot = func(string) (string, error) { return root, nil }
	binary := nativeTestBinary(root)
	files := map[string]string{
		filepath.Join(configRoot, "plugins", "issueops.js"): hostprotocol.OpencodeLifecyclePlugin(binary),
	}
	deps := nativeIntegrationValidationDeps{Validator: validator, readFile: func(path string) ([]byte, error) {
		body, ok := files[path]
		if !ok {
			return nil, errors.New("unexpected read")
		}
		return []byte(body), nil
	}}
	mcpPath := filepath.Join(configRoot, "opencode.json")
	bearer := strings.Repeat("A", 43)
	local := fmt.Sprintf(`{"mcp":{"servers":{"issueops":{"type":"local","command":[%q,"mcp"],"environment":{"ISSUEOPS_ROOT":%q},"codemode":false}}}}`, binary, root)
	for config, want := range map[string]bool{
		local: true,
		`{"mcp":{"servers":{"issueops":{"type":"remote","url":"http://127.0.0.1:47831/mcp","headers":{"Authorization":"Bearer ` + bearer + `"},"oauth":false,"codemode":false}}}}`: true,
		fmt.Sprintf(`{"mcp":{"servers":{"issueops":{"type":"local","command":[%q,"mcp"],"environment":{"ISSUEOPS_ROOT":%q}}}}}`, binary, root):                                     false,
		fmt.Sprintf(`{"mcpServers":{"issueops":{"command":%q,"args":["mcp"],"env":{"ISSUEOPS_ROOT":%q}}}}`, binary, root):                                                          false,
	} {
		files[mcpPath] = config
		errs := nativeIntegrationOpencodeConfigErrors(root, configRoot, deps)
		if (len(errs) == 0) != want {
			t.Fatalf("config %s: errs=%v, want accepted=%v", config, errs, want)
		}
	}
	files[mcpPath] = local
	files[filepath.Join(configRoot, "plugins", "issueops.js")] = hostprotocol.OmpLifecycleExtension(binary)
	errs := nativeIntegrationOpencodeConfigErrors(root, configRoot, deps)
	if len(errs) != 1 || errs[0] != "opencode lifecycle extension missing canonical context-hook surface" {
		t.Fatalf("omp module accepted as the opencode plugin: %v", errs)
	}
}

func TestValidateNativeIntegrationWithDepsCoversSkillConfigAndWarningFailures(t *testing.T) {
	root := t.TempDir()
	validator := testNativeValidator()
	validator.ResolveStableNativeRoot = func(string) (string, error) { return root, nil }
	home := t.TempDir()
	existing := nativeIntegrationExpectedPaths(root, home)
	validator.ListSkillNames = func(string) ([]string, error) { return nil, errors.New("skill list failed") }
	validator.SkillNamesForHost = func(string, []string, string) ([]string, []string) {
		return nil, nil
	}
	deps := nativeIntegrationValidationDeps{
		Validator:   validator,
		userHomeDir: func() (string, error) { return home, nil },
		exists:      func(path string) bool { return existing[path] },
		readFile: func(path string) ([]byte, error) {
			if strings.HasSuffix(path, "config.toml") {
				return []byte("[mcp_servers.other]\n"), nil
			}
			return []byte(`{"command":"other hook"}`), nil
		},
		duplicateWarningFixture: func() string { return "issueops: ./bin/issueops mcp - Connected\n" },
	}

	step := validateNativeIntegrationWithDeps(root, deps)
	if step.OK {
		t.Fatalf("expected aggregate failure, got %#v", step)
	}
	for _, want := range []string{
		"list native skills: skill list failed",
		"Codex MCP config missing issueops",
		"Codex thin context hooks missing issueops context hook surface",
		"Omo MCP config missing canonical issueops server",
		"Omo lifecycle extension missing canonical session_start/session_compact surface",
		"omp MCP config missing canonical issueops server",
		"omp lifecycle extension missing canonical session_start/session_switch/session_compact surface",
		"opencode MCP config missing canonical issueops server",
		"opencode lifecycle extension missing canonical context-hook surface",
		"Claude duplicate MCP warning fixture was not classified",
	} {
		if !strings.Contains(step.Error, want) {
			t.Fatalf("expected %q in error, got %#v", want, step)
		}
	}
}

func TestValidateNativeIntegrationReportsStableRootResolutionError(t *testing.T) {
	root := t.TempDir()
	validator := testNativeValidator()
	validator.ResolveStableNativeRoot = func(string) (string, error) { return "", errors.New("stable root unavailable") }
	home := t.TempDir()
	existing := nativeIntegrationExpectedPaths(root, home)
	validator.ListSkillNames = func(string) ([]string, error) { return []string{"shared", "codex-only", "claude-only"}, nil }
	validator.SkillNamesForHost = func(_ string, _ []string, host string) ([]string, []string) {
		if host == "codex" {
			return []string{"shared", "codex-only"}, nil
		}
		return []string{"shared", "claude-only"}, nil
	}
	deps := nativeIntegrationValidationDeps{
		Validator:   validator,
		userHomeDir: func() (string, error) { return home, nil },
		exists:      func(path string) bool { return existing[path] },
		readFile: func(path string) ([]byte, error) {
			switch filepath.Base(path) {
			case "config.toml":
				return []byte("[mcp_servers.issueops]\n"), nil
			case "hooks.json":
				return managedCodexHooksJSON(nativeTestBinary(root)), nil
			default:
				return nil, errors.New("unexpected read")
			}
		},
		duplicateWarningFixture: claudeMCPDuplicateWarningFixture,
	}

	step := validateNativeIntegrationWithDeps(root, deps)
	if step.OK || !strings.Contains(step.Error, "resolve stable native root: stable root unavailable") {
		t.Fatalf("native integration must fail closed on stable-root resolution: %#v", step)
	}
}

func TestHasThinCodexContextHooksPermitsThirdPartyLifecycleEvents(t *testing.T) {
	source := testsupport.AbsPath("source", "bin", "issueops")
	third := `{"type": "command", "command": "'/Users/example/.acme/agent-hooks/codex-hook.sh' observe", "timeout": 10}`
	config := `{
		"hooks": {
			"SubagentStart": [
				` + hookGroupJSON(third) + `,
				` + hookGroupJSON(managedHookHandler(source, "hook subagent-start --host codex")) + `
			],
			"SessionStart": [
				` + hookGroupJSON(third) + `,
				` + hookGroupJSON(managedHookHandler(source, "hook session-start --host codex")) + `
			],
			"PreToolUse": [` + hookGroupJSON(third) + `],
			"UserPromptSubmit": [{"hooks": [{"type": "command", "command": "codegraph observe", "timeout": 10}]}],
			"SubagentStop": [{"hooks": [{"type": "command", "command": "third-party stop", "timeout": 10}]}],
			"PermissionRequest": [{"hooks": [{"type": "command", "command": "third-party permission", "timeout": 10}]}]
		}
	}`
	if !(nativeIntegrationValidationDeps{Validator: testNativeValidator()}).hasThinCodexContextHooks(config, source) {
		t.Fatal("third-party lifecycle hooks must not invalidate the managed context hook")
	}
}

func TestHasThinCodexContextHooksRejectsRetiredManagedEvent(t *testing.T) {
	source := testsupport.AbsPath("source", "bin", "issueops")
	config := `{
		"hooks": {
			"SubagentStart": [` + hookGroupJSON(managedHookHandler(source, "hook subagent-start --host codex")) + `],
			"SessionStart": [` + hookGroupJSON(managedHookHandler(source, "hook session-start --host codex")) + `],
			"PreToolUse": [` + hookGroupJSON(managedHookHandler(source, "hook pre-tool-use --host codex --enforce-worktree")) + `]
		}
	}`
	if (nativeIntegrationValidationDeps{Validator: testNativeValidator()}).hasThinCodexContextHooks(config, source) {
		t.Fatal("retired issueops enforcement event must invalidate the managed context-hook surface")
	}
}

func TestHasThinCodexContextHooksUsesCanonicalGroupsForManagedCommands(t *testing.T) {
	source := testsupport.AbsPath("source", "bin", "issueops")
	spaced := testsupport.AbsPath("source with spaces", "bin", "issueops")
	other := testsupport.AbsPath("other", "bin", "issueops")
	start := func(binary string) string {
		return hookGroupJSON(managedHookHandler(binary, "hook session-start --host codex"))
	}
	subagent := func(binary string) string {
		return hookGroupJSON(managedHookHandler(binary, "hook subagent-start --host codex"))
	}
	for name, config := range map[string]string{
		"quoted canonical path with spaces": `{"hooks": {"SubagentStart": [` + subagent(spaced) + `], "SessionStart": [` + start(spaced) + `]}}`,
		"retired no-host event": `{"hooks": {
			"SubagentStart": [` + subagent(source) + `],
			"SessionStart": [` + start(source) + `],
			"UserPromptSubmit": [` + hookGroupJSON(managedHookHandler(source, "hook user-prompt")) + `]
		}}`,
		"wrong host alongside required hooks": `{"hooks": {
			"SubagentStart": [` + subagent(source) + `],
			"SessionStart": [` + start(source) + `, ` + hookGroupJSON(managedHookHandler(source, "hook session-start --host claude")) + `]
		}}`,
		"extra argument alongside required hooks": `{"hooks": {
			"SubagentStart": [` + subagent(source) + `],
			"SessionStart": [` + start(source) + `, ` + hookGroupJSON(managedHookHandler(source, "hook session-start --host codex --retired")) + `]
		}}`,
		"wrong binary path": `{"hooks": {
			"SubagentStart": [` + subagent(source) + `],
			"SessionStart": [` + start(other) + `]
		}}`,
		"SessionStart without SubagentStart": `{"hooks": {"SessionStart": [` + start(source) + `]}}`,
		"malformed JSON":                     `{"hooks":`,
	} {
		t.Run(name, func(t *testing.T) {
			expectedBinary := source
			if name == "quoted canonical path with spaces" {
				expectedBinary = spaced
				if !(nativeIntegrationValidationDeps{Validator: testNativeValidator()}).hasThinCodexContextHooks(config, expectedBinary) {
					t.Fatal("quoted canonical path must validate")
				}
				return
			}
			if (nativeIntegrationValidationDeps{Validator: testNativeValidator()}).hasThinCodexContextHooks(config, expectedBinary) {
				t.Fatal("non-canonical managed hook config was accepted")
			}
		})
	}
}

// nativeTestBinary is the canonical harness binary under root on this OS.
func nativeTestBinary(root string) string {
	return filepath.Join(root, "bin", testsupport.ExecutableName("issueops"))
}

// managedCodexHooksJSON is the installed Codex hooks.json for binary.
func managedCodexHooksJSON(binary string) []byte {
	b, err := json.Marshal(codex.HooksConfig(binary))
	if err != nil {
		panic(err)
	}
	return b
}

// managedHookHandler renders one issueops-managed hook handler for binary the
// way the Codex installer does: the shell form everywhere plus the PowerShell
// commandWindows form on Windows.
func managedHookHandler(binary, args string) string {
	handler := map[string]any{"type": "command", "command": shelltoken.QuoteWord(binary) + " " + args, "timeout": 5}
	if runtime.GOOS == "windows" {
		handler["commandWindows"] = "& '" + strings.ReplaceAll(binary, "'", "''") + "' " + args
	}
	b, err := json.Marshal(handler)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func hookGroupJSON(handlers ...string) string {
	return `{"hooks": [` + strings.Join(handlers, ",") + `]}`
}

func nativeIntegrationExpectedPaths(root, home string) map[string]bool {
	paths := []string{
		filepath.Join(root, "configs", "codex", "mcp.config.toml"),
		filepath.Join(root, "configs", "codex", "hooks.json"),
		filepath.Join(root, "configs", "claude", "mcp.project.json"),
		filepath.Join(root, "configs", "omo", "mcp.json"),
		filepath.Join(root, "configs", "omo", "issueops.js"),
		filepath.Join(root, "configs", "omp", "mcp.json"),
		filepath.Join(root, "configs", "omp", "issueops.js"),
		filepath.Join(root, "configs", "opencode", "opencode.json"),
		filepath.Join(root, "configs", "opencode", "issueops.js"),
		filepath.Join(home, ".codex", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".codex", "skills", "codex-only", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "claude-only", "SKILL.md"),
		filepath.Join(home, ".omo", "agent", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".omo", "agent", "skills", "omo-only", "SKILL.md"),
		filepath.Join(home, ".omp", "agent", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".omp", "agent", "skills", "omp-only", "SKILL.md"),
		filepath.Join(home, ".config", "opencode", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".config", "opencode", "skills", "opencode-only", "SKILL.md"),
	}
	out := map[string]bool{}
	for _, path := range paths {
		out[path] = true
	}
	return out
}

func TestValidateNativeIntegrationWithDepsCoversHomeFailure(t *testing.T) {
	step := validateNativeIntegrationWithDeps(t.TempDir(), nativeIntegrationValidationDeps{
		userHomeDir: func() (string, error) { return "", os.ErrNotExist },
	})
	if step.OK || !strings.Contains(step.Error, "user home:") {
		t.Fatalf("expected user home failure, got %#v", step)
	}
}

func TestDetectClaudeMCPDuplicateWarnings(t *testing.T) {
	warnings := detectClaudeMCPDuplicateWarnings(claudeMCPDuplicateWarningFixture())
	if len(warnings) != 1 {
		t.Fatalf("expected one duplicate warning, got %+v", warnings)
	}
	if warnings[0].Server != "issueops" || !strings.Contains(warnings[0].Message, "multiple scopes") {
		t.Fatalf("duplicate warning was not classified: %+v", warnings[0])
	}
	if len(warnings[0].Suggestions) != 1 || !strings.Contains(warnings[0].Suggestions[0], "claude mcp remove issueops") {
		t.Fatalf("duplicate warning suggestion missing: %+v", warnings[0].Suggestions)
	}
	if got := detectClaudeMCPDuplicateWarnings("issueops: ./bin/issueops mcp - ✓ Connected\n"); len(got) != 0 {
		t.Fatalf("non-conflicting output produced warnings: %+v", got)
	}
}

func TestNativeIntegrationKeepsPreparedStableRoot(t *testing.T) {
	firstRoot, secondRoot, home := t.TempDir(), t.TempDir(), t.TempDir()
	prepare := func(root string) nativeIntegrationValidationDeps {
		validator := testNativeValidator()
		validator.ResolveStableNativeRoot = func(string) (string, error) { return root, nil }
		binary := nativeTestBinary(root)
		return (nativeIntegrationValidationDeps{Validator: validator, readFile: func(path string) ([]byte, error) {
			switch path {
			case filepath.Join(home, ".omo", "mcp.json"):
				return fmt.Appendf(nil, `{"mcpServers":{"issueops":{"command":%q,"args":["mcp"],"env":{"ISSUEOPS_ROOT":%q}}}}`, binary, root), nil
			case filepath.Join(home, ".omo", "extensions", "issueops.js"):
				return []byte(hostprotocol.OmoLifecycleExtension(binary)), nil
			case filepath.Join(home, ".omp", "agent", "mcp.json"):
				return fmt.Appendf(nil, `{"mcpServers":{"issueops":{"type":"stdio","command":%q,"args":["mcp"],"env":{"ISSUEOPS_ROOT":%q}}}}`, binary, root), nil
			case filepath.Join(home, ".omp", "agent", "extensions", "issueops.js"):
				return []byte(hostprotocol.OmpLifecycleExtension(binary)), nil
			default:
				return nil, errors.New("unexpected read")
			}
		}}).withDefaults()
	}
	first := prepare(firstRoot)
	second := prepare(secondRoot)
	for _, deps := range []nativeIntegrationValidationDeps{first, second, first} {
		if errs := nativeIntegrationOmoConfigErrors("worktree", home, deps); len(errs) != 0 {
			t.Fatalf("prepared native root overwritten: %v", errs)
		}
		if errs := nativeIntegrationOmpConfigErrors("worktree", home, deps); len(errs) != 0 {
			t.Fatalf("prepared native root overwritten for omp: %v", errs)
		}
	}
}

func TestDetectClaudeMCPDuplicateWarningsIgnoresSuggestionWithoutPrecedingWarning(t *testing.T) {
	got := detectClaudeMCPDuplicateWarnings("   Suggestion: remove it\n")
	if len(got) != 0 {
		t.Fatalf("orphan suggestion produced warnings: %+v", got)
	}
	got = detectClaudeMCPDuplicateWarnings("   Suggestion: orphan\n" + claudeMCPDuplicateWarningFixture())
	if len(got) != 1 || len(got[0].Suggestions) != 1 || strings.Contains(got[0].Suggestions[0], "orphan") {
		t.Fatalf("orphan suggestion attached to a warning: %+v", got)
	}
}
