package hostprobe

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"issueops/internal/port"
)

const (
	opencodeReadyLimit     = 15 * time.Second
	opencodeWarmLimit      = 10 * time.Second
	opencodeReadyInterval  = 100 * time.Millisecond
	opencodeWarmInterval   = 200 * time.Millisecond
	opencodeStopGrace      = 5 * time.Second
	opencodeServerUser     = "opencode"
	opencodeCatalogMarker  = ".issueops/CONSTITUTION.md"
	opencodeGuardErrorText = "issueops context injection missing"
)

// ServerRequest starts one private background server.
type ServerRequest struct {
	Cwd  string
	Argv []string
	Env  []string
}

// ServerProcess is a started private server. Stop ends its whole process
// group and waits for it within a bounded time.
type ServerProcess interface {
	Stop() error
}

// OpencodeRunner runs one capture-only MCP episode against a private
// `opencode serve`. The user's shared opencode service is never contacted:
// every opencode root points into the episode, and `run --server` targets the
// private server only after its location has connected the probe MCP server,
// because a cold location sends the first model request without MCP tools.
type OpencodeRunner struct {
	canonicalPlugin func(string) string
	harnessBinary   string
	lifecyclePlugin string
	deps            Dependencies
	readyLimit      time.Duration
	warmLimit       time.Duration
}

type opencodeMCPServer struct {
	Type     string   `json:"type"`
	Command  []string `json:"command"`
	Codemode bool     `json:"codemode"`
}

type opencodePermission struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Effect   string `json:"effect"`
}

type opencodeEpisodeConfig struct {
	MCP struct {
		Servers map[string]opencodeMCPServer `json:"servers"`
	} `json:"mcp"`
	Permissions []opencodePermission `json:"permissions"`
}

func NewOpencodeRunner(harnessBinary, lifecyclePlugin string, deps Dependencies, canonicalPlugin func(string) string) OpencodeRunner {
	return OpencodeRunner{
		canonicalPlugin: canonicalPlugin,
		harnessBinary:   harnessBinary,
		lifecyclePlugin: lifecyclePlugin,
		deps:            normalizeDependencies(deps),
		readyLimit:      opencodeReadyLimit,
		warmLimit:       opencodeWarmLimit,
	}
}

func (OpencodeRunner) Name() string { return "opencode" }

func (r OpencodeRunner) Preflight(ctx context.Context, request port.HostProbeRequest) (result port.HostProbePreflight) {
	result = port.HostProbePreflight{Host: r.Name(), RequestedModel: request.Model, EvidenceSource: "opencode_preflight"}
	fail := func(code string) port.HostProbePreflight {
		result.Ready = false
		result.Cause = "harness_environment"
		result.Code = code
		return result
	}
	executable, err := r.resolveExecutable()
	if err != nil {
		return fail("executable_not_found")
	}
	result.Installed = true
	root, err := newEpisodeRoot(r.deps, r.Name())
	if err != nil {
		return fail(episodeRootFailureCode(err))
	}
	defer func() {
		if err := r.deps.RemoveAll(root); err != nil {
			result.MockExtensionVerified = false
			result = fail("private_root_cleanup_failed")
		}
	}()
	password, err := opencodePassword()
	if err != nil {
		return fail("episode_prepare_failed")
	}
	output, err := r.deps.Process.Run(ctx, CommandRequest{
		Cwd:     root,
		Argv:    []string{executable, "--version"},
		Env:     isolatedOpencodeEnv(r.deps, root, password),
		Timeout: VersionTimeout,
	})
	if err != nil || output.ExitCode != 0 {
		return fail("version_probe_failed")
	}
	result.Ready = true
	result.Version = boundedVersion(string(output.Stdout))
	result.MockExtensionVerified = r.validLifecyclePlugin()
	if !result.MockExtensionVerified {
		return fail("mock_extension_invalid")
	}
	if !explicitOmoModel(request.Model) {
		return fail("explicit_model_required")
	}
	return result
}

