package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestPRCompletionMissingAddsRemoteArtifactAndTargetBranch(t *testing.T) {
	record := model.IssueOpsRecord{
		BranchPrepare:  &model.IssueOpsBranchPrepare{BaseBranch: "main"},
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "https://example.test/pr/1", TargetBranch: "other"},
	}
	if got := PRCompletionMissing(record, []string{"review"}); !reflect.DeepEqual(got, []string{"review", "target_branch_match"}) {
		t.Fatalf("missing=%v", got)
	}
	record.RemoteArtifact = nil
	if got := PRCompletionMissing(record, nil); !reflect.DeepEqual(got, []string{"remote_artifact"}) {
		t.Fatalf("missing without artifact=%v", got)
	}
}

func TestDoneCompletionMissingRequiresPriorPRAndRemoteProof(t *testing.T) {
	record := model.IssueOpsRecord{Phase: model.IssueOpsPhaseFeedback}
	if got := DoneCompletionMissing(record, []string{"remote_artifact"}); !reflect.DeepEqual(got, []string{"prior_phase_pr", "remote_artifact"}) {
		t.Fatalf("missing=%v", got)
	}
	record.Phase = model.IssueOpsPhasePR
	if got := DoneCompletionMissing(record, nil); len(got) != 0 {
		t.Fatalf("ready done missing=%v", got)
	}
}
