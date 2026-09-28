package issueopscycle

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestDevilsAdvocateStagedPlanBoundUsesPhaseBoundary(t *testing.T) {
	record := model.IssueOpsRecord{Phase: model.IssueOpsPhasePlan, DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{ReviewedPlanDigest: "v1"}}
	if DevilsAdvocateStagedPlanBound(record, "v2") {
		t.Fatal("plan stage must reject a stale review")
	}
	record.Phase = model.IssueOpsPhaseImplement
	if !DevilsAdvocateStagedPlanBound(record, "v2") {
		t.Fatal("implementation plan edits must remain eligible for reseal")
	}
}
