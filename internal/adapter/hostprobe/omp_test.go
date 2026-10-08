package hostprobe

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/port"
)

func TestOmpRunnerPreflightUsesNativeOmpAndRequiresExplicitModel(t *testing.T) {
	t.Parallel()

	process := &omoFakeProcess{run: func(_ context.Context, request CommandRequest) (CommandOutput, error) {
		if !reflect.DeepEqual(request.Argv, []string{"/opt/bin/omp", "--version"}) {
			t.Fatalf("argv = %#v", request.Argv)
		}
		home := environmentValue(t, request.Env, "HOME")
		agent := environmentValue(t, request.Env, "PI_CODING_AGENT_DIR")
		if home == "/private/home" || filepath.Base(home) != "home" || filepath.Dir(home) != filepath.Dir(agent) || filepath.Base(agent) != "agent" ||
			environmentValue(t, request.Env, "PATH") != "/opt/bin" || environmentValue(t, request.Env, "OMP_MCP_REQUIRE_READY") != "1" {
			t.Fatalf("private env = %#v", request.Env)
		}
		for _, entry := range request.Env {
			if strings.HasPrefix(entry, "OPENAI_API_KEY=") || strings.HasPrefix(entry, "OMP_PROFILE=") {
				t.Fatalf("ambient env leaked: %#v", request.Env)
			}
		}
		return CommandOutput{Stdout: []byte("omp/18.8.4\n")}, nil
	}}
	runner := newTestOmpRunner("/opt/bin/issueops", hostprotocol.OmpLifecycleExtension("/opt/bin/issueops"), Dependencies{
		Process: process,
		LookPath: func(name string) (string, error) {
			if name != "omp" {
				t.Fatalf("looked up %q, want native omp", name)
			}
			return "/opt/bin/omp", nil
		},
		Environ: func() []string {
			return []string{"PATH=/opt/bin", "HOME=/private/home", "OPENAI_API_KEY=secret", "OMP_PROFILE=work"}
		},
		Getenv: func(string) string { return "" },
	})

	ready := runner.Preflight(context.Background(), port.HostProbeRequest{Model: "anthropic/claude-haiku-4-5"})
	if !ready.Ready || !ready.Installed || !ready.MockExtensionVerified || ready.Host != "omp" || ready.Version != "omp/18.8.4" {
		t.Fatalf("preflight = %+v", ready)
	}
	notReady := runner.Preflight(context.Background(), port.HostProbeRequest{Model: "default"})
	if notReady.Ready || notReady.Code != "explicit_model_required" || notReady.Version == "" || notReady.EvidenceSource != "omp_preflight" {
		t.Fatalf("default-model preflight = %+v", notReady)
	}
}

func TestOmpRunnerPreflightBindsExactCanonicalLifecycleModule(t *testing.T) {
	canonical := hostprotocol.OmpLifecycleExtension("/opt/bin/issueops")
	sources := map[string]string{
		"unrelated module": "export default function issueops() {}\n",
		"omo module":       hostprotocol.OmoLifecycleExtension("/opt/bin/issueops"),
	}
	for name, mutation := range map[string][2]string{
		"empty handler":      {"return runIssueopsLifecycle(pi, rule, ctx)", "return undefined"},
		"hook argv":          {`["hook", rule.subcommand, "--repo", ctx.cwd, "--json"]`, `["hook", rule.subcommand, "--json"]`},
		"session env export": {"process.env[sessionEnv.variable] = ctx.sessionManager.getSessionId()", "return"},
	} {
		source := strings.Replace(canonical, mutation[0], mutation[1], 1)
		if source == canonical {
			t.Fatalf("mutation target %q missing", mutation[0])
		}
		sources[name] = source
	}
	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			runner := newTestOmpRunner("/opt/bin/issueops", source, Dependencies{
				Process: &omoFakeProcess{run: func(_ context.Context, _ CommandRequest) (CommandOutput, error) {
					return CommandOutput{Stdout: []byte("omp/18.8.4\n")}, nil
				}},
				LookPath: func(string) (string, error) { return "/opt/bin/omp", nil },
				Getenv:   func(string) string { return "" },
			})

			got := runner.Preflight(context.Background(), port.HostProbeRequest{Model: "anthropic/claude-haiku-4-5"})
			if got.Ready || !got.Installed || got.MockExtensionVerified || got.Code != "mock_extension_invalid" || got.EvidenceSource != "omp_preflight" {
				t.Fatalf("mutated module preflight = %+v", got)
			}
		})
	}
}

