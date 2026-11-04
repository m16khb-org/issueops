package issueopscycle

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func TestPhaseCompletionRoutesWithoutUnneededObservations(t *testing.T) {
	calls := []string{}
	observations := cycleport.PhaseCompletionReadiness{
		Compatibility: func(model.Record) model.Readiness {
			calls = append(calls, "compatibility")
			return model.Readiness{Ready: true}
		},
		AISlopClean: func(model.Record) model.Readiness {
			calls = append(calls, "clean")
			return model.Readiness{Ready: true}
		},
		PR: func(model.Record) model.Readiness {
			calls = append(calls, "pr")
			return model.Readiness{Ready: true}
		},
		RemoteArtifactMissing: func(model.Record) []string {
			calls = append(calls, "remote")
			return []string{"remote_artifact"}
		},
	}
	record := model.Record{}
	if got := phaseCompletion(record, model.PhaseProblem, observations); got.Ready || !reflect.DeepEqual(got.Missing, []string{"intent_contract"}) || len(calls) != 0 {
		t.Fatalf("problem completion=%+v calls=%v", got, calls)
	}
	if got := phaseCompletion(record, model.PhasePlan, observations); !got.Ready || !reflect.DeepEqual(calls, []string{"compatibility"}) {
		t.Fatalf("plan completion=%+v calls=%v", got, calls)
	}
	calls = nil
	if got := phaseCompletion(record, model.PhaseDone, observations); got.Ready || !reflect.DeepEqual(got.Missing, []string{"prior_phase_pr", "remote_artifact"}) || !reflect.DeepEqual(calls, []string{"remote"}) {
		t.Fatalf("done completion=%+v calls=%v", got, calls)
	}
}
