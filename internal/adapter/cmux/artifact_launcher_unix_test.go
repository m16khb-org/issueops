//go:build darwin || linux

package cmux

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cmuxcontract "issueops/internal/contract/cmux"
)

// The private launcher is a /bin/sh script driven through cmux, which runs
// only on macOS and Linux (cmux is unsupported on windows).

func TestPrivateLauncherPreservesPromptAndExactHostArgv(t *testing.T) {
	root := canonicalTempDir(t)
	worktree := filepath.Join(root, "worktree")
	if err := os.Mkdir(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "interpolated")
	prompt := strings.Repeat("long-line\\value\n", 1024) + "'single' \"double\" $(touch " + marker + ") `touch " + marker + "` \\n literal\n\n"
	for _, test := range []struct {
		host, model, effort string
		wantPrefix          []string
	}{
		{host: "codex", model: "gpt-6-astra", effort: "high", wantPrefix: []string{"--model", "gpt-6-astra", "-c", "model_reasoning_effort=high", "--dangerously-bypass-approvals-and-sandbox", "--"}},
		{host: "claude", model: "claude-sonnet-5-5", effort: "high", wantPrefix: []string{"--model", "claude-sonnet-5-5", "--effort", "high", "--dangerously-skip-permissions", "--"}},
		{host: "omo", model: "chatgpt-subscription/gpt-6-sol", effort: "xhigh", wantPrefix: []string{"--model", "chatgpt-subscription/gpt-6-sol:xhigh", "--permission-preset", "full-access", "--"}},
		{host: "omp", model: "anthropic/claude-opus-5-5", effort: "high", wantPrefix: []string{"--model", "anthropic/claude-opus-5-5", "--thinking", "high", "--auto-approve", "--"}},
		// /usr/bin/env passes the model in OPENCODE_CONFIG_CONTENT, so the host sees only its own flags.
		{host: "opencode", model: "opencode/deepseek-v4.1-flash-free", effort: "max", wantPrefix: []string{"--standalone", "--auto", "--prompt"}},
	} {
		t.Run(test.host, func(t *testing.T) {
			capture := filepath.Join(root, test.host+"-argv")
			host := filepath.Join(root, test.host)
			hostSource := "#!/bin/sh\nprintf '%s\\000' \"$@\" > \"$HOST_CAPTURE\"\n"
			if err := os.WriteFile(host, []byte(hostSource), 0o700); err != nil {
				t.Fatal(err)
			}
			prepared, err := prepareTestLauncher(cmuxcontract.ArtifactRequest{
				Root: filepath.Join(root, "artifacts"), CWD: worktree,
				WindowID: testWindow, WorkspaceID: testWorkspace, SurfaceID: testSurface, SocketPath: socketPath,
				Host: test.host, HostExecutable: host, Model: test.model, Effort: test.effort,
				Prompt: []byte(prompt), PromptSHA256: digestBytes([]byte(prompt)), MaterialSHA256: strings.Repeat("b", 64),
			})
			if err != nil {
				t.Fatal(err)
			}
			assertFileMode(t, prepared.Directory, 0o700)
			assertFileMode(t, prepared.PromptPath, 0o600)
			assertFileMode(t, prepared.LauncherPath, 0o700)
			launcherBytes, err := os.ReadFile(prepared.LauncherPath)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(launcherBytes, []byte(prompt)) || strings.Contains(string(launcherBytes), "cmux omo") {
				t.Fatal("launcher embedded prompt text or cmux host shortcut")
			}
			if test.host == "opencode" {
				want := `exec '/usr/bin/env' 'OPENCODE_CONFIG_CONTENT={"model":"opencode/deepseek-v4.1-flash-free"}' '` + host + `' '--standalone' '--auto' '--prompt' "$prompt"`
				if !strings.Contains(string(launcherBytes), want) {
					t.Fatalf("opencode launcher line missing %q:\n%s", want, launcherBytes)
				}
			}
			for _, fence := range []string{"ISSUEOPS_CMUX_EXPECTED_WINDOW_ID=", "ISSUEOPS_CMUX_EXPECTED_WORKSPACE_ID=", "ISSUEOPS_CMUX_EXPECTED_SURFACE_ID=", "ISSUEOPS_CMUX_EXPECTED_SOCKET_PATH="} {
				if !strings.Contains(prepared.Command, fence) {
					t.Fatalf("bootstrap command missing sealed scope %q: %s", fence, prepared.Command)
				}
			}

			command := exec.CommandContext(context.Background(), "/bin/sh", "-c", prepared.Command)
			command.Dir = worktree
			command.Env = append(validCmuxEnvironment(os.Environ()), "HOST_CAPTURE="+capture)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("launcher: %v\n%s", err, output)
			}
			captured, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			argv := splitNUL(captured)
			want := append(append([]string(nil), test.wantPrefix...), prompt)
			if !reflect.DeepEqual(argv, want) {
				t.Fatalf("argv mismatch\ngot=%q\nwant=%q", argv, want)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("prompt text was interpolated by the launcher shell")
			}
			if _, err := os.Stat(prepared.PromptPath); !os.IsNotExist(err) {
				t.Fatalf("receiver did not delete prompt after opening it: %v", err)
			}
			if _, err := os.Stat(prepared.LauncherPath); !os.IsNotExist(err) {
				t.Fatalf("receiver did not delete launcher before exec: %v", err)
			}
			receipt, err := readBootstrapReceipt(prepared.ReceiptPath)
			if err != nil || receipt.Status != "ok" || receipt.CWD != worktree || receipt.WindowID != testWindow || receipt.WorkspaceID != testWorkspace || receipt.SurfaceID != testSurface {
				t.Fatalf("receipt=%+v err=%v", receipt, err)
			}
		})
	}
}

