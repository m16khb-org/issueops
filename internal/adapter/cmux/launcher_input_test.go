package cmux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cmuxcontract "issueops/internal/contract/cmux"
)

func TestPrepareLauncherRejectsPromptWithLeadingNULBeforeWritingArtifacts(t *testing.T) {
	root := canonicalTempDir(t)
	worktree := filepath.Join(root, "worktree")
	if err := os.Mkdir(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	prompt := []byte("\x00prompt")
	artifacts := filepath.Join(root, "artifacts")
	_, err := prepareTestLauncher(cmuxcontract.ArtifactRequest{
		Root: artifacts, CWD: worktree, WindowID: testWindow, WorkspaceID: testWorkspace, SurfaceID: testSurface,
		SocketPath: socketPath, Host: "codex", HostExecutable: filepath.Join(root, "codex"), Model: "model",
		Prompt: prompt, PromptSHA256: digestBytes(prompt), MaterialSHA256: strings.Repeat("b", 64),
	})
	if err == nil || !strings.Contains(err.Error(), "prompt or material digest is invalid") {
		t.Fatalf("leading NUL prompt error=%v", err)
	}
	if _, statErr := os.Stat(artifacts); !os.IsNotExist(statErr) {
		t.Fatalf("artifact root created before rejection: %v", statErr)
	}
}

func TestExecRunnerAcceptsOutputOfExactlyOneMiB(t *testing.T) {
	for name, script := range map[string]string{
		"stdout": "exec head -c 1048576 /dev/zero",
		"stderr": "exec head -c 1048576 /dev/zero >&2",
	} {
		t.Run(name, func(t *testing.T) {
			executable, args := shellCommand(t, script)
			output, err := ExecRunner{}.Run(t.Context(), CommandRequest{Executable: executable, Args: args, Timeout: 10 * time.Second})
			if err != nil {
				t.Fatalf("exact-limit output rejected: %v", err)
			}
			captured := output.Stdout
			if name == "stderr" {
				captured = output.Stderr
			}
			if len(captured) != 1<<20 {
				t.Fatalf("captured %d bytes want %d", len(captured), 1<<20)
			}
		})
	}
}
