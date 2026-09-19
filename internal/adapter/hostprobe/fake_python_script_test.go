package hostprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakePythonTestFixture struct {
	script     string
	tempDir    string
	toolTrace  string
	pythonPath string
	path       string
}

func TestFakePythonScriptDirectlyExecutesGeneralStdin(t *testing.T) {
	fixture := newFakePythonTestFixture(t)
	realPython := filepath.Join(filepath.Dir(fixture.script), "real-python")
	writeExecutable(t, realPython, `#!/usr/bin/env bash
set -u
payload=""
IFS= read -r payload || true
printf 'argv='
for argument in "$@"; do printf '<%s>' "$argument"; done
printf '\nenv=<%s>\nstdin=<%s>\n' "$FAKE_MARKER" "$payload"
printf 'fixture-stderr\n' >&2
exit 23
`)

	command := exec.Command(fixture.script, "-", "alpha", "beta value")
	command.Env = environmentWithOverrides(map[string]string{
		"FAKE_MARKER":     "direct-env",
		"FAKE_SCENARIO":   "general",
		"FAKE_TOOL_TRACE": fixture.toolTrace,
		"PATH":            fixture.path,
		"REAL_PYTHON":     realPython,
		"TMPDIR":          fixture.tempDir,
	})
	command.Stdin = strings.NewReader("direct-stdin\n")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 23 {
		t.Fatalf("exit error = %v, want exit 23", err)
	}

	wantStdout := "argv=<-><alpha><beta value>\nenv=<direct-env>\nstdin=<direct-stdin>\n"
	if stdout.String() != wantStdout {
		t.Errorf("stdout = %q, want %q", stdout.String(), wantStdout)
	}
	if stderr.String() != "fixture-stderr\n" {
		t.Errorf("stderr = %q, want fixture stderr", stderr.String())
	}
	if trace := readOptionalFile(t, fixture.toolTrace); trace != "" {
		t.Errorf("general execution used rewrite tools:\n%s", trace)
	}
	assertDirectoryEmpty(t, fixture.tempDir)
}

