package issueopscycle

import (
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func TestValidatePhaseEntryChecksPlanBeforeGrill(t *testing.T) {
	var calls []string
	store := cycleport.PhaseEntryReadiness{
		Plan: func(model.Record) model.Readiness {
			calls = append(calls, "plan")
			return model.Readiness{Missing: []string{"intent_contract"}}
		},
		Grill: func(model.Record) model.Readiness {
			calls = append(calls, "grill")
			return model.Readiness{Missing: []string{"domain_review"}}
		},
	}
	err := validatePhaseEntry(store, model.Record{Phase: model.PhaseGrill}, model.PhasePlan)
	if err == nil || !strings.Contains(err.Error(), "intent_contract") || !reflect.DeepEqual(calls, []string{"plan"}) {
		t.Fatalf("err=%v calls=%v", err, calls)
	}
	store.Plan = func(model.Record) model.Readiness {
		calls = append(calls, "plan")
		return model.Readiness{Ready: true}
	}
	calls = nil
	err = validatePhaseEntry(store, model.Record{Phase: model.PhaseGrill}, model.PhasePlan)
	if err == nil || !strings.Contains(err.Error(), "grill incomplete: missing domain_review") || !reflect.DeepEqual(calls, []string{"plan", "grill"}) {
		t.Fatalf("err=%v calls=%v", err, calls)
	}
}

func TestValidatePhaseEntryUsesPrefetchedPRReadiness(t *testing.T) {
	calls := 0
	store := cycleport.PhaseEntryReadiness{
		StrictPR: func(model.Record) model.Readiness {
			calls++
			return model.Readiness{Missing: []string{"upstream_fetch"}}
		},
	}
	err := validatePhaseEntry(store, model.Record{Phase: model.PhaseFeedback}, model.PhasePR)
	if err == nil || !strings.Contains(err.Error(), "upstream_fetch") || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestValidatePhaseEntryChecksRemoteArtifactBeforeCompletion(t *testing.T) {
	calls := 0
	store := cycleport.PhaseEntryReadiness{
		RemoteArtifactMissing: func(model.Record) []string {
			calls++
			return []string{"remote_artifact"}
		},
	}
	err := validatePhaseEntry(store, model.Record{Phase: model.PhasePR}, model.PhaseDone)
	if err == nil || !strings.Contains(err.Error(), "remote_artifact") || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
