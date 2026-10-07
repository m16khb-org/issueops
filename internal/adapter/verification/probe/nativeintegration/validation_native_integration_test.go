package nativeintegration

import (
	"errors"
	"fmt"
	"issueops/internal/adapter/hostprotocol"
	"os"
	"path/filepath"
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
		return []string{"shared", "codex-only", "claude-only", "omo-only"}, nil
	}
	validator.SkillNamesForHost = func(_ string, _ []string, host string) ([]string, []string) {
		switch host {
		case "codex":
			return []string{"shared", "codex-only"}, nil
		case "claude":
			return []string{"shared", "claude-only"}, nil
		case "omo":
			return []string{"shared", "omo-only"}, nil
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
				return []byte(fmt.Sprintf(`{"mcpServers":{"issueops":{"command":%q,"args":["mcp"],"env":{"ISSUEOPS_ROOT":%q}}}}`, filepath.Join(root, "bin", "issueops"), root)), nil
			case path == filepath.Join(home, ".omo", "extensions", "issueops.js"):
				return []byte(hostprotocol.OmoLifecycleExtension(filepath.Join(root, "bin", "issueops"))), nil
			}
			switch filepath.Base(path) {
			case "config.toml":
				return []byte("[mcp_servers.issueops]\ncommand = \"issueops\"\n"), nil
			case "hooks.json":
				return []byte(fmt.Sprintf(`{"hooks":{"SessionStart":[{"hooks":[{"command":"'%s' hook session-start --host codex","timeout":5,"type":"command"}]}]}}`, filepath.Join(root, "bin", "issueops"))), nil
			default:
				return nil, errors.New("unexpected read")
			}
		},
		duplicateWarningFixture: ClaudeMCPDuplicateWarningFixture,
	}

	step := validateNativeIntegrationWithDeps(root, deps)
	if !step.OK || step.Label != "native integration" || !strings.Contains(step.Stdout, "duplicate_mcp_warning_fixture") {
		t.Fatalf("unexpected success step: %#v", step)
	}

	missingPath := filepath.Join(home, ".omo", "agent", "skills", "omo-only", "SKILL.md")
	existing[missingPath] = false
	failed := validateNativeIntegrationWithDeps(root, deps)
	if failed.OK || !strings.Contains(failed.Error, "missing "+missingPath) {
		t.Fatalf("expected missing path failure, got %#v", failed)
	}
}

func TestNativeIntegrationOmoConfigAcceptsStableRootFromWorktree(t *testing.T) {
	worktreeRoot := t.TempDir()
	stableRoot := t.TempDir()
	home := t.TempDir()
	validator := testNativeValidator()
	validator.ResolveStableNativeRoot = func(string) (string, error) { return stableRoot, nil }
	expectedBinary := filepath.Join(stableRoot, "bin", "issueops")
	deps := nativeIntegrationValidationDeps{
		Validator: validator,
		readFile: func(path string) ([]byte, error) {
			switch path {
			case filepath.Join(home, ".omo", "mcp.json"):
				return []byte(fmt.Sprintf(`{"mcpServers":{"issueops":{"command":%q,"args":["mcp"],"env":{"ISSUEOPS_ROOT":%q}}}}`, expectedBinary, stableRoot)), nil
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
		"Codex thin context hooks missing issueops SessionStart surface",
		"Omo MCP config missing canonical issueops server",
		"Omo lifecycle extension missing canonical session_start/session_compact surface",
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
				return []byte(fmt.Sprintf(`{"hooks":{"SessionStart":[{"hooks":[{"command":"'%s' hook session-start --host codex","timeout":5,"type":"command"}]}]}}`, filepath.Join(root, "bin", "issueops"))), nil
			default:
				return nil, errors.New("unexpected read")
			}
		},
		duplicateWarningFixture: ClaudeMCPDuplicateWarningFixture,
	}

	step := validateNativeIntegrationWithDeps(root, deps)
	if step.OK || !strings.Contains(step.Error, "resolve stable native root: stable root unavailable") {
		t.Fatalf("native integration must fail closed on stable-root resolution: %#v", step)
	}
}

func TestHasThinCodexContextHooksPermitsThirdPartyLifecycleEvents(t *testing.T) {
	config := `{
		"hooks": {
			"SessionStart": [
				{"hooks": [{"type": "command", "command": "'/Users/example/.orca/agent-hooks/codex-hook.sh' observe", "timeout": 10}]},
				{"hooks": [{"type": "command", "command": "'/source/bin/issueops' hook session-start --host codex", "timeout": 5}]}
			],
			"PreToolUse": [{"hooks": [{"type": "command", "command": "'/Users/example/.orca/agent-hooks/codex-hook.sh' observe", "timeout": 10}]}],
			"UserPromptSubmit": [{"hooks": [{"type": "command", "command": "codegraph observe", "timeout": 10}]}],
			"SubagentStop": [{"hooks": [{"type": "command", "command": "third-party stop", "timeout": 10}]}],
			"PermissionRequest": [{"hooks": [{"type": "command", "command": "third-party permission", "timeout": 10}]}]
		}
	}`
	if !(nativeIntegrationValidationDeps{Validator: testNativeValidator()}).hasThinCodexContextHooks(config, "/source/bin/issueops") {
		t.Fatal("third-party lifecycle hooks must not invalidate the managed context hook")
	}
}

