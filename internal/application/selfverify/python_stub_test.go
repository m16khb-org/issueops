package selfverify

import (
	"strings"
	"testing"
	"time"

	contract "issueops/internal/contract/selfverify"
)

// runPythonStep runs the planned Python script tests step against a fake runner that returns result.
func runPythonStep(t *testing.T, result contract.StepResult) contract.StepResult {
	t.Helper()
	deps := StepDeps{RunCommandStep: func(root, label string, timeout time.Duration, stdin, name string, args ...string) contract.StepResult {
		if label != "Python script tests" || name != "python3" {
			t.Fatalf("unexpected command: label=%q name=%q", label, name)
		}
		result.Label = label
		return result
	}}
	var goTest contract.StepResult
	for _, step := range PlannedSteps("/repo", "/tmp/issueops", 100, &goTest, deps) {
		if step.Label == "Python script tests" {
			return step.Run()
		}
	}
	t.Fatal("Python script tests step is not planned")
	return contract.StepResult{}
}

func TestPythonStepNamesTheInterpreterWhenTheRuntimeCheckNeverRan(t *testing.T) {
	got := runPythonStep(t, contract.StepResult{OK: false, StdoutBytes: 0, StdoutTruncated: false, Error: "exit status 49"})
	if !strings.HasPrefix(got.Error, "exit status 49; python3 exited before the runtime check ran") || !strings.Contains(got.Error, "Microsoft Store") {
		t.Fatalf("stub exit was not diagnosed: %q", got.Error)
	}
}

func TestPythonStepKeepsSuiteFailuresUnchanged(t *testing.T) {
	got := runPythonStep(t, contract.StepResult{
		OK:          false,
		Stdout:      "Python runtime: 3.12.10 (requires Python 3.10+)\n",
		StdoutBytes: 48,
		Error:       "exit status 1",
	})
	if got.Error != "exit status 1" {
		t.Fatalf("suite failure error changed: %q", got.Error)
	}
}

func TestPythonStepKeepsTruncatedOutputFailuresUnchanged(t *testing.T) {
	got := runPythonStep(t, contract.StepResult{
		OK:              false,
		Stdout:          "tail of a long output without the runtime line\n",
		StdoutBytes:     0,
		StdoutTruncated: true,
		Error:           "exit status 1",
	})
	if got.Error != "exit status 1" {
		t.Fatalf("truncated output error changed: %q", got.Error)
	}
}

func TestPythonStepKeepsExecutableNotFoundUnchanged(t *testing.T) {
	const notFound = `exec: "python3": executable file not found in $PATH`
	got := runPythonStep(t, contract.StepResult{OK: false, StdoutBytes: 0, Error: notFound})
	if got.Error != notFound || strings.Contains(got.Error, "Microsoft Store") {
		t.Fatalf("executable-not-found error changed: %q", got.Error)
	}
}
