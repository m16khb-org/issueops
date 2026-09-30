package lintdiagnose

import (
	"strings"
	"testing"
)

func TestDiagnosisAdmissionAndTailPreserveFailureEvidence(t *testing.T) {
	if err := ValidateCommand(nil); err == nil || err.Error() != "missing command to execute" {
		t.Fatalf("err=%v", err)
	}
	if err := ValidateCommand([]string{""}); err != nil {
		t.Fatalf("argv presence validation changed: %v", err)
	}
	lines := make([]string, 151)
	for i := range lines {
		lines[i] = "line"
	}
	lines[0] = "drop"
	lines[1] = "keep"
	if got := FailureTail(strings.Join(lines, "\n")); strings.HasPrefix(got, "drop") || !strings.HasPrefix(got, "keep\n") || len(strings.Split(got, "\n")) != 150 {
		t.Fatalf("tail=%q", got)
	}
	if got := FailureTail("one\n"); got != "one\n" {
		t.Fatalf("trailing newline lost: %q", got)
	}
}