func TestHasThinCodexContextHooksRejectsRetiredManagedEvent(t *testing.T) {
	config := `{
		"hooks": {
			"SessionStart": [{"hooks": [{"type": "command", "command": "'/source/bin/issueops' hook session-start --host codex", "timeout": 5}]}],
			"PreToolUse": [{"hooks": [{"type": "command", "command": "'/source/bin/issueops' hook pre-tool-use --host codex --enforce-worktree", "timeout": 5}]}]
		}
	}`
	if (nativeIntegrationValidationDeps{Validator: testNativeValidator()}).hasThinCodexContextHooks(config, "/source/bin/issueops") {
		t.Fatal("retired issueops enforcement event must invalidate the managed context-hook surface")
	}
}

func TestHasThinCodexContextHooksUsesCanonicalGroupsForManagedCommands(t *testing.T) {
	for name, config := range map[string]string{
		"quoted canonical path with spaces": `{
			"hooks": {
				"SessionStart": [{"hooks": [{"type": "command", "command": "'/source with spaces/bin/issueops' hook session-start --host codex", "timeout": 5}]}]
			}
		}`,
		"retired no-host event": `{
			"hooks": {
				"SessionStart": [{"hooks": [{"type": "command", "command": "'/source/bin/issueops' hook session-start --host codex", "timeout": 5}]}],
				"UserPromptSubmit": [{"hooks": [{"type": "command", "command": "'/source/bin/issueops' hook user-prompt", "timeout": 5}]}]
			}
		}`,
		"wrong host alongside required hooks": `{
			"hooks": {
				"SessionStart": [
					{"hooks": [{"type": "command", "command": "'/source/bin/issueops' hook session-start --host codex", "timeout": 5}]},
					{"hooks": [{"type": "command", "command": "'/source/bin/issueops' hook session-start --host claude", "timeout": 5}]}
				]
			}
		}`,
		"extra argument alongside required hooks": `{
			"hooks": {
				"SessionStart": [
					{"hooks": [{"type": "command", "command": "'/source/bin/issueops' hook session-start --host codex", "timeout": 5}]},
					{"hooks": [{"type": "command", "command": "'/source/bin/issueops' hook session-start --host codex --retired", "timeout": 5}]}
				]
			}
		}`,
		"wrong binary path": `{
			"hooks": {
				"SessionStart": [{"hooks": [{"type": "command", "command": "'/other/bin/issueops' hook session-start --host codex", "timeout": 5}]}]
			}
		}`,
		"malformed JSON": `{"hooks":`,
	} {
		t.Run(name, func(t *testing.T) {
			expectedBinary := "/source/bin/issueops"
			if name == "quoted canonical path with spaces" {
				expectedBinary = "/source with spaces/bin/issueops"
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

func nativeIntegrationExpectedPaths(root, home string) map[string]bool {
	paths := []string{
		filepath.Join(root, "configs", "codex", "mcp.config.toml"),
		filepath.Join(root, "configs", "codex", "hooks.json"),
		filepath.Join(root, "configs", "claude", "mcp.project.json"),
		filepath.Join(root, "configs", "omo", "mcp.json"),
		filepath.Join(root, "configs", "omo", "issueops.js"),
		filepath.Join(home, ".codex", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".codex", "skills", "codex-only", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "claude-only", "SKILL.md"),
		filepath.Join(home, ".omo", "agent", "skills", "shared", "SKILL.md"),
		filepath.Join(home, ".omo", "agent", "skills", "omo-only", "SKILL.md"),
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
	warnings := DetectClaudeMCPDuplicateWarnings(ClaudeMCPDuplicateWarningFixture())
	if len(warnings) != 1 {
		t.Fatalf("expected one duplicate warning, got %+v", warnings)
	}
	if warnings[0].Server != "issueops" || !strings.Contains(warnings[0].Message, "multiple scopes") {
		t.Fatalf("duplicate warning was not classified: %+v", warnings[0])
	}
	if len(warnings[0].Suggestions) != 1 || !strings.Contains(warnings[0].Suggestions[0], "claude mcp remove issueops") {
		t.Fatalf("duplicate warning suggestion missing: %+v", warnings[0].Suggestions)
	}
	if got := DetectClaudeMCPDuplicateWarnings("issueops: ./bin/issueops mcp - ✓ Connected\n"); len(got) != 0 {
		t.Fatalf("non-conflicting output produced warnings: %+v", got)
	}
}

func TestNativeIntegrationKeepsPreparedStableRoot(t *testing.T) {
	firstRoot, secondRoot, home := t.TempDir(), t.TempDir(), t.TempDir()
	prepare := func(root string) nativeIntegrationValidationDeps {
		validator := testNativeValidator()
		validator.ResolveStableNativeRoot = func(string) (string, error) { return root, nil }
		binary := filepath.Join(root, "bin", "issueops")
		return (nativeIntegrationValidationDeps{Validator: validator, readFile: func(path string) ([]byte, error) {
			switch path {
			case filepath.Join(home, ".omo", "mcp.json"):
				return []byte(fmt.Sprintf(`{"mcpServers":{"issueops":{"command":%q,"args":["mcp"],"env":{"ISSUEOPS_ROOT":%q}}}}`, binary, root)), nil
			case filepath.Join(home, ".omo", "extensions", "issueops.js"):
				return []byte(hostprotocol.OmoLifecycleExtension(binary)), nil
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
	}
}
