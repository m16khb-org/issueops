package claude

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"runtime"
	"strings"

	"issueops/internal/domain/shelltoken"
	"issueops/internal/port"
)

func (installer Installer) writeClaudeSettings(path string, req port.NativeInstallRequest) (port.InstallFile, []string, error) {
	file := port.InstallFile{Path: path, Kind: "claude_user_settings"}
	config := map[string]any{}
	if existing, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(existing))) > 0 {
		if err := json.Unmarshal(existing, &config); err != nil {
			return file, nil, err
		}
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) && !req.DryRun {
		return file, nil, err
	}
	if err := installer.deps.ValidateHookConfigForMerge(config, claudeLifecycleHookEvents); err != nil {
		return file, nil, err
	}
	messages := installer.deps.HookTargetDriftMessages(config, "claude", req.BinPath)
	// 경로가 같아도 빌드 세대가 갈리면 이전 세대 hook이 새 typed command를
	// 모른 채 차단해 복구가 교착된다(#328). 그 축을 여기서 함께 보고한다.
	if installer.deps.HookTargetGenerationMessages != nil && installer.deps.RunningBuildGenerationString != nil && installer.deps.FileBuildGenerationString != nil {
		messages = append(messages, installer.deps.HookTargetGenerationMessages(config, "claude", req.BinPath, installer.deps.RunningBuildGenerationString(), installer.deps.FileBuildGenerationString)...)
	}
	written, err := installer.deps.WriteJSONPlan(path, file.Kind, installer.mergeClaudeHookConfig(config, req.BinPath, runtime.GOOS), 0o644, req.DryRun)
	return written, messages, err
}

// claudeSettingsConfig is the hook catalog this OS installs for binPath.
func (installer Installer) claudeSettingsConfig(binPath string) map[string]any {
	return installer.claudeSettingsConfigFor(binPath, runtime.GOOS)
}

// claudeSettingsConfigFor builds the catalog for goos; the tracked template
// passes "" so it stays the same whichever OS regenerates it.
func (installer Installer) claudeSettingsConfigFor(binPath, goos string) map[string]any {
	return installer.mergeClaudeHookConfig(map[string]any{}, binPath, goos)
}

func (installer Installer) mergeClaudeHookConfig(config map[string]any, binPath, goos string) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		config["hooks"] = hooks
	}
	desired := map[string]claudeLifecycleHookSpec{}
	for _, spec := range claudeLifecycleHookSpecs(binPath, goos) {
		desired[spec.Event] = spec
	}
	for _, event := range claudeLifecycleHookEvents {
		groups := []any{}
		if existing, ok := hooks[event].([]any); ok {
			for _, group := range existing {
				if !installer.deps.HookGroupContainsAgentHarness(group) && !installer.deps.HookGroupContainsCommand(group, shelltoken.QuoteWord(binPath)+" hook ") {
					groups = append(groups, group)
				}
			}
		}
		if spec, ok := desired[event]; ok {
			groups = append(groups, claudeHookGroup(spec))
		}
		if len(groups) == 0 {
			delete(hooks, event)
			continue
		}
		hooks[event] = groups
	}
	return config
}

type claudeLifecycleHookSpec struct {
	BinPath    string
	Event      string
	Subcommand string
	Matcher    string
	Timeout    int
	GOOS       string
}

func claudeLifecycleHookSpecs(binPath, goos string) []claudeLifecycleHookSpec {
	// SessionStart carries the catalog for the main session: Claude re-runs it
	// with source "compact" after compaction, while PostCompact output is only a
	// user display string there (verified against Claude Code 2.1.247).
	// SubagentStart gives each subagent the same catalog; it has no matcher
	// because the hook itself skips Explore and fork agents.
	return []claudeLifecycleHookSpec{
		{BinPath: binPath, Event: "SessionStart", Subcommand: "session-start", Timeout: 5, GOOS: goos},
		{BinPath: binPath, Event: "SubagentStart", Subcommand: "subagent-start", Timeout: 5, GOOS: goos},
	}
}

var claudeLifecycleHookEvents = []string{
	"SessionStart",
	"SubagentStart",
	"UserPromptSubmit",
	"PreToolUse",
	"PostToolUse",
	"PreCompact",
	"PostCompact",
	"Stop",
}

func claudeHookGroup(spec claudeLifecycleHookSpec) map[string]any {
	group := map[string]any{"hooks": []any{claudeHookHandler(spec)}}
	if spec.Matcher != "" {
		group["matcher"] = spec.Matcher
	}
	return group
}

// claudeHookHandler writes the shell form everywhere but Windows. There Claude
// runs shell-form hooks in Git Bash, or in PowerShell when Git Bash is absent,
// so the exec form spawns the .exe directly under either.
func claudeHookHandler(spec claudeLifecycleHookSpec) map[string]any {
	handler := map[string]any{"type": "command", "timeout": spec.Timeout}
	args := claudeHookArgs(spec.Subcommand)
	if spec.GOOS == "windows" {
		handler["command"] = spec.BinPath
		list := make([]any, 0, len(args))
		for _, arg := range args {
			list = append(list, arg)
		}
		handler["args"] = list
		return handler
	}
	handler["command"] = shelltoken.QuoteWord(spec.BinPath) + " " + strings.Join(args, " ")
	return handler
}

func claudeHookArgs(subcommand string) []string {
	args := []string{"hook", subcommand}
	if subcommand == "session-start" || subcommand == "subagent-start" || subcommand == "post-compact" {
		args = append(args, "--host", "claude")
	}
	return args
}
