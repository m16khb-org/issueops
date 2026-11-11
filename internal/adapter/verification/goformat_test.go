package verification

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
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

func TestGofmtBatchesRespectTheBudgetAndKeepOrder(t *testing.T) {
	const budget = 20
	files := []string{"a.go", "bb.go", "ccc.go", "dddd.go", "eeeee.go", "ffffff.go", strings.Repeat("x", 30) + ".go", "g.go", "h.go"}
	batches := gofmtBatches(files, budget)
	wantBatches := [][]string{
		{"a.go", "bb.go", "ccc.go"},
		{"dddd.go", "eeeee.go"},
		{"ffffff.go"},
		{strings.Repeat("x", 30) + ".go"},
		{"g.go", "h.go"},
	}
	if !reflect.DeepEqual(batches, wantBatches) {
		t.Fatalf("got batches %v want %v", batches, wantBatches)
	}
	var joined []string
	for _, batch := range batches {
		if len(batch) == 0 {
			t.Fatalf("empty batch in %v", batches)
		}
		size := 0
		for _, file := range batch {
			size += len(file) + 1
		}
		if size > budget && len(batch) != 1 {
			t.Fatalf("batch %v uses %d bytes over budget %d", batch, size, budget)
		}
		joined = append(joined, batch...)
	}
	if !reflect.DeepEqual(joined, files) {
		t.Fatalf("batches must concatenate to the input, got %v", batches)
	}
	if len(batches) < 2 {
		t.Fatalf("expected the input to be split, got %v", batches)
	}
	long := strings.Repeat("x", 30) + ".go"
	for _, batch := range batches {
		for _, file := range batch {
			if file == long && len(batch) != 1 {
				t.Fatalf("a path over the budget must stand alone, got %v", batch)
			}
		}
	}
	if got := gofmtBatches(nil, budget); len(got) != 0 {
		t.Fatalf("empty input must give no batches, got %v", got)
	}
	if got := gofmtBatches([]string{}, budget); len(got) != 0 {
		t.Fatalf("empty input must give no batches, got %v", got)
	}
}

func TestListUnformattedSplitsCallsUnderTheBudget(t *testing.T) {
	realGofmt, err := exec.LookPath("gofmt")
	if err != nil {
		t.Fatalf("gofmt not found: %v", err)
	}
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "calls.log")
	script := "#!/bin/sh\n" +
		"n=0\n" +
		"for a in \"$@\"; do n=$((n + ${#a} + 1)); done\n" +
		"echo \"$n\" >> '" + filepath.ToSlash(logPath) + "'\n" +
		"exec '" + filepath.ToSlash(realGofmt) + "' \"$@\"\n"
	testsupport.WriteFakeCommand(t, binDir, "gofmt", script)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	// 15-byte names count 16 each, so a budget of 200 holds 12 per batch and
	// the "-l" argument still fits under 200.
	files := make([]string, 22)
	for i := range files {
		files[i] = "file_" + strconv.Itoa(i/10) + strconv.Itoa(i%10) + "_abcd.go"
		body := "package a\n"
		if i == 1 || i == 20 {
			body = "package a\nfunc  f( ){}\n"
		}
		if err := os.WriteFile(filepath.Join(root, files[i]), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := listUnformatted(context.Background(), root, files, 200)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{files[1], files[20]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("fake gofmt left no call log: %v", err)
	}
	lines := strings.Fields(string(data))
	if len(lines) != 2 {
		t.Fatalf("want exactly 2 gofmt calls (12 + 10 files), got %v", lines)
	}
	for _, line := range lines {
		n, err := strconv.Atoi(line)
		if err != nil || n > 200 {
			t.Fatalf("gofmt call used %q argument bytes, want <= 200", line)
		}
	}
}

func TestListUnformattedReturnsSyntaxErrorFromTheLastBatch(t *testing.T) {
	root := t.TempDir()
	files := []string{"a1.go", "a2.go", "a3.go", "a4.go", "broken.go"}
	for _, file := range files {
		body := "package a\n"
		if file == "broken.go" {
			body = "package\n"
		}
		if err := os.WriteFile(filepath.Join(root, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if batches := gofmtBatches(files, 14); len(batches) < 2 || !reflect.DeepEqual(batches[len(batches)-1], []string{"broken.go"}) {
		t.Fatalf("fixture must leave broken.go alone in the last batch, got %v", batches)
	}
	got, err := listUnformatted(context.Background(), root, files, 14)
	if err == nil || got != nil {
		t.Fatalf("want nil result and error, got %v, %v", got, err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || err.Error() == exitErr.Error() || !strings.Contains(err.Error(), "broken.go") {
		t.Fatalf("expected wrapped exit error carrying gofmt stderr, got %q", err)
	}
}
