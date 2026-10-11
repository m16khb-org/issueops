package hostprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"issueops/internal/domain/policy"
	"issueops/internal/port"
)

const (
	omoProbeServer       = "issueops_probe"
	omoProbeSystemPrompt = "Follow the user instruction and call only the explicitly allowed conformance tool."
)

// OmoRunner runs one capture-only MCP episode in an isolated Omo native session.
type OmoRunner struct {
	canonicalExtension func(string) string
	harnessBinary      string
	lifecycleExtension string
	deps               Dependencies
}

type omoMCPConfig struct {
	Settings   omoMCPSettings          `json:"settings"`
	MCPServers map[string]omoMCPServer `json:"mcpServers"`
}

type omoMCPSettings struct {
	NativeToolSearch bool `json:"nativeToolSearch"`
}

type omoMCPServer struct {
	Type         string   `json:"type"`
	Command      string   `json:"command"`
	Args         []string `json:"args"`
	Cwd          string   `json:"cwd"`
	Enabled      bool     `json:"enabled"`
	Lifecycle    string   `json:"lifecycle"`
	IncludeTools []string `json:"includeTools"`
	DirectTools  []string `json:"directTools"`
	Exposure     string   `json:"exposure"`
}

type omoStreamObservation struct {
	Model            string
	AmbientToolCount int
	MCPCallCount     int
	ResponseSHA256   string
}

type omoToolExecution struct {
	name  string
	ended bool
}

type omoAuthSnapshot struct {
	present bool
	data    []byte
}

func NewOmoRunner(harnessBinary, lifecycleExtension string, deps Dependencies, canonicalExtension func(string) string) OmoRunner {
	return OmoRunner{canonicalExtension: canonicalExtension,
		harnessBinary:      harnessBinary,
		lifecycleExtension: lifecycleExtension,
		deps:               normalizeDependencies(deps),
	}
}

func (OmoRunner) Name() string { return "omo" }

func (r OmoRunner) Preflight(ctx context.Context, request port.HostProbeRequest) (result port.HostProbePreflight) {
	result = port.HostProbePreflight{
		Host:           r.Name(),
		RequestedModel: request.Model,
		EvidenceSource: "omo_preflight",
	}
	executable, err := r.resolveExecutable()
	if err != nil {
		result.Cause = "harness_environment"
		result.Code = "executable_not_found"
		return result
	}
	result.Installed = true
	auth, err := resolveOmoAuth(r.deps)
	if err != nil {
		result.Cause = "harness_environment"
		result.Code = "auth_copy_failed"
		return result
	}
	root, err := newEpisodeRoot(r.deps, r.Name())
	if err != nil {
		result.Cause = "harness_environment"
		result.Code = episodeRootFailureCode(err)
		return result
	}
	defer func() {
		if err := r.deps.RemoveAll(root); err != nil {
			result.Ready = false
			result.MockExtensionVerified = false
			result.Cause = "harness_environment"
			result.Code = "private_root_cleanup_failed"
			result.EvidenceSource = "omo_preflight"
		}
	}()
	if err := prepareOmoPrivateState(root, auth); err != nil {
		result.Cause = "harness_environment"
		result.Code = "episode_prepare_failed"
		return result
	}
	output, err := r.deps.Process.Run(ctx, CommandRequest{
		Argv:    []string{executable, "--version"},
		Env:     isolatedOmoEnv(r.deps, root),
		Timeout: VersionTimeout,
	})
	if err != nil || output.ExitCode != 0 {
		result.Cause = "harness_environment"
		result.Code = "version_probe_failed"
		return result
	}
	result.Ready = true
	result.Version = boundedVersion(string(output.Stdout))
	result.MockExtensionVerified = r.validLifecycleExtension()
	if result.Ready && !result.MockExtensionVerified {
		result.Ready = false
		result.Cause = "harness_environment"
		result.Code = "mock_extension_invalid"
		result.EvidenceSource = "omo_preflight"
	}
	if result.Ready && !explicitOmoModel(request.Model) {
		result.Ready = false
		result.Cause = "harness_environment"
		result.Code = "explicit_model_required"
		result.EvidenceSource = "omo_preflight"
	}
	return result
}

