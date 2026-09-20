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

	"issueops/internal/port"
)

type codexFakeProcess struct {
	requests []CommandRequest
	run      func(context.Context, CommandRequest) (CommandOutput, error)
}

func (f *codexFakeProcess) Run(ctx context.Context, request CommandRequest) (CommandOutput, error) {
	request.Argv = append([]string(nil), request.Argv...)
	request.Env = append([]string(nil), request.Env...)
	f.requests = append(f.requests, request)
	if f.run == nil {
		return CommandOutput{}, nil
	}
	return f.run(ctx, request)
}

func TestCodexRunnerPreflightUsesSharedExecutableCheck(t *testing.T) {
	t.Parallel()

	process := &codexFakeProcess{run: func(_ context.Context, request CommandRequest) (CommandOutput, error) {
		want := []string{"/opt/bin/codex", "--version"}
		if !reflect.DeepEqual(request.Argv, want) {
			t.Errorf("argv = %#v, want %#v", request.Argv, want)
		}
		if !reflect.DeepEqual(request.Env, []string{"PATH=/opt/bin"}) {
			t.Errorf("env = %#v", request.Env)
		}
		return CommandOutput{Stdout: []byte("codex 1.2.3\n")}, nil
	}}
	runner := NewCodexRunner("/opt/bin/issueops", Dependencies{
		Process:  process,
		LookPath: func(string) (string, error) { return "/opt/bin/codex", nil },
		Environ:  func() []string { return []string{"PATH=/opt/bin"} },
	})

	result := runner.Preflight(context.Background(), port.HostProbeRequest{Model: "default"})
	if !result.Ready || result.Version != "codex 1.2.3" {
		t.Fatalf("preflight = %+v", result)
	}
	if result.Host != "codex" || result.RequestedModel != "default" || result.ObservedModel != "" {
		t.Fatalf("preflight metadata = %+v", result)
	}
}

func TestCodexRunnerPreflightMissingExecutable(t *testing.T) {
	t.Parallel()

	runner := NewCodexRunner("/opt/bin/issueops", Dependencies{
		LookPath: func(string) (string, error) { return "", errors.New("not found") },
	})

	result := runner.Preflight(context.Background(), port.HostProbeRequest{})
	if result.Ready || result.Cause != "harness_environment" || result.Code != "executable_not_found" || result.EvidenceSource != "codex_preflight" {
		t.Fatalf("preflight = %+v", result)
	}
}