func TestPrepareLauncherUsesPortableSingleArgumentBoundary(t *testing.T) {
	const promptLimit = 64 << 10
	root := canonicalTempDir(t)
	worktree := filepath.Join(root, "worktree")
	if err := os.Mkdir(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(root, "codex")
	if err := os.WriteFile(host, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "maximum", size: promptLimit},
		{name: "maximum plus one", size: promptLimit + 1, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			prompt := []byte(strings.Repeat("p", test.size))
			prepared, err := prepareTestLauncher(cmuxcontract.ArtifactRequest{
				Root: filepath.Join(root, "artifacts-"+strings.ReplaceAll(test.name, " ", "-")), CWD: worktree,
				WindowID: testWindow, WorkspaceID: testWorkspace, SurfaceID: testSurface, SocketPath: socketPath,
				Host: "codex", HostExecutable: host, Model: "model", Prompt: prompt,
				PromptSHA256: digestBytes(prompt), MaterialSHA256: strings.Repeat("b", 64),
			})
			if test.wantErr {
				if err == nil {
					_ = CleanupLauncher(prepared)
					t.Fatalf("launcher accepted prompt size %d", test.size)
				}
				return
			}
			if err != nil {
				t.Fatalf("launcher rejected prompt size %d: %v", test.size, err)
			}
			command := exec.Command("/bin/sh", "-c", prepared.Command)
			command.Dir = worktree
			command.Env = validCmuxEnvironment(os.Environ())
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("launcher rejected prompt argv size %d: %v\n%s", test.size, err, output)
			}
			if err := CleanupLauncher(prepared); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPrivateLauncherWrongScopeFailsBeforeHostAndPreservesRecoveryArtifacts(t *testing.T) {
	root := canonicalTempDir(t)
	worktree := filepath.Join(root, "worktree")
	wrong := filepath.Join(root, "wrong")
	if err := os.Mkdir(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(wrong, 0o700); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(root, "codex")
	capture := filepath.Join(root, "called")
	if err := os.WriteFile(host, []byte("#!/bin/sh\ntouch \"$HOST_CAPTURE\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	prompt := []byte("prompt")
	prepared, err := prepareTestLauncher(cmuxcontract.ArtifactRequest{
		Root: filepath.Join(root, "artifacts"), CWD: worktree, WindowID: testWindow, WorkspaceID: testWorkspace,
		SurfaceID: testSurface, SocketPath: socketPath, Host: "codex", HostExecutable: host, Model: "model",
		Prompt: prompt, PromptSHA256: digestBytes(prompt), MaterialSHA256: strings.Repeat("b", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", "-c", prepared.Command)
	command.Dir = wrong
	command.Env = append(validCmuxEnvironment(os.Environ()), "HOST_CAPTURE="+capture)
	if err := command.Run(); err == nil {
		t.Fatal("wrong cwd launcher succeeded")
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatal("host executed after scope mismatch")
	}
	for _, path := range []string{prepared.PromptPath, prepared.LauncherPath, prepared.ReceiptPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("recovery artifact missing %s: %v", path, err)
		}
	}
	receipt, err := readBootstrapReceipt(prepared.ReceiptPath)
	if err != nil || receipt.Status != "identity_mismatch" || receipt.CWD != wrong {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if err := CleanupLauncher(prepared); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(prepared.Directory); !os.IsNotExist(err) {
		t.Fatalf("cleanup left artifact directory: %v", err)
	}
}

func TestPrivateLauncherRejectsWrongAmbientCmuxScope(t *testing.T) {
	root := canonicalTempDir(t)
	worktree := filepath.Join(root, "worktree")
	if err := os.Mkdir(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(root, "codex")
	capture := filepath.Join(root, "called")
	if err := os.WriteFile(host, []byte("#!/bin/sh\ntouch \"$HOST_CAPTURE\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	prompt := []byte("prompt")
	prepared, err := prepareTestLauncher(cmuxcontract.ArtifactRequest{
		Root: filepath.Join(root, "artifacts"), CWD: worktree, WindowID: testWindow, WorkspaceID: testWorkspace,
		SurfaceID: testSurface, SocketPath: socketPath, Host: "codex", HostExecutable: host, Model: "model",
		Prompt: prompt, PromptSHA256: digestBytes(prompt), MaterialSHA256: strings.Repeat("b", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", "-c", prepared.Command)
	command.Dir = worktree
	command.Env = append(withoutCmuxEnvironment(os.Environ()),
		"CMUX_WORKSPACE_ID=99999999-9999-4999-8999-999999999999",
		"CMUX_SURFACE_ID="+testSurface,
		"CMUX_SOCKET_PATH="+socketPath,
		"HOST_CAPTURE="+capture)
	if err := command.Run(); err == nil {
		t.Fatal("wrong ambient cmux scope succeeded")
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatal("host executed after ambient scope mismatch")
	}
	receipt, err := readBootstrapReceipt(prepared.ReceiptPath)
	if err != nil || receipt.Status != "identity_mismatch" || receipt.WorkspaceID == testWorkspace {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
}

func TestPrepareLauncherReturnsArgvBuilderErrorUnchangedAndRemovesArtifact(t *testing.T) {
	root := canonicalTempDir(t)
	worktree := filepath.Join(root, "worktree")
	if err := os.Mkdir(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(root, "codex")
	if err := os.WriteFile(host, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	prompt := []byte("prompt")
	artifacts := filepath.Join(root, "artifacts")
	cause := errors.New("argv builder refused")
	_, err := PrepareLauncher(cmuxcontract.ArtifactRequest{
		Root: artifacts, CWD: worktree, WindowID: testWindow, WorkspaceID: testWorkspace, SurfaceID: testSurface,
		SocketPath: socketPath, Host: "codex", HostExecutable: host, Model: "model",
		Prompt: prompt, PromptSHA256: digestBytes(prompt), MaterialSHA256: strings.Repeat("b", 64),
	}, func(string, string, string, string, string) ([]string, error) { return nil, cause })
	if err != cause {
		t.Fatalf("err=%v want the builder error unchanged", err)
	}
	entries, readErr := os.ReadDir(artifacts)
	if (readErr != nil && !os.IsNotExist(readErr)) || len(entries) != 0 {
		t.Fatalf("artifact directory left behind: entries=%v err=%v", entries, readErr)
	}
}

func validCmuxEnvironment(environment []string) []string {
	return append(withoutCmuxEnvironment(environment),
		"CMUX_WINDOW_ID="+testWindow,
		"CMUX_WORKSPACE_ID="+testWorkspace,
		"CMUX_SURFACE_ID="+testSurface,
		"CMUX_SOCKET_PATH="+socketPath)
}

func withoutCmuxEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if name != "ISSUEOPS_CMUX_EXPECTED_WINDOW_ID" && name != "ISSUEOPS_CMUX_EXPECTED_WORKSPACE_ID" &&
			name != "ISSUEOPS_CMUX_EXPECTED_SURFACE_ID" && name != "ISSUEOPS_CMUX_EXPECTED_SOCKET_PATH" &&
			name != "CMUX_WINDOW_ID" && name != "CMUX_WORKSPACE_ID" && name != "CMUX_SURFACE_ID" && name != "CMUX_SOCKET_PATH" && name != "CMUX_SOCKET" {
			result = append(result, entry)
		}
	}
	return result
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != want {
		t.Fatalf("mode %s=%#o want=%#o", path, info.Mode().Perm(), want)
	}
}

func splitNUL(value []byte) []string {
	parts := bytes.Split(value, []byte{0})
	if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		result = append(result, string(part))
	}
	return result
}
