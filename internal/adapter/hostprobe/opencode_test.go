package hostprobe

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"

	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/port"
)

const opencodeTestTarget = "issueops_probe_harness_probe_empty_object"

func opencodeProbeRequest() port.HostProbeRequest {
	request := omoProbeRequest()
	request.Model = "opencode/deepseek-v4.1-flash-free"
	return request
}

func opencodeTestPlugin(binary string) string {
	absolute, _ := filepath.Abs(binary)
	return hostprotocol.OpencodeLifecyclePlugin(absolute)
}

func opencodeToolUse(tool, status, output string) string {
	encoded, _ := json.Marshal(map[string]any{"type": "tool_use", "sessionID": "ses_1", "part": map[string]any{
		"type": "tool", "id": "call_0", "tool": tool, "state": map[string]any{"status": status, "input": map[string]any{}, "output": output},
	}})
	return string(encoded)
}

func opencodeSuccessfulStream() []byte {
	return []byte(strings.Join([]string{
		`{"type":"step_start","sessionID":"ses_1","part":{"type":"step-start"}}`,
		opencodeToolUse(opencodeTestTarget, "completed", "captured"),
		`{"type":"step_finish","sessionID":"ses_1","part":{"type":"step-finish","tokens":{"input":1,"output":1,"reasoning":0,"cache":{"read":0,"write":0}}}}`,
		`{"type":"text","sessionID":"ses_1","part":{"type":"text","text":"done"}}`,
	}, "\n") + "\n")
}

type fakeOpencodeServer struct {
	events  *[]string
	stopErr error
}

func (s fakeOpencodeServer) Stop() error {
	*s.events = append(*s.events, "stop")
	return s.stopErr
}

// opencodeHarness wires fake server, HTTP, and process dependencies and
// records the order of the runner's calls.
type opencodeHarness struct {
	root       string
	events     []string
	server     ServerRequest
	run        CommandRequest
	infoStatus int
	mcpBody    string
	runErr     error
	environ    []string
}

func newOpencodeHarness(t *testing.T) *opencodeHarness {
	return &opencodeHarness{
		root:       filepath.Join(t.TempDir(), "episode"),
		infoStatus: 200,
		mcpBody:    `{"data":[{"name":"issueops_probe","status":{"status":"connected"}}]}`,
		environ:    []string{"PATH=/test/bin", "HOME=/private/home", "USER=tester"},
	}
}

func (h *opencodeHarness) runner(t *testing.T) OpencodeRunner {
	harness, err := filepath.Abs(filepath.Join("testdata", "issueops"))
	if err != nil {
		t.Fatal(err)
	}
	runner := NewOpencodeRunner(harness, opencodeTestPlugin(harness), Dependencies{
		Process: &omoFakeProcess{run: func(_ context.Context, request CommandRequest) (CommandOutput, error) {
			if len(request.Argv) > 1 && request.Argv[1] == "--version" {
				return CommandOutput{Stdout: []byte("opencode v2.0.32\n")}, nil
			}
			h.events = append(h.events, "run")
			h.run = request
			if h.runErr != nil {
				return CommandOutput{}, h.runErr
			}
			writeOmoCapture(t, filepath.Join(h.root, "result.json"), opencodeProbeRequest())
			return CommandOutput{Stdout: opencodeSuccessfulStream()}, nil
		}},
		LookPath: func(name string) (string, error) {
			if name != "opencode" {
				t.Fatalf("looked up %q", name)
			}
			return "/test/bin/opencode", nil
		},
		TempDir: func(_, pattern string) (string, error) {
			if pattern != "issueops-conformance-opencode-" {
				t.Fatalf("temp pattern = %q", pattern)
			}
			return h.root, os.Mkdir(h.root, 0o700)
		},
		Environ: func() []string { return h.environ },
		StartServer: func(_ context.Context, request ServerRequest) (ServerProcess, error) {
			h.events = append(h.events, "serve")
			h.server = request
			return fakeOpencodeServer{events: &h.events}, nil
		},
		HTTPGet: func(_ context.Context, address, user, password string) (int, []byte, error) {
			if user != "opencode" || password == "" || password != environmentValue(t, h.server.Env, "OPENCODE_PASSWORD") {
				t.Fatalf("server auth = %q/%q", user, password)
			}
			if strings.Contains(address, "/api/info") {
				h.events = append(h.events, "info")
				return h.infoStatus, nil, nil
			}
			h.events = append(h.events, "mcp")
			return 200, []byte(h.mcpBody), nil
		},
	}, hostprotocol.OpencodeLifecyclePlugin)
	runner.readyLimit, runner.warmLimit = 300*time.Millisecond, 500*time.Millisecond
	return runner
}

