package lintdiagnose

import (
	"context"
	"errors"
	"reflect"
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
func (f *fakeEffects) Run(context.Context, string, []string) (string, int, bool, error) {
	f.called = true
	return f.output, f.exitCode, f.failed, nil
}

func TestDiagnoseRejectsEmptyArgvBeforeRun(t *testing.T) {
	f := &fakeEffects{}
	result, err := (Service{Effects: f}).Diagnose(t.Context(), lintdiagnosecontract.Request{})
	if err == nil || result.OK || f.called {
		t.Fatalf("result=%+v called=%v err=%v", result, f.called, err)
	}
}

func TestDiagnoseKeepsOnlyLast150Lines(t *testing.T) {
	f := &fakeEffects{output: "old-marker\n" + strings.Repeat("line\n", 151), exitCode: 3, failed: true}
	result, err := (Service{Effects: f}).Diagnose(t.Context(), lintdiagnosecontract.Request{CommandArgv: []string{"lint"}})
	if err != nil || !result.Failed || result.ExitCode != 3 || strings.Contains(result.Prompt, "old-marker") || !strings.Contains(result.Prompt, "line") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

type stoppedEffects struct {
	err      error
	deadline bool
}

func (*stoppedEffects) NormalizeRoot(string) (string, error) { return "/repo", nil }
func (f *stoppedEffects) Run(ctx context.Context, _ string, _ []string) (string, int, bool, error) {
	_, f.deadline = ctx.Deadline()
	return "partial output", -1, true, f.err
}

// A run stopped by its context returns the context error and no result, so no
// diagnosis is built from the partial output.
func TestDiagnoseReturnsContextErrorWithoutResult(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{name: "cancelled", err: context.Canceled, want: "lint command stopped by the caller"},
		{name: "deadline", err: context.DeadlineExceeded, want: "lint command timed out after 15m0s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &stoppedEffects{err: tc.err}
			result, err := (Service{Effects: f}).Diagnose(t.Context(), lintdiagnosecontract.Request{CommandArgv: []string{"lint"}})
			if !errors.Is(err, tc.err) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q wrapping %v", err, tc.want, tc.err)
			}
			if !reflect.DeepEqual(result, lintdiagnosecontract.Result{}) {
				t.Fatalf("result = %+v, want the zero result", result)
			}
			if !f.deadline {
				t.Fatal("Run got a context without the command deadline")
			}
		})
	}
}
