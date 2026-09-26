package verification

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"issueops/cmd/issueops/commandstep"
	contract "issueops/internal/contract/selfverify"
)

func TestRunPreservesLegacyCommandStepContract(t *testing.T) {
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
			legacy := commandstep.Run(root, tc.name, time.Second, tc.stdin, tc.budget, "sh", "-c", tc.script)
			actual := Run(root, tc.name, time.Second, tc.stdin, tc.budget, "sh", "-c", tc.script)
			legacy.DurationMS, actual.DurationMS = 0, 0
			if !reflect.DeepEqual(actual, legacy) || actual.OK != tc.ok {
				t.Fatalf("adapter=%+v legacy=%+v", actual, legacy)
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

func TestRunEnvPreservesLegacyOverridesAndBudget(t *testing.T) {
	root := t.TempDir()
	env := []string{"ISSUEOPS_VERIFICATION_TEST=overridden"}
	legacy := commandstep.RunEnv(root, "env", time.Second, "", env, 28, "sh", "-c", "printf '%s' \"$ISSUEOPS_VERIFICATION_TEST\"; printf error >&2")
	actual := RunEnv(root, "env", time.Second, "", env, 28, "sh", "-c", "printf '%s' \"$ISSUEOPS_VERIFICATION_TEST\"; printf error >&2")
	legacy.DurationMS, actual.DurationMS = 0, 0
	if !reflect.DeepEqual(actual, legacy) || !actual.OK || actual.Stdout != "overridden" {
		t.Fatalf("adapter=%+v legacy=%+v", actual, legacy)
	}
}