func TestCodexRunnerRunUsesIsolatedProbe(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	harnessRelative := filepath.Join("testdata", "issueops")
	harnessBinary, err := filepath.Abs(harnessRelative)
	if err != nil {
		t.Fatal(err)
	}
	codexBinary := filepath.Join(parent, "codex")
	var episodeRoot string
	process := &codexFakeProcess{run: func(_ context.Context, request CommandRequest) (CommandOutput, error) {
		episodeRoot = request.Cwd
		if request.Cwd == parent || !strings.HasPrefix(filepath.Base(request.Cwd), "issueops-conformance-codex-") {
			t.Errorf("cwd = %q, want a fresh Codex episode root", request.Cwd)
		}
		info, err := os.Stat(request.Cwd)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Errorf("episode root mode = %#o, want 0700", info.Mode().Perm())
		}
		resultPath := filepath.Join(request.Cwd, "result.json")
		serve := []string{
			"contract", "conformance", "serve",
			"--fixture-id", "fixture",
			"--result-file", resultPath,
			"--run-token", "run-token",
		}
		encodedServe, err := json.Marshal(serve)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			codexBinary,
			"exec",
			"--ignore-user-config",
			"--dangerously-bypass-hook-trust",
			"--ignore-rules",
			"--ephemeral",
			"--json",
			"--sandbox", "read-only",
			"--skip-git-repo-check",
			"-C", request.Cwd,
			"-c", "approval_policy=" + jsonString("never"),
			"-c", "mcp_servers.issueops_probe.command=" + jsonString(harnessBinary),
			"-c", "mcp_servers.issueops_probe.args=" + string(encodedServe),
			"-c", "mcp_servers.issueops_probe.default_tools_approval_mode=" + jsonString("approve"),
			"--model", "gpt-5",
			"call the probe",
		}
		if !reflect.DeepEqual(request.Argv, want) {
			t.Errorf("argv = %#v\nwant %#v", request.Argv, want)
		}
		runtimeHome := environmentValue(t, request.Env, "CODEX_HOME")
		observationPath := environmentValue(t, request.Env, "ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE")
		if runtimeHome != filepath.Join(request.Cwd, "codex-home") || environmentValue(t, request.Env, "ISSUEOPS_CHILD_SMOKE_HOOKS") != "1" || environmentValue(t, request.Env, "PATH") != "/opt/bin" {
			t.Errorf("env = %#v", request.Env)
		}
		if request.Timeout != EpisodeTimeout {
			t.Errorf("timeout = %s, want %s", request.Timeout, EpisodeTimeout)
		}
		if countCodexProbeOverrides(request.Argv) != 3 {
			t.Errorf("probe overrides = %d, want 3", countCodexProbeOverrides(request.Argv))
		}
		writeCodexCapture(t, resultPath, "run-token")
		writeChildSmokeHookMarkers(t, observationPath, "gpt-5")
		return CommandOutput{Stdout: codexSuccessfulStream("gpt-5"), Stderr: []byte("not persisted")}, nil
	}}
	runner := NewCodexRunner(harnessRelative, Dependencies{
		Process:  process,
		LookPath: func(string) (string, error) { return codexBinary, nil },
		TempDir: func(_, pattern string) (string, error) {
			return os.MkdirTemp(parent, pattern)
		},
		Environ: func() []string {
			return []string{"PATH=/opt/bin", "TOKEN=secret", "ZAI_API_KEY=gjc-secret", "CODEX_HOME=/private/codex"}
		},
		Now: func() time.Time { return time.Unix(1, 0) },
	})

	result := runner.Run(context.Background(), codexRequest("gpt-5"))
	if !result.Completed || result.Host != "codex" || result.AmbientToolCount != 1 || result.CallCount != 1 {
		t.Fatalf("result = %+v", result)
	}
	if result.Cause != "" || result.Code != "" || result.CanonicalArgumentsJSON != `{}` {
		t.Fatalf("result leaked failure/output state: %+v", result)
	}
	if len(process.requests) != 1 {
		t.Fatalf("process calls = %d, want 1", len(process.requests))
	}
	if _, err := os.Stat(episodeRoot); !os.IsNotExist(err) {
		t.Fatalf("episode root remains after run: %q, err=%v", episodeRoot, err)
	}
}