func (r OmoRunner) resolveExecutable() (string, error) {
	executable, err := r.deps.LookPath("omo")
	if err != nil {
		return "", err
	}
	executable, err = filepath.Abs(executable)
	if err != nil || !filepath.IsAbs(executable) {
		return "", fmt.Errorf("omo_executable_invalid")
	}
	return executable, nil
}

func (r OmoRunner) Run(ctx context.Context, request port.HostProbeRequest) (result port.HostProbeResult) {
	started := r.deps.Now()
	if r.harnessBinary == "" {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "harness_binary_missing")
	}
	if strings.TrimSpace(r.lifecycleExtension) == "" {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "lifecycle_extension_missing")
	}
	if !r.validLifecycleExtension() {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "mock_extension_invalid")
	}
	if !explicitOmoModel(request.Model) {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "explicit_model_required")
	}
	harnessBinary, err := filepath.Abs(r.harnessBinary)
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "harness_binary_path_invalid")
	}
	request.HarnessBinary = harnessBinary
	executable, err := r.resolveExecutable()
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "executable_not_found")
	}
	auth, err := resolveOmoAuth(r.deps)
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "auth_copy_failed")
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

	if err := prepareOmoPrivateState(root, auth); err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "episode_prepare_failed")
	}
	resultPath := filepath.Join(root, "result.json")
	targetTool := omoProbeToolName(request.ProbeTool)
	if err := prepareOmoEpisode(root, r.lifecycleExtension, targetTool, request, resultPath); err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "episode_prepare_failed")
	}
	output, err := r.deps.Process.Run(ctx, CommandRequest{
		Cwd:      root,
		Argv:     omoArgv(executable, root, targetTool, request),
		Env:      isolatedOmoEnv(r.deps, root),
		Timeout:  EpisodeTimeout,
		OmoJSONL: true,
	})
	return omoEpisodeFinish{
		name: r.Name(), request: request, started: started, deps: r.deps,
		resultPath: resultPath, targetTool: targetTool,
	}.finish(output, err)
}

func (r OmoRunner) validLifecycleExtension() bool {
	absolute, err := filepath.Abs(r.harnessBinary)
	return err == nil && r.canonicalExtension != nil && r.lifecycleExtension == r.canonicalExtension(absolute)
}

func explicitOmoModel(model string) bool {
	model = strings.TrimSpace(model)
	return model != "" && model != "default"
}

func prepareOmoEpisode(root, lifecycleExtension, targetTool string, request port.HostProbeRequest, resultPath string) error {
	if !filepath.IsAbs(root) || !filepath.IsAbs(request.HarnessBinary) || !filepath.IsAbs(resultPath) {
		return fmt.Errorf("episode_path_invalid")
	}
	if err := writePrivateFile(filepath.Join(root, "issueops-lifecycle.js"), []byte(lifecycleExtension)); err != nil {
		return err
	}
	if err := writePrivateFile(filepath.Join(root, "issueops-conformance.js"), []byte(omoContextGuardExtension(targetTool))); err != nil {
		return err
	}
	const projectDoc = "# Omo conformance context\n\nThis private document proves lifecycle context injection for one isolated host probe.\n"
	if err := writePrivateFile(filepath.Join(root, ".issueops", "CONSTITUTION.md"), []byte(projectDoc)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "sessions"), 0o700); err != nil {
		return err
	}
	serve := serveArgv(request, resultPath)
	config := omoMCPConfig{
		Settings: omoMCPSettings{NativeToolSearch: false},
		MCPServers: map[string]omoMCPServer{
			omoProbeServer: {
				Type:         "stdio",
				Command:      serve[0],
				Args:         serve[1:],
				Cwd:          root,
				Enabled:      true,
				Lifecycle:    "eager",
				IncludeTools: []string{request.ProbeTool},
				DirectTools:  []string{request.ProbeTool},
				Exposure:     "direct",
			},
		},
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return writePrivateFile(filepath.Join(root, "agent", "mcp.json"), data)
}