func TestFakePythonScriptKeepsActivatedDigestRewriteAndCleanup(t *testing.T) {
	fixture := newFakePythonTestFixture(t)
	pythonTrace := filepath.Join(filepath.Dir(fixture.script), "python.log")
	realPython := filepath.Join(filepath.Dir(fixture.script), "real-python")
	writeExecutable(t, realPython, `#!/usr/bin/env bash
set -euo pipefail
printf 'python' >>"$FAKE_PYTHON_TRACE"
for argument in "$@"; do printf '\t%s' "$argument" >>"$FAKE_PYTHON_TRACE"; done
printf '\n' >>"$FAKE_PYTHON_TRACE"
exec "$ACTUAL_PYTHON" "$@"
`)
	activated := filepath.Join(filepath.Dir(fixture.script), "activated.json")
	if err := os.WriteFile(activated, []byte(`{"binary_sha256":"before"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(fixture.script, "-", "fixture-arg", activated)
	command.Env = environmentWithOverrides(map[string]string{
		"ACTUAL_PYTHON":     fixture.pythonPath,
		"FAKE_MARKER":       "rewrite-env",
		"FAKE_PYTHON_TRACE": pythonTrace,
		"FAKE_SCENARIO":     "activated-digest-blank",
		"FAKE_TOOL_TRACE":   fixture.toolTrace,
		"PATH":              fixture.path,
		"REAL_PYTHON":       realPython,
		"TMPDIR":            fixture.tempDir,
	})
	command.Stdin = strings.NewReader("import os\nprint('program:' + os.environ['FAKE_MARKER'])\n")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("run activated digest fixture: %v\nstderr=%s", err, stderr.String())
	}
	if stdout.String() != "program:rewrite-env\n" || stderr.String() != "" {
		t.Errorf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	var document struct {
		BinarySHA256 string `json:"binary_sha256"`
	}
	data, err := os.ReadFile(activated)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document.BinarySHA256 != "" {
		t.Errorf("binary_sha256 = %q, want blank", document.BinarySHA256)
	}
	if trace := readOptionalFile(t, fixture.toolTrace); trace != "mktemp\nsed\nrm\n" {
		t.Errorf("rewrite tool trace = %q", trace)
	}
	calls := strings.Split(strings.TrimSpace(readOptionalFile(t, pythonTrace)), "\n")
	if len(calls) != 2 {
		t.Fatalf("python calls = %q, want program and mutation calls", calls)
	}
	programCall := strings.Split(calls[0], "\t")
	if len(programCall) != 4 || programCall[0] != "python" || programCall[1] == "-" || programCall[2] != "fixture-arg" || programCall[3] != activated {
		t.Errorf("program call = %q, want rewritten temp program and original argv", programCall)
	}
	mutationCall := strings.Split(calls[1], "\t")
	if len(mutationCall) != 3 || mutationCall[0] != "python" || mutationCall[1] != "-" || mutationCall[2] != activated {
		t.Errorf("mutation call = %q", mutationCall)
	}
	assertDirectoryEmpty(t, fixture.tempDir)
}

func TestFakePythonScriptCancellationStopsGeneralPython(t *testing.T) {
	fixture := newFakePythonTestFixture(t)
	started := filepath.Join(filepath.Dir(fixture.script), "started")
	completed := filepath.Join(filepath.Dir(fixture.script), "completed")
	program := "from pathlib import Path\nimport sys, time\nPath(sys.argv[1]).write_text('started')\ntime.sleep(0.5)\nPath(sys.argv[2]).write_text('completed')\n"
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, fixture.script, "-", started, completed)
	command.Env = environmentWithOverrides(map[string]string{
		"FAKE_SCENARIO":   "general",
		"FAKE_TOOL_TRACE": fixture.toolTrace,
		"PATH":            fixture.path,
		"REAL_PYTHON":     fixture.pythonPath,
		"TMPDIR":          fixture.tempDir,
	})
	command.Stdin = strings.NewReader(program)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			cancel()
			_ = command.Wait()
			t.Fatal("python program did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := command.Wait(); err == nil {
		t.Fatal("cancelled fake python unexpectedly succeeded")
	}
	time.Sleep(700 * time.Millisecond)
	if _, err := os.Stat(completed); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("cancelled Python continued to completion: err=%v", err)
	}
	if trace := readOptionalFile(t, fixture.toolTrace); trace != "" {
		t.Errorf("cancelled general execution used rewrite tools:\n%s", trace)
	}
	assertDirectoryEmpty(t, fixture.tempDir)
}

func newFakePythonTestFixture(t *testing.T) fakePythonTestFixture {
	t.Helper()
	root := t.TempDir()
	fakeBin := filepath.Join(root, "fake-bin")
	tempDir := filepath.Join(root, "tmp")
	if err := os.MkdirAll(fakeBin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		t.Fatal(err)
	}
	toolTrace := filepath.Join(root, "tools.log")
	for _, name := range []string{"mktemp", "sed", "rm"} {
		realTool := mustLookPath(t, name)
		wrapper := fmt.Sprintf("#!/usr/bin/env bash\nprintf '%%s\\n' %q >>\"$FAKE_TOOL_TRACE\"\nexec %q \"$@\"\n", name, realTool)
		writeExecutable(t, filepath.Join(fakeBin, name), wrapper)
	}
	script := filepath.Join(root, "python3")
	writeExecutable(t, script, fakePythonScript)
	return fakePythonTestFixture{
		script:     script,
		tempDir:    tempDir,
		toolTrace:  toolTrace,
		pythonPath: mustLookPath(t, "python3"),
		path:       fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
	}
}

func environmentWithOverrides(overrides map[string]string) []string {
	environment := os.Environ()
	for key, value := range overrides {
		prefix := key + "="
		filtered := environment[:0]
		for _, entry := range environment {
			if !strings.HasPrefix(entry, prefix) {
				filtered = append(filtered, entry)
			}
		}
		environment = append(filtered, prefix+value)
	}
	return environment
}

func readOptionalFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertDirectoryEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("temporary directory contains %v", entries)
	}
}