func TestCodexRunnerNormalLiveCollectsCanonicalRuntimeEvidenceWithoutActivatedHooks(t *testing.T) {
	rootParent := t.TempDir()
	sourceHome := filepath.Join(rootParent, "source-codex-home")
	if err := os.Mkdir(sourceHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceHome, "auth.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	harness, err := filepath.Abs(filepath.Join("testdata", "issueops"))
	if err != nil {
		t.Fatal(err)
	}
	request := codexRequest("gpt-5")
	request.HostVersion = "codex 1.2.3"
	var episodeRoot string
	now := time.Unix(100, 0)
	process := &codexFakeProcess{run: func(_ context.Context, command CommandRequest) (CommandOutput, error) {
		episodeRoot = command.Cwd
		observationPath := environmentValue(t, command.Env, "ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE")
		runtimeHome := environmentValue(t, command.Env, "CODEX_HOME")
		if runtimeHome == sourceHome || runtimeHome != filepath.Join(command.Cwd, "codex-home") {
			t.Fatalf("runtime CODEX_HOME = %q", runtimeHome)
		}
		assertProjectedCodexSmokeHooks(t, filepath.Join(runtimeHome, "hooks.json"), observationPath, harness, "codex")
		if !containsArgument(command.Argv, "--ignore-user-config") || !containsArgument(command.Argv, "--dangerously-bypass-hook-trust") {
			t.Fatalf("normal live argv does not isolate config and trust canonical hooks: %q", command.Argv)
		}
		writeCodexCapture(t, filepath.Join(command.Cwd, "result.json"), request.RunToken)
		writeChildSmokeHookMarkers(t, observationPath, request.Model)
		stream := strings.Join([]string{
			`{"type":"thread.started","thread_id":"thread-probe"}`,
			`{"type":"item.completed","item":{"type":"mcp_tool_call","id":"target","server":"issueops_probe","status":"completed","result":{"content":"captured"}}}`,
		}, "\n") + "\n"
		return CommandOutput{Stdout: []byte(stream), ExitCode: 0}, nil
	}}
	runner := NewCodexRunner(harness, Dependencies{
		Process:  process,
		LookPath: func(string) (string, error) { return filepath.Join(rootParent, "codex"), nil },
		TempDir:  func(_, pattern string) (string, error) { return os.MkdirTemp(rootParent, pattern) },
		Getenv: func(name string) string {
			if name == "CODEX_HOME" {
				return sourceHome
			}
			return ""
		},
		Environ: func() []string { return []string{"PATH=/opt/bin", "CODEX_HOME=" + sourceHome, "SECRET=redacted"} },
		Now: func() time.Time {
			now = now.Add(25 * time.Millisecond)
			return now
		},
	})

	got := runner.Run(context.Background(), request)
	if !got.Completed || got.HostVersion != request.HostVersion || got.ObservedModel != request.Model || got.DurationMS != 25 ||
		!got.SessionStartObserved || got.PreToolUseObserved || got.AmbientToolCount != 1 || got.CallCount != 1 || !validSHA256(got.ResponseSHA256) || got.ExitCode != 0 {
		t.Fatalf("normal live result = %+v", got)
	}
	if _, err := os.Stat(episodeRoot); !os.IsNotExist(err) {
		t.Fatalf("episode root cleanup err = %v", err)
	}
}

func TestCodexRunnerCleanupFailureSuppressesSuccessfulEpisode(t *testing.T) {
	parent := t.TempDir()
	request := codexRequest("gpt-5")
	request.HostVersion = "codex 1.2.3"
	root := filepath.Join(parent, "episode")
	runner := NewCodexRunner("issueops", Dependencies{
		TempDir: func(_, _ string) (string, error) {
			if err := os.Mkdir(root, 0o700); err != nil {
				return "", err
			}
			return root, nil
		},
		LookPath: func(string) (string, error) { return filepath.Join(parent, "codex"), nil },
		RemoveAll: func(path string) error {
			if path != root {
				t.Fatalf("cleanup path = %q", path)
			}
			return errors.New("sensitive cleanup detail")
		},
		Process: &codexFakeProcess{run: func(_ context.Context, command CommandRequest) (CommandOutput, error) {
			writeCodexCapture(t, filepath.Join(command.Cwd, "result.json"), request.RunToken)
			writeChildSmokeHookMarkers(t, environmentValue(t, command.Env, "ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE"), request.Model)
			return CommandOutput{Stdout: codexSuccessfulStream(request.Model)}, nil
		}},
		Now: func() time.Time { return time.Unix(100, 0) },
	})

	got := runner.Run(context.Background(), request)
	if got.Completed || got.Code != "private_root_cleanup_failed" || got.Cause != "harness_environment" || got.EvidenceSource != "codex_runner" {
		t.Fatalf("cleanup result = %+v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), root) || strings.Contains(string(encoded), "sensitive cleanup detail") {
		t.Fatalf("cleanup result leaked private details: %s", encoded)
	}
}

func containsArgument(argv []string, wanted string) bool {
	for _, arg := range argv {
		if arg == wanted {
			return true
		}
	}
	return false
}

func TestCodexRunnerRunModelSelection(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		model     string
		wantModel []string
	}{
		{name: "empty"},
		{name: "default", model: "default"},
		{name: "override", model: "gpt-5", wantModel: []string{"--model", "gpt-5"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent := t.TempDir()
			process := &codexFakeProcess{run: func(_ context.Context, request CommandRequest) (CommandOutput, error) {
				observedModel := test.model
				if observedModel == "" || observedModel == "default" {
					observedModel = "gpt-default"
				}
				for i, arg := range request.Argv {
					if arg != "--model" {
						continue
					}
					if i+1 >= len(request.Argv) {
						t.Fatal("model flag missing value")
					}
					if !reflect.DeepEqual([]string{arg, request.Argv[i+1]}, test.wantModel) {
						t.Errorf("model args = %#v, want %#v", []string{arg, request.Argv[i+1]}, test.wantModel)
					}
					writeCodexCapture(t, filepath.Join(request.Cwd, "result.json"), "run-token")
					writeChildSmokeHookMarkers(t, environmentValue(t, request.Env, "ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE"), observedModel)
					return CommandOutput{Stdout: codexSuccessfulStream(observedModel)}, nil
				}
				if len(test.wantModel) != 0 {
					t.Errorf("model args missing, want %#v", test.wantModel)
				}
				writeCodexCapture(t, filepath.Join(request.Cwd, "result.json"), "run-token")
				writeChildSmokeHookMarkers(t, environmentValue(t, request.Env, "ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE"), observedModel)
				return CommandOutput{Stdout: codexSuccessfulStream(observedModel)}, nil
			}}
			runner := NewCodexRunner(filepath.Join(parent, "issueops"), Dependencies{
				Process:  process,
				LookPath: func(string) (string, error) { return filepath.Join(parent, "codex"), nil },
				TempDir:  os.MkdirTemp,
			})

			result := runner.Run(context.Background(), codexRequest(test.model))
			if !result.Completed {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestCodexRunnerRunMissingCaptureIsTransportFailure(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	runner := NewCodexRunner(filepath.Join(parent, "issueops"), Dependencies{
		Process:  &codexFakeProcess{},
		LookPath: func(string) (string, error) { return filepath.Join(parent, "codex"), nil },
		TempDir:  os.MkdirTemp,
	})

	result := runner.Run(context.Background(), codexRequest("default"))
	if result.Completed || result.Cause != "unknown" || result.Code != "no_call" || result.EvidenceSource != "codex_runner" {
		t.Fatalf("result = %+v", result)
	}
}

func codexRequest(model string) port.HostProbeRequest {
	return port.HostProbeRequest{
		HarnessBinary: "/ignored/by-constructor",
		FixtureID:     "fixture",
		ProbeTool:     "harness_probe_empty_object",
		SchemaSHA256:  "schema",
		Prompt:        "call the probe",
		Model:         model,
		Profile:       "clean",
		Attempt:       1,
		RunToken:      "run-token",
	}
}

func countCodexProbeOverrides(argv []string) int {
	count := 0
	for i, arg := range argv {
		if arg == "-c" && i+1 < len(argv) && strings.HasPrefix(argv[i+1], "mcp_servers."+codexProbeServer+".") {
			count++
		}
	}
	return count
}

func writeCodexCapture(t *testing.T, path, runToken string) {
	t.Helper()
	tokenSum := sha256.Sum256([]byte(runToken))
	rawSum := sha256.Sum256([]byte(`{}`))
	data, err := json.Marshal(map[string]any{
		"fixture_id":          "fixture",
		"call_count":          1,
		"raw_sha256":          hex.EncodeToString(rawSum[:]),
		"canonical_arguments": map[string]any{},
		"schema_sha256":       "schema",
		"run_token_sha256":    hex.EncodeToString(tokenSum[:]),
		"classification":      "exact_valid",
		"advertised_valid":    true,
		"canonical_valid":     true,
		"diagnostics":         []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func codexSuccessfulStream(model string) []byte {
	return []byte(strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-probe","model":` + jsonString(model) + `}`,
		`{"type":"item.completed","item":{"type":"mcp_tool_call","id":"target","server":"issueops_probe","status":"completed","result":{"content":"captured"}}}`,
	}, "\n") + "\n")
}
