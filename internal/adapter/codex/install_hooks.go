package codex

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"issueops/internal/adapter/outbound/osfile"
	"issueops/internal/domain/shelltoken"
	"issueops/internal/port"
)

func (installer Installer) writeCodexHooks(path string, req port.NativeInstallRequest) (port.InstallFile, []string, error) {
	file := port.InstallFile{Path: path, Kind: "codex_user_hooks_config"}
	config, err := readCodexHooksConfig(path, req)
	if err != nil {
		return file, nil, err
	}
	if err := installer.deps.ValidateHookConfigForMerge(config, codexLifecycleHookEvents); err != nil {
		return file, nil, err
	}
	messages := installer.hookDriftMessages(config, req.BinPath)
	merged := installer.mergeHookConfig(config, req.BinPath)
	b, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return file, messages, err
	}
	text := string(append(b, '\n'))
	if existing, err := os.ReadFile(path); err == nil && string(existing) == text {
		return file, messages, nil
	}
	if req.DryRun {
		file.WouldWrite = true
		return file, messages, nil
	}
	if err := osfile.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, messages, err
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		return file, messages, err
	}
	file.Written = true
	return file, messages, nil
}

// readCodexHooksConfig parses the existing hooks.json; a missing or blank file
// yields an empty config, and read errors are tolerated during a dry run.
func readCodexHooksConfig(path string, req port.NativeInstallRequest) (map[string]any, error) {
	config := map[string]any{}
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		if err := json.Unmarshal(b, &config); err != nil {
			return nil, err
		}
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) && !req.DryRun {
		return nil, err
	}
	return config, nil
}

func (installer Installer) hookDriftMessages(config map[string]any, binPath string) []string {
	messages := installer.deps.HookTargetDriftMessages(config, "codex", binPath)
	// 경로가 같아도 빌드 세대가 갈리면 이전 세대 hook이 새 typed command를
	// 모른 채 차단해 복구가 교착된다(#328). 그 축을 여기서 함께 보고한다.
	if installer.deps.HookTargetGenerationMessages != nil && installer.deps.RunningBuildGenerationString != nil && installer.deps.FileBuildGenerationString != nil {
		messages = append(messages, installer.deps.HookTargetGenerationMessages(config, "codex", binPath, installer.deps.RunningBuildGenerationString(), installer.deps.FileBuildGenerationString)...)
	}
	return messages
}

// HooksConfig is the hook catalog this OS installs for binPath.
func HooksConfig(binPath string) map[string]any { return hooksConfig(binPath, runtime.GOOS) }

// hooksConfig builds the catalog for goos; the tracked template passes ""
// so it stays the same whichever OS regenerates it.
func hooksConfig(binPath, goos string) map[string]any {
	hooks := map[string]any{}
	for _, spec := range codexLifecycleHookSpecs(binPath, goos) {
		hooks[spec.Event] = []any{codexHookGroup(spec)}
	}
	return map[string]any{"hooks": hooks}
}

type codexLifecycleHookSpec struct {
	BinPath    string
	Event      string
	Subcommand string
	Timeout    int
	GOOS       string
}

func codexLifecycleHookSpecs(binPath, goos string) []codexLifecycleHookSpec {
	// SessionStart carries the catalog for the main session: Codex re-runs it
	// with source "compact" after compaction, and its post-compact.command.output
	// schema has no hookSpecificOutput (verified against codex-cli 0.150.1).
	// SubagentStart gives each subagent the same catalog (subagent-start output
	// accepts additionalContext in codex-cli 0.161.0).
	return []codexLifecycleHookSpec{
		{BinPath: binPath, Event: "SessionStart", Subcommand: "session-start", Timeout: 5, GOOS: goos},
		{BinPath: binPath, Event: "SubagentStart", Subcommand: "subagent-start", Timeout: 5, GOOS: goos},
	}
}

func codexHookGroup(spec codexLifecycleHookSpec) map[string]any {
	return map[string]any{"hooks": []any{codexHookHandler(spec)}}
}

// codexHookHandler keeps the shell-form command on every OS. Codex on Windows
// runs hooks through PowerShell, where a quoted path is a string expression,
// so Windows also gets commandWindows with the & call operator.
func codexHookHandler(spec codexLifecycleHookSpec) map[string]any {
	args := codexHookArgs(spec.Subcommand)
	handler := map[string]any{
		"type":    "command",
		"command": shelltoken.QuoteWord(spec.BinPath) + " " + strings.Join(args, " "),
		"timeout": spec.Timeout,
	}
	if spec.GOOS == "windows" {
		handler["commandWindows"] = "& '" + strings.ReplaceAll(spec.BinPath, "'", "''") + "' " + strings.Join(args, " ")
	}
	return handler
}

func codexHookArgs(subcommand string) []string {
	args := []string{"hook", subcommand}
	// additionalContext가 Codex TUI에 렌더링되는 이벤트는 --host codex를 넘겨
	// systemMessage를 생략한다.
	switch subcommand {
	case "session-start", "subagent-start", "post-compact":
		args = append(args, "--host", "codex")
	}
	return args
}

func (installer Installer) mergeHookConfig(config map[string]any, binPath string) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		config["hooks"] = hooks
	}
	desired := map[string]codexLifecycleHookSpec{}
	for _, spec := range codexLifecycleHookSpecs(binPath, runtime.GOOS) {
		desired[spec.Event] = spec
	}
	for _, event := range codexLifecycleHookEvents {
		installer.mergeHookEvent(hooks, event, binPath, desired)
	}
	return config
}

// mergeHookEvent rewrites the groups registered under one event: it drops
// issueops-managed groups, keeps the others, and appends the desired group.
func (installer Installer) mergeHookEvent(hooks map[string]any, event, binPath string, desired map[string]codexLifecycleHookSpec) {
	groups := []any{}
	replaced := false
	if existing, ok := hooks[event].([]any); ok {
		for _, group := range existing {
			if !hookGroupHasHooks(group) {
				continue
			}
			if installer.isManagedHookGroup(group, binPath) {
				if spec, desiredEvent := desired[event]; desiredEvent && !replaced {
					groups = append(groups, codexHookGroup(spec))
					replaced = true
				}
				continue
			}
			groups = append(groups, group)
		}
	}
	if spec, ok := desired[event]; ok && !replaced {
		groups = append(groups, codexHookGroup(spec))
	}
	if len(groups) == 0 {
		delete(hooks, event)
		return
	}
	hooks[event] = groups
}

// isManagedHookGroup reports whether group was registered by an issueops install.
func (installer Installer) isManagedHookGroup(group any, binPath string) bool {
	return installer.deps.HookGroupContainsAgentHarness(group) || installer.deps.HookGroupContainsCommand(group, shelltoken.QuoteWord(binPath)+" hook ")
}

// codexLifecycleHookEvents lists every event an earlier install may have
// registered an issueops hook under, so an upgrade strips those groups; the
// installer itself adds only the codexLifecycleHookSpecs events.
var codexLifecycleHookEvents = []string{
	"SessionStart",
	"SubagentStart",
	"UserPromptSubmit",
	"PreToolUse",
	"PostToolUse",
	"PreCompact",
	"PostCompact",
	"Stop",
}

func hookGroupHasHooks(group any) bool {
	m, ok := group.(map[string]any)
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]any)
	return ok && len(hooks) > 0
}
