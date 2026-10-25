package issueopsbranch

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

// After execution prepare, a new base must contain the old one; otherwise
// sync-base would carry the old base's extra commits into the PR.
func TestGuardBaseChangeRefusesABaseWithoutTheOldOne(t *testing.T) {
	record := model.IssueOpsRecord{Repo: "/repo", BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: "main", BaseSHA: "old"}, Execution: &model.Execution{}}
	ancestry := map[string]bool{"old..ahead": true}
	s := Preparer{IsAncestor: func(_, a, d string) (bool, error) { return ancestry[a+".."+d], nil }}
	if err := s.guardBaseChange(record, model.IssueOpsBranchPrepareRequest{BaseBranch: "develop", BaseSHA: "diverged"}); err == nil || !strings.Contains(err.Error(), "does not contain the old base") {
		t.Fatalf("diverged base: %v", err)
	}
	if err := s.guardBaseChange(record, model.IssueOpsBranchPrepareRequest{BaseBranch: "main", BaseSHA: "ahead"}); err != nil {
		t.Fatalf("base ahead: %v", err)
	}
	record.Execution = nil
	if err := s.guardBaseChange(record, model.IssueOpsBranchPrepareRequest{BaseBranch: "develop", BaseSHA: "diverged"}); err != nil {
		t.Fatalf("before execution prepare any base is fine: %v", err)
	}
}
