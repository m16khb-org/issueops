package lintdiagnose

import (
	"strings"
	"testing"

	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
)

type fakeEffects struct {
	output   string
	exitCode int
	failed   bool
	called   bool
}

func (*fakeEffects) NormalizeRoot(string) (string, error) { return "/repo", nil }
func (f *fakeEffects) Run(string, []string) (string, int, bool) {
	f.called = true
	return f.output, f.exitCode, f.failed
}

func TestDiagnoseRejectsEmptyArgvBeforeRun(t *testing.T) {
	f := &fakeEffects{}
	result, err := (Service{Effects: f}).Diagnose(lintdiagnosecontract.LintDiagnoseRequest{})
	if err == nil || result.OK || f.called {
		t.Fatalf("result=%+v called=%v err=%v", result, f.called, err)
	}
}

func TestDiagnoseKeepsOnlyLast150Lines(t *testing.T) {
	f := &fakeEffects{output: "old-marker\n" + strings.Repeat("line\n", 151), exitCode: 3, failed: true}
	result, err := (Service{Effects: f}).Diagnose(lintdiagnosecontract.LintDiagnoseRequest{CommandArgv: []string{"lint"}})
	if err != nil || !result.Failed || result.ExitCode != 3 || strings.Contains(result.Prompt, "old-marker") || !strings.Contains(result.Prompt, "line") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
