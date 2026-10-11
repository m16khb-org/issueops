package installcli

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/port"
	activationport "issueops/internal/port/nativeactivation"
)

// runInstallStub builds a Command whose installer must never run: every case
// below has to be rejected before any host file is planned or written.
func runInstallStub(t *testing.T, mutate func(root string, deps *Deps)) Command {
	t.Helper()
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("ISSUEOPS_NATIVE_ACTIVATION_STEP", "")
	t.Setenv("ISSUEOPS_NATIVE_ACTIVATION_TRANSITION_ID", "")
	deps := Deps{
		IssueOpsRoot:   func() string { return root },
		ExecutablePath: func() (string, error) { return filepath.Join(root, "bin", "issueops"), nil },
		NativeInstallRequest: func(root, home, codexHome, binPath string) port.NativeInstallRequest {
			return port.NativeInstallRequest{Root: root, Home: home, CodexHome: codexHome, BinPath: binPath}
		},
		InstallNative: func(port.NativeInstallRequest) (port.NativeInstallResult, error) {
			t.Error("installer must not run for a rejected configuration")
			return port.NativeInstallResult{}, nil
		},
		ActivationBackend:  &activationBackendFixture{},
		ActivationReadback: func(port.NativeInstallRequest) activationport.ReadbackVerifier { return nil },
	}
	if mutate != nil {
		mutate(root, &deps)
	}
	return testInstallCommand(t, deps)
}

func TestRunInstallRejectsBadConfigurationBeforeTouchingHosts(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		env     map[string]string
		mutate  func(root string, deps *Deps)
		wantErr string
	}{
		{name: "unknown flag", args: []string{"--no-such-flag"}, wantErr: "no-such-flag"},
		{name: "no home directory", args: []string{"--dry-run"}, env: map[string]string{"HOME": ""}, wantErr: "determine user home directory"},
		{name: "invalid path mode", args: []string{"--dry-run", "--path-mode=sometimes"}, wantErr: `invalid --path-mode "sometimes"`},
		{name: "invalid mcp transport", args: []string{"--dry-run", "--mcp-transport=smoke"}, wantErr: `invalid --mcp-transport "smoke"`},
		{name: "http without service setup", args: []string{"--dry-run", "--mcp-transport=http"}, wantErr: "mcp http service setup is not configured"},
		{name: "missing request builder", args: []string{"--dry-run"}, mutate: func(_ string, d *Deps) { d.NativeInstallRequest = nil }, wantErr: "native installer is not configured"},
		{name: "missing installer", args: []string{"--dry-run"}, mutate: func(_ string, d *Deps) { d.InstallNative = nil }, wantErr: "native installer is not configured"},
		{name: "missing executable inspector", args: []string{"--dry-run"}, mutate: func(_ string, d *Deps) { d.ExecutablePath = nil }, wantErr: "executable inspector is unavailable"},
		{name: "executable inspection failure", args: []string{"--dry-run"}, mutate: func(_ string, d *Deps) {
			d.ExecutablePath = func() (string, error) { return "", errors.New("no /proc") }
		}, wantErr: "inspect native install candidate: no /proc"},
		{name: "apply from a non canonical binary", args: []string{"--path-mode=skip"}, mutate: func(root string, d *Deps) {
			d.ExecutablePath = func() (string, error) { return filepath.Join(t.TempDir(), "issueops"), nil }
		}, wantErr: "canonical target or a same-directory staged binary"},
		{name: "apply without activation backend", args: []string{"--path-mode=skip"}, mutate: func(_ string, d *Deps) { d.ActivationBackend = nil }, wantErr: "native activation backend is unavailable"},
		{name: "step during dry run", args: []string{"--dry-run"}, env: map[string]string{"ISSUEOPS_NATIVE_ACTIVATION_STEP": "begin"}, wantErr: "not valid during dry-run"},
		{name: "unknown step", args: []string{"--path-mode=skip"}, env: map[string]string{"ISSUEOPS_NATIVE_ACTIVATION_STEP": "commit"}, wantErr: `invalid native activation step "commit"`},
		{name: "missing readback", args: []string{"--dry-run"}, mutate: func(_ string, d *Deps) { d.ActivationReadback = nil }, wantErr: "native activation readback is not configured"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cli := runInstallStub(t, tc.mutate)
			for key, value := range tc.env {
				t.Setenv(key, value)
			}
			_, _, err := captureInstallCommandOutput(t, nil, func() error { return cli.RunInstall(tc.args) })
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestRunInstallDryRunDoesNotNeedActivationBackend(t *testing.T) {
	var seen port.NativeInstallRequest
	cli := runInstallStub(t, func(_ string, d *Deps) {
		d.ActivationBackend = nil
		d.InstallNative = func(req port.NativeInstallRequest) (port.NativeInstallResult, error) {
			seen = req
			return port.NativeInstallResult{OK: true, DryRun: req.DryRun, ProjectLocal: req.ProjectLocal}, nil
		}
	})
	t.Setenv("CODEX_HOME", "")

	out, _, err := captureInstallCommandOutput(t, nil, func() error {
		return cli.RunInstall([]string{"--dry-run", "--path-mode=skip", "--project-local", "--json"})
	})
	if err != nil {
		t.Fatalf("dry-run install failed: %v\n%s", err, out)
	}
	if !seen.DryRun || !seen.ProjectLocal || seen.MCPTransport != "stdio" || filepath.Base(seen.CodexHome) != ".codex" {
		t.Fatalf("request = %+v, want dry-run project-local stdio request with the default codex home", seen)
	}
}

func TestCodexHomeDirPrefersEnvironmentOverride(t *testing.T) {
	t.Setenv("CODEX_HOME", "/custom/codex")
	if got := codexHomeDir("/home/u"); got != "/custom/codex" {
		t.Fatalf("codexHomeDir with override = %q", got)
	}
	t.Setenv("CODEX_HOME", "")
	if got := codexHomeDir("/home/u"); got != filepath.Join("/home/u", ".codex") {
		t.Fatalf("codexHomeDir default = %q", got)
	}
}

func TestNativeInstallCandidatePathAcceptsTargetAndSameDirectoryStagedBinaryOnly(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "bin", "issueops")
	at := func(path string) func() (string, error) {
		return func() (string, error) { return path, nil }
	}

	if got, err := nativeInstallCandidatePath(target, false, at(filepath.Join(dir, "bin", "..", "bin", "issueops"))); err != nil || got != target {
		t.Fatalf("canonical candidate = %q, %v", got, err)
	}
	staged := filepath.Join(dir, "bin", ".issueops.activate-x")
	if got, err := nativeInstallCandidatePath(target, false, at(staged)); err != nil || got != staged {
		t.Fatalf("staged sibling candidate = %q, %v", got, err)
	}
	if _, err := nativeInstallCandidatePath(target, false, at(filepath.Join(dir, "bin", "other"))); err == nil {
		t.Fatal("an unprefixed same-directory binary must be refused")
	}
	if got, err := nativeInstallCandidatePath(target, true, at(filepath.Join(dir, "other", "issueops"))); err != nil || got == "" {
		t.Fatalf("dry-run must not constrain the candidate: %q, %v", got, err)
	}
}

func TestExecuteInstallReportsActivationTransitionsInTextAndJSON(t *testing.T) {
	cases := []struct {
		name      string
		step      string
		args      []string
		wantOut   []string
		wantCalls []string
	}{
		{"abort text", "abort", []string{"--path-mode=skip"}, []string{"native activation transition " + validTransitionID + " aborted"}, []string{"abort"}},
		{"abort json", "abort", []string{"--path-mode=skip", "--json"}, []string{`"aborted": true`, validTransitionID}, []string{"abort"}},
		{"begin text", "begin", []string{"--path-mode=skip", "--adopt-command-file"}, []string{"native activation candidate " + hashFixture('a') + " is pending as transition " + validTransitionID}, []string{"begin"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, home, _, _ := installOrchestrationFixture(t)
			backend := &activationBackendFixture{}
			cli := installOrchestrationCommand(t, root, home, backend, installerFixture{})
			t.Setenv("ISSUEOPS_NATIVE_ACTIVATION_STEP", tc.step)
			t.Setenv("ISSUEOPS_NATIVE_ACTIVATION_TRANSITION_ID", validTransitionID)

			out, _, err := captureInstallCommandOutput(t, nil, func() error { return cli.RunInstall(tc.args) })
			if err != nil {
				t.Fatalf("install failed: %v\n%s", err, out)
			}
			for _, want := range tc.wantOut {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q:\n%s", want, out)
				}
			}
			if strings.Join(backend.calls, ",") != strings.Join(tc.wantCalls, ",") {
				t.Fatalf("backend calls = %v, want %v", backend.calls, tc.wantCalls)
			}
		})
	}
}

