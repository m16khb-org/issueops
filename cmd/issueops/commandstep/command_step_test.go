package commandstep

import (
	selfverify "issueops/internal/contract/selfverify"

	"fmt"
	"strings"
	"testing"
	"time"

	"issueops/internal/testsupport"
)

func TestCommandStepFormattingHelpers(t *testing.T) {
	started := time.Now()
	child := selfverify.StepResult{Label: "child", OK: false, Stdout: "child stdout", Stderr: "child stderr", Error: "", StderrBytes: 12}
	combined := CombineFailedStep("parent", started, child, []string{"first", "second"}, []string{"cmd one", "cmd two"}, 8*1024)
	if combined.OK || combined.Error != "child failed" || combined.Command != "cmd one && cmd two" || !strings.Contains(combined.Stdout, "first\nsecond") {
		t.Fatalf("unexpected combined failure: %+v", combined)
	}

	asserted := AssertionStepWithOutput("assert", started, []string{"one", "two"}, []string{"stdout"}, []string{"cmd"}, 8*1024)
	if asserted.OK || asserted.Error != "one; two" || asserted.Command != "cmd" || asserted.Stdout != "stdout" {
		t.Fatalf("unexpected assertion step: %+v", asserted)
	}
	if failed := FailedStep("label", fmt.Errorf("boom")); failed.OK || failed.Error != "boom" {
		t.Fatalf("unexpected failed step: %+v", failed)
	}

	if out, truncated, original := BudgetCommandOutput("abc", 0); out != "abc" || truncated || original != 3 {
		t.Fatalf("unexpected unbudgeted output: out=%q truncated=%v original=%d", out, truncated, original)
	}
	if got := Tail("abcdef", 4); !strings.HasPrefix(got, "[") || len(got) > 4 {
		t.Fatalf("unexpected tail output: %q", got)
	}
	if got := IndentLines("a\nb"); got != "  a\n  b" {
		t.Fatalf("unexpected indented lines: %q", got)
	}

	okOut := captureStatusVerifyStdout(t, func() error {
		PrintStep(selfverify.StepResult{Label: "ok step", OK: true, DurationMS: 3})
		return nil
	})
	if !strings.Contains(okOut, "ok step ok") {
		t.Fatalf("unexpected ok print:\n%s", okOut)
	}
	failOut := captureStatusVerifyStdout(t, func() error {
		PrintStep(selfverify.StepResult{Label: "bad step", OK: false, DurationMS: 4, Error: "bad", Stdout: "out", Stderr: "err"})
		return nil
	})
	if !strings.Contains(failOut, "bad step failed") || !strings.Contains(failOut, "stdout:") || !strings.Contains(failOut, "stderr:") {
		t.Fatalf("unexpected failed print:\n%s", failOut)
	}
}

func TestTailWithBudgetKeepsTruncatedOutputWithinBudget_whenMarkerDigitsGrow(t *testing.T) {
	input := strings.Repeat("x", 48)

	out, truncated, original := TailWithBudget(input, 47)

	if !truncated || original != len(input) {
		t.Fatalf("unexpected truncation metadata: truncated=%v original=%d", truncated, original)
	}
	if len(out) > 47 {
		t.Fatalf("truncated output exceeded budget: len=%d budget=47 out=%q", len(out), out)
	}
}

func captureStatusVerifyStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}
