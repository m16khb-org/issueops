package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestPublicationTransitionsPreserveLeaseAndInput(t *testing.T) {
	original := model.IssueOpsRecord{Phase: model.IssueOpsPhasePR, Execution: &model.Execution{Lease: model.WriteLease{Generation: 4, Status: model.LeaseStatusActive}, Failure: &model.ExecutionFailure{Code: "old"}}}
	pending := model.ExternalIntent{OperationID: "operation", Kind: "remote_pr_create", Marker: "marker", StartedAt: "started"}
	started := BeginPublication(original, pending)
	if started.Execution.Pending == nil || started.Execution.Pending.OperationID != "operation" || started.Execution.Failure != nil || original.Execution.Pending != nil || original.Execution.Failure.Code != "old" {
		t.Fatalf("begin changed input or wrong transition: %+v", started.Execution)
	}
	ambiguous := FailPublication(started, "operation", "message", "now", false)
	if ambiguous.Execution.Pending == nil || ambiguous.Execution.Failure.Code != "external_operation_ambiguous" || started.Execution.Failure != nil {
		t.Fatalf("ambiguous failure: %+v", ambiguous.Execution)
	}
	terminal := FailPublication(ambiguous, "operation", "message", "now", true)
	if terminal.Execution.Pending != nil || terminal.Execution.Failure.Code != "external_operation_not_invoked" || ambiguous.Execution.Pending == nil {
		t.Fatalf("terminal failure: %+v", terminal.Execution)
	}
	artifact := model.IssueOpsRemoteArtifactVerification{URL: "https://github.com/team/repo/pull/4"}
	completed := CompletePublication(ambiguous, artifact)
	if completed.Execution.Pending != nil || completed.Execution.Failure != nil || completed.RemoteArtifact == nil || completed.RemoteArtifact.URL != artifact.URL || ambiguous.RemoteArtifact != nil {
		t.Fatalf("receipt: %+v", completed)
	}
	for _, record := range []model.IssueOpsRecord{started, ambiguous, terminal, completed} {
		if !reflect.DeepEqual(record.Execution.Lease, original.Execution.Lease) || record.Execution.Completion != nil || record.Phase != model.IssueOpsPhasePR {
			t.Fatalf("publication completed execution: %+v", record.Execution)
		}
	}
}