func resolveOmoAuth(deps Dependencies) (omoAuthSnapshot, error) {
	sourceAgentDir := strings.TrimSpace(deps.Getenv("OMO_CODING_AGENT_DIR"))
	if sourceAgentDir == "" {
		home := strings.TrimSpace(deps.Getenv("HOME"))
		if home == "" {
			return omoAuthSnapshot{}, nil
		}
		if !filepath.IsAbs(home) {
			return omoAuthSnapshot{}, fmt.Errorf("omo_home_invalid")
		}
		sourceAgentDir = filepath.Join(home, ".omo", "agent")
	}
	if !filepath.IsAbs(sourceAgentDir) {
		return omoAuthSnapshot{}, fmt.Errorf("omo_agent_dir_invalid")
	}
	source := filepath.Join(sourceAgentDir, "auth.json")
	data, err := readOmoAuthFile(source, MaxOutputBytes)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return omoAuthSnapshot{}, nil
		}
		return omoAuthSnapshot{}, err
	}
	return omoAuthSnapshot{present: true, data: data}, nil
}

func prepareOmoPrivateState(root string, auth omoAuthSnapshot) error {
	for _, directory := range []string{filepath.Join(root, "home"), filepath.Join(root, "agent")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
	}
	if !auth.present {
		return nil
	}
	return writePrivateFile(filepath.Join(root, "agent", "auth.json"), auth.data)
}

func isolatedOmoEnv(deps Dependencies, root string) []string {
	environment := replaceEnvironmentValue(isolatedHostEnv(deps, "OMO_CODING_AGENT_DIR"), "HOME", filepath.Join(root, "home"))
	return replaceEnvironmentValue(environment, "OMO_CODING_AGENT_DIR", filepath.Join(root, "agent"))
}

func omoArgv(executable, root, targetTool string, request port.HostProbeRequest) []string {
	return []string{
		executable,
		"--mode", "json", "--print",
		"--model", request.Model,
		"--system-prompt", omoProbeSystemPrompt,
		"--no-session", "--session-dir", filepath.Join(root, "sessions"),
		"--no-builtin-tools", "--tools", targetTool,
		"--extension", filepath.Join(root, "issueops-lifecycle.js"),
		"--extension", filepath.Join(root, "issueops-conformance.js"),
		"--no-extensions", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files",
		"--no-approve", "--offline", "--omo-senpi-disabled", "--no-model-fallback", "--no-recommended-models", "--no-nested-agents", "--pi-rules-disabled", "--ttsr-disabled",
		"--", request.Prompt,
	}
}

func omoContextGuardExtension(targetTool string) string {
	return fmt.Sprintf(`const targetTool = %s

export default function issueopsConformance(pi) {
  pi.on("tool_call", (event, ctx) => {
    if (event.toolName !== targetTool) return
    const injected = ctx.sessionManager.getEntries().filter(
      (entry) => entry.type === "custom_message" &&
        entry.customType === "issueops:project-docs" &&
        entry.display === false &&
        typeof entry.content === "string" &&
        entry.content.length > 0,
    )
    if (injected.length !== 1) {
      return { block: true, reason: "issueops context injection missing" }
    }
  })
}
`, jsonString(targetTool))
}

func omoProbeToolName(tool string) string {
	return ellipsizeMiddle("mcp_"+sanitizeOmoToolName(omoProbeServer)+"_"+sanitizeOmoToolName(tool), 64)
}

func sanitizeOmoToolName(value string) string {
	var out strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			out.WriteRune(r)
		} else {
			out.WriteByte('_')
		}
	}
	return out.String()
}

func ellipsizeMiddle(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	remaining := limit - 3
	prefix := (remaining + 1) / 2
	suffix := remaining / 2
	return policy.TruncateBytes(value, prefix) + "..." + policy.TailBytes(value, suffix)
}

type omoStreamState struct {
	targetTool  string
	executions  map[string]*omoToolExecution
	results     []any
	observation omoStreamObservation
	events      int
	sessions    int
}