func (r OpencodeRunner) resolveExecutable() (string, error) {
	executable, err := r.deps.LookPath("opencode")
	if err != nil {
		return "", err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("opencode_executable_invalid")
	}
	return executable, nil
}

func (r OpencodeRunner) validLifecyclePlugin() bool {
	absolute, err := filepath.Abs(r.harnessBinary)
	return err == nil && r.canonicalPlugin != nil && r.lifecyclePlugin == r.canonicalPlugin(absolute)
}

func (r OpencodeRunner) Run(ctx context.Context, request port.HostProbeRequest) (result port.HostProbeResult) {
	started := r.deps.Now()
	fail := func(cause, code string) port.HostProbeResult {
		return failedResult(r.Name(), "", request, started, r.deps, cause, code)
	}
	harnessBinary, code := r.episodeInputs(request)
	if code != "" {
		return fail("harness_environment", code)
	}
	request.HarnessBinary = harnessBinary
	executable, err := r.resolveExecutable()
	if err != nil {
		return fail("harness_environment", "executable_not_found")
	}
	root, err := newEpisodeRoot(r.deps, r.Name())
	if err != nil {
		return fail("harness_environment", episodeRootFailureCode(err))
	}
	defer func() {
		if err := r.deps.RemoveAll(root); err != nil {
			result = fail("harness_environment", "private_root_cleanup_failed")
		}
	}()
	targetTool := omoProbeServer + "_" + request.ProbeTool
	resultPath := filepath.Join(root, "result.json")
	if err := prepareOpencodeEpisode(root, r.lifecyclePlugin, targetTool, request, resultPath); err != nil {
		return fail("harness_environment", "episode_prepare_failed")
	}
	work, err := filepath.EvalSymlinks(filepath.Join(root, "work"))
	if err != nil {
		return fail("harness_environment", "episode_prepare_failed")
	}
	password, err := opencodePassword()
	if err != nil {
		return fail("harness_environment", "episode_prepare_failed")
	}
	env := replaceEnvironmentValue(isolatedOpencodeEnv(r.deps, root, password), "PWD", work)
	server, base, code := r.startEpisodeServer(ctx, executable, work, env, password)
	if server != nil {
		defer func() {
			if err := server.Stop(); err != nil && result.Completed {
				result = fail("harness_environment", "server_stop_failed")
			}
		}()
	}
	if code != "" {
		return fail("harness_environment", code)
	}
	output, err := r.deps.Process.Run(ctx, CommandRequest{
		Cwd:     work,
		Argv:    []string{executable, "run", "--server", base, "--model", request.Model, "--format", "json"},
		Env:     env,
		Stdin:   request.Prompt,
		Timeout: EpisodeTimeout,
	})
	return opencodeEpisodeFinish(r.Name(), request, started, r.deps, resultPath, targetTool, output, err)
}

// episodeInputs checks the runner and request before any filesystem work and
// returns the absolute harness binary, or a failure code.
func (r OpencodeRunner) episodeInputs(request port.HostProbeRequest) (harnessBinary, code string) {
	if r.harnessBinary == "" {
		return "", "harness_binary_missing"
	}
	if !r.validLifecyclePlugin() {
		return "", "mock_extension_invalid"
	}
	if !explicitOmoModel(request.Model) {
		return "", "explicit_model_required"
	}
	harnessBinary, err := filepath.Abs(r.harnessBinary)
	if err != nil {
		return "", "harness_binary_path_invalid"
	}
	return harnessBinary, ""
}

