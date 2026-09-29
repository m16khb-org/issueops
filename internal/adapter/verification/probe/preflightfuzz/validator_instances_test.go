package preflightfuzz

import (
	"strings"
	"testing"
)

func TestPreparedValidatorsKeepTheirGitRunner(t *testing.T) {
	prepare := func(name string) func(string, string, int64) StepResult {
		return (Validator{Git: func(string, ...string) (int, string, string) { return 1, "", name }}).Validate
	}
	first := prepare("first-runner")
	second := prepare("second-runner")
	for _, tc := range []struct {
		run  func(string, string, int64) StepResult
		want string
	}{{first, "first-runner"}, {second, "second-runner"}, {first, "first-runner"}} {
		step := tc.run("unused", t.TempDir(), 1)
		if step.OK || !strings.Contains(step.Error, "git init: "+tc.want) {
			t.Fatalf("expected captured %s git failure, got %#v", tc.want, step)
		}
	}
}
