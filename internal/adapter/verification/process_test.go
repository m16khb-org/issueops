package verification

import (
	"reflect"
	"strings"
	"testing"
	"time"

	contract "issueops/internal/contract/selfverify"
)

func TestRunPreservesCommandStepContract(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stdin  string
		script string
		budget int
		ok     bool
	}{
		{name: "success", stdin: "input", script: "printf prefix; cat; printf stderr >&2", budget: 32, ok: true},
		{name: "truncated", script: "printf xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", budget: 32, ok: true},
		{name: "failure", script: "printf failure >&2; exit 7", budget: 32, ok: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			want := contract.StepResult{Label: tc.name, Command: "sh -c " + tc.script, OK: tc.ok}
			switch tc.name {
			case "success":
				want.Stdout = "prefixinput"
				want.StdoutBytes = 11
				want.Stderr = "stderr"
				want.StderrBytes = 6
			case "truncated":
				want.Stdout = "[truncated: original_bytes=48 om"
				want.StdoutBytes = 48
				want.StdoutTruncated = true
			case "failure":
				want.Stderr = "failure"
				want.StderrBytes = 7
				want.Error = "exit status 7"
			default:
				t.Fatalf("unknown fixture %q", tc.name)
			}
			actual := Run(root, tc.name, time.Second, tc.stdin, tc.budget, "sh", "-c", tc.script)
			actual.DurationMS = 0
			if !reflect.DeepEqual(actual, want) || actual.OK != tc.ok {
				t.Fatalf("adapter=%+v want=%+v", actual, want)
			}
		})
	}
}

func TestRunReportsTimeout(t *testing.T) {
	step := Run(t.TempDir(), "timeout", 10*time.Millisecond, "", 32, "sh", "-c", "exec sleep 1")
	if step.OK || !strings.Contains(step.Error, "timeout after 10ms") {
		t.Fatalf("timeout result=%+v", step)
	}
}

func TestRunUnknownExecutableFails(t *testing.T) {
	step := Run(t.TempDir(), "unknown", time.Second, "", 32, "issueops-no-such-executable")
	if step.OK || step.Error == "" || reflect.DeepEqual(step, contract.StepResult{}) {
		t.Fatalf("unknown executable was accepted: %+v", step)
	}
}

func TestRunEnvPreservesOverridesAndBudget(t *testing.T) {
	root := t.TempDir()
	env := []string{"ISSUEOPS_VERIFICATION_TEST=overridden"}
	want := contract.StepResult{Label: "env", Command: "sh -c printf '%s' \"$ISSUEOPS_VERIFICATION_TEST\"; printf error >&2", OK: true, Stdout: "overridden", StdoutBytes: 10, Stderr: "error", StderrBytes: 5}
	actual := RunEnv(root, "env", time.Second, "", env, 28, "sh", "-c", "printf '%s' \"$ISSUEOPS_VERIFICATION_TEST\"; printf error >&2")
	actual.DurationMS = 0
	if !reflect.DeepEqual(actual, want) || !actual.OK || actual.Stdout != "overridden" {
		t.Fatalf("adapter=%+v want=%+v", actual, want)
	}
}
