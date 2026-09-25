package looprun

import (
	"reflect"
	"strings"
	"testing"

	loopcontract "issueops/internal/contract/looprun"
)

func TestPrepareStartKeepsDefaultsAndRedaction(t *testing.T) {
	prepared, err := PrepareStart(loopcontract.StartLoopRequest{
		Name: " qa ", Goal: " verify token=secret-value ", VerifyArgv: []string{" go ", "", "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Name != "qa" || prepared.Goal != "verify token=<redacted>" || prepared.MaxAttempts != 5 ||
		!reflect.DeepEqual(prepared.VerifyArgv, []string{"go", "test"}) {
		t.Fatalf("prepared start changed defaults: %+v", prepared)
	}
	if _, err := PrepareStart(loopcontract.StartLoopRequest{Name: "qa", Goal: "verify", MaxAttempts: 51}); err == nil || err.Error() != "max_attempts_invalid" {
		t.Fatalf("max attempt rejection=%v", err)
	}
}

func TestApplyAttemptAndStopPreserveTransitionAndInput(t *testing.T) {
	current := loopcontract.LoopRun{ID: "loop-x", Status: "active", MaxAttempts: 1}
	prepared, err := PrepareAttempt(loopcontract.RecordAttemptRequest{Verdict: " fail ", Evidence: []string{"token=secret-value"}})
	if err != nil {
		t.Fatal(err)
	}
	next, err := ApplyAttempt(current, prepared, "now")
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != "active" || len(current.Attempts) != 0 || next.Status != "exhausted" || next.Attempts[0].Seq != 1 ||
		strings.Contains(next.Attempts[0].Evidence[0], "secret-value") {
		t.Fatalf("attempt transition changed input or redaction: before=%+v after=%+v", current, next)
	}
	if _, err := ApplyAttempt(next, prepared, "later"); err == nil || err.Error() != "loop_not_active" {
		t.Fatalf("exhausted loop accepted attempt: %v", err)
	}
	if _, err := Stop(current, true, "", "now"); err == nil || err.Error() != "loop_success_requires_pass" {
		t.Fatalf("pass requirement error=%v", err)
	}
	pass, err := PrepareAttempt(loopcontract.RecordAttemptRequest{Verdict: "pass", Evidence: []string{"green"}})
	if err != nil {
		t.Fatal(err)
	}
	passed, err := ApplyAttempt(current, pass, "now")
	if err != nil {
		t.Fatal(err)
	}
	succeeded, err := Stop(passed, true, "", "later")
	if err != nil || succeeded.Status != "succeeded" || passed.Status != "active" {
		t.Fatalf("success transition=%+v err=%v", succeeded, err)
	}
	if _, err := Stop(succeeded, false, "operator stopped", "later"); err == nil || err.Error() != "loop_terminal" {
		t.Fatalf("terminal precedence=%v", err)
	}
}

func TestIncompleteOnlyForUnfinishedLoop(t *testing.T) {
	for _, status := range []string{"active", "exhausted"} {
		if !Incomplete(loopcontract.LoopRun{Status: status}) {
			t.Fatalf("%s loop must block completion", status)
		}
	}
	for _, status := range []string{"succeeded", "stopped"} {
		if Incomplete(loopcontract.LoopRun{Status: status}) {
			t.Fatalf("%s loop must not block completion", status)
		}
	}
}
