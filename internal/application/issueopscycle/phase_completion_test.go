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
		Compatibility: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			calls = append(calls, "compatibility")
			return model.IssueOpsReadiness{Ready: true}
		},
		AISlopClean: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			calls = append(calls, "clean")
			return model.IssueOpsReadiness{Ready: true}
		},
		PR: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			calls = append(calls, "pr")
			return model.IssueOpsReadiness{Ready: true}
		},
		RemoteArtifactMissing: func(model.IssueOpsRecord) []string {
			calls = append(calls, "remote")
			return []string{"remote_artifact"}
		},
	}
	record := model.IssueOpsRecord{}
	if got := PhaseCompletion(record, model.IssueOpsPhaseProblem, observations); got.Ready || !reflect.DeepEqual(got.Missing, []string{"intent_contract"}) || len(calls) != 0 {
		t.Fatalf("problem completion=%+v calls=%v", got, calls)
	}
	if got := PhaseCompletion(record, model.IssueOpsPhasePlan, observations); !got.Ready || !reflect.DeepEqual(calls, []string{"compatibility"}) {
		t.Fatalf("plan completion=%+v calls=%v", got, calls)
	}
	calls = nil
	if got := PhaseCompletion(record, model.IssueOpsPhaseDone, observations); got.Ready || !reflect.DeepEqual(got.Missing, []string{"prior_phase_pr", "remote_artifact"}) || !reflect.DeepEqual(calls, []string{"remote"}) {
		t.Fatalf("done completion=%+v calls=%v", got, calls)
	}
}
