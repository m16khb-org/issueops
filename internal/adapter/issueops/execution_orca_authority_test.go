package issueops

import (
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestOrcaIntentExpectedRecordDelegatesAuthorityAndKeepsIdentity(t *testing.T) {
	const issueURL = "https://github.com/example/repo/issues/199"
	record := issueops.IssueOpsRecord{
		ID: "io-1", IssueURL: issueURL,
		BranchPrepare: &issueops.IssueOpsBranchPrepare{Provider: "github", IssueURL: issueURL},
		Execution: &issueops.Execution{
			Mode: issueops.ExecutionModeOrca,
			Workspace: issueops.Workspace{
				SourceRoot: "/repo", Root: "/repo.wt", Branch: "work", BaseHead: "base", Driver: "orca",
			},
			Lease: issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased},
			Pending: &issueops.ExternalIntent{
				OperationID: "op-1", Marker: "marker", Kind: "worktree_create",
			},
		},
	}
	intent := preparationcontract.Intent{
		LifecycleID: "io-1", OperationID: "op-1", Generation: 1,
		Stage: preparationcontract.IntentStageWorktree, Marker: "marker",
		Workspace: preparationcontract.WorkspaceRequest{
			SourceRoot: "/repo", Root: "/repo.wt", Branch: "work", BaseHead: "base",
		},
		Probe: preparationcontract.ProbeRequest{Provider: "github", Issue: 199},
	}
	if err := validateOrcaIntentExpectedRecord(record, intent); err != nil {
		t.Fatalf("matching intent rejected: %v", err)
	}
	record.Execution.Lease.Generation = 2
	if err := validateOrcaIntentExpectedRecord(record, intent); err == nil || !strings.Contains(err.Error(), "authority changed") {
		t.Fatalf("stale generation accepted: %v", err)
	}
	record.Execution.Lease.Generation = 1
	intent.Workspace.Branch = "other"
	if err := validateOrcaIntentExpectedRecord(record, intent); err == nil || !strings.Contains(err.Error(), "record identity changed") {
		t.Fatalf("mismatched workspace accepted: %v", err)
	}
}