func observeOmoStream(data []byte, targetTool string) (omoStreamObservation, error) {
	if len(data) == 0 || len(data) > MaxOutputBytes {
		return omoStreamObservation{}, fmt.Errorf("host_stream_invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	state := &omoStreamState{
		targetTool: targetTool,
		executions: map[string]*omoToolExecution{},
		results:    make([]any, 0, 1),
	}
	for {
		var event map[string]any
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return state.observation, fmt.Errorf("host_stream_invalid")
		}
		if err := state.consume(event); err != nil {
			return state.observation, err
		}
	}
	if err := state.finish(); err != nil {
		return state.observation, err
	}
	return state.observation, nil
}

func (s *omoStreamState) consume(event map[string]any) error {
	s.events++
	typeName, _ := event["type"].(string)
	switch typeName {
	case "session":
		s.sessions++
	case "message_start", "message_end":
		s.observeMessage(event)
	case "tool_execution_start":
		return s.observeToolStart(event)
	case "tool_execution_end":
		return s.observeToolEnd(event)
	}
	return nil
}

func (s *omoStreamState) observeMessage(event map[string]any) {
	message, _ := event["message"].(map[string]any)
	if message["role"] != "assistant" || s.observation.Model != "" {
		return
	}
	if model, ok := message["model"].(string); ok && strings.TrimSpace(model) != "" {
		s.observation.Model = boundedVersion(model)
	}
}

func (s *omoStreamState) observeToolStart(event map[string]any) error {
	toolName, _ := event["toolName"].(string)
	id, _ := event["toolCallId"].(string)
	if id == "" || toolName == "" {
		return fmt.Errorf("host_stream_invalid")
	}
	s.observation.AmbientToolCount++
	if _, exists := s.executions[id]; exists {
		return fmt.Errorf("host_stream_multiple_calls")
	}
	s.executions[id] = &omoToolExecution{name: toolName}
	if toolName == s.targetTool {
		s.observation.MCPCallCount++
	}
	return nil
}

func (s *omoStreamState) observeToolEnd(event map[string]any) error {
	toolName, _ := event["toolName"].(string)
	id, _ := event["toolCallId"].(string)
	execution := s.executions[id]
	if id == "" || toolName == "" || execution == nil || execution.name != toolName || execution.ended {
		return fmt.Errorf("host_stream_invalid")
	}
	execution.ended = true
	if isError, ok := event["isError"].(bool); !ok || isError {
		if toolName == s.targetTool {
			return fmt.Errorf("host_stream_tool_error")
		}
		return fmt.Errorf("host_stream_non_target_call")
	}
	result, ok := event["result"]
	if !ok || result == nil {
		return fmt.Errorf("host_stream_invalid")
	}
	if toolName == s.targetTool {
		s.results = append(s.results, result)
	}
	return nil
}

// finish validates the whole stream once every event has been consumed and
// records the target response digest on success.
func (s *omoStreamState) finish() error {
	if s.events == 0 || s.sessions != 1 {
		return fmt.Errorf("host_stream_invalid")
	}
	if len(s.executions) == 0 {
		return fmt.Errorf("host_stream_no_call")
	}
	for _, execution := range s.executions {
		if !execution.ended {
			return fmt.Errorf("host_stream_invalid")
		}
	}
	if s.observation.AmbientToolCount != 1 {
		return fmt.Errorf("host_stream_multiple_calls")
	}
	for _, execution := range s.executions {
		if execution.name != s.targetTool {
			return fmt.Errorf("host_stream_non_target_call")
		}
	}
	if s.observation.MCPCallCount != 1 || len(s.results) != 1 {
		return fmt.Errorf("host_stream_invalid")
	}
	if s.observation.Model == "" {
		return fmt.Errorf("host_stream_model_missing")
	}
	digest, err := semanticResponseDigest(s.results)
	if err != nil {
		return fmt.Errorf("host_stream_invalid")
	}
	s.observation.MCPCallCount = 1
	s.observation.ResponseSHA256 = digest
	return nil
}

func omoStreamFailure(err error) (string, string) {
	switch err.Error() {
	case "host_stream_no_call":
		return "unknown", "no_call"
	case "host_stream_invalid", "host_stream_multiple_calls", "host_stream_non_target_call", "host_stream_tool_error", "host_stream_model_missing":
		return "transport", err.Error()
	default:
		return "transport", "host_stream_invalid"
	}
}

var _ port.HostProbeRunner = OmoRunner{}
