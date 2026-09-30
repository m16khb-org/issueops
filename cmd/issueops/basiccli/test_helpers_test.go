package basiccli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"issueops/internal/domain/operationalhealth"
	"issueops/internal/testsupport"
)

func configureOperationalCollectorTest(t *testing.T, collect func(context.Context, string) operationalhealth.Snapshot) {
	t.Helper()
	old := testOperationalCollector
	t.Cleanup(func() { testOperationalCollector = old })
	testOperationalCollector = collect
}

func healthyCLIOperationalSnapshot(repo string) operationalhealth.Snapshot {
	return operationalhealth.Snapshot{
		RepoRoot: repo, CanonicalBranch: "main", SourceHead: "head-main", SourceClean: true,
		GitWorktrees:  []operationalhealth.GitWorktree{{Path: repo, Branch: "main", Head: "head-main", Clean: true, Canonical: true}},
		LocalRefs:     []operationalhealth.GitRef{{Name: "refs/heads/main", Branch: "main", OID: "head-main", Location: "local"}},
		RemoteRefs:    []operationalhealth.GitRef{{Name: "refs/heads/main", Branch: "main", OID: "head-main", Location: "remote"}},
		OrcaWorktrees: []operationalhealth.OrcaWorktree{{ID: "wt-main", InstanceID: "instance-main", Repo: repo, Path: repo, Branch: "main", Head: "head-main"}},
		Messages:      operationalhealth.MessagePresence{Empty: true, CompleteAbsence: true},
	}
}

func testIssueOpsRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

func testResolveTarget(target string) string {
	if target != "" {
		return target
	}
	if projectDir := os.Getenv("CLAUDE_PROJECT_DIR"); projectDir != "" {
		return projectDir
	}
	if pwd := os.Getenv("PWD"); pwd != "" {
		return pwd
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func captureStatusVerifyStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}

func runStatusVerifyTestCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(output))
	}
}

func captureTraceGuardPolicyStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	return testsupport.CaptureStdoutAndError(t, fn)
}

func writeFileForCLITest(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
