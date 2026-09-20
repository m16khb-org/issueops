package hostprobe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	omoadapter "issueops/internal/adapter/omo"
	"issueops/internal/port"
)

type omoFakeProcess struct {
	requests []CommandRequest
	run      func(context.Context, CommandRequest) (CommandOutput, error)
}

func (f *omoFakeProcess) Run(ctx context.Context, request CommandRequest) (CommandOutput, error) {
	request.Argv = append([]string(nil), request.Argv...)
	request.Env = append([]string(nil), request.Env...)
	f.requests = append(f.requests, request)
	if f.run == nil {
		return CommandOutput{}, nil
	}
	return f.run(ctx, request)
}

func TestOmoRunnerPreflightUsesNativeOmoAndRequiresExplicitModel(t *testing.T) {
	t.Parallel()

	process := &omoFakeProcess{run: func(_ context.Context, request CommandRequest) (CommandOutput, error) {
		if !reflect.DeepEqual(request.Argv, []string{"/opt/bin/omo", "--version"}) {
			t.Fatalf("argv = %#v", request.Argv)
		}
		home := environmentValue(t, request.Env, "HOME")
		agent := environmentValue(t, request.Env, "OMO_CODING_AGENT_DIR")
		if home == "/private/home" || filepath.Base(home) != "home" || filepath.Dir(home) != filepath.Dir(agent) || filepath.Base(agent) != "agent" || environmentValue(t, request.Env, "PATH") != "/opt/bin" {
			t.Fatalf("private env = %#v", request.Env)
		}
		return CommandOutput{Stdout: []byte("omo 5.0.0-0.beta.22 (engine: senpi 2026.8.26-2)\n")}, nil
	}}
	runner := NewOmoRunner("/opt/bin/issueops", omoadapter.LifecycleExtension("/opt/bin/issueops"), Dependencies{
		Process: process,
		LookPath: func(name string) (string, error) {
			if name != "omo" {
				t.Fatalf("looked up %q, want native omo", name)
			}
			return "/opt/bin/omo", nil
		},
		Environ: func() []string {
			return []string{"PATH=/opt/bin", "HOME=/private/home", "OPENAI_API_KEY=secret"}
		},
	})

	ready := runner.Preflight(context.Background(), port.HostProbeRequest{Model: "google/gemini-2.5-pro"})
	if !ready.Ready || !ready.Installed || !ready.MockExtensionVerified || ready.Host != "omo" || ready.Version != "omo 5.0.0-0.beta.22 (engine: senpi 2026.8.26-2)" {
		t.Fatalf("preflight = %+v", ready)
	}
	notReady := runner.Preflight(context.Background(), port.HostProbeRequest{Model: "default"})
	if notReady.Ready || notReady.Code != "explicit_model_required" || notReady.Version == "" || notReady.EvidenceSource != "omo_preflight" {
		t.Fatalf("default-model preflight = %+v", notReady)
	}
}

func TestOmoRunnerPreflightRejectsUnverifiedLifecycleContract(t *testing.T) {
	t.Parallel()

	runner := NewOmoRunner("/opt/bin/issueops", "export default function issueops() {}\n", Dependencies{
		Process: &omoFakeProcess{run: func(_ context.Context, _ CommandRequest) (CommandOutput, error) {
			return CommandOutput{Stdout: []byte("omo 5.0.0-0.beta.22\n")}, nil
		}},
		LookPath: func(string) (string, error) { return "/opt/bin/omo", nil },
	})

	got := runner.Preflight(context.Background(), port.HostProbeRequest{Model: "google/gemini-2.5-pro"})
	if got.Ready || !got.Installed || got.MockExtensionVerified || got.Code != "mock_extension_invalid" || got.EvidenceSource != "omo_preflight" {
		t.Fatalf("preflight = %+v", got)
	}
}

func TestOmoRunnerPreflightKeepsInstalledEvidenceWhenVersionProbeFails(t *testing.T) {
	t.Parallel()

	runner := NewOmoRunner("/opt/bin/issueops", omoTestLifecycleExtension(), Dependencies{
		Process: &omoFakeProcess{run: func(_ context.Context, _ CommandRequest) (CommandOutput, error) {
			return CommandOutput{}, errors.New("version unavailable")
		}},
		LookPath: func(string) (string, error) { return "/opt/bin/omo", nil },
	})

	got := runner.Preflight(context.Background(), port.HostProbeRequest{Model: "google/gemini-2.5-pro"})
	if got.Ready || !got.Installed || got.Code != "version_probe_failed" || got.EvidenceSource != "omo_preflight" {
		t.Fatalf("preflight = %+v", got)
	}
}

