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
		Plan: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			calls = append(calls, "plan")
			return model.IssueOpsReadiness{Missing: []string{"intent_contract"}}
		},
		Grill: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			calls = append(calls, "grill")
			return model.IssueOpsReadiness{Missing: []string{"domain_review"}}
		},
	}
	err := ValidatePhaseEntry(store, model.IssueOpsRecord{Phase: model.IssueOpsPhaseGrill}, model.IssueOpsPhasePlan)
	if err == nil || !strings.Contains(err.Error(), "intent_contract") || !reflect.DeepEqual(calls, []string{"plan"}) {
		t.Fatalf("err=%v calls=%v", err, calls)
	}
	store.Plan = func(model.IssueOpsRecord) model.IssueOpsReadiness {
		calls = append(calls, "plan")
		return model.IssueOpsReadiness{Ready: true}
	}
	calls = nil
	err = ValidatePhaseEntry(store, model.IssueOpsRecord{Phase: model.IssueOpsPhaseGrill}, model.IssueOpsPhasePlan)
	if err == nil || !strings.Contains(err.Error(), "grill incomplete: missing domain_review") || !reflect.DeepEqual(calls, []string{"plan", "grill"}) {
		t.Fatalf("err=%v calls=%v", err, calls)
	}
}

func TestValidatePhaseEntryUsesPrefetchedPRReadiness(t *testing.T) {
	calls := 0
	store := cycleport.PhaseEntryReadiness{
		StrictPR: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			calls++
			return model.IssueOpsReadiness{Missing: []string{"upstream_fetch"}}
		},
	}
	err := ValidatePhaseEntry(store, model.IssueOpsRecord{Phase: model.IssueOpsPhaseFeedback}, model.IssueOpsPhasePR)
	if err == nil || !strings.Contains(err.Error(), "upstream_fetch") || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestValidatePhaseEntryChecksRemoteArtifactBeforeCompletion(t *testing.T) {
	calls := 0
	store := cycleport.PhaseEntryReadiness{
		RemoteArtifactMissing: func(model.IssueOpsRecord) []string {
			calls++
			return []string{"remote_artifact"}
		},
	}
	err := ValidatePhaseEntry(store, model.IssueOpsRecord{Phase: model.IssueOpsPhasePR}, model.IssueOpsPhaseDone)
	if err == nil || !strings.Contains(err.Error(), "remote_artifact") || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