func TestOutputInstallResultAlwaysReturnsTheInstallError(t *testing.T) {
	boom := errors.New("install exploded")
	for _, jsonOut := range []bool{false, true} {
		var gotErr error
		out, _, err := captureInstallCommandOutput(t, nil, func() error {
			gotErr = outputInstallResult(port.NativeInstallResult{Root: "/r", BinPath: "/r/bin/issueops", DryRun: true}, boom, jsonOut)
			return nil
		})
		if err != nil || !errors.Is(gotErr, boom) {
			t.Fatalf("json=%v: returned %v, want the original error", jsonOut, gotErr)
		}
		want := "Dry-run plan for issueops native integrations:"
		if jsonOut {
			want = `"bin_path": "/r/bin/issueops"`
		}
		if !strings.Contains(out, want) {
			t.Fatalf("json=%v output missing %q:\n%s", jsonOut, want, out)
		}
	}
}

func TestInstallEffectsRemoveRetiredStateAppendsMessagesForTheStateDirectory(t *testing.T) {
	var gotDir string
	var gotDry bool
	effects := installTransactionEffects{command: Command{Deps: Deps{
		StateRoot: filepath.Join("/state", "issueops"),
		RemoveRetiredState: func(dir string, dry bool) []string {
			gotDir, gotDry = dir, dry
			return []string{"removed a", "removed b"}
		},
	}}}
	result := port.NativeInstallResult{Messages: []string{"existing"}}

	effects.RemoveRetiredState(&result, true)

	if gotDir != "/state" || !gotDry || strings.Join(result.Messages, "|") != "existing|removed a|removed b" {
		t.Fatalf("dir=%q dry=%v messages=%v", gotDir, gotDry, result.Messages)
	}
	unconfigured := installTransactionEffects{command: Command{}}
	unconfigured.RemoveRetiredState(&result, false)
	if len(result.Messages) != 3 {
		t.Fatalf("an unconfigured cleanup must leave messages alone: %v", result.Messages)
	}
}