func TestOmpRunnerPreflightKeepsInstalledEvidenceWhenVersionProbeFails(t *testing.T) {
	t.Parallel()

	runner := newTestOmpRunner("/opt/bin/issueops", ompTestLifecycleExtension("/opt/bin/issueops"), Dependencies{
		Process: &omoFakeProcess{run: func(_ context.Context, _ CommandRequest) (CommandOutput, error) {
			return CommandOutput{}, errors.New("version unavailable")
		}},
		LookPath: func(string) (string, error) { return "/opt/bin/omp", nil },
		Getenv:   func(string) string { return "" },
	})

	got := runner.Preflight(context.Background(), port.HostProbeRequest{Model: "anthropic/claude-haiku-4-5"})
	if got.Ready || !got.Installed || got.Code != "version_probe_failed" || got.EvidenceSource != "omp_preflight" {
		t.Fatalf("preflight = %+v", got)
	}
}

func TestOmpRunnerPreflightFailsClosedWhenPrivateRootCleanupFails(t *testing.T) {
	root := filepath.Join(t.TempDir(), "preflight")
	runner := newTestOmpRunner("/opt/bin/issueops", ompTestLifecycleExtension("/opt/bin/issueops"), Dependencies{
		Process: &omoFakeProcess{run: func(_ context.Context, _ CommandRequest) (CommandOutput, error) {
			return CommandOutput{Stdout: []byte("omp/18.8.4\n")}, nil
		}},
		LookPath: func(string) (string, error) { return "/opt/bin/omp", nil },
		TempDir: func(_, _ string) (string, error) {
			if err := os.Mkdir(root, 0o700); err != nil {
				return "", err
			}
			return root, nil
		},
		RemoveAll: func(path string) error {
			if path != root {
				t.Fatalf("cleanup path = %q", path)
			}
			return errors.New("sensitive cleanup detail")
		},
		Getenv: func(string) string { return "" },
	})

	got := runner.Preflight(context.Background(), port.HostProbeRequest{Model: "anthropic/claude-haiku-4-5"})
	if got.Ready || got.Code != "private_root_cleanup_failed" || got.Cause != "harness_environment" || got.EvidenceSource != "omp_preflight" {
		t.Fatalf("preflight = %+v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), root) || strings.Contains(string(encoded), "sensitive cleanup detail") {
		t.Fatalf("cleanup result leaked private details: %s", encoded)
	}
}

func TestOmpRunnerEpisodeFailsClosedWhenPrivateRootCleanupFails(t *testing.T) {
	root := filepath.Join(t.TempDir(), "episode")
	request := ompProbeRequest()
	runner := newTestOmpRunner("issueops", ompTestLifecycleExtension(), Dependencies{
		LookPath: func(string) (string, error) { return "/test/bin/omp", nil },
		TempDir: func(_, _ string) (string, error) {
			if err := os.Mkdir(root, 0o700); err != nil {
				return "", err
			}
			return root, nil
		},
		Process: &omoFakeProcess{run: func(context.Context, CommandRequest) (CommandOutput, error) {
			writeOmoCapture(t, filepath.Join(root, "result.json"), request)
			return CommandOutput{Stdout: ompSuccessfulStream(ompTestTarget)}, nil
		}},
		RemoveAll: func(path string) error {
			if path != root {
				t.Fatalf("cleanup path = %q", path)
			}
			return errors.New("sensitive cleanup detail")
		},
		Getenv: func(string) string { return "" },
	})

	got := runner.Run(context.Background(), request)
	if got.Completed || got.Code != "private_root_cleanup_failed" || got.Cause != "harness_environment" || got.EvidenceSource != "omp_runner" {
		t.Fatalf("episode = %+v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), root) || strings.Contains(string(encoded), "sensitive cleanup detail") {
		t.Fatalf("cleanup result leaked private details: %s", encoded)
	}
}

func TestOmpRunnerRunUsesIsolatedNativeProbeAndCapturesEvidence(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	episodeRoot := filepath.Join(parent, "episode")
	harnessBinary, err := filepath.Abs(filepath.Join("testdata", "issueops"))
	if err != nil {
		t.Fatal(err)
	}
	sourceAgentDir := t.TempDir()
	writeOmpAuthStore(t, filepath.Join(sourceAgentDir, "agent.db"))
	request := ompProbeRequest()
	lifecycleSource := ompTestLifecycleExtension(harnessBinary)
	var received CommandRequest
	process := &omoFakeProcess{run: func(_ context.Context, command CommandRequest) (CommandOutput, error) {
		received = command
		if command.Cwd != episodeRoot || command.Timeout != EpisodeTimeout || !command.OmoJSONL {
			t.Fatalf("command = %+v", command)
		}
		if !reflect.DeepEqual(command.Env, []string{
			"HOME=" + filepath.Join(episodeRoot, "home"),
			"OMP_MCP_REQUIRE_READY=1",
			"PATH=/test/bin",
			"PI_CODING_AGENT_DIR=" + filepath.Join(episodeRoot, "agent"),
			"USER=tester",
		}) {
			t.Fatalf("env = %#v", command.Env)
		}
		wantArgv := []string{
			"/test/bin/omp",
			"--mode", "json", "--print",
			"--model", request.Model,
			"--system-prompt", omoProbeSystemPrompt,
			"--no-session", "--session-dir", filepath.Join(episodeRoot, "sessions"),
			"--no-tools",
			"--extension", filepath.Join(episodeRoot, "issueops-lifecycle.js"),
			"--extension", filepath.Join(episodeRoot, "issueops-conformance.js"),
			"--no-extensions", "--no-skills", "--no-rules", "--no-lsp", "--no-title", "--auto-approve",
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
		assertOmoProbeGuard(t, filepath.Join(episodeRoot, "issueops-conformance.js"), ompTestTarget)
		assertOmpProbeProjectDoc(t, episodeRoot)
		assertOmpProbeConfig(t, filepath.Join(episodeRoot, "agent", "mcp.json"), harnessBinary, episodeRoot, request)
		assertOmpAuthSnapshot(t, filepath.Join(episodeRoot, "agent", "agent.db"))

		writeOmoCapture(t, filepath.Join(episodeRoot, "result.json"), request)
		return CommandOutput{Stdout: ompSuccessfulStream(ompTestTarget), ExitCode: 0}, nil
	}}
	now := time.Unix(100, 0)
	runner := newTestOmpRunner(filepath.Join("testdata", "issueops"), lifecycleSource, Dependencies{
		Process: process,
		LookPath: func(name string) (string, error) {
			if name != "omp" {
				t.Fatalf("looked up %q", name)
			}
			return "/test/bin/omp", nil
		},
		TempDir: func(_, pattern string) (string, error) {
			if pattern != "issueops-conformance-omp-" {
				t.Fatalf("temp pattern = %q", pattern)
			}
			if err := os.Mkdir(episodeRoot, 0o755); err != nil {
				return "", err
			}
			return episodeRoot, nil
		},
		Environ: func() []string {
			return []string{"PATH=/test/bin", "HOME=/private/home", "USER=tester", "OPENAI_API_KEY=secret", "PI_CODING_AGENT_DIR=/ambient/omp", "OMP_PROFILE=work"}
		},
		Getenv: func(name string) string {
			if name == "PI_CODING_AGENT_DIR" {
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
	if !result.Completed || result.Host != "omp" || result.ObservedModel != "claude-haiku-4-5" || result.DurationMS != 25 {
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
		t.Fatal("native omp process was not invoked")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{request.Prompt, "secret", "/private/home", "/ambient/omp", "captured"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("result retained %q: %s", forbidden, encoded)
		}
	}
	if _, err := os.Stat(episodeRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("episode root cleanup err=%v", err)
	}
}

func TestOmpRunnerRunFailsClosedOnStreamAndProcessFailures(t *testing.T) {
	t.Parallel()

	targetStart := `{"type":"tool_execution_start","toolCallId":"call-1","toolName":"` + ompTestTarget + `","args":{}}` + "\n"
	targetEnd := `{"type":"tool_execution_end","toolCallId":"call-1","toolName":"` + ompTestTarget + `","result":{"content":[{"type":"text","text":"captured"}]},"isError":false}` + "\n"
	tests := []struct {
		name       string
		output     CommandOutput
		processErr error
		wantCause  string
		wantCode   string
		wantExit   int
	}{
		{name: "malformed json", output: CommandOutput{Stdout: []byte("{\n")}, wantCause: "transport", wantCode: "host_stream_invalid"},
		{name: "missing call", output: CommandOutput{Stdout: []byte(`{"type":"session","version":3,"id":"s"}` + "\n")}, wantCause: "unknown", wantCode: "no_call"},
		{name: "missing observed model", output: CommandOutput{Stdout: []byte(`{"type":"session","version":3,"id":"s"}` + "\n" + targetStart + targetEnd)}, wantCause: "transport", wantCode: "host_stream_model_missing"},
		{name: "multiple calls", output: CommandOutput{Stdout: append(ompSuccessfulStream(ompTestTarget), []byte(strings.ReplaceAll(targetStart+targetEnd, "call-1", "call-2"))...)}, wantCause: "transport", wantCode: "host_stream_multiple_calls"},
		{name: "blocked by context guard", output: CommandOutput{Stdout: []byte(strings.Replace(string(ompSuccessfulStream(ompTestTarget)), `"isError":false`, `"isError":true`, 1))}, wantCause: "transport", wantCode: "host_stream_tool_error"},
		{name: "unavailable probe server", output: CommandOutput{ExitCode: 1}, processErr: errors.New("command_failed"), wantCause: "harness_environment", wantCode: "host_process_failed", wantExit: 1},
		{name: "timeout", output: CommandOutput{ExitCode: -1}, processErr: errors.New("command_timeout"), wantCause: "harness_environment", wantCode: "command_timeout", wantExit: -1},
		{name: "cancelled", output: CommandOutput{ExitCode: -1}, processErr: errors.New("command_cancelled"), wantCause: "harness_environment", wantCode: "command_cancelled", wantExit: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := ompProbeRequest()
			root := filepath.Join(t.TempDir(), "episode")
			runner := newTestOmpRunner("issueops", ompTestLifecycleExtension(), Dependencies{
				LookPath: func(string) (string, error) { return "/test/bin/omp", nil },
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
			if result.Completed || result.Cause != test.wantCause || result.Code != test.wantCode || result.ExitCode != test.wantExit || result.EvidenceSource != "omp_runner" {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestOmpRunnerRunRejectsCaptureCardinalityMismatch(t *testing.T) {
	t.Parallel()

	request := ompProbeRequest()
	root := filepath.Join(t.TempDir(), "episode")
	runner := newTestOmpRunner("issueops", ompTestLifecycleExtension(), Dependencies{
		LookPath: func(string) (string, error) { return "/test/bin/omp", nil },
		TempDir: func(_, _ string) (string, error) {
			if err := os.Mkdir(root, 0o700); err != nil {
				return "", err
			}
			return root, nil
		},
		Process: &omoFakeProcess{run: func(context.Context, CommandRequest) (CommandOutput, error) {
			writeOmoCapture(t, filepath.Join(root, "result.json"), request)
			sum := sha256.Sum256([]byte(request.RunToken))
			body, _ := json.Marshal(map[string]any{"call_count": 2, "run_token_sha256": hex.EncodeToString(sum[:])})
			if err := os.WriteFile(filepath.Join(root, "result.json.multiple"), body, 0o600); err != nil {
				t.Fatal(err)
			}
			return CommandOutput{Stdout: ompSuccessfulStream(ompTestTarget)}, nil
		}},
		Getenv: func(string) string { return "" },
	})

	result := runner.Run(context.Background(), request)
	if result.Completed || result.Cause != "transport" || result.Code != "probe_call_cardinality_invalid" {
		t.Fatalf("result = %+v", result)
	}
}

func TestOmpRunnerDerivesAmbientToolCountFromRejectedStream(t *testing.T) {
	request := ompProbeRequest()
	root := filepath.Join(t.TempDir(), "episode")
	stream := append(ompSuccessfulStream(ompTestTarget), []byte(
		`{"type":"tool_execution_start","toolCallId":"ambient-1","toolName":"read","args":{}}`+"\n"+
			`{"type":"tool_execution_end","toolCallId":"ambient-1","toolName":"read","result":{"content":[{"type":"text","text":"ambient"}]},"isError":false}`+"\n",
	)...)
	runner := newTestOmpRunner("issueops", ompTestLifecycleExtension(), Dependencies{
		LookPath: func(string) (string, error) { return "/test/bin/omp", nil },
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

func TestOmpProbeToolNameMirrorsOmpMCPNaming(t *testing.T) {
	for tool, want := range map[string]string{
		"harness_probe_empty_object": "mcp__issueops_probe_harness_probe_empty_object",
		"Issueops-Probe_Echo":        "mcp__issueops_probe_echo",
		"--Mixed..Case__tool--":      "mcp__issueops_probe_mixed_case_tool",
		"가나":                         "mcp__issueops_probe_tool",
	} {
		if got, ok := ompProbeToolName(tool); !ok || got != want {
			t.Errorf("ompProbeToolName(%q) = %q, %t; want %q", tool, got, ok, want)
		}
	}
	// omp caps names over 64 bytes with a Bun.hash suffix the runner cannot predict.
	if got, ok := ompProbeToolName(strings.Repeat("a", 45)); ok || len(got) != 65 {
		t.Fatalf("over-limit name = %q, %t", got, ok)
	}
}

func TestOmpRunnerRejectsProbeToolNameOmpWouldHash(t *testing.T) {
	process := &omoFakeProcess{}
	request := ompProbeRequest()
	request.ProbeTool = strings.Repeat("a", 45)
	runner := newTestOmpRunner("issueops", ompTestLifecycleExtension(), Dependencies{
		Process:  process,
		LookPath: func(string) (string, error) { return "/test/bin/omp", nil },
		Getenv:   func(string) string { return "" },
	})

	result := runner.Run(context.Background(), request)
	if result.Completed || result.Cause != "harness_environment" || result.Code != "probe_tool_name_too_long" || len(process.requests) != 0 {
		t.Fatalf("result = %+v requests=%d", result, len(process.requests))
	}
}

func TestResolveOmpAuthAcceptsOnlyPrivateRegularStore(t *testing.T) {
	home := t.TempDir()
	agentDir := filepath.Join(home, ".omp", "agent")
	if err := os.MkdirAll(agentDir, 0o700); err != nil {
		t.Fatal(err)
	}
	deps := normalizeDependencies(Dependencies{Getenv: func(name string) string {
		if name == "HOME" {
			return home
		}
		return ""
	}})
	store := filepath.Join(agentDir, "agent.db")

	if source, err := resolveOmpAuth(deps); err != nil || source != "" {
		t.Fatalf("missing store = %q, %v", source, err)
	}
	if err := snapshotOmpAuth(context.Background(), "", filepath.Join(t.TempDir(), "agent.db")); err != nil {
		t.Fatalf("missing store snapshot: %v", err)
	}

	target := filepath.Join(home, "real-agent.db")
	writeOmpAuthStore(t, target)
	if err := os.Symlink(target, store); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveOmpAuth(deps); err == nil {
		t.Fatal("symlinked credential store was accepted")
	}
	if err := os.Remove(store); err != nil {
		t.Fatal(err)
	}
	writeOmpAuthStore(t, store)
	if err := os.Chmod(store, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveOmpAuth(deps); err == nil {
		t.Fatal("group-readable credential store was accepted")
	}
	if err := os.Chmod(store, 0o600); err != nil {
		t.Fatal(err)
	}
	if source, err := resolveOmpAuth(deps); err != nil || source != store {
		t.Fatalf("private store = %q, %v", source, err)
	}
}

const ompTestTarget = "mcp__issueops_probe_harness_probe_empty_object"

func ompProbeRequest() port.HostProbeRequest {
	request := omoProbeRequest()
	request.Model = "anthropic/claude-haiku-4-5"
	return request
}

func ompTestLifecycleExtension(harnessBinary ...string) string {
	binary := "issueops"
	if len(harnessBinary) > 0 {
		binary = harnessBinary[0]
	}
	absolute, _ := filepath.Abs(binary)
	return hostprotocol.OmpLifecycleExtension(absolute)
}

func newTestOmpRunner(binary, source string, deps Dependencies) OmpRunner {
	return NewOmpRunner(binary, source, deps, hostprotocol.OmpLifecycleExtension)
}

// ompSuccessfulStream follows the event shapes recorded from omp 18.8.4 --mode json.
func ompSuccessfulStream(targetTool string) []byte {
	return []byte(strings.Join([]string{
		`{"type":"session","version":3,"id":"session-1","timestamp":"2026-10-08T00:00:00.000Z","cwd":"/episode"}`,
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"message_start","message":{"role":"assistant","content":[],"api":"anthropic-messages","provider":"anthropic","model":"claude-haiku-4-5"}}`,
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"toolCall","id":"call-1","name":"` + targetTool + `","arguments":{}}],"provider":"anthropic","model":"claude-haiku-4-5","stopReason":"toolUse"}}`,
		`{"type":"tool_execution_start","toolCallId":"call-1","toolName":"` + targetTool + `","args":{}}`,
		`{"type":"tool_execution_end","toolCallId":"call-1","toolName":"` + targetTool + `","result":{"content":[{"type":"text","text":"captured"}],"details":{"serverName":"issueops_probe","mcpToolName":"harness_probe_empty_object","provider":"native","providerName":"OMP"}},"isError":false}`,
		`{"type":"turn_end","message":{"role":"assistant","content":[]},"toolResults":[]}`,
		`{"type":"agent_end","messages":[],"isTerminal":true,"yielded":true}`,
	}, "\n") + "\n")
}

// writeOmpAuthStore creates a credential store with omp's auth schema plus rows
// and tables the episode snapshot must not carry.
func writeOmpAuthStore(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		`CREATE TABLE auth_schema_version (id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL)`,
		`CREATE TABLE auth_credentials (id INTEGER PRIMARY KEY AUTOINCREMENT, provider TEXT NOT NULL, credential_type TEXT NOT NULL, data TEXT NOT NULL, disabled_cause TEXT DEFAULT NULL, identity_key TEXT DEFAULT NULL, created_at INTEGER NOT NULL DEFAULT 1, updated_at INTEGER NOT NULL DEFAULT 1)`,
		`CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`INSERT INTO auth_schema_version VALUES (1, 8)`,
		`INSERT INTO auth_credentials (provider, credential_type, data, identity_key) VALUES ('zai', 'api_key', '{"key":"api-secret","source":"login"}', 'zai-key')`,
		`INSERT INTO auth_credentials (provider, credential_type, data) VALUES ('mcp_oauth:profile:default:https://mcp.example/mcp', 'oauth', '{"access":"mcp-access","refresh":"mcp-refresh","expires":1}')`,
		`INSERT INTO auth_credentials (provider, credential_type, data, identity_key) VALUES ('anthropic', 'oauth', '{"access":"access-secret","refresh":"refresh-secret","expires":4102444800000,"email":"user@example.test"}', 'anthropic-user')`,
		`INSERT INTO auth_credentials (provider, credential_type, data, disabled_cause) VALUES ('openai', 'api_key', '{"key":"revoked-secret"}', 'revoked')`,
		`INSERT INTO settings VALUES ('theme', 'ambient')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertOmpAuthSnapshot(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("auth snapshot info=%v err=%v", info, err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tables := queryOmpStrings(t, db, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if !reflect.DeepEqual(tables, []string{"auth_credentials", "auth_schema_version"}) {
		t.Fatalf("snapshot tables = %q", tables)
	}
	if version := queryOmpStrings(t, db, `SELECT version FROM auth_schema_version`); !reflect.DeepEqual(version, []string{"8"}) {
		t.Fatalf("schema version = %q", version)
	}
	rows := queryOmpStrings(t, db, `SELECT provider || '|' || credential_type || '|' || ifnull(identity_key, '') || '|' || data FROM auth_credentials ORDER BY id`)
	want := []string{
		`zai|api_key|zai-key|{"key":"api-secret","source":"login"}`,
		`anthropic|oauth|anthropic-user|{"access":"access-secret","refresh":"","expires":4102444800000,"email":"user@example.test"}`,
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("snapshot credentials = %q", rows)
	}
}

func queryOmpStrings(t *testing.T, db *sql.DB, query string) []string {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return values
}

func assertOmpProbeProjectDoc(t *testing.T, root string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, ".issueops", "CONSTITUTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Omp conformance context") {
		t.Fatalf("probe project doc = %q", body)
	}
}

func assertOmpProbeConfig(t *testing.T, path, harnessBinary, episodeRoot string, request port.HostProbeRequest) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config info=%v err=%v", info, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]map[string]ompMCPServer
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"mcpServers"}) || len(config["mcpServers"]) != 1 {
		t.Fatalf("MCP config = %s", body)
	}
	want := ompMCPServer{
		Type: "stdio", Command: harnessBinary, Cwd: episodeRoot,
		Args: []string{"contract", "conformance", "serve", "--fixture-id", request.FixtureID, "--result-file", filepath.Join(episodeRoot, "result.json"), "--run-token", request.RunToken},
	}
	if got := config["mcpServers"]["issueops_probe"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("server = %#v", got)
	}
}

var _ port.HostProbeRunner = OmpRunner{}