func TestOmoRunnerRunUsesIsolatedNativeProbeAndCapturesEvidence(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	episodeRoot := filepath.Join(parent, "episode")
	harnessBinary, err := filepath.Abs(filepath.Join("testdata", "issueops"))
	if err != nil {
		t.Fatal(err)
	}
	sourceAgentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceAgentDir, "auth.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := omoProbeRequest()
	targetTool := "mcp_issueops_probe_" + request.ProbeTool
	lifecycleSource := omoTestLifecycleExtension()
	var received CommandRequest
	process := &omoFakeProcess{run: func(_ context.Context, command CommandRequest) (CommandOutput, error) {
		received = command
		if command.Cwd != episodeRoot || command.Timeout != EpisodeTimeout {
			t.Fatalf("command = %+v", command)
		}
		if !reflect.DeepEqual(command.Env, []string{
			"HOME=" + filepath.Join(episodeRoot, "home"),
			"OMO_CODING_AGENT_DIR=" + filepath.Join(episodeRoot, "agent"),
			"PATH=/test/bin",
			"USER=tester",
		}) {
			t.Fatalf("env = %#v", command.Env)
		}
		wantArgv := []string{
			"/test/bin/omo",
			"--mode", "json", "--print",
			"--model", request.Model,
			"--system-prompt", omoProbeSystemPrompt,
			"--no-session", "--session-dir", filepath.Join(episodeRoot, "sessions"),
			"--no-builtin-tools", "--tools", targetTool,
			"--extension", filepath.Join(episodeRoot, "issueops-lifecycle.js"),
			"--extension", filepath.Join(episodeRoot, "issueops-conformance.js"),
			"--no-extensions", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files",
			"--no-approve", "--offline", "--omo-senpi-disabled", "--no-model-fallback", "--no-recommended-models", "--no-nested-agents", "--pi-rules-disabled", "--ttsr-disabled",
			"--", request.Prompt,
		}
		if !reflect.DeepEqual(command.Argv, wantArgv) {
			t.Fatalf("argv = %#v\nwant = %#v", command.Argv, wantArgv)
		}

		rootInfo, err := os.Stat(episodeRoot)
		if err != nil || rootInfo.Mode().Perm() != 0o700 {
			t.Fatalf("episode root info=%v err=%v", rootInfo, err)
		}
		assertPrivateFileEquals(t, filepath.Join(episodeRoot, "issueops-lifecycle.js"), lifecycleSource)
		assertOmoProbeGuard(t, filepath.Join(episodeRoot, "issueops-conformance.js"), targetTool)
		assertOmoProbeProjectDoc(t, episodeRoot)
		assertOmoProbeConfig(t, filepath.Join(episodeRoot, "agent", "mcp.json"), harnessBinary, episodeRoot, request)
		assertPrivateFileEquals(t, filepath.Join(episodeRoot, "agent", "auth.json"), "{}\n")

		writeOmoCapture(t, filepath.Join(episodeRoot, "result.json"), request)
		return CommandOutput{Stdout: omoSuccessfulStream(request, targetTool), ExitCode: 0}, nil
	}}
	now := time.Unix(100, 0)
	runner := NewOmoRunner(filepath.Join("testdata", "issueops"), lifecycleSource, Dependencies{
		Process: process,
		LookPath: func(name string) (string, error) {
			if name != "omo" {
				t.Fatalf("looked up %q", name)
			}
			return "/test/bin/omo", nil
		},
		TempDir: func(_, pattern string) (string, error) {
			if pattern != "issueops-conformance-omo-" {
				t.Fatalf("temp pattern = %q", pattern)
			}
			if err := os.Mkdir(episodeRoot, 0o755); err != nil {
				return "", err
			}
			return episodeRoot, nil
		},
		Environ: func() []string {
			return []string{"PATH=/test/bin", "HOME=/private/home", "USER=tester", "OPENAI_API_KEY=secret", "OMO_CODING_AGENT_DIR=/ambient/omo"}
		},
		Getenv: func(name string) string {
			if name == "OMO_CODING_AGENT_DIR" {
				return sourceAgentDir
			}
			return ""
		},
		Now: func() time.Time {
			now = now.Add(25 * time.Millisecond)
			return now
		},
	})

	result := runner.Run(context.Background(), request)
	if !result.Completed || result.Host != "omo" || result.ObservedModel != "gemini-2.5-pro" || result.DurationMS != 25 {
		t.Fatalf("result = %+v", result)
	}
	if !result.SessionStartObserved || result.PreToolUseObserved || result.CallCount != 1 || result.ExitCode != 0 || result.AmbientToolCount != 1 {
		t.Fatalf("runtime evidence = %+v", result)
	}
	wantDigest, err := semanticResponseDigest([]any{map[string]any{"content": "captured"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ResponseSHA256 != wantDigest {
		t.Fatalf("response digest = %q, want %s", result.ResponseSHA256, wantDigest)
	}
	if received.Argv == nil {
		t.Fatal("native Omo process was not invoked")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{request.Prompt, "secret", "/private/home", "/ambient/omo", "captured"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("result retained %q: %s", forbidden, encoded)
		}
	}
	if _, err := os.Stat(episodeRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("episode root cleanup err=%v", err)
	}
}

func TestOmoRunnerRunFailsClosedOnStreamAndProcessFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		output     CommandOutput
		processErr error
		wantCause  string
		wantCode   string
		wantExit   int
	}{
		{name: "malformed json", output: CommandOutput{Stdout: []byte("{\n")}, wantCause: "transport", wantCode: "host_stream_invalid"},
		{name: "missing call", output: CommandOutput{Stdout: []byte(`{"type":"session","id":"s"}` + "\n")}, wantCause: "unknown", wantCode: "no_call"},
		{name: "missing observed model", output: CommandOutput{Stdout: omoStreamWithoutModel(omoProbeRequest())}, wantCause: "transport", wantCode: "host_stream_model_missing"},
		{name: "multiple calls", output: CommandOutput{Stdout: omoMultipleCallStream(omoProbeRequest())}, wantCause: "transport", wantCode: "host_stream_multiple_calls"},
		{name: "tool error", output: CommandOutput{Stdout: omoToolErrorStream(omoProbeRequest())}, wantCause: "transport", wantCode: "host_stream_tool_error"},
		{name: "nonzero cli", output: CommandOutput{ExitCode: 7}, processErr: errors.New("command_failed"), wantCause: "harness_environment", wantCode: "host_process_failed", wantExit: 7},
		{name: "timeout", output: CommandOutput{ExitCode: -1}, processErr: errors.New("command_timeout"), wantCause: "harness_environment", wantCode: "command_timeout", wantExit: -1},
		{name: "cancelled", output: CommandOutput{ExitCode: -1}, processErr: errors.New("command_cancelled"), wantCause: "harness_environment", wantCode: "command_cancelled", wantExit: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := omoProbeRequest()
			root := filepath.Join(t.TempDir(), "episode")
			runner := NewOmoRunner("issueops", omoTestLifecycleExtension(), Dependencies{
				LookPath: func(string) (string, error) { return "/test/bin/omo", nil },
				TempDir: func(_, _ string) (string, error) {
					if err := os.Mkdir(root, 0o700); err != nil {
						return "", err
					}
					return root, nil
				},
				Process: &omoFakeProcess{run: func(context.Context, CommandRequest) (CommandOutput, error) {
					if test.processErr == nil {
						writeOmoCapture(t, filepath.Join(root, "result.json"), request)
					}
					return test.output, test.processErr
				}},
				Getenv: func(string) string { return "" },
			})
			result := runner.Run(context.Background(), request)
			if result.Completed || result.Cause != test.wantCause || result.Code != test.wantCode || result.ExitCode != test.wantExit || result.EvidenceSource != "omo_runner" {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestOmoRunnerRunRejectsCaptureCardinalityMismatch(t *testing.T) {
	t.Parallel()

	request := omoProbeRequest()
	root := filepath.Join(t.TempDir(), "episode")
	runner := NewOmoRunner("issueops", omoTestLifecycleExtension(), Dependencies{
		LookPath: func(string) (string, error) { return "/test/bin/omo", nil },
		TempDir: func(_, _ string) (string, error) {
			if err := os.Mkdir(root, 0o700); err != nil {
				return "", err
			}
			return root, nil
		},
		Process: &omoFakeProcess{run: func(context.Context, CommandRequest) (CommandOutput, error) {
			writeOmoCapture(t, filepath.Join(root, "result.json"), request)
			multiple := map[string]any{
				"call_count": 2,
				"run_token_sha256": func() string {
					sum := sha256.Sum256([]byte(request.RunToken))
					return hex.EncodeToString(sum[:])
				}(),
			}
			body, _ := json.Marshal(multiple)
			if err := os.WriteFile(filepath.Join(root, "result.json.multiple"), body, 0o600); err != nil {
				t.Fatal(err)
			}
			return CommandOutput{Stdout: omoSuccessfulStream(request, "mcp_issueops_probe_"+request.ProbeTool)}, nil
		}},
		Getenv: func(string) string { return "" },
	})

	result := runner.Run(context.Background(), request)
	if result.Completed || result.Cause != "transport" || result.Code != "probe_call_cardinality_invalid" {
		t.Fatalf("result = %+v", result)
	}
}

func TestObserveOmoStreamRejectsAmbientUnmatchedAndDuplicateExecutions(t *testing.T) {
	request := omoProbeRequest()
	target := "mcp_issueops_probe_" + request.ProbeTool
	base := string(omoSuccessfulStream(request, target))
	ambientPair := `{"type":"tool_execution_start","toolCallId":"ambient-1","toolName":"read_file","args":{}}` + "\n" +
		`{"type":"tool_execution_end","toolCallId":"ambient-1","toolName":"read_file","result":{"content":"ambient"},"isError":false}` + "\n"
	targetStart := `{"type":"tool_execution_start","toolCallId":"call-1","toolName":"` + target + `","args":{}}` + "\n"
	targetEnd := `{"type":"tool_execution_end","toolCallId":"call-1","toolName":"` + target + `","result":{"content":"captured"},"isError":false}` + "\n"
	tests := []struct {
		name   string
		stream string
	}{
		{name: "ambient before target", stream: ambientPair + base},
		{name: "ambient after target", stream: base + ambientPair},
		{name: "unmatched ambient start", stream: base + `{"type":"tool_execution_start","toolCallId":"ambient-1","toolName":"read_file","args":{}}` + "\n"},
		{name: "unmatched ambient end", stream: base + `{"type":"tool_execution_end","toolCallId":"ambient-1","toolName":"read_file","result":{"content":"ambient"},"isError":false}` + "\n"},
		{name: "duplicate target start", stream: strings.Replace(base, targetStart, targetStart+targetStart, 1)},
		{name: "duplicate target end", stream: base + targetEnd},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := observeOmoStream([]byte(test.stream), target); err == nil {
				t.Fatal("invalid all-tool execution stream was accepted")
			}
		})
	}
}

func TestOmoRunnerDerivesAmbientToolCountFromRejectedStream(t *testing.T) {
	request := omoProbeRequest()
	root := filepath.Join(t.TempDir(), "episode")
	target := "mcp_issueops_probe_" + request.ProbeTool
	stream := append(omoSuccessfulStream(request, target), []byte(
		`{"type":"tool_execution_start","toolCallId":"ambient-1","toolName":"read_file","args":{}}`+"\n"+
			`{"type":"tool_execution_end","toolCallId":"ambient-1","toolName":"read_file","result":{"content":"ambient"},"isError":false}`+"\n",
	)...)
	runner := NewOmoRunner("issueops", omoTestLifecycleExtension(), Dependencies{
		LookPath: func(string) (string, error) { return "/test/bin/omo", nil },
		TempDir: func(_, _ string) (string, error) {
			if err := os.Mkdir(root, 0o700); err != nil {
				return "", err
			}
			return root, nil
		},
		Process: &omoFakeProcess{run: func(context.Context, CommandRequest) (CommandOutput, error) {
			writeOmoCapture(t, filepath.Join(root, "result.json"), request)
			return CommandOutput{Stdout: stream}, nil
		}},
		Getenv: func(string) string { return "" },
	})

	result := runner.Run(context.Background(), request)
	if result.Completed || result.Code != "host_stream_multiple_calls" || result.AmbientToolCount != 2 {
		t.Fatalf("result = %+v", result)
	}
}

func omoProbeRequest() port.HostProbeRequest {
	return port.HostProbeRequest{
		FixtureID:    "empty_object",
		ProbeTool:    "harness_probe_empty_object",
		SourceTool:   "source_empty_object",
		SchemaSHA256: "schema-sha",
		Prompt:       "Call the only allowed probe once.",
		Model:        "google/gemini-2.5-pro",
		Profile:      "clean",
		Attempt:      1,
		RunToken:     "episode-token",
	}
}

func omoTestLifecycleExtension() string {
	return omoadapter.LifecycleExtension("/test/bin/issueops")
}

func writeOmoCapture(t *testing.T, path string, request port.HostProbeRequest) {
	t.Helper()
	tokenDigest := sha256.Sum256([]byte(request.RunToken))
	rawDigest := sha256.Sum256([]byte(`{}`))
	body, err := json.Marshal(episodeCapture{
		FixtureID:          request.FixtureID,
		CallCount:          1,
		RawSHA256:          hex.EncodeToString(rawDigest[:]),
		CanonicalArguments: map[string]any{},
		SchemaSHA256:       request.SchemaSHA256,
		RunTokenSHA256:     hex.EncodeToString(tokenDigest[:]),
		Classification:     "exact_valid",
		AdvertisedValid:    true,
		CanonicalValid:     true,
		Diagnostics:        nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func omoSuccessfulStream(request port.HostProbeRequest, targetTool string) []byte {
	return []byte(strings.Join([]string{
		`{"type":"session","id":"session-1"}`,
		`{"type":"message_start","message":{"role":"assistant","provider":"google","model":"gemini-2.5-pro","content":[]}}`,
		`{"type":"tool_execution_start","toolCallId":"call-1","toolName":"` + targetTool + `","args":{}}`,
		`{"type":"tool_execution_end","toolCallId":"call-1","toolName":"` + targetTool + `","result":{"content":[{"type":"text","text":"captured"}],"details":{"server":"issueops_probe","tool":"` + request.ProbeTool + `"}},"isError":false}`,
		`{"type":"message_end","message":{"role":"assistant","provider":"google","model":"gemini-2.5-pro","content":[]}}`,
	}, "\n") + "\n")
}

func omoMultipleCallStream(request port.HostProbeRequest) []byte {
	target := "mcp_issueops_probe_" + request.ProbeTool
	one := string(omoSuccessfulStream(request, target))
	extra := `{"type":"tool_execution_start","toolCallId":"call-2","toolName":"` + target + `","args":{}}` + "\n" +
		`{"type":"tool_execution_end","toolCallId":"call-2","toolName":"` + target + `","result":{},"isError":false}` + "\n"
	return []byte(one + extra)
}

func omoToolErrorStream(request port.HostProbeRequest) []byte {
	target := "mcp_issueops_probe_" + request.ProbeTool
	return []byte(`{"type":"tool_execution_start","toolCallId":"call-1","toolName":"` + target + `","args":{}}` + "\n" +
		`{"type":"tool_execution_end","toolCallId":"call-1","toolName":"` + target + `","result":{"content":[]},"isError":true}` + "\n")
}

func omoStreamWithoutModel(request port.HostProbeRequest) []byte {
	target := "mcp_issueops_probe_" + request.ProbeTool
	return []byte(strings.Join([]string{
		`{"type":"session","id":"session-1"}`,
		`{"type":"tool_execution_start","toolCallId":"call-1","toolName":"` + target + `","args":{}}`,
		`{"type":"tool_execution_end","toolCallId":"call-1","toolName":"` + target + `","result":{},"isError":false}`,
	}, "\n") + "\n")
}

func assertPrivateFileEquals(t *testing.T, path, want string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("private file %s info=%v err=%v", path, info, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Fatalf("%s = %q, want %q", path, body, want)
	}
}

func assertOmoProbeGuard(t *testing.T, path, targetTool string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{targetTool, "issueops:project-docs", "custom_message", "display === false", "block: true"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("probe guard missing %q: %s", want, body)
		}
	}
}

func assertOmoProbeProjectDoc(t *testing.T, root string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, ".issueops", "CONSTITUTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Omo conformance context") {
		t.Fatalf("probe project doc = %q", body)
	}
}

func assertOmoProbeConfig(t *testing.T, path, harnessBinary, episodeRoot string, request port.HostProbeRequest) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config info=%v err=%v", info, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Settings struct {
			NativeToolSearch bool `json:"nativeToolSearch"`
		} `json:"settings"`
		MCPServers map[string]struct {
			Type         string   `json:"type"`
			Command      string   `json:"command"`
			Args         []string `json:"args"`
			Cwd          string   `json:"cwd"`
			Lifecycle    string   `json:"lifecycle"`
			IncludeTools []string `json:"includeTools"`
			DirectTools  []string `json:"directTools"`
			Exposure     string   `json:"exposure"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.MCPServers) != 1 {
		t.Fatalf("MCP servers = %#v", config.MCPServers)
	}
	server := config.MCPServers["issueops_probe"]
	if server.Type != "stdio" || server.Command != harnessBinary || server.Cwd != episodeRoot || server.Lifecycle != "eager" || server.Exposure != "direct" {
		t.Fatalf("server = %#v", server)
	}
	wantArgs := []string{"contract", "conformance", "serve", "--fixture-id", request.FixtureID, "--result-file", filepath.Join(episodeRoot, "result.json"), "--run-token", request.RunToken}
	if !reflect.DeepEqual(server.Args, wantArgs) || !reflect.DeepEqual(server.IncludeTools, []string{request.ProbeTool}) || !reflect.DeepEqual(server.DirectTools, []string{request.ProbeTool}) {
		t.Fatalf("server = %#v", server)
	}
}

var _ port.HostProbeRunner = OmoRunner{}
