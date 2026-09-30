package installcli

import (
	"bytes"
	"encoding/json"
	"io"

	"issueops/internal/adapter/install"

	activationport "issueops/internal/port/nativeactivation"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"issueops/internal/port"
)

func installCommandFixture(t *testing.T, home string) (Command, string) {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolve harness root: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("ISSUEOPS_ROOT", root)
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("PATH", "/usr/bin:/bin")
	cli := testInstallCommand(t, Deps{
		IssueOpsRoot:         func() string { return root },
		ExecutablePath:       func() (string, error) { return filepath.Join(root, "bin", "issueops"), nil },
		NativeInstallRequest: install.DefaultNativeInstallRequest,
		InstallNative: func(req port.NativeInstallRequest) (port.NativeInstallResult, error) {
			return installNativeForTest(req, testCodexInstaller(), testClaudeInstaller())
		},
		ActivationReadback: func(port.NativeInstallRequest) activationport.ReadbackVerifier { return nil },
	})
	return cli, installCommandTestStableRoot(t, root)
}

func installCommandTestStableRoot(t *testing.T, invokingRoot string) string {
	t.Helper()
	command := exec.Command("git", "-C", invokingRoot, "rev-parse", "--path-format=absolute", "--git-common-dir")
	raw, err := command.Output()
	if err != nil {
		t.Fatalf("resolve independent stable root: %v", err)
	}
	commonDir := filepath.Clean(strings.TrimSpace(string(raw)))
	if filepath.Base(commonDir) != ".git" {
		t.Fatalf("git common dir = %q, want physical .git", commonDir)
	}
	return filepath.Dir(commonDir)
}

func runInstallDryRunJSON(t *testing.T, home, pathMode string) port.NativeInstallResult {
	t.Helper()
	cli, _ := installCommandFixture(t, home)
	out, _, err := captureInstallCommandOutput(t, nil, func() error {
		return cli.RunInstall([]string{"--dry-run", "--json", "--path-mode=" + pathMode})
	})
	if err != nil {
		t.Fatalf("install --dry-run --json --path-mode=%s failed: %v\n%s", pathMode, err, out)
	}
	var result port.NativeInstallResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("install dry-run returned invalid JSON: %v\n%s", err, out)
	}
	return result
}

func captureInstallCommandOutput(t *testing.T, input io.Reader, fn func() error) (string, string, error) {
	t.Helper()
	oldStdin, oldStdout, oldStderr := os.Stdin, os.Stdout, os.Stderr
	defer func() {
		os.Stdin, os.Stdout, os.Stderr = oldStdin, oldStdout, oldStderr
	}()

	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	var inWrite *os.File
	if input != nil {
		inRead, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("create stdin pipe: %v", err)
		}
		os.Stdin = inRead
		inWrite = w
	}
	os.Stdout = outWrite
	os.Stderr = errWrite

	// Drain both pipes concurrently so a command that writes more than the OS
	// pipe buffer (~64KB) does not deadlock on a full pipe before fn() returns
	// (and before the writers are closed below).
	var stdout, stderr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(&stdout, outRead) }()
	go func() { defer wg.Done(); _, _ = io.Copy(&stderr, errRead) }()

	if input != nil {
		if _, err := io.Copy(inWrite, input); err != nil {
			t.Fatalf("write stdin pipe: %v", err)
		}
		if err := inWrite.Close(); err != nil {
			t.Fatalf("close stdin writer: %v", err)
		}
	}
	callErr := fn()
	if err := outWrite.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	if err := errWrite.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}
	wg.Wait()
	return stdout.String(), stderr.String(), callErr
}

func hasInstallLink(links []port.InstallLink, path, target string, wouldCreate bool) bool {
	for _, link := range links {
		if link.Path == path && link.Target == target && link.WouldCreate == wouldCreate {
			return true
		}
	}
	return false
}

func hasInstallFile(files []port.InstallFile, path, kind string, wouldWrite bool) bool {
	for _, file := range files {
		if file.Path == path && file.Kind == kind && file.WouldWrite == wouldWrite {
			return true
		}
	}
	return false
}

func hasInstallFileKind(files []port.InstallFile, kind string) bool {
	for _, file := range files {
		if file.Kind == kind {
			return true
		}
	}
	return false
}

func messagesContain(messages []string, needle string) bool {
	for _, message := range messages {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}
