package verification

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"issueops/internal/testsupport"
)

func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestListTrackedGoFilesSkipsDeletedAndNonGo(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	for name, body := range map[string]string{"a.go": "package a\n", "gone.go": "package g\n", "note.txt": "x\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "b.go"), []byte("package b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	if err := os.Remove(filepath.Join(root, "gone.go")); err != nil {
		t.Fatal(err)
	}
	got, err := ListTrackedGoFiles(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.go", "sub/b.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestListTrackedGoFilesEmptyRepoReturnsEmptyNonNil(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	got, err := ListTrackedGoFiles(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("want empty non-nil, got %#v", got)
	}
}

func TestListTrackedGoFilesNotARepoReportsGitStderr(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	_, err := ListTrackedGoFiles(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("expected error outside a git repository")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error should wrap *exec.ExitError: %v", err)
	}
	if err.Error() == exitErr.Error() {
		t.Fatalf("stderr detail missing from %q", err)
	}
}

func TestListUnformattedReportsOnlyMisformattedFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bad.go"), []byte("package b\nfunc  f( ){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListUnformatted(context.Background(), root, []string{"ok.go", "bad.go"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"bad.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestListUnformattedSyntaxErrorIncludesStderr(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "broken.go"), []byte("package\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListUnformatted(context.Background(), root, []string{"broken.go"})
	if err == nil || got != nil {
		t.Fatalf("want nil result and error, got %v, %v", got, err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || err.Error() == exitErr.Error() {
		t.Fatalf("expected wrapped exit error with stderr detail, got %q", err)
	}
}

func TestCommandErrorAndNonEmptyLines(t *testing.T) {
	base := errors.New("boom")
	if got := commandError(base, " \n"); got != base {
		t.Fatalf("blank stderr must return original error, got %v", got)
	}
	got := commandError(base, " detail \n")
	if !errors.Is(got, base) || got.Error() != "boom: detail" {
		t.Fatalf("got %q", got)
	}
	if lines := nonEmptyLines(" a \n\n  \nb\n"); !reflect.DeepEqual(lines, []string{"a", "b"}) {
		t.Fatalf("got %v", lines)
	}
	if lines := nonEmptyLines(""); lines == nil || len(lines) != 0 {
		t.Fatalf("want empty non-nil, got %#v", lines)
	}
}
