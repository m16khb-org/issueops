package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestLinkedBranchCleanupRequiresPreparedIdentityAndCapabilities(t *testing.T) {
	if got := LinkedBranchCleanupGates(model.IssueOpsRecord{}, false, false); !reflect.DeepEqual(got, []string{"linked_branch_observation_unavailable", "linked_branch_deletion_unavailable", "branch_prepare_missing"}) {
		t.Fatalf("missing capabilities and prepare: %v", got)
	}
	record := model.IssueOpsRecord{BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "gitlab"}}
	want := []string{"linked_branch_cleanup_is_github_only", "issue_url_missing", "branch_missing", "sealed_base_missing", "repo_missing"}
	if got := LinkedBranchCleanupGates(record, true, true); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing identity: %v", got)
	}
	record.Repo = "/repo"
	record.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: "issue", Branch: "topic", BaseSHA: "base"}
	if got := LinkedBranchCleanupGates(record, true, true); len(got) != 0 {
		t.Fatalf("valid identity rejected: %v", got)
	}
}
