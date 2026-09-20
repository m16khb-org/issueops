package hostprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"issueops/internal/port"
)

const claudeProbeServer = "issueops_probe"

type ClaudeRunner struct {
	harnessBinary string
	deps          Dependencies
}

type claudeMCPConfig struct {
	MCPServers map[string]claudeMCPServer `json:"mcpServers"`
}

type claudeMCPServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func NewClaudeRunner(harnessBinary string, deps Dependencies) ClaudeRunner {
	return ClaudeRunner{
		harnessBinary: harnessBinary,
		deps:          normalizeDependencies(deps),
	}
}

func (ClaudeRunner) Name() string { return "claude" }

func (r ClaudeRunner) Preflight(ctx context.Context, request port.HostProbeRequest) port.HostProbePreflight {
	return preflight(ctx, r.deps, r.Name(), "claude", request, isolatedHostEnv(r.deps))
}

func (r ClaudeRunner) Run(ctx context.Context, request port.HostProbeRequest) (result port.HostProbeResult) {
	started := r.deps.Now()
	if r.harnessBinary == "" {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "harness_binary_missing")
	}
	harnessBinary, err := filepath.Abs(r.harnessBinary)
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "harness_binary_path_invalid")
	}
	request.HarnessBinary = harnessBinary

	executable, err := r.deps.LookPath("claude")
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "executable_not_found")
	}
	root, err := newEpisodeRoot(r.deps, r.Name())
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", episodeRootFailureCode(err))
	}
	defer func() {
		if err := r.deps.RemoveAll(root); err != nil {
			result = failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "private_root_cleanup_failed")
		}
	}()

	resultPath := filepath.Join(root, "result.json")
	serve := serveArgv(request, resultPath)
	config, err := json.Marshal(claudeMCPConfig{
		MCPServers: map[string]claudeMCPServer{
			claudeProbeServer: {
				Type:    "stdio",
				Command: serve[0],
				Args:    serve[1:],
			},
		},
	})
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "mcp_config_encode_failed")
	}
	configPath := filepath.Join(root, ".mcp.json")
	if err := writePrivateFile(configPath, config); err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "mcp_config_write_failed")
	}

	hookSmoke := r.deps.Getenv("ISSUEOPS_CHILD_SMOKE_HOOKS") == "1"
	observationPath := filepath.Join(root, "live-observation.json")
	if hookSmoke {
		observationPath = strings.TrimSpace(r.deps.Getenv("ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE"))
	}
	settingsPath := ""
	if !hookSmoke {
		settingsPath = filepath.Join(root, "hooks.settings.json")
		if err := prepareClaudeLiveSettings(settingsPath, harnessBinary, observationPath); err != nil {
			return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "claude_live_settings_invalid")
		}
	}
	environment := isolatedHostEnv(r.deps)
	environment = replaceEnvironmentValue(environment, "ISSUEOPS_CHILD_SMOKE_HOOKS", "1")
	environment = replaceEnvironmentValue(environment, "ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE", observationPath)
	output, err := r.deps.Process.Run(ctx, CommandRequest{
		Cwd:     root,
		Argv:    claudeArgvMode(executable, configPath, settingsPath, request, hookSmoke),
		Env:     environment,
		Timeout: EpisodeTimeout,
	})
	if err != nil {
		cause, code := claudeProcessFailure(err)
		return failedResult(r.Name(), "", request, started, r.deps, cause, code)
	}
	capture, err := decodeEpisodeCapture(resultPath, request)
	if err != nil {
		cause, code := claudeCaptureFailure(err)
		return failedResult(r.Name(), "", request, started, r.deps, cause, code)
	}
	result = completedResult(r.Name(), "", request, started, r.deps, capture)
	observation, err := observeHostStream(output.Stdout)
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "transport", "host_stream_invalid")
	}
	recorded, err := observeRecordedHookEvents(observationPath)
	if err != nil || mergeHookObservation(&observation, recorded) != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "transport", "hook_observation_invalid")
	}
	if !validHostRuntimeObservation(observation) {
		return failedResult(r.Name(), "", request, started, r.deps, "transport", "host_stream_invalid")
	}
	applyHostStreamObservation(&result, observation, output.ExitCode)
	result.ObservedModel = observation.Model
	result.AmbientToolCount = observation.AmbientToolCount
	if hookSmoke {
		if err := persistChildSmokeObservation(r.deps, result, observation.MCPCallCount); err != nil {
			return failedResult(r.Name(), "", request, started, r.deps, "transport", err.Error())
		}
	}
	return result
}

func prepareClaudeLiveSettings(path, harnessBinary, observationPath string) error {
	if !filepath.IsAbs(path) || !filepath.IsAbs(harnessBinary) || !filepath.IsAbs(observationPath) {
		return fmt.Errorf("claude_live_settings_path_invalid")
	}
	command := "/usr/bin/env ISSUEOPS_CHILD_SMOKE_HOOKS=1 ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE=" + shellSingleQuote(observationPath) + " " +
		shellSingleQuote(harnessBinary) + " hook session-start --host claude"
	document := codexSmokeHookDocument{Hooks: map[string][]codexSmokeHookGroup{
		"SessionStart": {{Hooks: []codexSmokeHook{{Type: "command", Command: command, Timeout: 5}}}},
	}}
	data, err := json.Marshal(document)
	if err != nil {
		return err
	}
	return writePrivateFile(path, append(data, '\n'))
}

func claudeArgvMode(executable, configPath, settingsPath string, request port.HostProbeRequest, hookSmoke bool) []string {
	settingSources := ""
	if hookSmoke {
		settingSources = "user"
	}
	argv := []string{
		executable,
		"-p",
		"--verbose",
		"--setting-sources", settingSources,
		"--output-format", "stream-json",
		"--strict-mcp-config",
		"--mcp-config", configPath,
		"--no-session-persistence",
		"--permission-mode", "dontAsk",
		"--tools", "",
		"--allowedTools=mcp__" + claudeProbeServer + "__" + request.ProbeTool,
	}
	if !hookSmoke {
		argv = append(argv, "--settings", settingsPath, "--include-hook-events")
	}
	if request.Model != "" && request.Model != "default" {
		argv = append(argv, "--model", request.Model)
	}
	return append(argv, request.Prompt)
}

func claudeProcessFailure(err error) (string, string) {
	return normalizedProcessFailure(err, "host_process_failed")
}

func claudeCaptureFailure(err error) (string, string) {
	return normalizedCaptureFailure(err)
}
