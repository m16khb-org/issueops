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
		Head:        func(model.Record) string { calls = append(calls, "head"); return "head" },
		Fingerprint: func(model.Record) string { calls = append(calls, "fingerprint"); return "fingerprint" },
	}
	record := model.Record{Phase: model.PhaseImplement}
	result := applyPhaseTransition(store, record, model.PhaseAISlopClean)
	if !reflect.DeepEqual(calls, []string{"clock", "head", "fingerprint"}) || result.AISlopCleanHead != "head" {
		t.Fatalf("result=%+v calls=%v", result, calls)
	}
	calls = nil
	result = applyPhaseTransition(store, record, model.PhaseFeedback)
	if !reflect.DeepEqual(calls, []string{"clock"}) || result.AISlopCleanHead != "" || result.PRHead != "" {
		t.Fatalf("result=%+v calls=%v", result, calls)
	}
	calls = nil
	result = applyPhaseTransition(store, model.Record{Phase: model.PhaseFeedback}, model.PhasePR)
	if !reflect.DeepEqual(calls, []string{"clock", "head"}) || result.PRHead != "head" || result.AISlopCleanHead != "" {
		t.Fatalf("pr entry must record the verified head completion checks against: result=%+v calls=%v", result, calls)
	}
}
