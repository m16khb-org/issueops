package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestBranchEvidenceMissingPreservesOrderedGates(t *testing.T) {
	if got := BranchEvidenceMissing(model.IssueOpsRecord{}); !reflect.DeepEqual(got, []string{"issue_url", "branch", "branch_prepare"}) {
		t.Fatalf("empty branch evidence=%v", got)
	}
	record := model.IssueOpsRecord{IssueURL: "issue", Branch: "feature", BranchPrepare: &model.IssueOpsBranchPrepare{}}
	if got := BranchEvidenceMissing(record); !reflect.DeepEqual(got, []string{"branch_link_verified"}) {
		t.Fatalf("unverified link=%v", got)
	}
	record.BranchPrepare.LinkVerified = true
	if got := BranchEvidenceMissing(record); len(got) != 0 {
		t.Fatalf("verified branch=%v", got)
	}
}
