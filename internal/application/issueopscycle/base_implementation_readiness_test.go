package issueopscycle

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestBaseImplementationMissingComposesBranchIntentDesignAndPlan(t *testing.T) {
	if got := BaseImplementationMissing(model.IssueOpsRecord{}); !reflect.DeepEqual(got, []string{"issue_url", "branch", "branch_prepare", "intent_contract", "design_review", "plan_path"}) {
		t.Fatalf("empty implementation prerequisites=%v", got)
	}
}
