package issueopscompletion

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Environment outbound 어댑터의 경로/HEAD 검증 래퍼를 잠근다.
func TestEnvironmentPathsMatchResolvesAndTrims(t *testing.T) {
	env := NewEnvironment()
	dir := t.TempDir()
	trimmed := "  " + dir + "  "
	if !env.PathsMatch(dir, trimmed) {
		t.Fatal("trimmed and resolved identical paths must match")
	}
	if env.PathsMatch(dir, filepath.Join(t.TempDir(), "other")) {
		t.Fatal("distinct paths must not match")
	}
}

func TestEnvironmentCurrentHeadRunsGit(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@e.invalid")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "f")
	run("commit", "-q", "-m", "init")
	want := strings.TrimSpace(mustCombinedOutput(t, repo, "rev-parse", "HEAD"))
	got, err := NewEnvironment().CurrentHead(context.Background(), repo)
	if err != nil || got != want || got == "" {
		t.Fatalf("CurrentHead = %q err=%v want=%q", got, err, want)
	}
	if _, err := NewEnvironment().CurrentHead(context.Background(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing repo must fail")
	}
}

func TestEnvironmentChangedPathsSinceLeavesOutOnlyTheReport(t *testing.T) {
	repo := t.TempDir() // on macOS /var is a symlink to /private/var; the report path arrives resolved
	commit := func(path, body string) string {
		full := filepath.Join(repo, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		mustCombinedOutput(t, repo, "add", path)
		mustCombinedOutput(t, repo, "commit", "-q", "-m", path)
		return strings.TrimSpace(mustCombinedOutput(t, repo, "rev-parse", "HEAD"))
	}
	mustCombinedOutput(t, repo, "init", "-q")
	mustCombinedOutput(t, repo, "config", "user.email", "t@e.invalid")
	mustCombinedOutput(t, repo, "config", "user.name", "t")
	prHead := commit("code.go", "package a\n")
	reportHead := commit(".issueops/issues/12/verified-execution-report.md", "report\n")
	report, err := NewEnvironment().VerifyReport(repo, filepath.Join(repo, ".issueops/issues/12/verified-execution-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	changed, err := NewEnvironment().ChangedPathsSince(context.Background(), repo, prHead, reportHead, report)
	if err != nil || len(changed) != 0 {
		t.Fatalf("report-only commit: changed=%q err=%v, want none", changed, err)
	}
	codeHead := commit("code.go", "package a\n\nvar x = 1\n")
	changed, err = NewEnvironment().ChangedPathsSince(context.Background(), repo, prHead, codeHead, report)
	if err != nil || len(changed) != 1 || changed[0] != "code.go" {
		t.Fatalf("code commit after pr head: changed=%q err=%v, want [code.go]", changed, err)
	}
}

// An agent in the source checkout passed --verification-report as a worktree-
// relative path with --cwd <worktree> and was refused: the path was resolved
// against the process cwd. It names a file in the canonical worktree.
func TestVerifyReportResolvesRelativePathAgainstTheWorktree(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "r"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "r", "report.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	got, err := NewEnvironment().VerifyReport(root, "r/report.md")
	if err != nil || filepath.Base(got) != "report.md" {
		t.Fatalf("got %q err=%v", got, err)
	}
	if _, err := NewEnvironment().VerifyReport(root, "../outside.md"); err == nil {
		t.Fatal("a relative path escaping the worktree must be refused")
	}
}

func mustCombinedOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return string(out)
}