// startEpisodeServer starts the private server and waits until it is ready and
// its location has connected the probe MCP server. The caller stops a non-nil
// server even when a failure code comes back.
func (r OpencodeRunner) startEpisodeServer(ctx context.Context, executable, work string, env []string, password string) (server ServerProcess, base, code string) {
	serverPort, err := freeLoopbackPort()
	if err != nil {
		return nil, "", "server_port_unavailable"
	}
	server, err = r.deps.StartServer(ctx, ServerRequest{
		Cwd:  work,
		Argv: []string{executable, "serve", "--hostname", "127.0.0.1", "--port", strconv.Itoa(serverPort)},
		Env:  env,
	})
	if err != nil {
		return nil, "", "server_start_failed"
	}
	base = "http://127.0.0.1:" + strconv.Itoa(serverPort)
	if err := r.waitReady(ctx, base, password); err != nil {
		return server, "", "server_not_ready"
	}
	if code := r.warmProbeMCP(ctx, base, password, work); code != "" {
		return server, "", code
	}
	return server, base, ""
}

// waitReady polls the authenticated info endpoint; the unauthenticated root
// serves the web app even while the server is misconfigured.
func (r OpencodeRunner) waitReady(ctx context.Context, base, password string) error {
	ready, cancel := context.WithTimeout(ctx, r.readyLimit)
	defer cancel()
	for {
		if status, _, err := r.deps.HTTPGet(ready, base+"/api/info", opencodeServerUser, password); err == nil && status == http.StatusOK {
			return nil
		}
		select {
		case <-ready.Done():
			return ready.Err()
		case <-time.After(opencodeReadyInterval):
		}
	}
}

// warmProbeMCP creates the run's location, which starts its MCP servers, and
// waits until the probe server has connected.
func (r OpencodeRunner) warmProbeMCP(ctx context.Context, base, password, directory string) string {
	warm, cancel := context.WithTimeout(ctx, r.warmLimit)
	defer cancel()
	endpoint := base + "/api/mcp?" + url.Values{"location[directory]": {directory}}.Encode()
	for {
		if status, body, err := r.deps.HTTPGet(warm, endpoint, opencodeServerUser, password); err == nil && status == http.StatusOK {
			switch opencodeMCPStatus(body) {
			case "connected":
				return ""
			case "failed", "disabled":
				return "probe_mcp_unavailable"
			}
		}
		select {
		case <-warm.Done():
			return "probe_mcp_not_connected"
		case <-time.After(opencodeWarmInterval):
		}
	}
}

