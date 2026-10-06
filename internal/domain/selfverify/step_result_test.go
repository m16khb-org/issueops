package selfverify

import (
	"strings"
	"testing"

	contract "issueops/internal/contract/selfverify"
)

func TestCombineFailedStepJoinsCommandsAndStdout(t *testing.T) {
	child := contract.StepResult{Label: "child", OK: false, Stdout: "child stdout", Stderr: "child stderr", StderrBytes: 12}
	combined := CombineFailedStep("parent", 7, child, []string{"first", "second"}, []string{"cmd one", "cmd two"}, 8*1024)
	if combined.OK || combined.Error != "child failed" || combined.Command != "cmd one && cmd two" || !strings.Contains(combined.Stdout, "first\nsecond") {
		t.Fatalf("unexpected combined failure: %+v", combined)
	}
}

func TestAssertionStepWithOutputJoinsErrors(t *testing.T) {
	asserted := AssertionStepWithOutput("assert", 7, []string{"one", "two"}, []string{"stdout"}, []string{"cmd"}, 8*1024)
	if asserted.OK || asserted.Error != "one; two" || asserted.Command != "cmd" || asserted.Stdout != "stdout" {
		t.Fatalf("unexpected assertion step: %+v", asserted)
	}
}
