package commandstep

import (
	selfverify "issueops/internal/contract/selfverify"

	"fmt"
	"strings"
	"testing"

	"issueops/internal/testsupport"
)

func TestCommandStepFormattingHelpers(t *testing.T) {
	if failed := FailedStep("label", fmt.Errorf("boom")); failed.OK || failed.Error != "boom" {
		t.Fatalf("unexpected failed step: %+v", failed)
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
