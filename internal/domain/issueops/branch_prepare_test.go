package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

// QA: after execution prepare the default branch was renamed (master ->
// main); a re-prepare with only --base-branch erased base_sha and
// link_verified, so await-link failed with sealed_base_missing.
func TestRequireSealedPreparationKeptAfterExecution(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-1", Execution: &model.Execution{}, BranchPrepare: &model.IssueOpsBranchPrepare{
		Provider: "github", Branch: "41-x", BaseBranch: "master", BaseSHA: "abc123", LinkVerified: true,
	}}
	bare := model.IssueOpsBranchPrepareRequest{Branch: "41-x", BaseBranch: "main"}
	err := RequireSealedPreparationKept(record, bare)
	if err == nil || !strings.Contains(err.Error(), "--base-sha, --link-verified") ||
		!strings.Contains(err.Error(), "--base-branch main --base-sha abc123 --link-verified $RECORD_ACTOR_FLAGS") {
		t.Fatalf("err = %v", err)
	}
	kept := model.IssueOpsBranchPrepareRequest{Branch: "41-x", BaseBranch: "main", BaseSHA: "abc123", LinkVerified: true}
	if err := RequireSealedPreparationKept(record, kept); err != nil {
		t.Fatalf("kept facts refused: %v", err)
	}
	// Before execution prepare a re-prepare may still reset the preparation.
	record.Execution = nil
	if err := RequireSealedPreparationKept(record, bare); err != nil {
		t.Fatalf("pre-execution re-prepare refused: %v", err)
	}
}