func TestOpencodeRunnerBuildsIsolatedEnvWithoutUserSessionIDs(t *testing.T) {
	h := newOpencodeHarness(t)
	h.environ = append(h.environ, "TERM=xterm", "ANTHROPIC_API_KEY=provider-key",
		"ISSUEOPS_OMP_SESSION_ID=omp-session", "OPENCODE_SESSION_ID=ses_user", "CODEX_THREAD_ID=codex-thread",
		"OPENCODE_SERVER_PASSWORD=user-password", "OPENCODE_CONFIG_CONTENT={}", "XDG_CONFIG_HOME=/user/config")
	request := opencodeProbeRequest()
	result := h.runner(t).Run(context.Background(), request)
	if !result.Completed {
		t.Fatalf("result = %+v", result)
	}
	if !reflect.DeepEqual(h.server.Env, h.run.Env) {
		t.Fatalf("server env %q differs from run env %q", h.server.Env, h.run.Env)
	}
	names := []string{}
	for _, entry := range h.run.Env {
		name, _, _ := strings.Cut(entry, "=")
		names = append(names, name)
	}
	slices.Sort(names)
	want := []string{"ANTHROPIC_API_KEY", "HOME", "OPENCODE_DISABLE_AUTOUPDATE", "OPENCODE_DISABLE_CLAUDE_CODE", "OPENCODE_DISABLE_MODELS_FETCH", "OPENCODE_PASSWORD", "PATH", "PWD", "TERM", "USER", "XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("env names = %q, want %q", names, want)
	}
	for name, dir := range map[string]string{"HOME": "home", "XDG_CONFIG_HOME": "config", "XDG_DATA_HOME": "data", "XDG_STATE_HOME": "state", "XDG_CACHE_HOME": "cache"} {
		if got := environmentValue(t, h.run.Env, name); got != filepath.Join(h.root, dir) {
			t.Fatalf("%s = %q", name, got)
		}
	}
	password := environmentValue(t, h.run.Env, "OPENCODE_PASSWORD")
	if password == "user-password" || len(password) < 32 {
		t.Fatalf("server password = %q", password)
	}
	if h.run.Stdin != request.Prompt || slices.Contains(h.run.Argv, request.Prompt) {
		t.Fatalf("prompt must go on stdin: argv=%q stdin=%q", h.run.Argv, h.run.Stdin)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{password, "provider-key", "user-password", request.Prompt} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("result retained %q: %s", forbidden, encoded)
		}
	}
	h2 := newOpencodeHarness(t)
	h2.infoStatus = 401
	failed := h2.runner(t).Run(context.Background(), request)
	if encoded, _ := json.Marshal(failed); failed.Completed || strings.Contains(string(encoded), environmentValue(t, h2.server.Env, "OPENCODE_PASSWORD")) {
		t.Fatalf("failed result leaked the password or completed: %s", encoded)
	}
}