func opencodeMCPStatus(body []byte) string {
	var response struct {
		Data []struct {
			Name   string `json:"name"`
			Status struct {
				Status string `json:"status"`
			} `json:"status"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &response) != nil {
		return ""
	}
	for _, server := range response.Data {
		if server.Name == omoProbeServer {
			return server.Status.Status
		}
	}
	return ""
}

func prepareOpencodeEpisode(root, lifecyclePlugin, targetTool string, request port.HostProbeRequest, resultPath string) error {
	if !filepath.IsAbs(root) || !filepath.IsAbs(request.HarnessBinary) || !filepath.IsAbs(resultPath) {
		return fmt.Errorf("episode_path_invalid")
	}
	for _, directory := range []string{"home", "data", "state", "cache"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
			return err
		}
	}
	const projectDoc = "# opencode conformance context\n\nThis private document proves lifecycle context injection for one isolated host probe.\n"
	if err := writePrivateFile(filepath.Join(root, "work", ".issueops", "CONSTITUTION.md"), []byte(projectDoc)); err != nil {
		return err
	}
	configRoot := filepath.Join(root, "config", "opencode")
	// Plugins run in sorted path order, so the guard sees issueops's push.
	if err := writePrivateFile(filepath.Join(configRoot, "plugins", "issueops.js"), []byte(lifecyclePlugin)); err != nil {
		return err
	}
	if err := writePrivateFile(filepath.Join(configRoot, "plugins", "z-guard.js"), []byte(opencodeGuardPlugin(targetTool))); err != nil {
		return err
	}
	var config opencodeEpisodeConfig
	config.MCP.Servers = map[string]opencodeMCPServer{
		omoProbeServer: {Type: "local", Command: serveArgv(request, resultPath), Codemode: false},
	}
	// Denied tools leave the request entirely, so the model sees only the probe server.
	config.Permissions = []opencodePermission{
		{Action: "*", Resource: "*", Effect: "deny"},
		{Action: omoProbeServer + "_*", Resource: "*", Effect: "allow"},
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return writePrivateFile(filepath.Join(configRoot, "opencode.json"), data)
}

// opencodeGuardPlugin blocks the probe tool in a session whose model request
// did not carry the issueops catalog. A throw in execute.before is the only
// supported opencode tool block, and its message reaches the model verbatim.
func opencodeGuardPlugin(targetTool string) string {
	return fmt.Sprintf(`const targetTool = %s
const catalogMarker = %s

export default {
  id: "issueops-conformance-guard",
  setup(ctx) {
    const injected = new Set()
    ctx.session.hook("context", async (event) => {
      if (event.system.some((part) => typeof part.text === "string" && part.text.includes(catalogMarker))) {
        injected.add(event.sessionID)
      }
    })
    ctx.tool.hook("execute.before", async (event) => {
      if (event.tool === targetTool && !injected.has(event.sessionID)) {
        throw new Error(%s)
      }
    })
  },
}
`, jsonString(targetTool), jsonString(opencodeCatalogMarker), jsonString(opencodeGuardErrorText))
}

// isolatedOpencodeEnv points every opencode root at the episode and passes only
// provider API keys from the user's environment. Session ids of other hosts and
// any OPENCODE_* the test did not set stay out.
func isolatedOpencodeEnv(deps Dependencies, root, password string) []string {
	var keys []string
	for _, entry := range deps.Environ() {
		if name, _, found := strings.Cut(entry, "="); found && strings.HasSuffix(name, "_API_KEY") {
			keys = append(keys, name)
		}
	}
	environment := isolatedHostEnv(deps, append(keys, "TERM")...)
	for _, value := range [][2]string{
		{"HOME", filepath.Join(root, "home")},
		{"XDG_CONFIG_HOME", filepath.Join(root, "config")},
		{"XDG_DATA_HOME", filepath.Join(root, "data")},
		{"XDG_STATE_HOME", filepath.Join(root, "state")},
		{"XDG_CACHE_HOME", filepath.Join(root, "cache")},
		{"OPENCODE_DISABLE_AUTOUPDATE", "1"},
		{"OPENCODE_DISABLE_MODELS_FETCH", "1"},
		{"OPENCODE_DISABLE_CLAUDE_CODE", "1"},
		{"OPENCODE_PASSWORD", password},
	} {
		environment = replaceEnvironmentValue(environment, value[0], value[1])
	}
	return environment
}

func opencodePassword() (string, error) {
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return hex.EncodeToString(secret), nil
}

func freeLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func opencodeEpisodeFinish(name string, request port.HostProbeRequest, started time.Time, deps Dependencies, resultPath, targetTool string, output CommandOutput, runErr error) port.HostProbeResult {
	fail := func(cause, code string) port.HostProbeResult {
		result := failedResult(name, "", request, started, deps, cause, code)
		result.ExitCode = output.ExitCode
		return result
	}
	// A failed run prints its reason as an error event; report it, not the exit.
	if code := opencodeReportedError(output.Stdout); code != "" {
		return fail("harness_environment", code)
	}
	if runErr != nil {
		return fail(normalizedProcessFailure(runErr, "host_process_failed"))
	}
	if output.ExitCode != 0 {
		return fail("harness_environment", "host_process_failed")
	}
	observation, err := observeOpencodeStream(output.Stdout, targetTool)
	if err != nil {
		cause, code := omoStreamFailure(err)
		result := fail(cause, code)
		result.AmbientToolCount = observation.AmbientToolCount
		result.CallCount = observation.MCPCallCount
		return result
	}
	capture, err := decodeEpisodeCapture(resultPath, request)
	if err != nil {
		return fail(normalizedCaptureFailure(err))
	}
	if capture.CallCount != 1 {
		return fail("transport", "probe_call_cardinality_invalid")
	}
	result := completedResult(name, "", request, started, deps, capture)
	// The guard plugin lets the probe tool run only after the managed plugin
	// put the catalog into that session's model request.
	result.SessionStartObserved = true
	result.ResponseSHA256 = observation.ResponseSHA256
	result.AmbientToolCount = observation.AmbientToolCount
	result.ExitCode = output.ExitCode
	return result
}

// opencodeReportedError maps the stream's error event to a failure code. The
// provider rejecting the request (for example, a free model refused outside
// the opencode client) is an environment problem, not model behavior.
func opencodeReportedError(stdout []byte) string {
	decoder := json.NewDecoder(bytes.NewReader(stdout))
	for {
		var event struct {
			Type  string `json:"type"`
			Error struct {
				Type string `json:"type"`
			} `json:"error"`
		}
		if decoder.Decode(&event) != nil {
			return ""
		}
		if event.Type != "error" {
			continue
		}
		if event.Error.Type == "provider.auth" {
			return "provider_auth_failed"
		}
		return "host_reported_error"
	}
}

// observeOpencodeStream projects `opencode run --format json`: every
// tool_use line is one executed tool call, and only the probe tool may run.
func observeOpencodeStream(data []byte, targetTool string) (omoStreamObservation, error) {
	var observation omoStreamObservation
	if len(data) == 0 || len(data) > MaxOutputBytes {
		return observation, fmt.Errorf("host_stream_invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var results []any
	for {
		var event struct {
			Type string `json:"type"`
			Part struct {
				Tool  string `json:"tool"`
				State struct {
					Status string `json:"status"`
					Output any    `json:"output"`
				} `json:"state"`
			} `json:"part"`
		}
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return observation, fmt.Errorf("host_stream_invalid")
		}
		if event.Type != "tool_use" {
			continue
		}
		observation.AmbientToolCount++
		if event.Part.Tool != targetTool {
			return observation, fmt.Errorf("host_stream_non_target_call")
		}
		observation.MCPCallCount++
		if event.Part.State.Status != "completed" {
			return observation, fmt.Errorf("host_stream_tool_error")
		}
		results = append(results, map[string]any{"content": event.Part.State.Output})
	}
	if observation.AmbientToolCount == 0 {
		return observation, fmt.Errorf("host_stream_no_call")
	}
	if observation.MCPCallCount != 1 {
		return observation, fmt.Errorf("host_stream_multiple_calls")
	}
	digest, err := semanticResponseDigest(results)
	if err != nil {
		return observation, fmt.Errorf("host_stream_invalid")
	}
	observation.ResponseSHA256 = digest
	return observation, nil
}

type execServer struct {
	cmd    *exec.Cmd
	waited chan error
}

// startExecServer starts the server in its own process group and discards its
// output, so a full pipe can never stall it.
func startExecServer(_ context.Context, request ServerRequest) (ServerProcess, error) {
	if len(request.Argv) == 0 {
		return nil, fmt.Errorf("empty_argv")
	}
	cmd := exec.Command(request.Argv[0], request.Argv[1:]...)
	configureProcessGroup(cmd)
	cmd.Dir = request.Cwd
	cmd.Env = append([]string(nil), request.Env...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	server := &execServer{cmd: cmd, waited: make(chan error, 1)}
	go func() { server.waited <- cmd.Wait() }()
	return server, nil
}

// Stop sends SIGTERM to the group, escalates to SIGKILL after the grace
// period, and gives up once the hard limit passes.
func (s *execServer) Stop() error {
	_ = interruptProcessTree(s.cmd)
	select {
	case <-s.waited:
		return nil
	case <-time.After(opencodeStopGrace):
	}
	_ = terminateProcessTree(s.cmd)
	select {
	case <-s.waited:
		return nil
	case <-time.After(processTerminationHardLimit):
		return fmt.Errorf("server_termination_failed")
	}
}

func basicAuthGet(ctx context.Context, address, user, password string) (int, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return 0, nil, err
	}
	request.SetBasicAuth(user, password)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxOutputBytes))
	return response.StatusCode, body, err
}

var _ port.HostProbeRunner = OpencodeRunner{}
