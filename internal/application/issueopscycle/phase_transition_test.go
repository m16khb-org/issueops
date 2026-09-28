package issueopscycle

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func TestApplyPhaseTransitionObservesCleanupOnlyWhenNeeded(t *testing.T) {
	var calls []string
	store := cycleport.PhaseTransitionObservations{
		Now:         func() string { calls = append(calls, "clock"); return "now" },
		Head:        func(model.IssueOpsRecord) string { calls = append(calls, "head"); return "head" },
		Fingerprint: func(model.IssueOpsRecord) string { calls = append(calls, "fingerprint"); return "fingerprint" },
	}
	record := model.IssueOpsRecord{Phase: model.IssueOpsPhaseImplement}
	result := ApplyPhaseTransition(store, record, model.IssueOpsPhaseAISlopClean)
	if !reflect.DeepEqual(calls, []string{"clock", "head", "fingerprint"}) || result.AISlopCleanHead != "head" {
		t.Fatalf("result=%+v calls=%v", result, calls)
	}
	calls = nil
	result = ApplyPhaseTransition(store, record, model.IssueOpsPhaseFeedback)
	if !reflect.DeepEqual(calls, []string{"clock"}) || result.AISlopCleanHead != "" {
		t.Fatalf("result=%+v calls=%v", result, calls)
	}
}