func TestOpencodeRunnerWarmsMCPBeforeRun(t *testing.T) {
	for _, tc := range []struct {
		name       string
		infoStatus int
		mcpBody    string
		runErr     error
		wantCode   string
		wantEvents func([]string) bool
	}{
		{name: "connected", wantEvents: func(e []string) bool {
			return slices.Equal(e, []string{"serve", "info", "mcp", "run", "stop"})
		}},
		{name: "server never ready", infoStatus: 503, wantCode: "server_not_ready", wantEvents: noRunOneStop},
		{name: "probe never connects", mcpBody: `{"data":[]}`, wantCode: "probe_mcp_not_connected", wantEvents: noRunOneStop},
		{name: "probe failed", mcpBody: `{"data":[{"name":"issueops_probe","status":{"status":"failed","error":"x"}}]}`, wantCode: "probe_mcp_unavailable", wantEvents: func(e []string) bool {
			return slices.Equal(e, []string{"serve", "info", "mcp", "stop"})
		}},
		{name: "probe disabled", mcpBody: `{"data":[{"name":"issueops_probe","status":{"status":"disabled"}}]}`, wantCode: "probe_mcp_unavailable", wantEvents: noRunOneStop},
		{name: "run fails", runErr: errors.New("command_timeout"), wantCode: "command_timeout", wantEvents: func(e []string) bool {
			return slices.Equal(e[len(e)-2:], []string{"run", "stop"}) && countEvent(e, "stop") == 1
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newOpencodeHarness(t)
			if tc.infoStatus != 0 {
				h.infoStatus = tc.infoStatus
			}
			if tc.mcpBody != "" {
				h.mcpBody = tc.mcpBody
			}
			h.runErr = tc.runErr
			result := h.runner(t).Run(context.Background(), opencodeProbeRequest())
			if result.Code != tc.wantCode || result.Completed != (tc.wantCode == "") {
				t.Fatalf("result = %+v, want code %q", result, tc.wantCode)
			}
			if !tc.wantEvents(h.events) {
				t.Fatalf("events = %q", h.events)
			}
			work, err := filepath.EvalSymlinks(filepath.Join(filepath.Dir(h.root), "episode"))
			if err == nil {
				t.Fatalf("episode root survived: %s", work)
			}
			if h.server.Cwd == "" || filepath.Base(h.server.Cwd) != "work" || environmentValue(t, h.server.Env, "PWD") != h.server.Cwd {
				t.Fatalf("server cwd = %q env PWD = %q", h.server.Cwd, environmentValue(t, h.server.Env, "PWD"))
			}
			if tc.wantCode == "" {
				if !slices.Equal(h.run.Argv, []string{"/test/bin/opencode", "run", "--server", h.run.Argv[3], "--model", "opencode/deepseek-v4.1-flash-free", "--format", "json"}) ||
					!strings.HasPrefix(h.run.Argv[3], "http://127.0.0.1:") || h.run.Cwd != h.server.Cwd {
					t.Fatalf("run = %+v", h.run)
				}
				if !result.SessionStartObserved || result.CallCount != 1 || result.AmbientToolCount != 1 {
					t.Fatalf("runtime evidence = %+v", result)
				}
			}
		})
	}
}

func noRunOneStop(events []string) bool {
	return !slices.Contains(events, "run") && countEvent(events, "stop") == 1
}

func countEvent(events []string, name string) int {
	count := 0
	for _, event := range events {
		if event == name {
			count++
		}
	}
	return count
}

func TestOpencodeGuardPluginBlocksProbeToolWithoutCatalog(t *testing.T) {
	source := opencodeGuardPlugin(opencodeTestTarget)
	script := strings.Replace(source, "export default {", "globalThis.__guard = {", 1)
	for _, tc := range []struct {
		name    string
		system  string
		tool    string
		blocked bool
	}{
		{name: "catalog injected", system: "catalog of /repo/.issueops/CONSTITUTION.md", tool: opencodeTestTarget},
		{name: "catalog missing", system: "base prompt only", tool: opencodeTestTarget, blocked: true},
		{name: "other tool without catalog", system: "base prompt only", tool: "shell"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := goja.New()
			if _, err := runtime.RunString(script); err != nil {
				t.Fatal(err)
			}
			hooks := map[string]goja.Callable{}
			register := func(call goja.FunctionCall) goja.Value {
				handler, _ := goja.AssertFunction(call.Argument(1))
				hooks[call.Argument(0).String()] = handler
				return goja.Undefined()
			}
			ctx := runtime.NewObject()
			session, tool := runtime.NewObject(), runtime.NewObject()
			_ = session.Set("hook", register)
			_ = tool.Set("hook", register)
			_ = ctx.Set("session", session)
			_ = ctx.Set("tool", tool)
			setup, _ := goja.AssertFunction(runtime.Get("__guard").ToObject(runtime).Get("setup"))
			if _, err := setup(goja.Undefined(), ctx); err != nil {
				t.Fatal(err)
			}
			contextEvent := runtime.ToValue(map[string]any{"sessionID": "ses_1", "system": []any{map[string]any{"type": "text", "text": tc.system}}})
			if _, err := hooks["context"](goja.Undefined(), contextEvent); err != nil {
				t.Fatal(err)
			}
			returned, err := hooks["execute.before"](goja.Undefined(), runtime.ToValue(map[string]any{"sessionID": "ses_1", "tool": tc.tool}))
			if err != nil {
				t.Fatal(err)
			}
			_, _ = runtime.RunString("void 0")
			promise := returned.Export().(*goja.Promise)
			if blocked := promise.State() == goja.PromiseStateRejected; blocked != tc.blocked {
				t.Fatalf("blocked = %v, want %v", blocked, tc.blocked)
			}
			if tc.blocked && !strings.Contains(promise.Result().String(), "issueops context injection missing") {
				t.Fatalf("block reason = %v", promise.Result())
			}
		})
	}
}

