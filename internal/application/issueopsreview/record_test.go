package issueopsreview

import (
	"errors"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func TestRecordDevilsAdvocateDoesNotWriteWithoutPlanDigest(t *testing.T) {
	writes := 0
	store := reviewport.DevilsAdvocateStore{
		Read: func(_, id string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{ID: id}, nil },
		TouchWrite: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			writes++
			return record, nil
		},
		PlanDigest: func(string, model.IssueOpsRecord) (string, error) { return "", errors.New("no plan") },
	}
	_, err := RecordDevilsAdvocate(store, "state", "io-review", model.IssueOpsDevilsAdvocateReviewRequest{
		Verdict: "pass", ReviewerContext: "subagent", Findings: []string{"tested rollback"},
	}, "now")
	if err == nil || err.Error() != "no plan" || writes != 0 {
		t.Fatalf("error = %v, writes = %d", err, writes)
	}
}

func TestRecordCompatibilityReviewAdvancesOnlyAfterReadiness(t *testing.T) {
	writes := 0
	ready := false
	store := reviewport.CompatibilityStore{
		Read: func(_, id string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{ID: id, Phase: model.IssueOpsPhasePlan}, nil
		},
		TouchWrite: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			writes++
			return record, nil
		},
		Ready: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			if ready {
				return model.IssueOpsReadiness{Ready: true}
			}
			return model.IssueOpsReadiness{Missing: []string{"plan_path"}}
		},
		PhaseRank: func(phase model.IssueOpsPhase) int {
			if phase == model.IssueOpsPhaseCompatibilityReview {
				return 2
			}
			return 1
		},
	}
	req := model.IssueOpsCompatibilityReviewRequest{
		BackwardCompatibility: []string{"compatible"}, SideEffects: []string{"none"},
		Verification: []string{"tested"}, RollbackPlan: "revert",
	}
	if _, err := RecordCompatibilityReview(store, "state", "io-review", req, "now"); err == nil || writes != 0 {
		t.Fatalf("unready result = %v, writes = %d", err, writes)
	}
	ready = true
	record, err := RecordCompatibilityReview(store, "state", "io-review", req, "now")
	if err != nil || writes != 1 || record.Phase != model.IssueOpsPhaseCompatibilityReview {
		t.Fatalf("ready record = %+v, error = %v, writes = %d", record, err, writes)
	}
}