func TestOpencodeRunnerProjectsRunJSONToolUse(t *testing.T) {
	observation, err := observeOpencodeStream(opencodeSuccessfulStream(), opencodeTestTarget)
	want, _ := semanticResponseDigest([]any{map[string]any{"content": "captured"}})
	if err != nil || observation.MCPCallCount != 1 || observation.AmbientToolCount != 1 || observation.ResponseSHA256 != want {
		t.Fatalf("observation = %+v err=%v", observation, err)
	}
	for name, tc := range map[string]struct {
		lines []string
		want  string
	}{
		"guard blocked": {[]string{opencodeToolUse(opencodeTestTarget, "error", "")}, "host_stream_tool_error"},
		"other tool":    {[]string{opencodeToolUse("shell", "completed", "x")}, "host_stream_non_target_call"},
		"no tool call":  {[]string{`{"type":"text","part":{"text":"no"}}`}, "host_stream_no_call"},
		"called twice":  {[]string{opencodeToolUse(opencodeTestTarget, "completed", "a"), opencodeToolUse(opencodeTestTarget, "completed", "b")}, "host_stream_multiple_calls"},
		"not json":      {[]string{"not json"}, "host_stream_invalid"},
	} {
		if _, err := observeOpencodeStream([]byte(strings.Join(tc.lines, "\n")), opencodeTestTarget); err == nil || err.Error() != tc.want {
			t.Fatalf("%s: err = %v, want %s", name, err, tc.want)
		}
	}
}

func TestOpencodeRunnerReportsRunErrorEvent(t *testing.T) {
	freeTier := `{"type":"error","sessionID":"ses_1","error":{"type":"provider.auth","message":"OpenCode's free tier can only be used from within OpenCode.","status":403}}` + "\n"
	result := opencodeEpisodeFinish("opencode", opencodeProbeRequest(), time.Unix(0, 0), normalizeDependencies(Dependencies{}), "/missing/result.json", opencodeTestTarget,
		CommandOutput{Stdout: []byte(freeTier), ExitCode: 1}, errors.New("command_failed"))
	if result.Completed || result.Cause != "harness_environment" || result.Code != "provider_auth_failed" || result.ExitCode != 1 {
		t.Fatalf("result = %+v", result)
	}
	other := opencodeEpisodeFinish("opencode", opencodeProbeRequest(), time.Unix(0, 0), normalizeDependencies(Dependencies{}), "/missing/result.json", opencodeTestTarget,
		CommandOutput{Stdout: []byte(`{"type":"error","error":{"type":"unknown"}}` + "\n"), ExitCode: 1}, errors.New("command_failed"))
	if other.Code != "host_reported_error" {
		t.Fatalf("other error = %+v", other)
	}
}

func TestOpencodeRunnerRequiresExplicitModel(t *testing.T) {
	for _, model := range []string{"", "default"} {
		h := newOpencodeHarness(t)
		runner := h.runner(t)
		request := opencodeProbeRequest()
		request.Model = model
		preflight := runner.Preflight(context.Background(), request)
		if preflight.Ready || !preflight.Installed || preflight.Code != "explicit_model_required" {
			t.Fatalf("preflight = %+v", preflight)
		}
		result := runner.Run(context.Background(), request)
		if result.Completed || result.Code != "explicit_model_required" || len(h.events) != 0 {
			t.Fatalf("run = %+v events = %q", result, h.events)
		}
	}
}
